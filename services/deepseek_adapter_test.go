package services

import (
	"bufio"
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/daodao97/xgo/xdb"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

func TestTranslateResponsesRequestToDeepSeekChatCompletion(t *testing.T) {
	body := []byte(`{
		"model": "gpt-5",
		"instructions": "You are helpful.",
		"input": "hello",
		"temperature": 0.2,
		"max_output_tokens": 128,
		"stream": false
	}`)

	got, err := translateResponsesRequestToDeepSeekChatCompletion(body)
	if err != nil {
		t.Fatalf("translate request: %v", err)
	}

	if gotModel := gjson.GetBytes(got, "model").String(); gotModel != "gpt-5" {
		t.Fatalf("model = %q, want %q", gotModel, "gpt-5")
	}
	if gotStream := gjson.GetBytes(got, "stream").Bool(); gotStream {
		t.Fatalf("stream = %v, want false", gotStream)
	}
	if gotRole := gjson.GetBytes(got, "messages.0.role").String(); gotRole != "system" {
		t.Fatalf("system role = %q, want %q", gotRole, "system")
	}
	if gotContent := gjson.GetBytes(got, "messages.0.content").String(); gotContent != "You are helpful." {
		t.Fatalf("system content = %q, want %q", gotContent, "You are helpful.")
	}
	if gotRole := gjson.GetBytes(got, "messages.1.role").String(); gotRole != "user" {
		t.Fatalf("user role = %q, want %q", gotRole, "user")
	}
	if gotContent := gjson.GetBytes(got, "messages.1.content").String(); gotContent != "hello" {
		t.Fatalf("user content = %q, want %q", gotContent, "hello")
	}
	if gotMaxTokens := gjson.GetBytes(got, "max_tokens").Int(); gotMaxTokens != 128 {
		t.Fatalf("max_tokens = %d, want 128", gotMaxTokens)
	}
}

func TestTranslateResponsesRequestPropagatesDeepSeekStream(t *testing.T) {
	body := []byte(`{
		"model": "gpt-5",
		"input": "hello",
		"stream": true
	}`)

	got, err := translateResponsesRequestToDeepSeekChatCompletion(body)
	if err != nil {
		t.Fatalf("translate request: %v", err)
	}
	if gotStream := gjson.GetBytes(got, "stream").Bool(); !gotStream {
		t.Fatalf("upstream stream = %v, want true", gotStream)
	}
}

func TestTranslateResponsesRequestPassesChatCompatibleFields(t *testing.T) {
	body := []byte(`{
		"model": "gpt-5",
		"input": "hello",
		"top_p": 0.8,
		"frequency_penalty": 0.2,
		"presence_penalty": 0.3,
		"logit_bias": {"100": -1},
		"logprobs": true,
		"top_logprobs": 2,
		"n": 1,
		"stop": ["END"],
		"metadata": {"trace_id": "trace-1"},
		"user": "user-1",
		"parallel_tool_calls": false,
		"response_format": {"type": "json_object"},
		"seed": 123,
		"service_tier": "auto",
		"stream_options": {"include_usage": true}
	}`)

	got, err := translateResponsesRequestToDeepSeekChatCompletion(body)
	if err != nil {
		t.Fatalf("translate request: %v", err)
	}

	if gotTopP := gjson.GetBytes(got, "top_p").Float(); gotTopP != 0.8 {
		t.Fatalf("top_p = %v, want 0.8; body=%s", gotTopP, string(got))
	}
	if gotPenalty := gjson.GetBytes(got, "frequency_penalty").Float(); gotPenalty != 0.2 {
		t.Fatalf("frequency_penalty = %v, want 0.2; body=%s", gotPenalty, string(got))
	}
	if gotPenalty := gjson.GetBytes(got, "presence_penalty").Float(); gotPenalty != 0.3 {
		t.Fatalf("presence_penalty = %v, want 0.3; body=%s", gotPenalty, string(got))
	}
	if gotBias := gjson.GetBytes(got, "logit_bias.100").Int(); gotBias != -1 {
		t.Fatalf("logit_bias.100 = %d, want -1; body=%s", gotBias, string(got))
	}
	if gotLogprobs := gjson.GetBytes(got, "logprobs").Bool(); !gotLogprobs {
		t.Fatalf("logprobs = false, want true; body=%s", string(got))
	}
	if gotTopLogprobs := gjson.GetBytes(got, "top_logprobs").Int(); gotTopLogprobs != 2 {
		t.Fatalf("top_logprobs = %d, want 2; body=%s", gotTopLogprobs, string(got))
	}
	if gotN := gjson.GetBytes(got, "n").Int(); gotN != 1 {
		t.Fatalf("n = %d, want 1; body=%s", gotN, string(got))
	}
	if gotStop := gjson.GetBytes(got, "stop.0").String(); gotStop != "END" {
		t.Fatalf("stop.0 = %q, want END; body=%s", gotStop, string(got))
	}
	if gotTrace := gjson.GetBytes(got, "metadata.trace_id").String(); gotTrace != "trace-1" {
		t.Fatalf("metadata.trace_id = %q, want trace-1; body=%s", gotTrace, string(got))
	}
	if gotUser := gjson.GetBytes(got, "user").String(); gotUser != "user-1" {
		t.Fatalf("user = %q, want user-1; body=%s", gotUser, string(got))
	}
	if gotParallel := gjson.GetBytes(got, "parallel_tool_calls").Bool(); gotParallel {
		t.Fatalf("parallel_tool_calls = true, want false; body=%s", string(got))
	}
	if gotFormat := gjson.GetBytes(got, "response_format.type").String(); gotFormat != "json_object" {
		t.Fatalf("response_format.type = %q, want json_object; body=%s", gotFormat, string(got))
	}
	if gotSeed := gjson.GetBytes(got, "seed").Int(); gotSeed != 123 {
		t.Fatalf("seed = %d, want 123; body=%s", gotSeed, string(got))
	}
	if gotTier := gjson.GetBytes(got, "service_tier").String(); gotTier != "auto" {
		t.Fatalf("service_tier = %q, want auto; body=%s", gotTier, string(got))
	}
	if gotUsage := gjson.GetBytes(got, "stream_options.include_usage").Bool(); !gotUsage {
		t.Fatalf("stream_options.include_usage = false, want true; body=%s", string(got))
	}
}

func TestTranslateResponsesRequestInjectsStreamUsageOption(t *testing.T) {
	body := []byte(`{
		"model": "gpt-5",
		"input": "hello",
		"stream": true
	}`)

	got, err := translateResponsesRequestToDeepSeekChatCompletion(body)
	if err != nil {
		t.Fatalf("translate request: %v", err)
	}
	if gotUsage := gjson.GetBytes(got, "stream_options.include_usage").Bool(); !gotUsage {
		t.Fatalf("stream_options.include_usage = false, want true; body=%s", string(got))
	}
}

func TestTranslateResponsesDeveloperRoleToDeepSeekSystemRole(t *testing.T) {
	body := []byte(`{
		"model": "gpt-5",
		"input": [
			{
				"type": "message",
				"role": "developer",
				"content": [{"type": "input_text", "text": "You are running inside Codex."}]
			},
			{
				"type": "message",
				"role": "user",
				"content": [{"type": "input_text", "text": "hello"}]
			}
		]
	}`)

	got, err := translateResponsesRequestToDeepSeekChatCompletion(body)
	if err != nil {
		t.Fatalf("translate request: %v", err)
	}

	if gotRole := gjson.GetBytes(got, "messages.0.role").String(); gotRole != "system" {
		t.Fatalf("developer role mapped to %q, want system; body=%s", gotRole, string(got))
	}
	if gotContent := gjson.GetBytes(got, "messages.0.content").String(); gotContent != "You are running inside Codex." {
		t.Fatalf("developer content = %q, want original content; body=%s", gotContent, string(got))
	}
	if gotRole := gjson.GetBytes(got, "messages.1.role").String(); gotRole != "user" {
		t.Fatalf("user role = %q, want user; body=%s", gotRole, string(got))
	}
}

func TestTranslateResponsesSystemMessagesCollapseToHead(t *testing.T) {
	body := []byte(`{
		"model": "gpt-5",
		"instructions": "root instructions",
		"input": [
			{"type": "message", "role": "user", "content": [{"type": "input_text", "text": "first"}]},
			{"type": "message", "role": "developer", "content": [{"type": "input_text", "text": "developer rules"}]},
			{"type": "message", "role": "user", "content": [{"type": "input_text", "text": "second"}]}
		]
	}`)

	got, err := translateResponsesRequestToDeepSeekChatCompletion(body)
	if err != nil {
		t.Fatalf("translate request: %v", err)
	}
	if gotRole := gjson.GetBytes(got, "messages.0.role").String(); gotRole != "system" {
		t.Fatalf("messages.0.role = %q, want system; body=%s", gotRole, string(got))
	}
	gotSystem := gjson.GetBytes(got, "messages.0.content").String()
	if !strings.Contains(gotSystem, "root instructions") || !strings.Contains(gotSystem, "developer rules") {
		t.Fatalf("collapsed system content missing parts: %q; body=%s", gotSystem, string(got))
	}
	if gotRole := gjson.GetBytes(got, "messages.1.role").String(); gotRole != "user" {
		t.Fatalf("messages.1.role = %q, want user; body=%s", gotRole, string(got))
	}
	if gotRole := gjson.GetBytes(got, "messages.2.role").String(); gotRole != "user" {
		t.Fatalf("messages.2.role = %q, want user; body=%s", gotRole, string(got))
	}
	if gotSystemRole := gjson.GetBytes(got, "messages.3.role").String(); gotSystemRole != "" {
		t.Fatalf("unexpected extra message role %q; body=%s", gotSystemRole, string(got))
	}
}

func TestTranslateResponsesLatestReminderRoleToDeepSeekUserRole(t *testing.T) {
	body := []byte(`{
		"model": "gpt-5",
		"input": {
			"type": "message",
			"role": "latest_reminder",
			"content": [{"type": "input_text", "text": "remember this"}]
		}
	}`)

	got, err := translateResponsesRequestToDeepSeekChatCompletion(body)
	if err != nil {
		t.Fatalf("translate request: %v", err)
	}
	if gotRole := gjson.GetBytes(got, "messages.0.role").String(); gotRole != "user" {
		t.Fatalf("latest_reminder role mapped to %q, want user; body=%s", gotRole, string(got))
	}
}

func TestTranslateResponsesUnknownRoleToDeepSeekUserRole(t *testing.T) {
	body := []byte(`{
		"model": "gpt-5",
		"input": {
			"type": "message",
			"role": "unknown_codex_role",
			"content": "fallback content"
		}
	}`)

	got, err := translateResponsesRequestToDeepSeekChatCompletion(body)
	if err != nil {
		t.Fatalf("translate request: %v", err)
	}
	if gotRole := gjson.GetBytes(got, "messages.0.role").String(); gotRole != "user" {
		t.Fatalf("unknown role mapped to %q, want user; body=%s", gotRole, string(got))
	}
}

func TestTranslateResponsesMultimodalContentToDeepSeekChatParts(t *testing.T) {
	body := []byte(`{
		"model": "gpt-5",
		"input": {
			"type": "message",
			"role": "user",
			"content": [
				{"type": "input_text", "text": "describe"},
				{"type": "input_image", "image_url": "https://example.com/cat.png"}
			]
		}
	}`)

	got, err := translateResponsesRequestToDeepSeekChatCompletion(body)
	if err != nil {
		t.Fatalf("translate request: %v", err)
	}
	if gotType := gjson.GetBytes(got, "messages.0.content.0.type").String(); gotType != "text" {
		t.Fatalf("content.0.type = %q, want text; body=%s", gotType, string(got))
	}
	if gotText := gjson.GetBytes(got, "messages.0.content.0.text").String(); gotText != "describe" {
		t.Fatalf("content.0.text = %q, want describe; body=%s", gotText, string(got))
	}
	if gotType := gjson.GetBytes(got, "messages.0.content.1.type").String(); gotType != "image_url" {
		t.Fatalf("content.1.type = %q, want image_url; body=%s", gotType, string(got))
	}
	if gotURL := gjson.GetBytes(got, "messages.0.content.1.image_url.url").String(); gotURL != "https://example.com/cat.png" {
		t.Fatalf("image_url.url = %q, want image URL; body=%s", gotURL, string(got))
	}
}

func TestTranslateResponsesUnsupportedContentPartIsNotDropped(t *testing.T) {
	body := []byte(`{
		"model": "gpt-5",
		"input": {
			"type": "message",
			"role": "user",
			"content": [
				{"type": "input_file", "file_id": "file_123"}
			]
		}
	}`)

	got, err := translateResponsesRequestToDeepSeekChatCompletion(body)
	if err != nil {
		t.Fatalf("translate request: %v", err)
	}
	if gotContent := gjson.GetBytes(got, "messages.0.content").String(); !strings.Contains(gotContent, "file_123") {
		t.Fatalf("unsupported content part was dropped; content=%q body=%s", gotContent, string(got))
	}
}

func TestTranslateResponsesReasoningItemAttachesToToolCall(t *testing.T) {
	body := []byte(`{
		"model": "gpt-5",
		"input": [
			{"type": "function_call", "call_id": "call_1", "name": "shell", "arguments": "{\"cmd\":\"pwd\"}"},
			{"type": "reasoning", "summary": [{"type": "summary_text", "text": "Need shell"}]},
			{"type": "function_call_output", "call_id": "call_1", "output": {"stdout": "/tmp"}}
		]
	}`)

	got, err := translateResponsesRequestToDeepSeekChatCompletion(body)
	if err != nil {
		t.Fatalf("translate request: %v", err)
	}
	if gotReasoning := gjson.GetBytes(got, "messages.0.reasoning_content").String(); gotReasoning != "Need shell" {
		t.Fatalf("reasoning_content = %q, want Need shell; body=%s", gotReasoning, string(got))
	}
	if gotArgs := gjson.GetBytes(got, "messages.0.tool_calls.0.function.arguments").String(); gotArgs != `{"cmd":"pwd"}` {
		t.Fatalf("function arguments = %q, want canonical JSON; body=%s", gotArgs, string(got))
	}
	if gotOutput := gjson.GetBytes(got, "messages.1.content").String(); gotOutput != `{"stdout":"/tmp"}` {
		t.Fatalf("tool output = %q, want canonical JSON; body=%s", gotOutput, string(got))
	}
}

func TestTranslateResponsesToolCallAssistantIncludesNullContent(t *testing.T) {
	body := []byte(`{
		"model": "gpt-5",
		"input": [
			{"type": "function_call", "call_id": "call_1", "name": "shell", "arguments": "{\"cmd\":\"pwd\"}"}
		]
	}`)

	got, err := translateResponsesRequestToDeepSeekChatCompletion(body)
	if err != nil {
		t.Fatalf("translate request: %v", err)
	}
	content := gjson.GetBytes(got, "messages.0.content")
	if !content.Exists() || content.Type != gjson.Null {
		t.Fatalf("assistant tool-call content = %s (%v), want explicit null; body=%s", content.Raw, content.Type, string(got))
	}
}

func TestTranslateResponsesFunctionToolsToDeepSeekChatTools(t *testing.T) {
	body := []byte(`{
		"model": "gpt-5",
		"input": [
			{
				"type": "message",
				"role": "user",
				"content": [{"type": "input_text", "text": "run pwd"}]
			},
			{
				"type": "function_call_output",
				"call_id": "call_123",
				"output": "done"
			}
		],
		"tools": [
			{
				"type": "function",
				"name": "shell",
				"description": "Run a shell command",
				"parameters": {"type": "object"}
			}
		],
		"tool_choice": "auto"
	}`)

	got, err := translateResponsesRequestToDeepSeekChatCompletion(body)
	if err != nil {
		t.Fatalf("translate request: %v", err)
	}

	if gotName := gjson.GetBytes(got, "tools.0.function.name").String(); gotName != "shell" {
		t.Fatalf("tool name = %q, want %q, body=%s", gotName, "shell", string(got))
	}
	if gotChoice := gjson.GetBytes(got, "tool_choice").String(); gotChoice != "auto" {
		t.Fatalf("tool_choice = %q, want %q, body=%s", gotChoice, "auto", string(got))
	}
	if gotRole := gjson.GetBytes(got, "messages.1.role").String(); gotRole != "tool" {
		t.Fatalf("tool message role = %q, want %q, body=%s", gotRole, "tool", string(got))
	}
	if gotCallID := gjson.GetBytes(got, "messages.1.tool_call_id").String(); gotCallID != "call_123" {
		t.Fatalf("tool_call_id = %q, want %q, body=%s", gotCallID, "call_123", string(got))
	}
}

func TestTranslateResponsesFunctionToolStrictMovesIntoFunction(t *testing.T) {
	body := []byte(`{
		"model": "gpt-5",
		"input": "run pwd",
		"tools": [
			{
				"type": "function",
				"name": "shell",
				"description": "Run a shell command",
				"parameters": {"type": "object"},
				"strict": true
			}
		]
	}`)

	got, err := translateResponsesRequestToDeepSeekChatCompletion(body)
	if err != nil {
		t.Fatalf("translate request: %v", err)
	}

	if gotStrict := gjson.GetBytes(got, "tools.0.function.strict").Bool(); !gotStrict {
		t.Fatalf("tools.0.function.strict = false, want true; body=%s", string(got))
	}
	if gjson.GetBytes(got, "tools.0.strict").Exists() {
		t.Fatalf("top-level tool strict should not be present in Chat format; body=%s", string(got))
	}
}

func TestTranslateResponsesFunctionToolChoiceObjectToDeepSeekChatToolChoice(t *testing.T) {
	body := []byte(`{
		"model": "gpt-5",
		"input": "run pwd",
		"tools": [
			{
				"type": "function",
				"name": "shell",
				"description": "Run a shell command",
				"parameters": {"type": "object"}
			}
		],
		"tool_choice": {"type": "function", "name": "shell"}
	}`)

	got, err := translateResponsesRequestToDeepSeekChatCompletion(body)
	if err != nil {
		t.Fatalf("translate request: %v", err)
	}

	if gotType := gjson.GetBytes(got, "tool_choice.type").String(); gotType != "function" {
		t.Fatalf("tool_choice.type = %q, want function; body=%s", gotType, string(got))
	}
	if gotName := gjson.GetBytes(got, "tool_choice.function.name").String(); gotName != "shell" {
		t.Fatalf("tool_choice.function.name = %q, want shell; body=%s", gotName, string(got))
	}
	if gjson.GetBytes(got, "tool_choice.name").Exists() {
		t.Fatalf("tool_choice.name should not be present in Chat Completions format; body=%s", string(got))
	}
}

func TestTranslateResponsesObjectContentToDeepSeekMessage(t *testing.T) {
	body := []byte(`{
		"model": "gpt-5",
		"input": {
			"type": "message",
			"role": "user",
			"content": {"type": "input_text", "text": "hello object"}
		}
	}`)

	got, err := translateResponsesRequestToDeepSeekChatCompletion(body)
	if err != nil {
		t.Fatalf("translate request: %v", err)
	}

	if gotRole := gjson.GetBytes(got, "messages.0.role").String(); gotRole != "user" {
		t.Fatalf("role = %q, want user; body=%s", gotRole, string(got))
	}
	if gotText := gjson.GetBytes(got, "messages.0.content").String(); gotText != "hello object" {
		t.Fatalf("content = %q, want hello object; body=%s", gotText, string(got))
	}
}

func TestTranslateResponsesMergesConsecutiveFunctionCalls(t *testing.T) {
	body := []byte(`{
		"model": "gpt-5",
		"input": [
			{
				"type": "function_call",
				"call_id": "call_1",
				"name": "read_file",
				"arguments": "{\"path\":\"README.md\"}"
			},
			{
				"type": "function_call",
				"call_id": "call_2",
				"name": "list_dir",
				"arguments": "{\"path\":\".\"}"
			},
			{
				"type": "function_call_output",
				"call_id": "call_1",
				"output": "read ok"
			},
			{
				"type": "function_call_output",
				"call_id": "call_2",
				"output": "list ok"
			}
		]
	}`)

	got, err := translateResponsesRequestToDeepSeekChatCompletion(body)
	if err != nil {
		t.Fatalf("translate request: %v", err)
	}

	if gotLen := len(gjson.GetBytes(got, "messages").Array()); gotLen != 3 {
		t.Fatalf("messages length = %d, want 3; body=%s", gotLen, string(got))
	}
	if gotRole := gjson.GetBytes(got, "messages.0.role").String(); gotRole != "assistant" {
		t.Fatalf("messages.0.role = %q, want assistant; body=%s", gotRole, string(got))
	}
	if gotCalls := len(gjson.GetBytes(got, "messages.0.tool_calls").Array()); gotCalls != 2 {
		t.Fatalf("tool_calls length = %d, want 2; body=%s", gotCalls, string(got))
	}
	if gotReasoning := gjson.GetBytes(got, "messages.0.reasoning_content").String(); strings.TrimSpace(gotReasoning) == "" {
		t.Fatalf("assistant tool call missing reasoning_content placeholder; body=%s", string(got))
	}
	if gotRole := gjson.GetBytes(got, "messages.1.role").String(); gotRole != "tool" {
		t.Fatalf("messages.1.role = %q, want tool; body=%s", gotRole, string(got))
	}
}

func TestTranslateResponsesMergesReasoningFromConsecutiveFunctionCalls(t *testing.T) {
	body := []byte(`{
		"model": "gpt-5",
		"input": [
			{
				"type": "function_call",
				"call_id": "call_1",
				"name": "read_file",
				"arguments": "{\"path\":\"README.md\"}",
				"reasoning_content": "Read file first."
			},
			{
				"type": "function_call",
				"call_id": "call_2",
				"name": "list_dir",
				"arguments": "{\"path\":\".\"}",
				"reasoning_content": "Then list directory."
			}
		]
	}`)

	got, err := translateResponsesRequestToDeepSeekChatCompletion(body)
	if err != nil {
		t.Fatalf("translate request: %v", err)
	}

	gotReasoning := gjson.GetBytes(got, "messages.0.reasoning_content").String()
	if !strings.Contains(gotReasoning, "Read file first.") || !strings.Contains(gotReasoning, "Then list directory.") {
		t.Fatalf("merged reasoning_content = %q, want both tool call reasoning entries; body=%s", gotReasoning, string(got))
	}
}

func TestTranslateResponsesReasoningEffortToDeepSeekThinking(t *testing.T) {
	body := []byte(`{
		"model": "deepseek-v4-pro",
		"input": "think carefully",
		"reasoning": {"effort": "max"}
	}`)

	got, err := translateResponsesRequestToDeepSeekChatCompletion(body)
	if err != nil {
		t.Fatalf("translate request: %v", err)
	}

	if gotThinking := gjson.GetBytes(got, "thinking.type").String(); gotThinking != "enabled" {
		t.Fatalf("thinking.type = %q, want enabled; body=%s", gotThinking, string(got))
	}
	if gotEffort := gjson.GetBytes(got, "reasoning_effort").String(); gotEffort != "max" {
		t.Fatalf("reasoning_effort = %q, want max; body=%s", gotEffort, string(got))
	}
}

func TestTranslateDeepSeekChatCompletionToResponses(t *testing.T) {
	body := []byte(`{
		"id": "chatcmpl-123",
		"model": "deepseek-chat",
		"choices": [
			{
				"index": 0,
				"message": {"role": "assistant", "content": "hello"},
				"finish_reason": "stop"
			}
		],
		"usage": {
			"prompt_tokens": 10,
			"completion_tokens": 4,
			"total_tokens": 14
		}
	}`)

	got, err := translateDeepSeekChatCompletionToResponses(body, false)
	if err != nil {
		t.Fatalf("translate response: %v", err)
	}

	if gotObject := gjson.GetBytes(got, "object").String(); gotObject != "response" {
		t.Fatalf("object = %q, want %q", gotObject, "response")
	}
	if gotStatus := gjson.GetBytes(got, "status").String(); gotStatus != "completed" {
		t.Fatalf("status = %q, want %q", gotStatus, "completed")
	}
	if gotText := gjson.GetBytes(got, "output_text").String(); gotText != "hello" {
		t.Fatalf("output_text = %q, want %q", gotText, "hello")
	}
	if gotText := gjson.GetBytes(got, "output.0.content.0.text").String(); gotText != "hello" {
		t.Fatalf("output content = %q, want %q", gotText, "hello")
	}
	if gotInputTokens := gjson.GetBytes(got, "usage.input_tokens").Int(); gotInputTokens != 10 {
		t.Fatalf("input_tokens = %d, want 10", gotInputTokens)
	}
	if gotOutputTokens := gjson.GetBytes(got, "usage.output_tokens").Int(); gotOutputTokens != 4 {
		t.Fatalf("output_tokens = %d, want 4", gotOutputTokens)
	}
}

func TestTranslateDeepSeekToolCallToResponsesFunctionCallOutput(t *testing.T) {
	body := []byte(`{
		"id": "chatcmpl-123",
		"model": "deepseek-chat",
		"choices": [
			{
				"index": 0,
				"message": {
					"role": "assistant",
					"tool_calls": [
						{
							"id": "call_123",
							"type": "function",
							"function": {
								"name": "shell",
								"arguments": "{\"cmd\":\"pwd\"}"
							}
						}
					]
				},
				"finish_reason": "tool_calls"
			}
		],
		"usage": {
			"prompt_tokens": 10,
			"completion_tokens": 4,
			"total_tokens": 14
		}
	}`)

	got, err := translateDeepSeekChatCompletionToResponses(body, false)
	if err != nil {
		t.Fatalf("translate response: %v", err)
	}

	if gotType := gjson.GetBytes(got, "output.0.type").String(); gotType != "function_call" {
		t.Fatalf("output type = %q, want %q, body=%s", gotType, "function_call", string(got))
	}
	if gotName := gjson.GetBytes(got, "output.0.name").String(); gotName != "shell" {
		t.Fatalf("function name = %q, want %q, body=%s", gotName, "shell", string(got))
	}
	if gotCallID := gjson.GetBytes(got, "output.0.call_id").String(); gotCallID != "call_123" {
		t.Fatalf("call_id = %q, want %q, body=%s", gotCallID, "call_123", string(got))
	}
	if gotID := gjson.GetBytes(got, "output.0.id").String(); gotID != "fc_call_123" {
		t.Fatalf("function_call id = %q, want %q, body=%s", gotID, "fc_call_123", string(got))
	}
	if gotArgs := gjson.GetBytes(got, "output.0.arguments").String(); gotArgs != `{"cmd":"pwd"}` {
		t.Fatalf("arguments = %q, want %q, body=%s", gotArgs, `{"cmd":"pwd"}`, string(got))
	}
}

func TestTranslateDeepSeekReasoningUsageToResponsesUsageDetails(t *testing.T) {
	body := []byte(`{
		"id": "chatcmpl-123",
		"model": "deepseek-reasoner",
		"choices": [
			{
				"index": 0,
				"message": {"role": "assistant", "content": "hello"},
				"finish_reason": "stop"
			}
		],
		"usage": {
			"prompt_tokens": 10,
			"completion_tokens": 6,
			"total_tokens": 16,
			"completion_tokens_details": {
				"reasoning_tokens": 2
			}
		}
	}`)

	got, err := translateDeepSeekChatCompletionToResponses(body, false)
	if err != nil {
		t.Fatalf("translate response: %v", err)
	}

	if gotReasoning := gjson.GetBytes(got, "usage.output_tokens_details.reasoning_tokens").Int(); gotReasoning != 2 {
		t.Fatalf("reasoning_tokens = %d, want 2, body=%s", gotReasoning, string(got))
	}
}

func TestTranslateDeepSeekResponsesStyleUsageFallback(t *testing.T) {
	body := []byte(`{
		"id": "chatcmpl-usage",
		"model": "deepseek-chat",
		"choices": [
			{
				"index": 0,
				"message": {"role": "assistant", "content": "hello"},
				"finish_reason": "stop"
			}
		],
		"usage": {
			"input_tokens": 11,
			"output_tokens": 5,
			"total_tokens": 16
		}
	}`)

	got, err := translateDeepSeekChatCompletionToResponses(body, false)
	if err != nil {
		t.Fatalf("translate response: %v", err)
	}

	if gotInput := gjson.GetBytes(got, "usage.input_tokens").Int(); gotInput != 11 {
		t.Fatalf("usage.input_tokens = %d, want 11; body=%s", gotInput, string(got))
	}
	if gotOutput := gjson.GetBytes(got, "usage.output_tokens").Int(); gotOutput != 5 {
		t.Fatalf("usage.output_tokens = %d, want 5; body=%s", gotOutput, string(got))
	}
}

func TestTranslateDeepSeekReasoningDetailsToResponsesReasoning(t *testing.T) {
	body := []byte(`{
		"id": "chatcmpl-details",
		"model": "deepseek-chat",
		"choices": [
			{
				"index": 0,
				"message": {
					"role": "assistant",
					"reasoning_details": [
						{"type": "reasoning_text", "text": "Need to inspect."}
					],
					"content": "done"
				},
				"finish_reason": "stop"
			}
		]
	}`)

	got, err := translateDeepSeekChatCompletionToResponses(body, false)
	if err != nil {
		t.Fatalf("translate response: %v", err)
	}

	if gotType := gjson.GetBytes(got, "output.0.type").String(); gotType != "reasoning" {
		t.Fatalf("output.0.type = %q, want reasoning; body=%s", gotType, string(got))
	}
	if gotReasoning := gjson.GetBytes(got, "output.0.summary.0.text").String(); gotReasoning != "Need to inspect." {
		t.Fatalf("reasoning details text = %q, want preserved reasoning; body=%s", gotReasoning, string(got))
	}
}

func TestTranslateDeepSeekLeadingThinkBlockToReasoningAndAnswer(t *testing.T) {
	body := []byte(`{
		"id": "chatcmpl-think",
		"model": "deepseek-chat",
		"choices": [
			{
				"index": 0,
				"message": {
					"role": "assistant",
					"content": "<think>Need context.</think>\n\nFinal answer."
				},
				"finish_reason": "stop"
			}
		]
	}`)

	got, err := translateDeepSeekChatCompletionToResponses(body, false)
	if err != nil {
		t.Fatalf("translate response: %v", err)
	}

	if gotReasoning := gjson.GetBytes(got, "output.0.summary.0.text").String(); gotReasoning != "Need context." {
		t.Fatalf("reasoning = %q, want split think block; body=%s", gotReasoning, string(got))
	}
	if gotText := gjson.GetBytes(got, "output.1.content.0.text").String(); gotText != "Final answer." {
		t.Fatalf("answer text = %q, want final answer without think block; body=%s", gotText, string(got))
	}
	if gotOutputText := gjson.GetBytes(got, "output_text").String(); gotOutputText != "Final answer." {
		t.Fatalf("output_text = %q, want final answer without think block; body=%s", gotOutputText, string(got))
	}
}

func TestTranslateDeepSeekChatCompletionToResponsesPreservesStatusCreatedRefusalAndUsageDetails(t *testing.T) {
	body := []byte(`{
		"id": "chatcmpl-refusal",
		"created": 1700000000,
		"model": "deepseek-chat",
		"choices": [
			{
				"index": 0,
				"message": {
					"role": "assistant",
					"content": null,
					"refusal": "I cannot comply."
				},
				"finish_reason": "length"
			}
		],
		"usage": {
			"prompt_tokens": 10,
			"completion_tokens": 4,
			"total_tokens": 14,
			"prompt_tokens_details": {
				"cached_tokens": 7
			}
		}
	}`)

	got, err := translateDeepSeekChatCompletionToResponses(body, false)
	if err != nil {
		t.Fatalf("translate response: %v", err)
	}

	if gotCreated := gjson.GetBytes(got, "created_at").Int(); gotCreated != 1700000000 {
		t.Fatalf("created_at = %d, want 1700000000; body=%s", gotCreated, string(got))
	}
	if gotStatus := gjson.GetBytes(got, "status").String(); gotStatus != "incomplete" {
		t.Fatalf("status = %q, want incomplete; body=%s", gotStatus, string(got))
	}
	if gotReason := gjson.GetBytes(got, "incomplete_details.reason").String(); gotReason != "max_output_tokens" {
		t.Fatalf("incomplete_details.reason = %q, want max_output_tokens; body=%s", gotReason, string(got))
	}
	if gotType := gjson.GetBytes(got, "output.0.content.0.type").String(); gotType != "refusal" {
		t.Fatalf("output refusal type = %q, want refusal; body=%s", gotType, string(got))
	}
	if gotRefusal := gjson.GetBytes(got, "output.0.content.0.refusal").String(); gotRefusal != "I cannot comply." {
		t.Fatalf("refusal = %q, want preserved refusal; body=%s", gotRefusal, string(got))
	}
	if gotCached := gjson.GetBytes(got, "usage.input_tokens_details.cached_tokens").Int(); gotCached != 7 {
		t.Fatalf("cached_tokens = %d, want 7; body=%s", gotCached, string(got))
	}
}

func TestTranslateDeepSeekReasoningAndToolCallsToResponses(t *testing.T) {
	body := []byte(`{
		"id": "chatcmpl-123",
		"model": "deepseek-reasoner",
		"choices": [
			{
				"index": 0,
				"message": {
					"role": "assistant",
					"reasoning_content": "Need to inspect the workspace.",
					"content": "I will run pwd.",
					"tool_calls": [
						{
							"id": "call_123",
							"type": "function",
							"function": {
								"name": "shell",
								"arguments": "{\"cmd\":\"pwd\"}"
							}
						}
					]
				},
				"finish_reason": "tool_calls"
			}
		],
		"usage": {
			"prompt_tokens": 10,
			"completion_tokens": 6,
			"total_tokens": 16,
			"completion_tokens_details": {
				"reasoning_tokens": 2
			}
		}
	}`)

	got, err := translateDeepSeekChatCompletionToResponses(body, false)
	if err != nil {
		t.Fatalf("translate response: %v", err)
	}

	if gotType := gjson.GetBytes(got, "output.0.type").String(); gotType != "reasoning" {
		t.Fatalf("output.0.type = %q, want reasoning; body=%s", gotType, string(got))
	}
	if gotReasoning := gjson.GetBytes(got, "output.0.summary.0.text").String(); gotReasoning != "Need to inspect the workspace." {
		t.Fatalf("reasoning text = %q, want preserved reasoning; body=%s", gotReasoning, string(got))
	}
	if gotType := gjson.GetBytes(got, "output.1.type").String(); gotType != "message" {
		t.Fatalf("output.1.type = %q, want message; body=%s", gotType, string(got))
	}
	if gotText := gjson.GetBytes(got, "output.1.content.0.text").String(); gotText != "I will run pwd." {
		t.Fatalf("message text = %q, want content; body=%s", gotText, string(got))
	}
	if gotType := gjson.GetBytes(got, "output.2.type").String(); gotType != "function_call" {
		t.Fatalf("output.2.type = %q, want function_call; body=%s", gotType, string(got))
	}
	if gotReasoning := gjson.GetBytes(got, "output.2.reasoning_content").String(); gotReasoning != "Need to inspect the workspace." {
		t.Fatalf("function_call reasoning_content = %q, want preserved reasoning; body=%s", gotReasoning, string(got))
	}
}

func TestTranslateDeepSeekChatCompletionToResponsesStream(t *testing.T) {
	body := []byte(`{
		"id": "chatcmpl-123",
		"model": "deepseek-chat",
		"choices": [
			{
				"index": 0,
				"message": {"role": "assistant", "content": "hello"},
				"finish_reason": "stop"
			}
		],
		"usage": {
			"prompt_tokens": 10,
			"completion_tokens": 4,
			"total_tokens": 14
		}
	}`)

	got, err := translateDeepSeekChatCompletionToResponses(body, true)
	if err != nil {
		t.Fatalf("translate stream response: %v", err)
	}
	if !strings.Contains(string(got), "event: response.completed") {
		t.Fatalf("stream response missing completed event: %s", string(got))
	}
	if !strings.Contains(string(got), "hello") {
		t.Fatalf("stream response missing text: %s", string(got))
	}
}

func TestTranslateDeepSeekChatCompletionToResponsesStreamIncludesLifecycleEvents(t *testing.T) {
	body := []byte(`{
		"id": "chatcmpl-123",
		"model": "deepseek-chat",
		"choices": [
			{
				"index": 0,
				"message": {"role": "assistant", "content": "hello"},
				"finish_reason": "stop"
			}
		],
		"usage": {
			"prompt_tokens": 10,
			"completion_tokens": 4,
			"total_tokens": 14
		}
	}`)

	got, err := translateDeepSeekChatCompletionToResponses(body, true)
	if err != nil {
		t.Fatalf("translate stream response: %v", err)
	}
	for _, event := range []string{
		"event: response.created",
		"event: response.in_progress",
		"event: response.output_item.added",
		"event: response.output_text.delta",
		"event: response.output_item.done",
		"event: response.completed",
	} {
		if !strings.Contains(string(got), event) {
			t.Fatalf("stream response missing %s: %s", event, string(got))
		}
	}
}

func TestTranslateDeepSeekChatSSEToResponsesSSE(t *testing.T) {
	body := []byte(strings.Join([]string{
		`data: {"id":"chatcmpl-sse","created":1700000000,"model":"deepseek-chat","choices":[{"index":0,"delta":{"role":"assistant","reasoning":"think "}}]}`,
		`data: {"id":"chatcmpl-sse","model":"deepseek-chat","choices":[{"index":0,"delta":{"content":"hello"}}]}`,
		`data: {"id":"chatcmpl-sse","model":"deepseek-chat","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"shell","arguments":"{\"cmd\""}}]}}]}`,
		`data: {"id":"chatcmpl-sse","model":"deepseek-chat","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":":\"pwd\"}"}}]},"finish_reason":"tool_calls"}]}`,
		`data: {"id":"chatcmpl-sse","model":"deepseek-chat","choices":[],"usage":{"prompt_tokens":3,"completion_tokens":4,"total_tokens":7}}`,
		`data: [DONE]`,
		``,
	}, "\n\n"))

	got, err := translateDeepSeekChatSSEToResponsesSSE(body)
	if err != nil {
		t.Fatalf("translate chat sse: %v", err)
	}
	for _, expected := range []string{
		"event: response.created",
		"event: response.reasoning_summary_text.delta",
		"think ",
		"event: response.output_text.delta",
		"hello",
		"event: response.function_call_arguments.delta",
		`"item_id":"fc_call_1"`,
		`\"cmd\":\"pwd\"`,
		"event: response.completed",
		`"input_tokens":3`,
		`"output_tokens":4`,
	} {
		if !strings.Contains(string(got), expected) {
			t.Fatalf("responses SSE missing %q: %s", expected, string(got))
		}
	}
}

func TestTranslateDeepSeekChatSSEErrorToResponsesFailedEvent(t *testing.T) {
	body := []byte(strings.Join([]string{
		`event: error`,
		`data: {"error":{"message":"upstream exploded","type":"server_error"}}`,
		``,
	}, "\n"))

	got, err := translateDeepSeekChatSSEToResponsesSSE(body)
	if err != nil {
		t.Fatalf("translate chat sse error: %v", err)
	}
	for _, expected := range []string{
		"event: response.failed",
		"upstream exploded",
		"server_error",
	} {
		if !strings.Contains(string(got), expected) {
			t.Fatalf("responses failed SSE missing %q: %s", expected, string(got))
		}
	}
}

func TestStreamDeepSeekChatSSEToResponsesWritesDeltaBeforeEOF(t *testing.T) {
	reader, writer := io.Pipe()
	output := newNotifyingWriter("event: response.output_text.delta")
	done := make(chan error, 1)
	go func() {
		_, err := streamDeepSeekChatSSEToResponses(reader, output)
		done <- err
	}()

	_, err := writer.Write([]byte(`data: {"id":"chatcmpl-live","created":1700000000,"model":"deepseek-chat","choices":[{"index":0,"delta":{"role":"assistant","content":"hello"}}]}` + "\n\n"))
	if err != nil {
		t.Fatalf("write first sse block: %v", err)
	}

	select {
	case <-output.seen:
	case <-time.After(time.Second):
		t.Fatalf("stream converter did not write output_text delta before upstream EOF; output=%s", output.String())
	}

	_, _ = writer.Write([]byte("data: [DONE]\n\n"))
	if err := writer.Close(); err != nil {
		t.Fatalf("close pipe writer: %v", err)
	}
	if err := <-done; err != nil {
		t.Fatalf("stream converter returned error: %v", err)
	}
}

func TestProviderRelayRoutesDeepSeekCodexThroughChatCompletions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var upstreamPath string
	var upstreamBody string

	upstream := newTCP4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		upstreamBody = string(body)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "chatcmpl-123",
			"model": "deepseek-chat",
			"choices": [
				{
					"index": 0,
					"message": {"role": "assistant", "content": "hello"},
					"finish_reason": "stop"
				}
			],
			"usage": {
				"prompt_tokens": 10,
				"completion_tokens": 4,
				"total_tokens": 14
			}
		}`))
	}))
	defer upstream.Close()

	router := setupRelayTestRouter(t, "codex", []Provider{{
		ID:           1,
		Name:         "DeepSeek",
		APIURL:       upstream.URL,
		APIKey:       "test-key",
		Enabled:      true,
		ProviderType: "deepseek",
	}})

	req := httptest.NewRequest(http.MethodPost, "/responses", strings.NewReader(`{"model":"gpt-5","input":"hi","stream":false}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if upstreamPath != "/chat/completions" {
		t.Fatalf("upstream path = %q, want %q", upstreamPath, "/chat/completions")
	}
	if got := gjson.Get(upstreamBody, "messages.0.role").String(); got != "user" {
		t.Fatalf("upstream user role = %q, want %q, body=%s", got, "user", upstreamBody)
	}
	if got := gjson.Get(upstreamBody, "messages.0.content").String(); got != "hi" {
		t.Fatalf("upstream user content = %q, want %q, body=%s", got, "hi", upstreamBody)
	}
	if got := gjson.Get(rec.Body.String(), "output_text").String(); got != "hello" {
		t.Fatalf("relay output_text = %q, want %q, body=%s", got, "hello", rec.Body.String())
	}
}

