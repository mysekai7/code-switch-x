package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const deepSeekChatCompletionsEndpoint = "/chat/completions"

type deepSeekChatMessage struct {
	Role             string           `json:"role"`
	Content          string           `json:"content,omitempty"`
	ToolCallID       string           `json:"tool_call_id,omitempty"`
	ToolCalls        []map[string]any `json:"tool_calls,omitempty"`
	ReasoningContent string           `json:"reasoning_content,omitempty"`
}

type deepSeekChatRequest struct {
	Model           string                `json:"model"`
	Messages        []deepSeekChatMessage `json:"messages"`
	Temperature     *float64              `json:"temperature,omitempty"`
	MaxTokens       int64                 `json:"max_tokens,omitempty"`
	Stream          bool                  `json:"stream"`
	Tools           []map[string]any      `json:"tools,omitempty"`
	ToolChoice      any                   `json:"tool_choice,omitempty"`
	Thinking        map[string]string     `json:"thinking,omitempty"`
	ReasoningEffort string                `json:"reasoning_effort,omitempty"`
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
	applyResponsesReasoningToDeepSeekChatRequest(&request, body)
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

	messages := make([]deepSeekChatMessage, 0)
	if input.IsArray() {
		input.ForEach(func(_, item gjson.Result) bool {
			appendResponsesInputItemToDeepSeekMessages(&messages, item)
			return true
		})
		return messages
	}
	if input.IsObject() {
		appendResponsesInputItemToDeepSeekMessages(&messages, input)
	}
	return messages
}

func appendResponsesInputItemToDeepSeekMessages(messages *[]deepSeekChatMessage, item gjson.Result) {
	switch item.Get("type").String() {
	case "function_call_output":
		content := responsesOutputText(item.Get("output"))
		if strings.TrimSpace(content) != "" {
			*messages = append(*messages, deepSeekChatMessage{
				Role:       "tool",
				ToolCallID: firstNonEmpty(item.Get("call_id").String(), item.Get("id").String()),
				Content:    content,
			})
		}
		return
	case "function_call":
		appendDeepSeekToolCall(messages, responsesFunctionCallToDeepSeekToolCall(item), responsesItemReasoningContent(item))
		return
	}

	role := responsesRoleToDeepSeekRole(item.Get("role").String())
	content := responsesMessageContentText(item.Get("content"))
	if strings.TrimSpace(content) != "" {
		*messages = append(*messages, deepSeekChatMessage{Role: role, Content: content})
	}
}

func appendDeepSeekToolCall(messages *[]deepSeekChatMessage, toolCall map[string]any, reasoning string) {
	reasoning = strings.TrimSpace(reasoning)
	if reasoning == "" {
		reasoning = "tool call"
	}

	if len(*messages) > 0 {
		last := &(*messages)[len(*messages)-1]
		if last.Role == "assistant" {
			last.ToolCalls = append(last.ToolCalls, toolCall)
			if strings.TrimSpace(last.ReasoningContent) == "" {
				last.ReasoningContent = reasoning
			}
			return
		}
	}

	*messages = append(*messages, deepSeekChatMessage{
		Role:             "assistant",
		ToolCalls:        []map[string]any{toolCall},
		ReasoningContent: reasoning,
	})
}

func responsesFunctionCallToDeepSeekToolCall(item gjson.Result) map[string]any {
	return map[string]any{
		"id":   firstNonEmpty(item.Get("call_id").String(), item.Get("id").String()),
		"type": "function",
		"function": map[string]any{
			"name":      item.Get("name").String(),
			"arguments": item.Get("arguments").String(),
		},
	}
}

func responsesItemReasoningContent(item gjson.Result) string {
	if reasoning := item.Get("reasoning_content").String(); strings.TrimSpace(reasoning) != "" {
		return reasoning
	}
	if reasoning := item.Get("reasoning").String(); strings.TrimSpace(reasoning) != "" {
		return reasoning
	}
	return ""
}

