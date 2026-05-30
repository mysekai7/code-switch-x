package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/daodao97/xgo/xrequest"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const deepSeekChatCompletionsEndpoint = "/chat/completions"

type deepSeekChatMessage struct {
	Role       string           `json:"role"`
	Content    string           `json:"content,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
	ToolCalls  []map[string]any `json:"tool_calls,omitempty"`
}

type deepSeekChatRequest struct {
	Model       string                `json:"model"`
	Messages    []deepSeekChatMessage `json:"messages"`
	Temperature *float64              `json:"temperature,omitempty"`
	MaxTokens   int64                 `json:"max_tokens,omitempty"`
	Stream      bool                  `json:"stream"`
	Tools       []map[string]any      `json:"tools,omitempty"`
	ToolChoice  any                   `json:"tool_choice,omitempty"`
}

func translateResponsesRequestToDeepSeekChatCompletion(body []byte) ([]byte, error) {
	model := gjson.GetBytes(body, "model").String()
	if model == "" {
		return nil, fmt.Errorf("responses request missing model")
	}

	request := deepSeekChatRequest{
		Model:    model,
		Messages: make([]deepSeekChatMessage, 0, 2),
		Stream:   false,
	}
	if temperature := gjson.GetBytes(body, "temperature"); temperature.Exists() {
		value := temperature.Float()
		request.Temperature = &value
	}
	if maxTokens := gjson.GetBytes(body, "max_output_tokens"); maxTokens.Exists() {
		request.MaxTokens = maxTokens.Int()
	}
	request.Tools = responsesToolsToDeepSeekTools(body)
	request.ToolChoice = responsesToolChoiceToDeepSeekToolChoice(body)
	if instructions := gjson.GetBytes(body, "instructions").String(); strings.TrimSpace(instructions) != "" {
		request.Messages = append(request.Messages, deepSeekChatMessage{Role: "system", Content: instructions})
	}
	request.Messages = append(request.Messages, responsesInputToDeepSeekMessages(body)...)
	if len(request.Messages) == 0 {
		return nil, fmt.Errorf("responses request missing input")
	}

	return json.Marshal(request)
}

func responsesInputToDeepSeekMessages(body []byte) []deepSeekChatMessage {
	input := gjson.GetBytes(body, "input")
	if !input.Exists() {
		return nil
	}
	if input.Type == gjson.String {
		content := input.String()
		if strings.TrimSpace(content) == "" {
			return nil
		}
		return []deepSeekChatMessage{{Role: "user", Content: content}}
	}
	if !input.IsArray() {
		return nil
	}

	messages := make([]deepSeekChatMessage, 0)
	input.ForEach(func(_, item gjson.Result) bool {
		switch item.Get("type").String() {
		case "function_call_output":
			content := item.Get("output").String()
			if strings.TrimSpace(content) != "" {
				messages = append(messages, deepSeekChatMessage{
					Role:       "tool",
					ToolCallID: firstNonEmpty(item.Get("call_id").String(), item.Get("id").String()),
					Content:    content,
				})
			}
			return true
		case "function_call":
			toolCallID := firstNonEmpty(item.Get("call_id").String(), item.Get("id").String())
			toolCall := map[string]any{
				"id":   toolCallID,
				"type": "function",
				"function": map[string]any{
					"name":      item.Get("name").String(),
					"arguments": item.Get("arguments").String(),
				},
			}
			messages = append(messages, deepSeekChatMessage{
				Role:      "assistant",
				ToolCalls: []map[string]any{toolCall},
			})
			return true
		}

		role := item.Get("role").String()
		if role == "" {
			role = "user"
		}
		content := responsesMessageContentText(item.Get("content"))
		if strings.TrimSpace(content) != "" {
			messages = append(messages, deepSeekChatMessage{Role: role, Content: content})
		}
		return true
	})
	return messages
}

func responsesToolsToDeepSeekTools(body []byte) []map[string]any {
	tools := gjson.GetBytes(body, "tools")
	if !tools.IsArray() {
		return nil
	}
	result := make([]map[string]any, 0)
	tools.ForEach(func(_, item gjson.Result) bool {
		if item.Get("type").String() != "function" {
			return true
		}
		tool := map[string]any{"type": "function"}
		if function := item.Get("function"); function.Exists() {
			var fn map[string]any
			if err := json.Unmarshal([]byte(function.Raw), &fn); err == nil {
				tool["function"] = fn
				result = append(result, tool)
			}
			return true
		}
		fn := map[string]any{
			"name":        item.Get("name").String(),
			"description": item.Get("description").String(),
		}
		if parameters := item.Get("parameters"); parameters.Exists() {
			var params any
			if err := json.Unmarshal([]byte(parameters.Raw), &params); err == nil {
				fn["parameters"] = params
			}
		}
		tool["function"] = fn
		result = append(result, tool)
		return true
	})
	if len(result) == 0 {
		return nil
	}
	return result
}

func responsesToolChoiceToDeepSeekToolChoice(body []byte) any {
	choice := gjson.GetBytes(body, "tool_choice")
	if !choice.Exists() {
		return nil
	}
	if choice.Type == gjson.String {
		return choice.String()
	}
	var parsed any
	if err := json.Unmarshal([]byte(choice.Raw), &parsed); err != nil {
		return nil
	}
	return parsed
}

func responsesMessageContentText(content gjson.Result) string {
	if !content.Exists() {
		return ""
	}
	if content.Type == gjson.String {
		return content.String()
	}
	if !content.IsArray() {
		return ""
	}

	parts := make([]string, 0)
	content.ForEach(func(_, part gjson.Result) bool {
		if text := part.Get("text").String(); text != "" {
			parts = append(parts, text)
			return true
		}
		if text := part.Get("input_text").String(); text != "" {
			parts = append(parts, text)
		}
		return true
	})
	return strings.Join(parts, "\n")
}

func translateDeepSeekChatCompletionToResponses(body []byte, stream bool) ([]byte, error) {
	text := gjson.GetBytes(body, "choices.0.message.content").String()
	output := deepSeekToolCallsToResponsesOutput(body)
	if len(output) == 0 {
		output = []map[string]any{
			{
				"type":   "message",
				"id":     "msg_" + fallbackResponseID(gjson.GetBytes(body, "id").String()),
				"status": "completed",
				"role":   "assistant",
				"content": []map[string]any{
					{
						"type": "output_text",
						"text": text,
					},
				},
			},
		}
	}
	response := map[string]any{
		"id":          gjson.GetBytes(body, "id").String(),
		"object":      "response",
		"created_at":  time.Now().Unix(),
		"status":      "completed",
		"model":       gjson.GetBytes(body, "model").String(),
		"output_text": text,
		"output":      output,
		"usage": map[string]any{
			"input_tokens":  gjson.GetBytes(body, "usage.prompt_tokens").Int(),
			"output_tokens": gjson.GetBytes(body, "usage.completion_tokens").Int(),
			"total_tokens":  gjson.GetBytes(body, "usage.total_tokens").Int(),
			"output_tokens_details": map[string]any{
				"reasoning_tokens": gjson.GetBytes(body, "usage.completion_tokens_details.reasoning_tokens").Int(),
			},
		},
	}
	payload, err := json.Marshal(response)
	if err != nil {
		return nil, err
	}
	if !stream {
		return payload, nil
	}

	var out bytes.Buffer
	out.WriteString("event: response.completed\n")
	out.WriteString("data: ")
	out.Write(payload)
	out.WriteString("\n\n")
	return out.Bytes(), nil
}

func deepSeekToolCallsToResponsesOutput(body []byte) []map[string]any {
	toolCalls := gjson.GetBytes(body, "choices.0.message.tool_calls")
	if !toolCalls.IsArray() {
		return nil
	}
	output := make([]map[string]any, 0)
	toolCalls.ForEach(func(_, toolCall gjson.Result) bool {
		callID := toolCall.Get("id").String()
		output = append(output, map[string]any{
			"type":      "function_call",
			"id":        callID,
			"call_id":   callID,
			"name":      toolCall.Get("function.name").String(),
			"arguments": toolCall.Get("function.arguments").String(),
			"status":    "completed",
		})
		return true
	})
	if len(output) == 0 {
		return nil
	}
	return output
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func fallbackResponseID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return "deepseek"
	}
	return id
}

func (prs *ProviderRelayService) forwardDeepSeekCodexRequest(
	c *gin.Context,
	provider Provider,
	query map[string]string,
	headers map[string]string,
	bodyBytes []byte,
	isStream bool,
	requestLog *ReqeustLog,
) (bool, error) {
	translatedBody, err := translateResponsesRequestToDeepSeekChatCompletion(bodyBytes)
	if err != nil {
		return false, err
	}

	req := xrequest.New().
		SetHeaders(headers).
		SetQueryParams(query).
		SetBody(bytes.NewReader(translatedBody))

	resp, err := req.Post(joinURL(provider.APIURL, deepSeekChatCompletionsEndpoint))
	if err != nil {
		return false, err
	}
	if resp == nil {
		return false, fmt.Errorf("empty response")
	}
	status := resp.StatusCode()
	requestLog.HttpCode = status
	if status < http.StatusOK || status >= http.StatusMultipleChoices {
		return false, newUpstreamResponseError(status, resp.Headers(), resp.Bytes())
	}

	body := resp.Bytes()
	translatedResponse, err := translateDeepSeekChatCompletionToResponses(body, isStream)
	if err != nil {
		return false, err
	}

	if isStream {
		c.Header("Content-Type", "text/event-stream")
		parseEventPayload(string(translatedResponse), CodexParseTokenUsageFromResponse, requestLog)
	} else {
		c.Header("Content-Type", "application/json")
		CodexParseTokenUsageFromResponse(string(translatedResponse), requestLog)
	}
	_, err = io.Copy(c.Writer, bytes.NewReader(translatedResponse))
	return err == nil, err
}