type notifyingWriter struct {
	mu      sync.Mutex
	buf     bytes.Buffer
	pattern string
	seen    chan struct{}
	closed  bool
}

func newNotifyingWriter(pattern string) *notifyingWriter {
	return &notifyingWriter{
		pattern: pattern,
		seen:    make(chan struct{}),
	}
}

func (w *notifyingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n, err := w.buf.Write(p)
	if !w.closed && strings.Contains(w.buf.String(), w.pattern) {
		close(w.seen)
		w.closed = true
	}
	return n, err
}

func (w *notifyingWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

func TestProviderRelayPassesDeepSeekCodexChatCompletionsThrough(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var upstreamPath string
	var upstreamBody string

	upstream := newTCP4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		upstreamBody = string(body)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "chatcmpl-pass",
			"model": "deepseek-chat",
			"choices": [
				{
					"index": 0,
					"message": {"role": "assistant", "content": "chat ok"},
					"finish_reason": "stop"
				}
			],
			"usage": {
				"prompt_tokens": 2,
				"completion_tokens": 2,
				"total_tokens": 4
			}
		}`))
	}))
	defer upstream.Close()

	router := setupRelayTestRouter(t, "codex", []Provider{{
		ID:           1,
		Name:         "DeepSeek",
		APIURL:       upstream.URL,
		APIKey:       "test-key",
		Enabled:      true,
		ProviderType: "deepseek",
	}})

	req := httptest.NewRequest(http.MethodPost, "/chat/completions", strings.NewReader(`{
		"model": "deepseek-chat",
		"messages": [{"role":"user","content":"hi"}],
		"stream": false
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if upstreamPath != "/chat/completions" {
		t.Fatalf("upstream path = %q, want /chat/completions", upstreamPath)
	}
	if got := gjson.Get(upstreamBody, "messages.0.content").String(); got != "hi" {
		t.Fatalf("upstream body was not passed through as Chat: %s", upstreamBody)
	}
	if got := gjson.Get(rec.Body.String(), "choices.0.message.content").String(); got != "chat ok" {
		t.Fatalf("response body was not passed through as Chat: %s", rec.Body.String())
	}
}

func TestProviderRelayNormalizesDeepSeekChatErrorToResponsesError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := newTCP4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"base_resp":{"status_code":2013,"status_msg":"quota exceeded"}}`))
	}))
	defer upstream.Close()

	router := setupRelayTestRouter(t, "codex", []Provider{{
		ID:           1,
		Name:         "DeepSeek",
		APIURL:       upstream.URL,
		APIKey:       "test-key",
		Enabled:      true,
		ProviderType: "deepseek",
	}})

	req := httptest.NewRequest(http.MethodPost, "/responses", strings.NewReader(`{"model":"gpt-5","input":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusTooManyRequests, rec.Body.String())
	}
	if got := gjson.Get(rec.Body.String(), "error.message").String(); got != "quota exceeded" {
		t.Fatalf("error.message = %q, want quota exceeded; body=%s", got, rec.Body.String())
	}
	if got := gjson.Get(rec.Body.String(), "error.type").String(); got != "upstream_error" {
		t.Fatalf("error.type = %q, want upstream_error; body=%s", got, rec.Body.String())
	}
	if !gjson.Get(rec.Body.String(), "error.param").Exists() {
		t.Fatalf("error.param missing; body=%s", rec.Body.String())
	}
}