func responsesRoleToDeepSeekRole(role string) string {
	normalized := strings.TrimSpace(strings.ToLower(role))
	switch normalized {
	case "":
		return "user"
	case "developer":
		return "system"
	default:
		return normalized
	}
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

func applyResponsesReasoningToDeepSeekChatRequest(request *deepSeekChatRequest, body []byte) {
	if thinking := gjson.GetBytes(body, "thinking"); thinking.Exists() {
		if thinking.Type == gjson.True {
			request.Thinking = map[string]string{"type": "enabled"}
			return
		}
		switch strings.TrimSpace(strings.ToLower(thinking.Get("type").String())) {
		case "enabled":
			request.Thinking = map[string]string{"type": "enabled"}
			return
		case "disabled":
			request.Thinking = map[string]string{"type": "disabled"}
			return
		}
	}

	reasoning := gjson.GetBytes(body, "reasoning")
	if !reasoning.Exists() {
		return
	}
	if reasoning.Type == gjson.Null {
		request.Thinking = map[string]string{"type": "disabled"}
		return
	}

	effort := strings.TrimSpace(strings.ToLower(reasoning.Get("effort").String()))
	if effort == "" {
		request.Thinking = map[string]string{"type": "enabled"}
		return
	}
	if matchesAny(effort, "none", "off", "disabled") {
		request.Thinking = map[string]string{"type": "disabled"}
		return
	}

	request.Thinking = map[string]string{"type": "enabled"}
	request.ReasoningEffort = mapDeepSeekReasoningEffort(effort)
}

func mapDeepSeekReasoningEffort(effort string) string {
	switch strings.TrimSpace(strings.ToLower(effort)) {
	case "max", "xhigh":
		return "max"
	default:
		return "high"
	}
}

func matchesAny(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if value == candidate {
			return true
		}
	}
	return false
}

