package services

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const deepSeekChatCompletionsEndpoint = "/chat/completions"
const thinkOpenTag = "<think>"
const thinkCloseTag = "</think>"

type deepSeekChatMessage struct {
	Role             string           `json:"role"`
	Content          any              `json:"content"`
	ToolCallID       string           `json:"tool_call_id,omitempty"`
	ToolCalls        []map[string]any `json:"tool_calls,omitempty"`
	ReasoningContent string           `json:"reasoning_content,omitempty"`
}

type deepSeekChatRequest struct {
	Model             string                `json:"model"`
	Messages          []deepSeekChatMessage `json:"messages"`
	Temperature       *float64              `json:"temperature,omitempty"`
	TopP              *float64              `json:"top_p,omitempty"`
	FrequencyPenalty  *float64              `json:"frequency_penalty,omitempty"`
	PresencePenalty   *float64              `json:"presence_penalty,omitempty"`
	MaxTokens         int64                 `json:"max_tokens,omitempty"`
	Stream            bool                  `json:"stream"`
	Tools             []map[string]any      `json:"tools,omitempty"`
	ToolChoice        any                   `json:"tool_choice,omitempty"`
	Thinking          map[string]string     `json:"thinking,omitempty"`
	ReasoningEffort   string                `json:"reasoning_effort,omitempty"`
	LogitBias         any                   `json:"logit_bias,omitempty"`
	Logprobs          *bool                 `json:"logprobs,omitempty"`
	TopLogprobs       any                   `json:"top_logprobs,omitempty"`
	N                 any                   `json:"n,omitempty"`
	Stop              any                   `json:"stop,omitempty"`
	Metadata          any                   `json:"metadata,omitempty"`
	User              string                `json:"user,omitempty"`
	ParallelToolCalls *bool                 `json:"parallel_tool_calls,omitempty"`
	ResponseFormat    any                   `json:"response_format,omitempty"`
	Seed              any                   `json:"seed,omitempty"`
	ServiceTier       string                `json:"service_tier,omitempty"`
	StreamOptions     any                   `json:"stream_options,omitempty"`
}

func translateResponsesRequestToDeepSeekChatCompletion(body []byte) ([]byte, error) {
	model := gjson.GetBytes(body, "model").String()
	if model == "" {
		return nil, fmt.Errorf("responses request missing model")
	}

	request := deepSeekChatRequest{
		Model:    model,
		Messages: make([]deepSeekChatMessage, 0, 2),
		Stream:   gjson.GetBytes(body, "stream").Bool(),
	}
	if temperature := gjson.GetBytes(body, "temperature"); temperature.Exists() {
		value := temperature.Float()
		request.Temperature = &value
	}
	if maxTokens := gjson.GetBytes(body, "max_output_tokens"); maxTokens.Exists() {
		request.MaxTokens = maxTokens.Int()
	}
	applyResponsesChatPassthroughFields(&request, body)
	request.Tools = responsesToolsToDeepSeekTools(body)
	request.ToolChoice = responsesToolChoiceToDeepSeekToolChoice(body)
	applyResponsesReasoningToDeepSeekChatRequest(&request, body)
	if instructions := gjson.GetBytes(body, "instructions").String(); strings.TrimSpace(instructions) != "" {
		request.Messages = append(request.Messages, deepSeekChatMessage{Role: "system", Content: instructions})
	}
	request.Messages = append(request.Messages, responsesInputToDeepSeekMessages(body)...)
	request.Messages = collapseDeepSeekSystemMessagesToHead(request.Messages)
	ensureDeepSeekStreamUsageOption(&request)
	if len(request.Messages) == 0 {
		return nil, fmt.Errorf("responses request missing input")
	}

	return json.Marshal(request)
}

func applyResponsesChatPassthroughFields(request *deepSeekChatRequest, body []byte) {
	if topP := gjson.GetBytes(body, "top_p"); topP.Exists() {
		value := topP.Float()
		request.TopP = &value
	}
	if frequencyPenalty := gjson.GetBytes(body, "frequency_penalty"); frequencyPenalty.Exists() {
		value := frequencyPenalty.Float()
		request.FrequencyPenalty = &value
	}
	if presencePenalty := gjson.GetBytes(body, "presence_penalty"); presencePenalty.Exists() {
		value := presencePenalty.Float()
		request.PresencePenalty = &value
	}
	if logitBias := gjson.GetBytes(body, "logit_bias"); logitBias.Exists() {
		request.LogitBias = rawJSONValue(logitBias)
	}
	if logprobs := gjson.GetBytes(body, "logprobs"); logprobs.Exists() {
		value := logprobs.Bool()
		request.Logprobs = &value
	}
	if topLogprobs := gjson.GetBytes(body, "top_logprobs"); topLogprobs.Exists() {
		request.TopLogprobs = rawJSONValue(topLogprobs)
	}
	if n := gjson.GetBytes(body, "n"); n.Exists() {
		request.N = rawJSONValue(n)
	}
	if stop := gjson.GetBytes(body, "stop"); stop.Exists() {
		request.Stop = rawJSONValue(stop)
	}
	if metadata := gjson.GetBytes(body, "metadata"); metadata.Exists() {
		request.Metadata = rawJSONValue(metadata)
	}
	if user := gjson.GetBytes(body, "user").String(); strings.TrimSpace(user) != "" {
		request.User = user
	}
	if parallelToolCalls := gjson.GetBytes(body, "parallel_tool_calls"); parallelToolCalls.Exists() {
		value := parallelToolCalls.Bool()
		request.ParallelToolCalls = &value
	}
	if responseFormat := gjson.GetBytes(body, "response_format"); responseFormat.Exists() {
		request.ResponseFormat = rawJSONValue(responseFormat)
	}
	if seed := gjson.GetBytes(body, "seed"); seed.Exists() {
		request.Seed = rawJSONValue(seed)
	}
	if serviceTier := gjson.GetBytes(body, "service_tier").String(); strings.TrimSpace(serviceTier) != "" {
		request.ServiceTier = serviceTier
	}
	if streamOptions := gjson.GetBytes(body, "stream_options"); streamOptions.Exists() {
		request.StreamOptions = rawJSONValue(streamOptions)
	}
}

func ensureDeepSeekStreamUsageOption(request *deepSeekChatRequest) {
	if !request.Stream {
		return
	}
	options, _ := request.StreamOptions.(map[string]any)
	if options == nil {
		options = make(map[string]any)
	}
	options["include_usage"] = true
	request.StreamOptions = options
}

func rawJSONValue(value gjson.Result) any {
	if !value.Exists() {
		return nil
	}
	var parsed any
	if err := json.Unmarshal([]byte(value.Raw), &parsed); err != nil {
		return value.Value()
	}
	return parsed
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
	pendingReasoning := ""
	if input.IsArray() {
		input.ForEach(func(_, item gjson.Result) bool {
			appendResponsesInputItemToDeepSeekMessages(&messages, item, &pendingReasoning)
			return true
		})
		return messages
	}
	if input.IsObject() {
		appendResponsesInputItemToDeepSeekMessages(&messages, input, &pendingReasoning)
	}
	return messages
}