func TestProviderRelayNormalizesDeepSeekChatErrorDropsEntityHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rawBody := `{"base_resp":{"status_code":2013,"status_msg":"quota exceeded"}}`
	upstream := newTCP4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Encoding", "br")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(rawBody))
	}))
	defer upstream.Close()

	router := setupRelayTestRouter(t, "codex", []Provider{{
		ID:           1,
		Name:         "DeepSeek",
		APIURL:       upstream.URL,
		APIKey:       "test-key",
		Enabled:      true,
		ProviderType: "deepseek",
	}})

	req := httptest.NewRequest(http.MethodPost, "/responses", strings.NewReader(`{"model":"gpt-5","input":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusTooManyRequests, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("Content-Encoding = %q, want stripped for rebuilt error body", got)
	}
	if got := gjson.Get(rec.Body.String(), "error.message").String(); got != "quota exceeded" {
		t.Fatalf("error.message = %q, want quota exceeded; body=%s", got, rec.Body.String())
	}
}

func TestProviderRelayRestoresDeepSeekToolCallForPreviousResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var upstreamBodies []string

	upstream := newTCP4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		upstreamBodies = append(upstreamBodies, string(body))

		w.Header().Set("Content-Type", "application/json")
		if len(upstreamBodies) == 1 {
			_, _ = w.Write([]byte(`{
				"id": "chatcmpl-tool",
				"model": "deepseek-reasoner",
				"choices": [
					{
						"index": 0,
						"message": {
							"role": "assistant",
							"reasoning_content": "Need to call shell.",
							"tool_calls": [
								{
									"id": "call_1",
									"type": "function",
									"function": {
										"name": "shell",
										"arguments": "{\"cmd\":\"pwd\"}"
									}
								}
							]
						},
						"finish_reason": "tool_calls"
					}
				],
				"usage": {
					"prompt_tokens": 10,
					"completion_tokens": 6,
					"total_tokens": 16
				}
			}`))
			return
		}
		_, _ = w.Write([]byte(`{
			"id": "chatcmpl-final",
			"model": "deepseek-reasoner",
			"choices": [
				{
					"index": 0,
					"message": {"role": "assistant", "content": "done"},
					"finish_reason": "stop"
				}
			],
			"usage": {
				"prompt_tokens": 12,
				"completion_tokens": 3,
				"total_tokens": 15
			}
		}`))
	}))
	defer upstream.Close()

	router := setupRelayTestRouter(t, "codex", []Provider{{
		ID:           1,
		Name:         "DeepSeek",
		APIURL:       upstream.URL,
		APIKey:       "test-key",
		Enabled:      true,
		ProviderType: "deepseek",
	}})

	firstReq := httptest.NewRequest(http.MethodPost, "/responses", strings.NewReader(`{"model":"gpt-5","input":"run pwd"}`))
	firstReq.Header.Set("Content-Type", "application/json")
	firstRec := httptest.NewRecorder()
	router.ServeHTTP(firstRec, firstReq)
	if firstRec.Code != http.StatusOK {
		t.Fatalf("first status = %d, want %d, body=%s", firstRec.Code, http.StatusOK, firstRec.Body.String())
	}

	secondReq := httptest.NewRequest(http.MethodPost, "/responses", strings.NewReader(`{
		"model": "gpt-5",
		"previous_response_id": "chatcmpl-tool",
		"input": [
			{
				"type": "function_call_output",
				"call_id": "call_1",
				"output": "workspace"
			}
		]
	}`))
	secondReq.Header.Set("Content-Type", "application/json")
	secondRec := httptest.NewRecorder()
	router.ServeHTTP(secondRec, secondReq)
	if secondRec.Code != http.StatusOK {
		t.Fatalf("second status = %d, want %d, body=%s", secondRec.Code, http.StatusOK, secondRec.Body.String())
	}
	if len(upstreamBodies) != 2 {
		t.Fatalf("upstream request count = %d, want 2", len(upstreamBodies))
	}

	secondBody := upstreamBodies[1]
	if gotRole := gjson.Get(secondBody, "messages.0.role").String(); gotRole != "assistant" {
		t.Fatalf("restored messages.0.role = %q, want assistant; body=%s", gotRole, secondBody)
	}
	if gotCallID := gjson.Get(secondBody, "messages.0.tool_calls.0.id").String(); gotCallID != "call_1" {
		t.Fatalf("restored tool call id = %q, want call_1; body=%s", gotCallID, secondBody)
	}
	if gotReasoning := gjson.Get(secondBody, "messages.0.reasoning_content").String(); gotReasoning != "Need to call shell." {
		t.Fatalf("restored reasoning_content = %q, want cached reasoning; body=%s", gotReasoning, secondBody)
	}
	if gotRole := gjson.Get(secondBody, "messages.1.role").String(); gotRole != "tool" {
		t.Fatalf("messages.1.role = %q, want tool; body=%s", gotRole, secondBody)
	}
}

func TestCodexChatHistoryRestoresParallelToolCallsAsGroup(t *testing.T) {
	history := newCodexChatHistoryStore()
	history.recordResponsePayload([]byte(`{
		"id": "resp_parallel",
		"object": "response",
		"status": "completed",
		"model": "deepseek-chat",
		"output": [
			{"type": "function_call", "id": "call_read", "call_id": "call_read", "name": "read_file", "arguments": "{\"path\":\"README.md\"}", "reasoning_content": "Need both tools"},
			{"type": "function_call", "id": "call_list", "call_id": "call_list", "name": "list_files", "arguments": "{\"path\":\"src\"}", "reasoning_content": "Need both tools"}
		]
	}`), false)

	enriched := history.enrichRequest([]byte(`{
		"model": "gpt-5",
		"previous_response_id": "resp_parallel",
		"input": [
			{"type": "function_call_output", "call_id": "call_read", "output": "Readme content"},
			{"type": "function_call_output", "call_id": "call_list", "output": ["main.go", "services"]}
		]
	}`))
	got, err := translateResponsesRequestToDeepSeekChatCompletion(enriched)
	if err != nil {
		t.Fatalf("translate enriched request: %v; enriched=%s", err, string(enriched))
	}

	if gotRole := gjson.GetBytes(got, "messages.0.role").String(); gotRole != "assistant" {
		t.Fatalf("messages.0.role = %q, want assistant; body=%s", gotRole, string(got))
	}
	if gotCalls := len(gjson.GetBytes(got, "messages.0.tool_calls").Array()); gotCalls != 2 {
		t.Fatalf("messages.0.tool_calls = %d, want 2 restored as one assistant group; body=%s", gotCalls, string(got))
	}
	if gotRole := gjson.GetBytes(got, "messages.1.role").String(); gotRole != "tool" {
		t.Fatalf("messages.1.role = %q, want first tool output; body=%s", gotRole, string(got))
	}
	if gotRole := gjson.GetBytes(got, "messages.2.role").String(); gotRole != "tool" {
		t.Fatalf("messages.2.role = %q, want second tool output; body=%s", gotRole, string(got))
	}
	if gotRole := gjson.GetBytes(got, "messages.3.role").String(); gotRole != "" {
		t.Fatalf("unexpected extra message role %q; body=%s", gotRole, string(got))
	}
}

func TestProviderRelayKeepsCustomCodexProviderOnResponsesPath(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var upstreamPath string

	upstream := newTCP4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_test","object":"response"}`))
	}))
	defer upstream.Close()

	router := setupRelayTestRouter(t, "codex", []Provider{{
		ID:           1,
		Name:         "OpenAI",
		APIURL:       upstream.URL,
		APIKey:       "test-key",
		Enabled:      true,
		ProviderType: "custom",
	}})

	req := httptest.NewRequest(http.MethodPost, "/responses", strings.NewReader(`{"model":"gpt-5","input":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if upstreamPath != "/responses" {
		t.Fatalf("upstream path = %q, want %q", upstreamPath, "/responses")
	}
}

func TestProviderRelaySupportsDeepSeekCodexCompactResponsesPath(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var upstreamPath string

	upstream := newTCP4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "chatcmpl-compact",
			"model": "deepseek-chat",
			"choices": [
				{
					"index": 0,
					"message": {"role": "assistant", "content": "compact"},
					"finish_reason": "stop"
				}
			],
			"usage": {
				"prompt_tokens": 1,
				"completion_tokens": 1,
				"total_tokens": 2
			}
		}`))
	}))
	defer upstream.Close()

	router := setupRelayTestRouter(t, "codex", []Provider{{
		ID:           1,
		Name:         "DeepSeek",
		APIURL:       upstream.URL,
		APIKey:       "test-key",
		Enabled:      true,
		ProviderType: "deepseek",
	}})

	req := httptest.NewRequest(http.MethodPost, "/v1/responses/compact", strings.NewReader(`{"model":"gpt-5","input":"compact"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if upstreamPath != "/chat/completions" {
		t.Fatalf("upstream path = %q, want /chat/completions", upstreamPath)
	}
}

func TestProviderRelayRequestsDeepSeekStreamAndSynthesizesFallbackForJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var upstreamBody string

	upstream := newTCP4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		upstreamBody = string(body)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "chatcmpl-123",
			"model": "deepseek-chat",
			"choices": [
				{
					"index": 0,
					"message": {"role": "assistant", "content": "hello"},
					"finish_reason": "stop"
				}
			],
			"usage": {
				"prompt_tokens": 10,
				"completion_tokens": 4,
				"total_tokens": 14
			}
		}`))
	}))
	defer upstream.Close()

	router := setupRelayTestRouter(t, "codex", []Provider{{
		ID:           1,
		Name:         "DeepSeek",
		APIURL:       upstream.URL,
		APIKey:       "test-key",
		Enabled:      true,
		ProviderType: "deepseek",
	}})

	req := httptest.NewRequest(http.MethodPost, "/responses", strings.NewReader(`{"model":"gpt-5","input":"hi","stream":true}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if gotStream := gjson.Get(upstreamBody, "stream").Bool(); !gotStream {
		t.Fatalf("upstream stream = %v, want true, body=%s", gotStream, upstreamBody)
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "text/event-stream") {
		t.Fatalf("Content-Type = %q, want text/event-stream", got)
	}
	if !strings.Contains(rec.Body.String(), "event: response.completed") {
		t.Fatalf("stream response missing completed event: %s", rec.Body.String())
	}
}

func TestProviderRelayConvertsDeepSeekChatSSEToResponsesSSE(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var upstreamBody string

	upstream := newTCP4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		upstreamBody = string(body)

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(strings.Join([]string{
			`data: {"id":"chatcmpl-sse","created":1700000000,"model":"deepseek-chat","choices":[{"index":0,"delta":{"role":"assistant","content":"hello"}}]}`,
			`data: {"id":"chatcmpl-sse","model":"deepseek-chat","choices":[],"usage":{"prompt_tokens":3,"completion_tokens":4,"total_tokens":7}}`,
			`data: [DONE]`,
			``,
		}, "\n\n")))
	}))
	defer upstream.Close()

	router := setupRelayTestRouter(t, "codex", []Provider{{
		ID:           1,
		Name:         "DeepSeek",
		APIURL:       upstream.URL,
		APIKey:       "test-key",
		Enabled:      true,
		ProviderType: "deepseek",
	}})

	req := httptest.NewRequest(http.MethodPost, "/responses", strings.NewReader(`{"model":"gpt-5","input":"hi","stream":true}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if gotStream := gjson.Get(upstreamBody, "stream").Bool(); !gotStream {
		t.Fatalf("upstream stream = %v, want true, body=%s", gotStream, upstreamBody)
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "text/event-stream") {
		t.Fatalf("Content-Type = %q, want text/event-stream", got)
	}
	if !strings.Contains(rec.Body.String(), "event: response.output_text.delta") {
		t.Fatalf("stream response missing output text delta: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"input_tokens":3`) {
		t.Fatalf("stream response missing usage: %s", rec.Body.String())
	}
}

func TestProviderRelayStreamsDeepSeekChatSSEDeltaBeforeUpstreamEOF(t *testing.T) {
	gin.SetMode(gin.TestMode)

	releaseUpstream := make(chan struct{})
	var releaseOnce sync.Once

	upstream := newTCP4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(`data: {"id":"chatcmpl-live","created":1700000000,"model":"deepseek-chat","choices":[{"index":0,"delta":{"role":"assistant","content":"hello"}}]}` + "\n\n"))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		<-releaseUpstream
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer upstream.Close()

	router := setupRelayTestRouter(t, "codex", []Provider{{
		ID:           1,
		Name:         "DeepSeek",
		APIURL:       upstream.URL,
		APIKey:       "test-key",
		Enabled:      true,
		ProviderType: "deepseek",
	}})
	relay := httptest.NewServer(router)
	defer relay.Close()
	defer releaseOnce.Do(func() { close(releaseUpstream) })

	req, err := http.NewRequest(http.MethodPost, relay.URL+"/responses", strings.NewReader(`{"model":"gpt-5","input":"hi","stream":true}`))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	deltaSeen := make(chan error, 1)
	go func() {
		resp, err := relay.Client().Do(req)
		if err != nil {
			deltaSeen <- err
			return
		}
		defer resp.Body.Close()

		reader := bufio.NewReader(resp.Body)
		for {
			line, err := reader.ReadString('\n')
			if strings.Contains(line, "event: response.output_text.delta") {
				deltaSeen <- nil
				return
			}
			if err != nil {
				deltaSeen <- err
				return
			}
		}
	}()

	select {
	case err := <-deltaSeen:
		if err != nil {
			t.Fatalf("read streamed delta: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatalf("relay did not stream output_text delta before upstream EOF")
	}

	releaseOnce.Do(func() { close(releaseUpstream) })
}

func TestCodexChatHistoryRecordsFunctionCallsFromCompletedSSEEvent(t *testing.T) {
	payload := []byte(`{
		"id": "resp_stream_tool",
		"object": "response",
		"status": "completed",
		"model": "deepseek-chat",
		"output": [
			{
				"type": "function_call",
				"id": "call_stream",
				"call_id": "call_stream",
				"name": "shell",
				"arguments": "{\"cmd\":\"pwd\"}",
				"reasoning_content": "need shell",
				"status": "completed"
			}
		]
	}`)
	sse, err := responsesPayloadToSyntheticSSE(payload)
	if err != nil {
		t.Fatalf("synthetic sse: %v", err)
	}

	history := newCodexChatHistoryStore()
	history.recordResponsePayload(sse, true)

	calls := history.callsForResponse("resp_stream_tool")
	if len(calls) != 1 {
		t.Fatalf("cached calls = %d, want 1; sse=%s", len(calls), string(sse))
	}
	if calls[0].CallID != "call_stream" {
		t.Fatalf("cached call id = %q, want call_stream", calls[0].CallID)
	}
}

func TestProviderRelayStoresDeepSeekRawPayloads(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := newTCP4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "chatcmpl-123",
			"model": "deepseek-chat",
			"choices": [
				{
					"index": 0,
					"message": {"role": "assistant", "content": "hello"},
					"finish_reason": "stop"
				}
			],
			"usage": {
				"prompt_tokens": 10,
				"completion_tokens": 4,
				"total_tokens": 14
			}
		}`))
	}))
	defer upstream.Close()

	router := setupRelayTestRouter(t, "codex", []Provider{{
		ID:           1,
		Name:         "DeepSeek",
		APIURL:       upstream.URL,
		APIKey:       "test-key",
		Enabled:      true,
		ProviderType: "deepseek",
	}}, AppSettings{
		CaptureRawLogs: true,
		RelayPort:      defaultRelayPort,
		RawLogMaxBytes: defaultRawLogMaxBytes,
	})

	req := httptest.NewRequest(http.MethodPost, "/responses", strings.NewReader(`{"model":"gpt-5","input":"hi","stream":false}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	db, err := xdb.DB("default")
	if err != nil {
		t.Fatalf("xdb.DB(default) error = %v", err)
	}
	var requestBody, responseBody, upstreamRequestBody, upstreamResponseBody string
	if err := db.QueryRow(`
		SELECT request_body, response_body, upstream_request_body, upstream_response_body
		FROM request_log_payload
		ORDER BY log_id DESC
		LIMIT 1
	`).Scan(&requestBody, &responseBody, &upstreamRequestBody, &upstreamResponseBody); err != nil {
		t.Fatalf("select request_log_payload: %v", err)
	}

	if !strings.Contains(requestBody, `"input":"hi"`) {
		t.Fatalf("request_body = %q, want original Codex request", requestBody)
	}
	if !strings.Contains(responseBody, `"output_text":"hello"`) {
		t.Fatalf("response_body = %q, want translated Responses body", responseBody)
	}
	if got := gjson.Get(upstreamRequestBody, "messages.0.content").String(); got != "hi" {
		t.Fatalf("upstream request user content = %q, want hi; body=%s", got, upstreamRequestBody)
	}
	if !strings.Contains(upstreamResponseBody, `"chatcmpl-123"`) {
		t.Fatalf("upstream_response_body = %q, want raw DeepSeek response", upstreamResponseBody)
	}
}