func responsesToolChoiceToDeepSeekToolChoice(body []byte) any {
	choice := gjson.GetBytes(body, "tool_choice")
	if !choice.Exists() {
		return nil
	}
	if choice.Type == gjson.String {
		return choice.String()
	}
	if choice.Get("type").String() == "function" && choice.Get("name").String() != "" {
		return map[string]any{
			"type": "function",
			"function": map[string]any{
				"name": choice.Get("name").String(),
			},
		}
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
	if content.IsObject() {
		return responsesContentPartText(content)
	}
	if !content.IsArray() {
		return ""
	}

	parts := make([]string, 0)
	content.ForEach(func(_, part gjson.Result) bool {
		if text := responsesContentPartText(part); text != "" {
			parts = append(parts, text)
		}
		return true
	})
	return strings.Join(parts, "\n")
}

func responsesContentPartText(part gjson.Result) string {
	if text := part.Get("text").String(); text != "" {
		return text
	}
	if text := part.Get("input_text").String(); text != "" {
		return text
	}
	if text := part.Get("output_text").String(); text != "" {
		return text
	}
	return ""
}

func responsesOutputText(output gjson.Result) string {
	if !output.Exists() {
		return ""
	}
	if output.Type == gjson.String {
		return output.String()
	}
	return output.Raw
}

func translateDeepSeekChatCompletionToResponses(body []byte, stream bool) ([]byte, error) {
	text := gjson.GetBytes(body, "choices.0.message.content").String()
	responseID := gjson.GetBytes(body, "id").String()
	reasoning := deepSeekReasoningContent(body)
	output := make([]map[string]any, 0)
	if strings.TrimSpace(reasoning) != "" {
		output = append(output, map[string]any{
			"type":   "reasoning",
			"id":     "rs_" + fallbackResponseID(responseID),
			"status": "completed",
			"summary": []map[string]any{
				{
					"type": "summary_text",
					"text": reasoning,
				},
			},
		})
	}
	if strings.TrimSpace(text) != "" {
		output = append(output, deepSeekMessageToResponsesOutput(responseID, text))
	}
	output = append(output, deepSeekToolCallsToResponsesOutput(body, reasoning)...)
	if len(output) == 0 {
		output = []map[string]any{
			deepSeekMessageToResponsesOutput(responseID, text),
		}
	}
	response := map[string]any{
		"id":          responseID,
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

	return responsesPayloadToSyntheticSSE(payload)
}

func responsesPayloadToSyntheticSSE(payload []byte) ([]byte, error) {
	var response map[string]any
	if err := json.Unmarshal(payload, &response); err != nil {
		return nil, err
	}

	var out bytes.Buffer
	writeSSEEvent(&out, "response.created", map[string]any{
		"type":     "response.created",
		"response": responseSnapshot(response, "in_progress", []any{}),
	})
	writeSSEEvent(&out, "response.in_progress", map[string]any{
		"type":     "response.in_progress",
		"response": responseSnapshot(response, "in_progress", []any{}),
	})

	output, _ := response["output"].([]any)
	for index, rawItem := range output {
		item, _ := rawItem.(map[string]any)
		if item == nil {
			continue
		}
		writeSSEEvent(&out, "response.output_item.added", map[string]any{
			"type":         "response.output_item.added",
			"output_index": index,
			"item":         itemSnapshot(item, "in_progress"),
		})
		writeSyntheticItemDeltaEvents(&out, index, item)
		writeSSEEvent(&out, "response.output_item.done", map[string]any{
			"type":         "response.output_item.done",
			"output_index": index,
			"item":         item,
		})
	}

	writeSSEEvent(&out, "response.completed", map[string]any{
		"type":     "response.completed",
		"response": response,
	})
	return out.Bytes(), nil
}

func writeSyntheticItemDeltaEvents(out *bytes.Buffer, outputIndex int, item map[string]any) {
	itemID, _ := item["id"].(string)
	switch item["type"] {
	case "message":
		text := responseMessageText(item)
		writeSSEEvent(out, "response.content_part.added", map[string]any{
			"type":          "response.content_part.added",
			"item_id":       itemID,
			"output_index":  outputIndex,
			"content_index": 0,
			"part": map[string]any{
				"type":        "output_text",
				"text":        "",
				"annotations": []any{},
			},
		})
		if text != "" {
			writeSSEEvent(out, "response.output_text.delta", map[string]any{
				"type":          "response.output_text.delta",
				"item_id":       itemID,
				"output_index":  outputIndex,
				"content_index": 0,
				"delta":         text,
			})
		}
		writeSSEEvent(out, "response.output_text.done", map[string]any{
			"type":          "response.output_text.done",
			"item_id":       itemID,
			"output_index":  outputIndex,
			"content_index": 0,
			"text":          text,
		})
		writeSSEEvent(out, "response.content_part.done", map[string]any{
			"type":          "response.content_part.done",
			"item_id":       itemID,
			"output_index":  outputIndex,
			"content_index": 0,
			"part": map[string]any{
				"type":        "output_text",
				"text":        text,
				"annotations": []any{},
			},
		})
	case "reasoning":
		text := responseReasoningText(item)
		writeSSEEvent(out, "response.reasoning_summary_part.added", map[string]any{
			"type":          "response.reasoning_summary_part.added",
			"item_id":       itemID,
			"output_index":  outputIndex,
			"summary_index": 0,
			"part": map[string]any{
				"type": "summary_text",
				"text": "",
			},
		})
		if text != "" {
			writeSSEEvent(out, "response.reasoning_summary_text.delta", map[string]any{
				"type":          "response.reasoning_summary_text.delta",
				"item_id":       itemID,
				"output_index":  outputIndex,
				"summary_index": 0,
				"delta":         text,
			})
		}
		writeSSEEvent(out, "response.reasoning_summary_text.done", map[string]any{
			"type":          "response.reasoning_summary_text.done",
			"item_id":       itemID,
			"output_index":  outputIndex,
			"summary_index": 0,
			"text":          text,
		})
		writeSSEEvent(out, "response.reasoning_summary_part.done", map[string]any{
			"type":          "response.reasoning_summary_part.done",
			"item_id":       itemID,
			"output_index":  outputIndex,
			"summary_index": 0,
			"part": map[string]any{
				"type": "summary_text",
				"text": text,
			},
		})
	case "function_call":
		arguments, _ := item["arguments"].(string)
		if arguments != "" {
			writeSSEEvent(out, "response.function_call_arguments.delta", map[string]any{
				"type":         "response.function_call_arguments.delta",
				"item_id":      itemID,
				"output_index": outputIndex,
				"delta":        arguments,
			})
		}
		writeSSEEvent(out, "response.function_call_arguments.done", map[string]any{
			"type":         "response.function_call_arguments.done",
			"item_id":      itemID,
			"output_index": outputIndex,
			"arguments":    arguments,
			"name":         item["name"],
			"call_id":      item["call_id"],
		})
	}
}

func responseSnapshot(response map[string]any, status string, output []any) map[string]any {
	snapshot := make(map[string]any, len(response))
	for key, value := range response {
		snapshot[key] = value
	}
	snapshot["status"] = status
	snapshot["output"] = output
	return snapshot
}

func itemSnapshot(item map[string]any, status string) map[string]any {
	snapshot := make(map[string]any, len(item))
	for key, value := range item {
		snapshot[key] = value
	}
	if _, ok := snapshot["status"]; ok {
		snapshot["status"] = status
	}
	return snapshot
}

func responseMessageText(item map[string]any) string {
	content, _ := item["content"].([]any)
	for _, rawPart := range content {
		part, _ := rawPart.(map[string]any)
		if part == nil {
			continue
		}
		if part["type"] == "output_text" {
			text, _ := part["text"].(string)
			return text
		}
	}
	return ""
}

func responseReasoningText(item map[string]any) string {
	summary, _ := item["summary"].([]any)
	for _, rawPart := range summary {
		part, _ := rawPart.(map[string]any)
		if part == nil {
			continue
		}
		if part["type"] == "summary_text" {
			text, _ := part["text"].(string)
			return text
		}
	}
	return ""
}

func writeSSEEvent(out *bytes.Buffer, event string, data any) {
	payload, _ := json.Marshal(data)
	out.WriteString("event: ")
	out.WriteString(event)
	out.WriteString("\n")
	out.WriteString("data: ")
	out.Write(payload)
	out.WriteString("\n\n")
}

func deepSeekReasoningContent(body []byte) string {
	return gjson.GetBytes(body, "choices.0.message.reasoning_content").String()
}

func deepSeekMessageToResponsesOutput(responseID string, text string) map[string]any {
	return map[string]any{
		"type":   "message",
		"id":     "msg_" + fallbackResponseID(responseID),
		"status": "completed",
		"role":   "assistant",
		"content": []map[string]any{
			{
				"type": "output_text",
				"text": text,
			},
		},
	}
}

func deepSeekToolCallsToResponsesOutput(body []byte, reasoning string) []map[string]any {
	toolCalls := gjson.GetBytes(body, "choices.0.message.tool_calls")
	if !toolCalls.IsArray() {
		return nil
	}
	output := make([]map[string]any, 0)
	toolCalls.ForEach(func(_, toolCall gjson.Result) bool {
		callID := toolCall.Get("id").String()
		item := map[string]any{
			"type":      "function_call",
			"id":        callID,
			"call_id":   callID,
			"name":      toolCall.Get("function.name").String(),
			"arguments": toolCall.Get("function.arguments").String(),
			"status":    "completed",
		}
		if strings.TrimSpace(reasoning) != "" {
			item["reasoning_content"] = reasoning
		}
		output = append(output, item)
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

type codexChatHistoryStore struct {
	mu        sync.Mutex
	responses map[string][]codexChatCachedFunctionCall
}

type codexChatCachedFunctionCall struct {
	CallID           string
	Name             string
	Arguments        string
	ReasoningContent string
}

func newCodexChatHistoryStore() *codexChatHistoryStore {
	return &codexChatHistoryStore{
		responses: make(map[string][]codexChatCachedFunctionCall),
	}
}

func (h *codexChatHistoryStore) enrichRequest(body []byte) []byte {
	if h == nil {
		return body
	}
	var request map[string]any
	if err := json.Unmarshal(body, &request); err != nil {
		return body
	}
	previousResponseID, _ := request["previous_response_id"].(string)
	if strings.TrimSpace(previousResponseID) == "" {
		return body
	}

	calls := h.callsForResponse(previousResponseID)
	if len(calls) == 0 {
		return body
	}

	input, exists := request["input"]
	if !exists {
		return body
	}

	items, wasSingleObject := inputItems(input)
	if len(items) == 0 {
		return body
	}

	callsByID := make(map[string]codexChatCachedFunctionCall, len(calls))
	for _, call := range calls {
		callsByID[call.CallID] = call
	}

	existingCallIDs := make(map[string]bool)
	for _, item := range items {
		if itemType(item) == "function_call" {
			if callID := itemCallID(item); callID != "" {
				existingCallIDs[callID] = true
			}
		}
	}

	changed := false
	enriched := make([]any, 0, len(items)+len(calls))
	for _, item := range items {
		if itemType(item) == "function_call_output" {
			callID := itemCallID(item)
			if call, ok := callsByID[callID]; ok && !existingCallIDs[callID] {
				enriched = append(enriched, call.toResponsesInputItem())
				existingCallIDs[callID] = true
				changed = true
			}
		}
		enriched = append(enriched, item)
	}
	if !changed {
		return body
	}

	if wasSingleObject && len(enriched) == 1 {
		request["input"] = enriched[0]
	} else {
		request["input"] = enriched
	}
	out, err := json.Marshal(request)
	if err != nil {
		return body
	}
	return out
}

func (h *codexChatHistoryStore) callsForResponse(responseID string) []codexChatCachedFunctionCall {
	h.mu.Lock()
	defer h.mu.Unlock()
	calls := h.responses[responseID]
	if len(calls) == 0 {
		return nil
	}
	result := make([]codexChatCachedFunctionCall, len(calls))
	copy(result, calls)
	return result
}

func (h *codexChatHistoryStore) recordResponsePayload(payload []byte, stream bool) {
	if h == nil {
		return
	}
	responsePayload := payload
	if stream {
		responsePayload = responsePayloadFromSSE(payload)
	}
	responseID := gjson.GetBytes(responsePayload, "id").String()
	if strings.TrimSpace(responseID) == "" {
		if nested := gjson.GetBytes(responsePayload, "response"); nested.Exists() {
			responsePayload = []byte(nested.Raw)
			responseID = gjson.GetBytes(responsePayload, "id").String()
		}
	}
	if strings.TrimSpace(responseID) == "" {
		return
	}

	output := gjson.GetBytes(responsePayload, "output")
	if !output.IsArray() {
		return
	}
	calls := make([]codexChatCachedFunctionCall, 0)
	output.ForEach(func(_, item gjson.Result) bool {
		if item.Get("type").String() != "function_call" {
			return true
		}
		callID := firstNonEmpty(item.Get("call_id").String(), item.Get("id").String())
		if strings.TrimSpace(callID) == "" {
			return true
		}
		calls = append(calls, codexChatCachedFunctionCall{
			CallID:           callID,
			Name:             item.Get("name").String(),
			Arguments:        item.Get("arguments").String(),
			ReasoningContent: item.Get("reasoning_content").String(),
		})
		return true
	})
	if len(calls) == 0 {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	h.responses[responseID] = calls
}

func responsePayloadFromSSE(payload []byte) []byte {
	for _, block := range strings.Split(string(payload), "\n\n") {
		for _, line := range strings.Split(block, "\n") {
			data, ok := strings.CutPrefix(line, "data:")
			if !ok {
				continue
			}
			data = strings.TrimSpace(data)
			if data == "" || data == "[DONE]" {
				continue
			}
			value := gjson.Parse(data)
			if nested := value.Get("response"); nested.Exists() {
				return []byte(nested.Raw)
			}
			if value.Get("object").String() == "response" {
				return []byte(data)
			}
		}
	}
	return payload
}

func inputItems(input any) ([]map[string]any, bool) {
	switch value := input.(type) {
	case []any:
		items := make([]map[string]any, 0, len(value))
		for _, raw := range value {
			item, ok := raw.(map[string]any)
			if !ok {
				return nil, false
			}
			items = append(items, item)
		}
		return items, false
	case map[string]any:
		return []map[string]any{value}, true
	default:
		return nil, false
	}
}

func itemType(item map[string]any) string {
	value, _ := item["type"].(string)
	return value
}

func itemCallID(item map[string]any) string {
	if callID, _ := item["call_id"].(string); strings.TrimSpace(callID) != "" {
		return callID
	}
	id, _ := item["id"].(string)
	return id
}

func (call codexChatCachedFunctionCall) toResponsesInputItem() map[string]any {
	item := map[string]any{
		"type":      "function_call",
		"id":        call.CallID,
		"call_id":   call.CallID,
		"name":      call.Name,
		"arguments": call.Arguments,
	}
	if strings.TrimSpace(call.ReasoningContent) != "" {
		item["reasoning_content"] = call.ReasoningContent
	}
	return item
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
	if prs.codexChatHistory != nil {
		bodyBytes = prs.codexChatHistory.enrichRequest(bodyBytes)
	}
	translatedBody, err := translateResponsesRequestToDeepSeekChatCompletion(bodyBytes)
	if err != nil {
		return false, err
	}
	if requestLog.RawLog != nil {
		requestLog.RawLog.captureUpstreamRequestBody(translatedBody)
	}

	resp, err := postUpstream(joinURL(provider.APIURL, deepSeekChatCompletionsEndpoint), query, headers, translatedBody)
	if err != nil {
		return false, err
	}
	if resp == nil {
		return false, fmt.Errorf("empty response")
	}
	status := resp.StatusCode
	requestLog.HttpCode = status
	if status < http.StatusOK || status >= http.StatusMultipleChoices {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if requestLog.RawLog != nil {
			requestLog.RawLog.captureResponseHeaders(resp.Header)
			requestLog.RawLog.captureResponseBody(body)
			requestLog.RawLog.captureUpstreamResponseBody(body)
		}
		return false, newUpstreamResponseError(status, resp.Header, body)
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}
	if requestLog.RawLog != nil {
		requestLog.RawLog.captureUpstreamResponseBody(body)
	}
	translatedResponse, err := translateDeepSeekChatCompletionToResponses(body, isStream)
	if err != nil {
		return false, err
	}
	if prs.codexChatHistory != nil {
		prs.codexChatHistory.recordResponsePayload(translatedResponse, isStream)
	}

	if isStream {
		c.Header("Content-Type", "text/event-stream")
		parseEventPayload(string(translatedResponse), CodexParseTokenUsageFromResponse, requestLog)
	} else {
		c.Header("Content-Type", "application/json")
		CodexParseTokenUsageFromResponse(string(translatedResponse), requestLog)
	}
	if requestLog.RawLog != nil {
		requestLog.RawLog.captureResponseHeaders(c.Writer.Header())
		requestLog.RawLog.captureResponseBody(translatedResponse)
	}
	_, err = io.Copy(c.Writer, bytes.NewReader(translatedResponse))
	return err == nil, err
}