func appendResponsesInputItemToDeepSeekMessages(messages *[]deepSeekChatMessage, item gjson.Result, pendingReasoning *string) {
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
		reasoning := firstNonEmpty(responsesItemReasoningContent(item), *pendingReasoning)
		*pendingReasoning = ""
		appendDeepSeekToolCall(messages, responsesFunctionCallToDeepSeekToolCall(item), reasoning)
		return
	case "reasoning":
		reasoning := responsesReasoningItemText(item)
		if attachDeepSeekReasoningToLastAssistant(messages, reasoning) {
			return
		}
		*pendingReasoning = appendReasoningText(*pendingReasoning, reasoning)
		return
	}

	role := responsesRoleToDeepSeekRole(item.Get("role").String())
	content := responsesMessageContent(item.Get("content"))
	if !isEmptyChatContent(content) {
		message := deepSeekChatMessage{Role: role, Content: content}
		if role == "assistant" {
			message.ReasoningContent = *pendingReasoning
			*pendingReasoning = ""
		} else if *pendingReasoning != "" {
			*pendingReasoning = ""
		}
		*messages = append(*messages, message)
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
			last.ReasoningContent = appendReasoningTextReplacingPlaceholder(last.ReasoningContent, reasoning)
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
			"arguments": canonicalJSONText(item.Get("arguments").String()),
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

func responsesReasoningItemText(item gjson.Result) string {
	if reasoning := responsesItemReasoningContent(item); strings.TrimSpace(reasoning) != "" {
		return reasoning
	}
	if summary := item.Get("summary"); summary.IsArray() {
		parts := make([]string, 0)
		summary.ForEach(func(_, part gjson.Result) bool {
			if text := part.Get("text").String(); strings.TrimSpace(text) != "" {
				parts = append(parts, text)
			}
			return true
		})
		return strings.Join(parts, "\n\n")
	}
	if content := item.Get("content"); content.Exists() {
		return responsesMessageContentAsText(content)
	}
	return ""
}

func responsesMessageContentAsText(content gjson.Result) string {
	value := responsesMessageContent(content)
	switch typed := value.(type) {
	case string:
		return typed
	case []any:
		parts := make([]string, 0, len(typed))
		for _, part := range typed {
			encoded, err := json.Marshal(part)
			if err == nil {
				parts = append(parts, string(encoded))
			}
		}
		return strings.Join(parts, "\n")
	default:
		if value == nil {
			return ""
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			return ""
		}
		return string(encoded)
	}
}

func attachDeepSeekReasoningToLastAssistant(messages *[]deepSeekChatMessage, reasoning string) bool {
	reasoning = strings.TrimSpace(reasoning)
	if reasoning == "" || len(*messages) == 0 {
		return false
	}
	for i := len(*messages) - 1; i >= 0; i-- {
		msg := &(*messages)[i]
		if msg.Role == "assistant" {
			msg.ReasoningContent = appendReasoningTextReplacingPlaceholder(msg.ReasoningContent, reasoning)
			return true
		}
		if msg.Role != "tool" {
			return false
		}
	}
	return false
}

func appendReasoningText(existing string, next string) string {
	existing = strings.TrimSpace(existing)
	next = strings.TrimSpace(next)
	if next == "" {
		return existing
	}
	if existing == "" {
		return next
	}
	if strings.Contains(existing, next) {
		return existing
	}
	return existing + "\n\n" + next
}

func appendReasoningTextReplacingPlaceholder(existing string, next string) string {
	if strings.TrimSpace(existing) == "tool call" {
		existing = ""
	}
	return appendReasoningText(existing, next)
}

func responsesRoleToDeepSeekRole(role string) string {
	normalized := strings.TrimSpace(strings.ToLower(role))
	switch normalized {
	case "":
		return "user"
	case "system", "user", "assistant", "tool":
		return normalized
	case "developer":
		return "system"
	case "latest_reminder":
		return "user"
	default:
		return "user"
	}
}

func collapseDeepSeekSystemMessagesToHead(messages []deepSeekChatMessage) []deepSeekChatMessage {
	systemChunks := make([]string, 0)
	rest := make([]deepSeekChatMessage, 0, len(messages))
	for _, message := range messages {
		if message.Role == "system" {
			if text := chatContentAsSystemText(message.Content); strings.TrimSpace(text) != "" {
				systemChunks = append(systemChunks, text)
			}
			continue
		}
		rest = append(rest, message)
	}
	if len(systemChunks) == 0 {
		return rest
	}
	out := make([]deepSeekChatMessage, 0, len(rest)+1)
	out = append(out, deepSeekChatMessage{
		Role:    "system",
		Content: strings.Join(systemChunks, "\n\n"),
	})
	out = append(out, rest...)
	return out
}

func chatContentAsSystemText(content any) string {
	switch value := content.(type) {
	case string:
		return value
	case nil:
		return ""
	default:
		encoded, err := json.Marshal(value)
		if err != nil {
			return ""
		}
		return string(encoded)
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
				if strict := item.Get("strict"); strict.Exists() {
					if _, exists := fn["strict"]; !exists {
						fn["strict"] = rawJSONValue(strict)
					}
				}
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
		if strict := item.Get("strict"); strict.Exists() {
			fn["strict"] = rawJSONValue(strict)
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

func canonicalJSONText(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return raw
	}
	var parsed any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return raw
	}
	encoded, err := json.Marshal(parsed)
	if err != nil {
		return raw
	}
	return string(encoded)
}

func chatReasoningText(value gjson.Result) string {
	if !value.Exists() || value.Type == gjson.Null {
		return ""
	}
	if value.Type == gjson.String {
		return value.String()
	}
	if text := value.Get("text").String(); strings.TrimSpace(text) != "" {
		return text
	}
	if text := value.Get("summary").String(); strings.TrimSpace(text) != "" {
		return text
	}
	if value.IsArray() {
		parts := make([]string, 0)
		value.ForEach(func(_, part gjson.Result) bool {
			if text := chatReasoningText(part); strings.TrimSpace(text) != "" {
				parts = append(parts, text)
			}
			return true
		})
		return strings.Join(parts, "\n\n")
	}
	return ""
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

func responsesMessageContent(content gjson.Result) any {
	if !content.Exists() {
		return ""
	}
	if content.Type == gjson.String {
		return content.String()
	}
	if content.IsObject() {
		return responsesSingleContentPart(content)
	}
	if !content.IsArray() {
		return ""
	}

	textParts := make([]string, 0)
	chatParts := make([]any, 0)
	hasNonTextPart := false
	content.ForEach(func(_, part gjson.Result) bool {
		converted, kind := responsesContentPart(part)
		switch kind {
		case "text":
			text, _ := converted.(string)
			if text != "" {
				textParts = append(textParts, text)
				chatParts = append(chatParts, map[string]any{
					"type": "text",
					"text": text,
				})
			}
		case "part":
			hasNonTextPart = true
			chatParts = append(chatParts, converted)
		}
		return true
	})
	if hasNonTextPart {
		return chatParts
	}
	return strings.Join(textParts, "\n")
}

func responsesSingleContentPart(part gjson.Result) any {
	converted, kind := responsesContentPart(part)
	if kind == "part" {
		return []any{converted}
	}
	if text, ok := converted.(string); ok {
		return text
	}
	return ""
}

func responsesContentPart(part gjson.Result) (any, string) {
	if text := part.Get("text").String(); text != "" {
		return text, "text"
	}
	if text := part.Get("input_text").String(); text != "" {
		return text, "text"
	}
	if text := part.Get("output_text").String(); text != "" {
		return text, "text"
	}
	if text := part.Get("refusal").String(); text != "" {
		return text, "text"
	}
	if part.Get("type").String() == "input_image" {
		if imageURL := part.Get("image_url"); imageURL.Exists() {
			var value any
			if imageURL.IsObject() {
				value = rawJSONValue(imageURL)
			} else {
				value = map[string]any{"url": imageURL.String()}
			}
			return map[string]any{
				"type":      "image_url",
				"image_url": value,
			}, "part"
		}
	}
	if part.Raw != "" {
		return part.Raw, "text"
	}
	return "", ""
}

func isEmptyChatContent(content any) bool {
	switch value := content.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(value) == ""
	case []any:
		return len(value) == 0
	default:
		return false
	}
}

func responsesOutputText(output gjson.Result) string {
	if !output.Exists() {
		return ""
	}
	if output.Type == gjson.String {
		return canonicalJSONText(output.String())
	}
	if output.Raw == "" {
		return ""
	}
	var parsed any
	if err := json.Unmarshal([]byte(output.Raw), &parsed); err != nil {
		return output.Raw
	}
	encoded, err := json.Marshal(parsed)
	if err != nil {
		return output.Raw
	}
	return string(encoded)
}

func translateDeepSeekChatCompletionToResponses(body []byte, stream bool) ([]byte, error) {
	text := gjson.GetBytes(body, "choices.0.message.content").String()
	refusal := gjson.GetBytes(body, "choices.0.message.refusal").String()
	responseID := gjson.GetBytes(body, "id").String()
	reasoning := deepSeekReasoningContent(body)
	if thinkReasoning, answer, ok := splitLeadingThinkBlock(text); ok {
		text = answer
		if strings.TrimSpace(reasoning) == "" {
			reasoning = thinkReasoning
		}
	}
	finishReason := gjson.GetBytes(body, "choices.0.finish_reason").String()
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
	if strings.TrimSpace(text) != "" || strings.TrimSpace(refusal) != "" {
		output = append(output, deepSeekMessageToResponsesOutput(responseID, text, refusal))
	}
	output = append(output, deepSeekToolCallsToResponsesOutput(body, reasoning)...)
	if len(output) == 0 {
		output = []map[string]any{
			deepSeekMessageToResponsesOutput(responseID, text, refusal),
		}
	}
	usage := deepSeekUsageToResponsesUsage(body)
	response := map[string]any{
		"id":          responseID,
		"object":      "response",
		"created_at":  deepSeekCreatedAt(body),
		"status":      deepSeekFinishReasonToResponsesStatus(finishReason),
		"model":       gjson.GetBytes(body, "model").String(),
		"output_text": text,
		"output":      output,
		"usage":       usage,
	}
	if finishReason == "length" {
		response["incomplete_details"] = map[string]any{"reason": "max_output_tokens"}
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

func deepSeekCreatedAt(body []byte) int64 {
	if created := gjson.GetBytes(body, "created"); created.Exists() {
		return created.Int()
	}
	return time.Now().Unix()
}

func deepSeekFinishReasonToResponsesStatus(finishReason string) string {
	if finishReason == "length" {
		return "incomplete"
	}
	return "completed"
}

func deepSeekUsageToResponsesUsage(body []byte) map[string]any {
	inputTokens := firstExistingInt(
		gjson.GetBytes(body, "usage.prompt_tokens"),
		gjson.GetBytes(body, "usage.input_tokens"),
	)
	outputTokens := firstExistingInt(
		gjson.GetBytes(body, "usage.completion_tokens"),
		gjson.GetBytes(body, "usage.output_tokens"),
	)
	totalTokens := firstExistingInt(gjson.GetBytes(body, "usage.total_tokens"))
	if totalTokens == 0 && (inputTokens != 0 || outputTokens != 0) {
		totalTokens = inputTokens + outputTokens
	}
	usage := map[string]any{
		"input_tokens":  inputTokens,
		"output_tokens": outputTokens,
		"total_tokens":  totalTokens,
	}
	if details := gjson.GetBytes(body, "usage.completion_tokens_details"); details.Exists() && details.IsObject() {
		usage["output_tokens_details"] = rawJSONValue(details)
	} else {
		usage["output_tokens_details"] = map[string]any{
			"reasoning_tokens": int64(0),
		}
	}
	if cached := gjson.GetBytes(body, "usage.prompt_tokens_details.cached_tokens"); cached.Exists() {
		usage["input_tokens_details"] = map[string]any{
			"cached_tokens": cached.Int(),
		}
	} else if cached := gjson.GetBytes(body, "usage.input_tokens_details.cached_tokens"); cached.Exists() {
		usage["input_tokens_details"] = map[string]any{
			"cached_tokens": cached.Int(),
		}
	}
	return usage
}

func firstExistingInt(values ...gjson.Result) int64 {
	for _, value := range values {
		if value.Exists() {
			return value.Int()
		}
	}
	return 0
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

func translateDeepSeekChatSSEToResponsesSSE(body []byte) ([]byte, error) {
	if message, errorType, ok := chatSSEError(body); ok {
		return responsesFailedSSE(message, errorType), nil
	}
	completion := chatSSEToCompletion(body)
	payload, err := json.Marshal(completion)
	if err != nil {
		return nil, err
	}
	return translateDeepSeekChatCompletionToResponses(payload, true)
}

func streamDeepSeekChatSSEToResponses(reader io.Reader, writer io.Writer) ([]byte, error) {
	state := newResponsesSSEStreamState()
	buffered := bufio.NewReader(reader)
	var block strings.Builder
	var converted bytes.Buffer

	writeEvents := func(events [][]byte) error {
		for _, event := range events {
			if len(event) == 0 {
				continue
			}
			converted.Write(event)
			if _, err := writer.Write(event); err != nil {
				return err
			}
			if flusher, ok := writer.(interface{ Flush() }); ok {
				flusher.Flush()
			}
		}
		return nil
	}

	handleBlock := func(raw string) error {
		events, err := state.handleSSEBlock(raw)
		if err != nil {
			return err
		}
		return writeEvents(events)
	}

	for {
		line, err := buffered.ReadString('\n')
		if len(line) > 0 {
			if strings.TrimRight(line, "\r\n") == "" {
				if err := handleBlock(block.String()); err != nil {
					return converted.Bytes(), err
				}
				block.Reset()
			} else {
				block.WriteString(line)
			}
		}

		if err != nil {
			if err != io.EOF {
				failed := state.failedEvents(fmt.Sprintf("Stream error: %v", err), "stream_error")
				_ = writeEvents(failed)
				return converted.Bytes(), err
			}
			if strings.TrimSpace(block.String()) != "" {
				if err := handleBlock(block.String()); err != nil {
					return converted.Bytes(), err
				}
			}
			if !state.completed {
				if err := writeEvents(state.finalizeEvents()); err != nil {
					return converted.Bytes(), err
				}
			}
			return converted.Bytes(), nil
		}
	}
}

func chatSSEError(body []byte) (string, string, bool) {
	for _, block := range strings.Split(string(body), "\n\n") {
		eventName := ""
		dataParts := make([]string, 0)
		for _, line := range strings.Split(block, "\n") {
			line = strings.TrimSpace(line)
			if event, ok := strings.CutPrefix(line, "event:"); ok {
				eventName = strings.TrimSpace(event)
				continue
			}
			if data, ok := strings.CutPrefix(line, "data:"); ok {
				dataParts = append(dataParts, strings.TrimSpace(data))
			}
		}
		if len(dataParts) == 0 {
			continue
		}
		data := strings.Join(dataParts, "\n")
		if data == "" || data == "[DONE]" {
			continue
		}
		value := gjson.Parse(data)
		if eventName == "error" || value.Get("error").Exists() {
			message, errorType := chatSSEErrorFields(value)
			return message, errorType, true
		}
	}
	return "", "", false
}

func chatSSEErrorFields(value gjson.Result) (string, string) {
	source := value.Get("error")
	if !source.Exists() {
		source = value
	}
	message := ""
	if source.Type == gjson.String {
		message = source.String()
	} else {
		message = firstNonEmpty(
			source.Get("message").String(),
			source.Get("detail").String(),
			source.Get("status_msg").String(),
			source.Get("base_resp.status_msg").String(),
		)
	}
	if strings.TrimSpace(message) == "" {
		message = "Upstream stream error"
	}
	errorType := firstNonEmpty(source.Get("type").String(), source.Get("code").String(), "stream_error")
	return message, errorType
}

type responsesSSEStreamState struct {
	responseID      string
	model           string
	createdAt       int64
	started         bool
	completed       bool
	nextOutputIndex int
	text            streamTextItemState
	reasoning       streamReasoningItemState
	inlineThink     streamInlineThinkState
	toolCalls       map[int]*streamToolCallState
	outputItems     []streamOutputItem
	usage           map[string]any
	finishReason    string
}

type streamTextItemState struct {
	outputIndex int
	itemID      string
	text        strings.Builder
	added       bool
	done        bool
}

type streamReasoningItemState struct {
	outputIndex int
	itemID      string
	text        strings.Builder
	added       bool
	done        bool
}

type streamInlineThinkState struct {
	mode   string
	buffer string
}

type streamToolCallState struct {
	outputIndex      int
	itemID           string
	callID           string
	name             string
	arguments        strings.Builder
	reasoningContent string
	added            bool
	done             bool
}

type streamOutputItem struct {
	index int
	item  map[string]any
}

func newResponsesSSEStreamState() *responsesSSEStreamState {
	return &responsesSSEStreamState{
		responseID:  "chatcmpl-stream",
		createdAt:   time.Now().Unix(),
		inlineThink: streamInlineThinkState{mode: "detecting"},
		toolCalls:   make(map[int]*streamToolCallState),
		usage: map[string]any{
			"input_tokens":  int64(0),
			"output_tokens": int64(0),
			"total_tokens":  int64(0),
		},
	}
}

func (s *responsesSSEStreamState) handleSSEBlock(block string) ([][]byte, error) {
	if strings.TrimSpace(block) == "" || s.completed {
		return nil, nil
	}
	eventName := ""
	dataParts := make([]string, 0)
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		if event, ok := strings.CutPrefix(line, "event:"); ok {
			eventName = strings.TrimSpace(event)
			continue
		}
		if data, ok := strings.CutPrefix(line, "data:"); ok {
			dataParts = append(dataParts, strings.TrimSpace(data))
		}
	}
	if len(dataParts) == 0 {
		return nil, nil
	}
	data := strings.Join(dataParts, "\n")
	if data == "" {
		return nil, nil
	}
	if data == "[DONE]" {
		return s.finalizeEvents(), nil
	}
	value := gjson.Parse(data)
	if eventName == "error" || value.Get("error").Exists() {
		message, errorType := chatSSEErrorFields(value)
		return s.failedEvents(message, errorType), nil
	}
	return s.handleChatChunk(value), nil
}

func (s *responsesSSEStreamState) handleChatChunk(value gjson.Result) [][]byte {
	if id := value.Get("id").String(); strings.TrimSpace(id) != "" {
		s.responseID = id
	}
	if model := value.Get("model").String(); strings.TrimSpace(model) != "" {
		s.model = model
	}
	if created := value.Get("created"); created.Exists() {
		s.createdAt = created.Int()
	}
	if usage := value.Get("usage"); usage.Exists() && usage.IsObject() {
		s.usage = chatUsageToResponsesUsage(usage)
	}

	events := s.ensureStartedEvents()
	value.Get("choices").ForEach(func(_, choice gjson.Result) bool {
		delta := choice.Get("delta")
		if reasoning := firstNonEmpty(
			delta.Get("reasoning_content").String(),
			chatReasoningText(delta.Get("reasoning")),
			chatReasoningText(delta.Get("reasoning_details")),
		); reasoning != "" {
			events = append(events, s.pushReasoningDeltaEvents(reasoning)...)
		}
		if content := delta.Get("content").String(); content != "" {
			events = append(events, s.pushContentDeltaEvents(content)...)
		}
		if toolCalls := delta.Get("tool_calls"); toolCalls.IsArray() {
			events = append(events, s.flushInlineThinkAtBoundaryEvents()...)
			reasoning := strings.TrimSpace(s.reasoning.text.String())
			events = append(events, s.finalizeReasoningEvents()...)
			toolCalls.ForEach(func(_, toolCall gjson.Result) bool {
				events = append(events, s.pushToolCallDeltaEvents(toolCall, reasoning)...)
				return true
			})
		}
		if finishReason := choice.Get("finish_reason").String(); strings.TrimSpace(finishReason) != "" {
			s.finishReason = finishReason
		}
		return true
	})
	return events
}

func (s *responsesSSEStreamState) ensureStartedEvents() [][]byte {
	if s.started {
		return nil
	}
	s.started = true
	return [][]byte{
		sseEventBytes("response.created", map[string]any{
			"type":     "response.created",
			"response": s.baseResponse("in_progress", []map[string]any{}),
		}),
		sseEventBytes("response.in_progress", map[string]any{
			"type":     "response.in_progress",
			"response": s.baseResponse("in_progress", []map[string]any{}),
		}),
	}
}

func (s *responsesSSEStreamState) pushContentDeltaEvents(delta string) [][]byte {
	switch s.inlineThink.mode {
	case "text":
		events := s.finalizeReasoningEvents()
		return append(events, s.pushTextDeltaEvents(delta)...)
	case "reasoning":
		s.inlineThink.buffer += delta
		return s.drainCompleteInlineThinkEvents()
	default:
		s.inlineThink.buffer += delta
		switch leadingThinkPrefixDecision(s.inlineThink.buffer) {
		case "need_more":
			return nil
		case "reasoning":
			s.inlineThink.mode = "reasoning"
			return s.drainCompleteInlineThinkEvents()
		default:
			s.inlineThink.mode = "text"
			text := s.inlineThink.buffer
			s.inlineThink.buffer = ""
			events := s.finalizeReasoningEvents()
			return append(events, s.pushTextDeltaEvents(text)...)
		}
	}
}

func (s *responsesSSEStreamState) drainCompleteInlineThinkEvents() [][]byte {
	reasoning, answer, ok := splitLeadingThinkBlock(s.inlineThink.buffer)
	if !ok {
		return nil
	}
	s.inlineThink.mode = "text"
	s.inlineThink.buffer = ""
	events := make([][]byte, 0)
	if reasoning != "" {
		events = append(events, s.pushReasoningDeltaEvents(reasoning)...)
		events = append(events, s.finalizeReasoningEvents()...)
	}
	if answer != "" {
		events = append(events, s.pushTextDeltaEvents(answer)...)
	}
	return events
}

func (s *responsesSSEStreamState) flushInlineThinkAtBoundaryEvents() [][]byte {
	switch s.inlineThink.mode {
	case "text":
		return nil
	case "reasoning":
		buffered := s.inlineThink.buffer
		s.inlineThink.buffer = ""
		s.inlineThink.mode = "text"
		if reasoning, answer, ok := splitLeadingThinkBlock(buffered); ok {
			events := make([][]byte, 0)
			if reasoning != "" {
				events = append(events, s.pushReasoningDeltaEvents(reasoning)...)
				events = append(events, s.finalizeReasoningEvents()...)
			}
			if answer != "" {
				events = append(events, s.pushTextDeltaEvents(answer)...)
			}
			return events
		}
		reasoning := stripLeadingThinkOpenTag(buffered)
		if reasoning == "" {
			return nil
		}
		events := s.pushReasoningDeltaEvents(reasoning)
		return append(events, s.finalizeReasoningEvents()...)
	default:
		text := s.inlineThink.buffer
		s.inlineThink.buffer = ""
		s.inlineThink.mode = "text"
		if text == "" {
			return nil
		}
		events := s.finalizeReasoningEvents()
		return append(events, s.pushTextDeltaEvents(text)...)
	}
}

func (s *responsesSSEStreamState) pushReasoningDeltaEvents(delta string) [][]byte {
	events := s.ensureStartedEvents()
	if !s.reasoning.added {
		outputIndex := s.nextOutputIndexValue()
		itemID := "rs_" + fallbackResponseID(s.responseID)
		s.reasoning.outputIndex = outputIndex
		s.reasoning.itemID = itemID
		s.reasoning.added = true
		events = append(events,
			sseEventBytes("response.output_item.added", map[string]any{
				"type":         "response.output_item.added",
				"output_index": outputIndex,
				"item": map[string]any{
					"id":      itemID,
					"type":    "reasoning",
					"status":  "in_progress",
					"summary": []any{},
				},
			}),
			sseEventBytes("response.reasoning_summary_part.added", map[string]any{
				"type":          "response.reasoning_summary_part.added",
				"item_id":       itemID,
				"output_index":  outputIndex,
				"summary_index": 0,
				"part": map[string]any{
					"type": "summary_text",
					"text": "",
				},
			}),
		)
	}
	s.reasoning.text.WriteString(delta)
	events = append(events, sseEventBytes("response.reasoning_summary_text.delta", map[string]any{
		"type":          "response.reasoning_summary_text.delta",
		"item_id":       s.reasoning.itemID,
		"output_index":  s.reasoning.outputIndex,
		"summary_index": 0,
		"delta":         delta,
	}))
	return events
}

func (s *responsesSSEStreamState) pushTextDeltaEvents(delta string) [][]byte {
	events := s.ensureStartedEvents()
	if !s.text.added {
		outputIndex := s.nextOutputIndexValue()
		itemID := "msg_" + fallbackResponseID(s.responseID)
		s.text.outputIndex = outputIndex
		s.text.itemID = itemID
		s.text.added = true
		events = append(events,
			sseEventBytes("response.output_item.added", map[string]any{
				"type":         "response.output_item.added",
				"output_index": outputIndex,
				"item": map[string]any{
					"id":      itemID,
					"type":    "message",
					"status":  "in_progress",
					"role":    "assistant",
					"content": []any{},
				},
			}),
			sseEventBytes("response.content_part.added", map[string]any{
				"type":          "response.content_part.added",
				"item_id":       itemID,
				"output_index":  outputIndex,
				"content_index": 0,
				"part": map[string]any{
					"type":        "output_text",
					"text":        "",
					"annotations": []any{},
				},
			}),
		)
	}
	s.text.text.WriteString(delta)
	events = append(events, sseEventBytes("response.output_text.delta", map[string]any{
		"type":          "response.output_text.delta",
		"item_id":       s.text.itemID,
		"output_index":  s.text.outputIndex,
		"content_index": 0,
		"delta":         delta,
	}))
	return events
}

func (s *responsesSSEStreamState) pushToolCallDeltaEvents(toolCall gjson.Result, reasoning string) [][]byte {
	index := int(toolCall.Get("index").Int())
	state := s.toolCalls[index]
	if state == nil {
		state = &streamToolCallState{outputIndex: -1}
		s.toolCalls[index] = state
	}
	if id := toolCall.Get("id").String(); id != "" {
		state.callID = id
	}
	if typ := toolCall.Get("type").String(); typ != "" && state.callID == "" {
		state.callID = fmt.Sprintf("call_%d", index)
	}
	if name := toolCall.Get("function.name").String(); name != "" {
		state.name = name
	}
	argsDelta := toolCall.Get("function.arguments").String()
	if argsDelta != "" {
		state.arguments.WriteString(argsDelta)
	}
	if strings.TrimSpace(state.reasoningContent) == "" {
		state.reasoningContent = reasoning
	}

	events := s.ensureStartedEvents()
	if !state.added && (state.callID != "" || state.name != "") {
		state.added = true
		if state.callID == "" {
			state.callID = fmt.Sprintf("call_%d", index)
		}
		if state.name == "" {
			state.name = "unknown_tool"
		}
		state.outputIndex = s.nextOutputIndexValue()
		state.itemID = responsesFunctionCallItemID(state.callID)
		events = append(events, sseEventBytes("response.output_item.added", map[string]any{
			"type":         "response.output_item.added",
			"output_index": state.outputIndex,
			"item":         s.toolCallItem(state, "in_progress", ""),
		}))
	}
	if state.added && argsDelta != "" {
		events = append(events, sseEventBytes("response.function_call_arguments.delta", map[string]any{
			"type":         "response.function_call_arguments.delta",
			"item_id":      state.itemID,
			"output_index": state.outputIndex,
			"delta":        argsDelta,
		}))
	}
	return events
}

func (s *responsesSSEStreamState) finalizeEvents() [][]byte {
	if s.completed {
		return nil
	}
	events := s.ensureStartedEvents()
	events = append(events, s.flushInlineThinkAtBoundaryEvents()...)
	events = append(events, s.finalizeReasoningEvents()...)
	events = append(events, s.finalizeTextEvents()...)
	events = append(events, s.finalizeToolEvents()...)
	status := deepSeekFinishReasonToResponsesStatus(s.finishReason)
	response := s.baseResponse(status, s.completedOutputItems())
	if status == "incomplete" {
		response["incomplete_details"] = map[string]any{"reason": "max_output_tokens"}
	}
	events = append(events, sseEventBytes("response.completed", map[string]any{
		"type":     "response.completed",
		"response": response,
	}))
	s.completed = true
	return events
}

func (s *responsesSSEStreamState) finalizeReasoningEvents() [][]byte {
	if !s.reasoning.added || s.reasoning.done {
		return nil
	}
	text := s.reasoning.text.String()
	item := map[string]any{
		"id":     s.reasoning.itemID,
		"type":   "reasoning",
		"status": "completed",
		"summary": []map[string]any{
			{
				"type": "summary_text",
				"text": text,
			},
		},
	}
	s.outputItems = append(s.outputItems, streamOutputItem{index: s.reasoning.outputIndex, item: item})
	s.reasoning.done = true
	return [][]byte{
		sseEventBytes("response.reasoning_summary_text.done", map[string]any{
			"type":          "response.reasoning_summary_text.done",
			"item_id":       s.reasoning.itemID,
			"output_index":  s.reasoning.outputIndex,
			"summary_index": 0,
			"text":          text,
		}),
		sseEventBytes("response.reasoning_summary_part.done", map[string]any{
			"type":          "response.reasoning_summary_part.done",
			"item_id":       s.reasoning.itemID,
			"output_index":  s.reasoning.outputIndex,
			"summary_index": 0,
			"part": map[string]any{
				"type": "summary_text",
				"text": text,
			},
		}),
		sseEventBytes("response.output_item.done", map[string]any{
			"type":         "response.output_item.done",
			"output_index": s.reasoning.outputIndex,
			"item":         item,
		}),
	}
}

func (s *responsesSSEStreamState) finalizeTextEvents() [][]byte {
	if !s.text.added || s.text.done {
		return nil
	}
	text := s.text.text.String()
	item := map[string]any{
		"id":     s.text.itemID,
		"type":   "message",
		"status": "completed",
		"role":   "assistant",
		"content": []map[string]any{
			{
				"type":        "output_text",
				"text":        text,
				"annotations": []any{},
			},
		},
	}
	s.outputItems = append(s.outputItems, streamOutputItem{index: s.text.outputIndex, item: item})
	s.text.done = true
	return [][]byte{
		sseEventBytes("response.output_text.done", map[string]any{
			"type":          "response.output_text.done",
			"item_id":       s.text.itemID,
			"output_index":  s.text.outputIndex,
			"content_index": 0,
			"text":          text,
		}),
		sseEventBytes("response.content_part.done", map[string]any{
			"type":          "response.content_part.done",
			"item_id":       s.text.itemID,
			"output_index":  s.text.outputIndex,
			"content_index": 0,
			"part": map[string]any{
				"type":        "output_text",
				"text":        text,
				"annotations": []any{},
			},
		}),
		sseEventBytes("response.output_item.done", map[string]any{
			"type":         "response.output_item.done",
			"output_index": s.text.outputIndex,
			"item":         item,
		}),
	}
}

func (s *responsesSSEStreamState) finalizeToolEvents() [][]byte {
	keys := make([]int, 0, len(s.toolCalls))
	for key := range s.toolCalls {
		keys = append(keys, key)
	}
	sort.Ints(keys)
	events := make([][]byte, 0)
	for _, key := range keys {
		state := s.toolCalls[key]
		if state == nil || state.done {
			continue
		}
		if !state.added {
			if state.callID == "" {
				state.callID = fmt.Sprintf("call_%d", key)
			}
			if state.name == "" {
				state.name = "unknown_tool"
			}
			state.outputIndex = s.nextOutputIndexValue()
			state.itemID = responsesFunctionCallItemID(state.callID)
			state.added = true
			events = append(events, sseEventBytes("response.output_item.added", map[string]any{
				"type":         "response.output_item.added",
				"output_index": state.outputIndex,
				"item":         s.toolCallItem(state, "in_progress", ""),
			}))
		}
		arguments := canonicalJSONText(state.arguments.String())
		item := s.toolCallItem(state, "completed", arguments)
		s.outputItems = append(s.outputItems, streamOutputItem{index: state.outputIndex, item: item})
		state.done = true
		events = append(events,
			sseEventBytes("response.function_call_arguments.done", map[string]any{
				"type":         "response.function_call_arguments.done",
				"item_id":      state.itemID,
				"output_index": state.outputIndex,
				"arguments":    arguments,
				"name":         state.name,
				"call_id":      state.callID,
			}),
			sseEventBytes("response.output_item.done", map[string]any{
				"type":         "response.output_item.done",
				"output_index": state.outputIndex,
				"item":         item,
			}),
		)
	}
	return events
}

func (s *responsesSSEStreamState) failedEvents(message string, errorType string) [][]byte {
	if s.completed {
		return nil
	}
	response := s.baseResponse("failed", s.completedOutputItems())
	response["error"] = map[string]any{
		"message": message,
		"type":    errorType,
	}
	s.completed = true
	return [][]byte{
		sseEventBytes("response.failed", map[string]any{
			"type":     "response.failed",
			"response": response,
		}),
	}
}

func (s *responsesSSEStreamState) toolCallItem(state *streamToolCallState, status string, arguments string) map[string]any {
	item := map[string]any{
		"id":        state.itemID,
		"type":      "function_call",
		"status":    status,
		"call_id":   state.callID,
		"name":      state.name,
		"arguments": arguments,
	}
	if strings.TrimSpace(state.reasoningContent) != "" {
		item["reasoning_content"] = state.reasoningContent
	}
	return item
}

func (s *responsesSSEStreamState) baseResponse(status string, output []map[string]any) map[string]any {
	return map[string]any{
		"id":         s.responseID,
		"object":     "response",
		"created_at": s.createdAt,
		"status":     status,
		"model":      s.model,
		"output":     output,
		"usage":      s.usage,
	}
}

func (s *responsesSSEStreamState) completedOutputItems() []map[string]any {
	items := append([]streamOutputItem(nil), s.outputItems...)
	sort.Slice(items, func(i, j int) bool { return items[i].index < items[j].index })
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, item.item)
	}
	return out
}

func (s *responsesSSEStreamState) nextOutputIndexValue() int {
	index := s.nextOutputIndex
	s.nextOutputIndex++
	return index
}

func chatUsageToResponsesUsage(usage gjson.Result) map[string]any {
	inputTokens := firstExistingInt(usage.Get("prompt_tokens"), usage.Get("input_tokens"))
	outputTokens := firstExistingInt(usage.Get("completion_tokens"), usage.Get("output_tokens"))
	totalTokens := firstExistingInt(usage.Get("total_tokens"))
	if totalTokens == 0 && (inputTokens != 0 || outputTokens != 0) {
		totalTokens = inputTokens + outputTokens
	}
	out := map[string]any{
		"input_tokens":  inputTokens,
		"output_tokens": outputTokens,
		"total_tokens":  totalTokens,
	}
	if details := usage.Get("completion_tokens_details"); details.Exists() && details.IsObject() {
		out["output_tokens_details"] = rawJSONValue(details)
	}
	if cached := usage.Get("prompt_tokens_details.cached_tokens"); cached.Exists() {
		out["input_tokens_details"] = map[string]any{"cached_tokens": cached.Int()}
	} else if cached := usage.Get("input_tokens_details.cached_tokens"); cached.Exists() {
		out["input_tokens_details"] = map[string]any{"cached_tokens": cached.Int()}
	}
	return out
}

func sseEventBytes(event string, data any) []byte {
	var out bytes.Buffer
	writeSSEEvent(&out, event, data)
	return out.Bytes()
}

func leadingThinkPrefixDecision(buffer string) string {
	trimmed := strings.TrimLeft(buffer, " \t\r\n")
	if trimmed == "" || strings.HasPrefix(thinkOpenTag, trimmed) {
		return "need_more"
	}
	if strings.HasPrefix(trimmed, thinkOpenTag) {
		return "reasoning"
	}
	return "text"
}

func stripLeadingThinkOpenTag(text string) string {
	leadingWhitespaceLen := len(text) - len(strings.TrimLeft(text, " \t\r\n"))
	afterWhitespace := text[leadingWhitespaceLen:]
	if strings.HasPrefix(afterWhitespace, thinkOpenTag) {
		return strings.TrimSpace(afterWhitespace[len(thinkOpenTag):])
	}
	return strings.TrimSpace(text)
}

func responsesFailedSSE(message string, errorType string) []byte {
	var out bytes.Buffer
	response := map[string]any{
		"id":         "chatcmpl-stream",
		"object":     "response",
		"created_at": time.Now().Unix(),
		"status":     "failed",
		"model":      "",
		"output":     []any{},
		"usage": map[string]any{
			"input_tokens":  int64(0),
			"output_tokens": int64(0),
			"total_tokens":  int64(0),
		},
		"error": map[string]any{
			"message": message,
			"type":    errorType,
		},
	}
	writeSSEEvent(&out, "response.failed", map[string]any{
		"type":     "response.failed",
		"response": response,
	})
	return out.Bytes()
}

func chatSSEToCompletion(body []byte) map[string]any {
	acc := newChatSSEAccumulator()
	for _, block := range strings.Split(string(body), "\n\n") {
		for _, line := range strings.Split(block, "\n") {
			data, ok := strings.CutPrefix(strings.TrimSpace(line), "data:")
			if !ok {
				continue
			}
			data = strings.TrimSpace(data)
			if data == "" || data == "[DONE]" {
				continue
			}
			acc.addChunk([]byte(data))
		}
	}
	return acc.completion()
}

type chatSSEAccumulator struct {
	id           string
	model        string
	created      int64
	content      strings.Builder
	reasoning    strings.Builder
	finishReason string
	usage        map[string]any
	toolCalls    map[int]*chatSSEToolCall
}

type chatSSEToolCall struct {
	ID        string
	Type      string
	Name      string
	Arguments strings.Builder
}

func newChatSSEAccumulator() *chatSSEAccumulator {
	return &chatSSEAccumulator{
		id:        "chatcmpl-stream",
		created:   time.Now().Unix(),
		usage:     map[string]any{},
		toolCalls: make(map[int]*chatSSEToolCall),
	}
}

func (a *chatSSEAccumulator) addChunk(chunk []byte) {
	value := gjson.ParseBytes(chunk)
	if id := value.Get("id").String(); strings.TrimSpace(id) != "" {
		a.id = id
	}
	if model := value.Get("model").String(); strings.TrimSpace(model) != "" {
		a.model = model
	}
	if created := value.Get("created"); created.Exists() {
		a.created = created.Int()
	}
	if usage := value.Get("usage"); usage.Exists() && usage.IsObject() {
		if parsed := rawJSONValue(usage); parsed != nil {
			if usageMap, ok := parsed.(map[string]any); ok {
				a.usage = usageMap
			}
		}
	}
	value.Get("choices").ForEach(func(_, choice gjson.Result) bool {
		if finishReason := choice.Get("finish_reason").String(); strings.TrimSpace(finishReason) != "" {
			a.finishReason = finishReason
		}
		delta := choice.Get("delta")
		if text := delta.Get("content").String(); text != "" {
			a.content.WriteString(text)
		}
		if reasoning := delta.Get("reasoning_content").String(); reasoning != "" {
			a.reasoning.WriteString(reasoning)
		} else if reasoning := chatReasoningText(delta.Get("reasoning")); reasoning != "" {
			a.reasoning.WriteString(reasoning)
		} else if reasoning := chatReasoningText(delta.Get("reasoning_details")); reasoning != "" {
			a.reasoning.WriteString(reasoning)
		}
		delta.Get("tool_calls").ForEach(func(_, toolCall gjson.Result) bool {
			index := int(toolCall.Get("index").Int())
			call := a.toolCalls[index]
			if call == nil {
				call = &chatSSEToolCall{}
				a.toolCalls[index] = call
			}
			if id := toolCall.Get("id").String(); id != "" {
				call.ID = id
			}
			if typ := toolCall.Get("type").String(); typ != "" {
				call.Type = typ
			}
			if name := toolCall.Get("function.name").String(); name != "" {
				call.Name = name
			}
			if arguments := toolCall.Get("function.arguments").String(); arguments != "" {
				call.Arguments.WriteString(arguments)
			}
			return true
		})
		return true
	})
}

func (a *chatSSEAccumulator) completion() map[string]any {
	message := map[string]any{
		"role": "assistant",
	}
	if content := a.content.String(); content != "" {
		message["content"] = content
	}
	if reasoning := a.reasoning.String(); reasoning != "" {
		message["reasoning_content"] = reasoning
	}
	if len(a.toolCalls) > 0 {
		calls := make([]map[string]any, 0, len(a.toolCalls))
		for index := 0; index < len(a.toolCalls); index++ {
			call := a.toolCalls[index]
			if call == nil {
				continue
			}
			callID := call.ID
			if callID == "" {
				callID = fmt.Sprintf("call_%d", index)
			}
			callType := call.Type
			if callType == "" {
				callType = "function"
			}
			calls = append(calls, map[string]any{
				"id":   callID,
				"type": callType,
				"function": map[string]any{
					"name":      call.Name,
					"arguments": canonicalJSONText(call.Arguments.String()),
				},
			})
		}
		message["tool_calls"] = calls
	}
	usage := a.usage
	if len(usage) == 0 {
		usage = map[string]any{
			"prompt_tokens":     0,
			"completion_tokens": 0,
			"total_tokens":      0,
		}
	}
	return map[string]any{
		"id":      a.id,
		"created": a.created,
		"model":   a.model,
		"choices": []map[string]any{
			{
				"index":         0,
				"message":       message,
				"finish_reason": a.finishReason,
			},
		},
		"usage": usage,
	}
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
	return firstNonEmpty(
		gjson.GetBytes(body, "choices.0.message.reasoning_content").String(),
		chatReasoningText(gjson.GetBytes(body, "choices.0.message.reasoning")),
		chatReasoningText(gjson.GetBytes(body, "choices.0.message.reasoning_details")),
	)
}

func splitLeadingThinkBlock(text string) (string, string, bool) {
	leadingWhitespaceLen := len(text) - len(strings.TrimLeft(text, " \t\r\n"))
	afterWhitespace := text[leadingWhitespaceLen:]
	if !strings.HasPrefix(afterWhitespace, thinkOpenTag) {
		return "", text, false
	}

	bodyStart := leadingWhitespaceLen + len(thinkOpenTag)
	closeRelative := strings.Index(text[bodyStart:], thinkCloseTag)
	if closeRelative < 0 {
		return "", text, false
	}
	closeStart := bodyStart + closeRelative
	answerStart := closeStart + len(thinkCloseTag)
	return strings.TrimSpace(text[bodyStart:closeStart]), strings.TrimLeft(text[answerStart:], " \t\r\n"), true
}

func deepSeekMessageToResponsesOutput(responseID string, text string, refusal string) map[string]any {
	content := make([]map[string]any, 0, 2)
	if strings.TrimSpace(text) != "" {
		content = append(content, map[string]any{
			"type": "output_text",
			"text": text,
		})
	}
	if strings.TrimSpace(refusal) != "" {
		content = append(content, map[string]any{
			"type":    "refusal",
			"refusal": refusal,
		})
	}
	if len(content) == 0 {
		content = append(content, map[string]any{
			"type": "output_text",
			"text": text,
		})
	}
	return map[string]any{
		"type":    "message",
		"id":      "msg_" + fallbackResponseID(responseID),
		"status":  "completed",
		"role":    "assistant",
		"content": content,
	}
}

func deepSeekToolCallsToResponsesOutput(body []byte, reasoning string) []map[string]any {
	toolCalls := gjson.GetBytes(body, "choices.0.message.tool_calls")
	if !toolCalls.IsArray() {
		return nil
	}
	output := make([]map[string]any, 0)
	index := 0
	toolCalls.ForEach(func(_, toolCall gjson.Result) bool {
		callID := toolCall.Get("id").String()
		if strings.TrimSpace(callID) == "" {
			callID = fmt.Sprintf("call_%d", index)
		}
		item := map[string]any{
			"type":      "function_call",
			"id":        responsesFunctionCallItemID(callID),
			"call_id":   callID,
			"name":      toolCall.Get("function.name").String(),
			"arguments": canonicalJSONText(toolCall.Get("function.arguments").String()),
			"status":    "completed",
		}
		if strings.TrimSpace(reasoning) != "" {
			item["reasoning_content"] = reasoning
		}
		output = append(output, item)
		index++
		return true
	})
	if len(output) == 0 {
		return nil
	}
	return output
}

func responsesFunctionCallItemID(callID string) string {
	return "fc_" + strings.TrimSpace(callID)
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
	outputCallIDs := make(map[string]bool)
	for _, item := range items {
		switch itemType(item) {
		case "function_call":
			if callID := itemCallID(item); callID != "" {
				existingCallIDs[callID] = true
			}
		case "function_call_output":
			if callID := itemCallID(item); callID != "" {
				outputCallIDs[callID] = true
			}
		}
	}

	restoreGroup := make([]codexChatCachedFunctionCall, 0, len(calls))
	restoreGroupIDs := make(map[string]bool)
	for _, call := range calls {
		if outputCallIDs[call.CallID] && !existingCallIDs[call.CallID] {
			restoreGroup = append(restoreGroup, call)
			restoreGroupIDs[call.CallID] = true
		}
	}

	changed := false
	restoredGroup := false
	enriched := make([]any, 0, len(items)+len(calls))
	for _, item := range items {
		if itemType(item) == "function_call_output" {
			if !restoredGroup && len(restoreGroup) > 0 {
				for _, call := range restoreGroup {
					enriched = append(enriched, call.toResponsesInputItem())
					existingCallIDs[call.CallID] = true
				}
				restoredGroup = true
				changed = true
			}

			callID := itemCallID(item)
			if call, ok := callsByID[callID]; ok && !existingCallIDs[callID] && !restoreGroupIDs[callID] {
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
	var fallback []byte
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
				if value.Get("type").String() == "response.completed" || nested.Get("status").String() == "completed" {
					return []byte(nested.Raw)
				}
				if fallback == nil {
					fallback = []byte(nested.Raw)
				}
				continue
			}
			if value.Get("object").String() == "response" {
				if value.Get("status").String() == "completed" {
					return []byte(data)
				}
				if fallback == nil {
					fallback = []byte(data)
				}
			}
		}
	}
	if fallback != nil {
		return fallback
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
		normalizedBody := chatErrorToResponsesError(body)
		resp.Header.Set("Content-Type", "application/json")
		stripRebuiltBodyHeaders(resp.Header)
		if requestLog.RawLog != nil {
			requestLog.RawLog.captureResponseHeaders(resp.Header)
			requestLog.RawLog.captureResponseBody(normalizedBody)
			requestLog.RawLog.captureUpstreamResponseBody(body)
		}
		return false, newUpstreamResponseError(status, resp.Header, normalizedBody)
	}

	defer resp.Body.Close()
	if isStream && strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		c.Header("Content-Type", "text/event-stream")
		var upstreamBody bytes.Buffer
		translatedResponse, err := streamDeepSeekChatSSEToResponses(io.TeeReader(resp.Body, &upstreamBody), c.Writer)
		if requestLog.RawLog != nil {
			requestLog.RawLog.captureUpstreamResponseBody(upstreamBody.Bytes())
			requestLog.RawLog.captureResponseHeaders(c.Writer.Header())
			requestLog.RawLog.captureResponseBody(translatedResponse)
		}
		if err != nil {
			return false, err
		}
		if prs.codexChatHistory != nil {
			prs.codexChatHistory.recordResponsePayload(translatedResponse, true)
		}
		parseEventPayload(string(translatedResponse), CodexParseTokenUsageFromResponse, requestLog)
		return true, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}
	if requestLog.RawLog != nil {
		requestLog.RawLog.captureUpstreamResponseBody(body)
	}
	var translatedResponse []byte
	translatedResponse, err = translateDeepSeekChatCompletionToResponses(body, isStream)
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

func chatErrorToResponsesError(body []byte) []byte {
	parsed := gjson.ParseBytes(body)
	message := firstNonEmpty(
		parsed.Get("error.message").String(),
		parsed.Get("message").String(),
		parsed.Get("detail").String(),
		parsed.Get("status_msg").String(),
		parsed.Get("base_resp.status_msg").String(),
	)
	if strings.TrimSpace(message) == "" {
		message = strings.TrimSpace(string(body))
	}
	if message == "" {
		message = "Upstream returned an empty error response"
	}

	errorType := firstNonEmpty(parsed.Get("error.type").String(), parsed.Get("type").String(), "upstream_error")
	code := rawJSONValue(firstExistingJSONResult(
		parsed.Get("error.code"),
		parsed.Get("code"),
		parsed.Get("status_code"),
		parsed.Get("base_resp.status_code"),
	))
	param := rawJSONValue(firstExistingJSONResult(parsed.Get("error.param"), parsed.Get("param")))

	payload := map[string]any{
		"error": map[string]any{
			"message": message,
			"type":    errorType,
			"code":    code,
			"param":   param,
		},
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return []byte(`{"error":{"message":"Upstream error","type":"upstream_error","code":null,"param":null}}`)
	}
	return encoded
}

func stripRebuiltBodyHeaders(headers http.Header) {
	for _, key := range []string{
		"Content-Encoding",
		"Content-Length",
		"Content-Range",
		"Transfer-Encoding",
		"Trailer",
	} {
		headers.Del(key)
	}
}

func firstExistingJSONResult(values ...gjson.Result) gjson.Result {
	for _, value := range values {
		if value.Exists() {
			return value
		}
	}
	return gjson.Result{Type: gjson.Null, Raw: "null"}
}
