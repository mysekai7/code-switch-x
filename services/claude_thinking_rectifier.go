package services

import (
	"bytes"
	"encoding/json"
	"strings"
)

const (
	claudeRectifierThinkingBudget = 32000
	claudeRectifierMaxTokens      = 64000
)

func shouldRectifyClaudeThinkingSignature(errorBody []byte) bool {
	lower := strings.ToLower(claudeThinkingErrorText(errorBody))
	if lower == "" {
		return false
	}
	if strings.Contains(lower, "invalid") &&
		strings.Contains(lower, "signature") &&
		strings.Contains(lower, "thinking") &&
		strings.Contains(lower, "block") {
		return true
	}
	if strings.Contains(lower, "thought signature") &&
		(strings.Contains(lower, "not valid") || strings.Contains(lower, "invalid")) {
		return true
	}
	if strings.Contains(lower, "must start with a thinking block") {
		return true
	}
	if strings.Contains(lower, "expected") &&
		(strings.Contains(lower, "thinking") || strings.Contains(lower, "redacted_thinking")) &&
		strings.Contains(lower, "found") &&
		strings.Contains(lower, "tool_use") {
		return true
	}
	if strings.Contains(lower, "signature") && strings.Contains(lower, "field required") {
		return true
	}
	if strings.Contains(lower, "signature") && strings.Contains(lower, "extra inputs are not permitted") {
		return true
	}
	if (strings.Contains(lower, "thinking") || strings.Contains(lower, "redacted_thinking")) &&
		strings.Contains(lower, "cannot be modified") {
		return true
	}
	return strings.Contains(lower, "非法请求") ||
		strings.Contains(lower, "illegal request") ||
		strings.Contains(lower, "invalid request")
}

func shouldRectifyClaudeThinkingBudget(errorBody []byte) bool {
	lower := strings.ToLower(claudeThinkingErrorText(errorBody))
	if lower == "" {
		return false
	}
	hasBudget := strings.Contains(lower, "budget_tokens") || strings.Contains(lower, "budget tokens")
	hasThinking := strings.Contains(lower, "thinking")
	has1024 := strings.Contains(lower, "greater than or equal to 1024") ||
		strings.Contains(lower, ">= 1024") ||
		(strings.Contains(lower, "1024") && strings.Contains(lower, "input should be"))
	return hasBudget && hasThinking && has1024
}

func rectifyClaudeThinkingSignatureRequest(body []byte) ([]byte, bool, error) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, false, err
	}

	applied := false
	messages, _ := payload["messages"].([]any)
	for _, rawMessage := range messages {
		message, _ := rawMessage.(map[string]any)
		if message == nil {
			continue
		}
		content, _ := message["content"].([]any)
		if len(content) == 0 {
			continue
		}
		nextContent := make([]any, 0, len(content))
		for _, rawBlock := range content {
			block, _ := rawBlock.(map[string]any)
			if block == nil {
				nextContent = append(nextContent, rawBlock)
				continue
			}
			switch blockType(block) {
			case "thinking", "redacted_thinking":
				applied = true
				continue
			}
			if _, ok := block["signature"]; ok {
				delete(block, "signature")
				applied = true
			}
			nextContent = append(nextContent, block)
		}
		if len(nextContent) != len(content) {
			message["content"] = nextContent
		}
	}

	if shouldRemoveClaudeTopLevelThinking(payload) {
		delete(payload, "thinking")
		applied = true
	}
	if !applied {
		return body, false, nil
	}
	out, err := json.Marshal(payload)
	if err != nil {
		return nil, false, err
	}
	return out, true, nil
}

func rectifyClaudeThinkingBudgetRequest(body []byte) ([]byte, bool, error) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, false, err
	}
	thinking, _ := payload["thinking"].(map[string]any)
	if thinkingType(thinking) == "adaptive" {
		return body, false, nil
	}

	beforeType := thinkingType(thinking)
	beforeBudget := numberAsInt64(thinking["budget_tokens"])
	beforeMaxTokens := numberAsInt64(payload["max_tokens"])
	if thinking == nil {
		thinking = make(map[string]any)
		payload["thinking"] = thinking
	}
	thinking["type"] = "enabled"
	thinking["budget_tokens"] = claudeRectifierThinkingBudget
	if beforeMaxTokens == 0 || beforeMaxTokens < claudeRectifierThinkingBudget+1 {
		payload["max_tokens"] = claudeRectifierMaxTokens
	}

	afterBudget := numberAsInt64(thinking["budget_tokens"])
	afterMaxTokens := numberAsInt64(payload["max_tokens"])
	applied := beforeType != "enabled" ||
		beforeBudget != afterBudget ||
		beforeMaxTokens != afterMaxTokens
	if !applied {
		return body, false, nil
	}
	out, err := json.Marshal(payload)
	if err != nil {
		return nil, false, err
	}
	return out, true, nil
}

func claudeThinkingErrorText(body []byte) string {
	var value any
	if err := json.Unmarshal(body, &value); err != nil {
		return string(body)
	}
	var out bytes.Buffer
	collectJSONStrings(&out, value)
	if out.Len() == 0 {
		return string(body)
	}
	return out.String()
}

func collectJSONStrings(out *bytes.Buffer, value any) {
	switch typed := value.(type) {
	case string:
		out.WriteString(typed)
		out.WriteByte('\n')
	case []any:
		for _, item := range typed {
			collectJSONStrings(out, item)
		}
	case map[string]any:
		for _, item := range typed {
			collectJSONStrings(out, item)
		}
	}
}

func shouldRemoveClaudeTopLevelThinking(payload map[string]any) bool {
	thinking, _ := payload["thinking"].(map[string]any)
	if thinkingType(thinking) != "enabled" {
		return false
	}
	messages, _ := payload["messages"].([]any)
	for i := len(messages) - 1; i >= 0; i-- {
		message, _ := messages[i].(map[string]any)
		if message == nil || stringField(message, "role") != "assistant" {
			continue
		}
		content, _ := message["content"].([]any)
		if len(content) == 0 {
			return false
		}
		firstBlock, _ := content[0].(map[string]any)
		firstType := blockType(firstBlock)
		if firstType == "thinking" || firstType == "redacted_thinking" {
			return false
		}
		for _, rawBlock := range content {
			block, _ := rawBlock.(map[string]any)
			if blockType(block) == "tool_use" {
				return true
			}
		}
		return false
	}
	return false
}

func blockType(block map[string]any) string {
	return stringField(block, "type")
}

func thinkingType(thinking map[string]any) string {
	return stringField(thinking, "type")
}

func stringField(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	value, _ := values[key].(string)
	return strings.TrimSpace(value)
}

func numberAsInt64(value any) int64 {
	switch typed := value.(type) {
	case int:
		return int64(typed)
	case int64:
		return typed
	case float64:
		return int64(typed)
	case json.Number:
		result, _ := typed.Int64()
		return result
	default:
		return 0
	}
}
