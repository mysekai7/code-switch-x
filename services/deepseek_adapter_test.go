package services

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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

func TestTranslateResponsesRequestForcesDeepSeekNonStream(t *testing.T) {
	body := []byte(`{
		"model": "gpt-5",
		"input": "hello",
		"stream": true
	}`)

	got, err := translateResponsesRequestToDeepSeekChatCompletion(body)
	if err != nil {
		t.Fatalf("translate request: %v", err)
	}
	if gotStream := gjson.GetBytes(got, "stream").Bool(); gotStream {
		t.Fatalf("upstream stream = %v, want false", gotStream)
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

func TestProviderRelaySynthesizesResponsesStreamForDeepSeekCodex(t *testing.T) {
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
	if gotStream := gjson.Get(upstreamBody, "stream").Bool(); gotStream {
		t.Fatalf("upstream stream = %v, want false, body=%s", gotStream, upstreamBody)
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "text/event-stream") {
		t.Fatalf("Content-Type = %q, want text/event-stream", got)
	}
	if !strings.Contains(rec.Body.String(), "event: response.completed") {
		t.Fatalf("stream response missing completed event: %s", rec.Body.String())
	}
}
