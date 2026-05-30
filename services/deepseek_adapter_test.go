package services

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
