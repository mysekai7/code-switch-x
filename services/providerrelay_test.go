package services

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/daodao97/xgo/xdb"
	"github.com/daodao97/xgo/xrequest"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// ==================== ReplaceModelInRequestBody 测试 ====================

func TestReplaceModelInRequestBody(t *testing.T) {
	tests := []struct {
		name          string
		inputJSON     string
		newModel      string
		expectError   bool
		expectedModel string
	}{
		// 成功场景
		{
			name: "简单替换",
			inputJSON: `{
				"model": "claude-sonnet-4",
				"messages": [{"role": "user", "content": "Hello"}]
			}`,
			newModel:      "anthropic/claude-sonnet-4",
			expectError:   false,
			expectedModel: "anthropic/claude-sonnet-4",
		},
		{
			name: "复杂嵌套JSON",
			inputJSON: `{
				"model": "claude-opus-4",
				"messages": [
					{
						"role": "user",
						"content": "Test"
					}
				],
				"temperature": 0.7,
				"max_tokens": 1000,
				"metadata": {
					"user_id": "12345"
				}
			}`,
			newModel:      "gpt-4",
			expectError:   false,
			expectedModel: "gpt-4",
		},
		{
			name: "模型名包含特殊字符",
			inputJSON: `{
				"model": "claude-sonnet-4",
				"messages": []
			}`,
			newModel:      "anthropic/claude-3.5-sonnet@20241022",
			expectError:   false,
			expectedModel: "anthropic/claude-3.5-sonnet@20241022",
		},

		// 错误场景
		{
			name: "缺少model字段",
			inputJSON: `{
				"messages": [{"role": "user", "content": "Hello"}]
			}`,
			newModel:    "any-model",
			expectError: true,
		},
		{
			name: "空JSON",
			inputJSON: `{
			}`,
			newModel:    "any-model",
			expectError: true,
		},
		{
			name:        "无效JSON",
			inputJSON:   `{invalid json}`,
			newModel:    "any-model",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bodyBytes := []byte(tt.inputJSON)
			result, err := ReplaceModelInRequestBody(bodyBytes, tt.newModel)

			// 检查错误预期
			if tt.expectError && err == nil {
				t.Errorf("期望返回错误，但没有错误")
			}
			if !tt.expectError && err != nil {
				t.Errorf("不期望错误，但返回了: %v", err)
			}

			// 如果不期望错误，验证结果
			if !tt.expectError {
				// 验证返回的JSON是否有效
				if !json.Valid(result) {
					t.Errorf("返回的JSON无效")
				}

				// 验证模型名是否正确替换
				actualModel := gjson.GetBytes(result, "model").String()
				if actualModel != tt.expectedModel {
					t.Errorf("替换后的模型名 = %q, 期望 %q", actualModel, tt.expectedModel)
				}

				// 验证其他字段未被修改
				if gjson.GetBytes(bodyBytes, "messages").Exists() {
					originalMessages := gjson.GetBytes(bodyBytes, "messages").Raw
					resultMessages := gjson.GetBytes(result, "messages").Raw
					if originalMessages != resultMessages {
						t.Errorf("messages 字段被意外修改")
					}
				}
			}
		})
	}
}

// ==================== 端到端场景测试 ====================

func TestModelMappingEndToEnd(t *testing.T) {
	// 模拟真实场景：用户请求 claude-sonnet-4，需要映射到 OpenRouter 的格式
	provider := Provider{
		Name: "OpenRouter",
		SupportedModels: map[string]bool{
			"anthropic/claude-sonnet-4":   true,
			"anthropic/claude-opus-4":     true,
			"openai/gpt-4":                true,
			"google/gemini-pro":           true,
			"meta-llama/llama-3.1-405b":   true,
			"anthropic/claude-3.5-sonnet": true,
			"anthropic/claude-3.5-haiku":  true,
		},
		ModelMapping: map[string]string{
			"claude-*": "anthropic/claude-*",
			"gpt-*":    "openai/gpt-*",
			"gemini-*": "google/gemini-*",
			"llama-*":  "meta-llama/llama-*",
		},
	}

	scenarios := []struct {
		requestedModel string
		shouldSupport  bool
		effectiveModel string
	}{
		// 通配符映射场景
		{"claude-sonnet-4", true, "anthropic/claude-sonnet-4"},
		{"claude-opus-4", true, "anthropic/claude-opus-4"},
		{"claude-3.5-sonnet", true, "anthropic/claude-3.5-sonnet"},
		{"gpt-4", true, "openai/gpt-4"},
		{"gpt-4-turbo", true, "openai/gpt-4-turbo"},
		{"gemini-pro", true, "google/gemini-pro"},
		{"llama-3.1-405b", true, "meta-llama/llama-3.1-405b"},

		// 不支持的模型
		{"deepseek-v3", false, "deepseek-v3"},
		{"qwen-max", false, "qwen-max"},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.requestedModel, func(t *testing.T) {
			// 1. 检查是否支持
			supported := provider.IsModelSupported(scenario.requestedModel)
			if supported != scenario.shouldSupport {
				t.Errorf("IsModelSupported(%q) = %v, 期望 %v",
					scenario.requestedModel, supported, scenario.shouldSupport)
			}

			// 2. 获取有效模型名
			effectiveModel := provider.GetEffectiveModel(scenario.requestedModel)
			if effectiveModel != scenario.effectiveModel {
				t.Errorf("GetEffectiveModel(%q) = %q, 期望 %q",
					scenario.requestedModel, effectiveModel, scenario.effectiveModel)
			}

			// 3. 如果支持，测试请求体替换
			if scenario.shouldSupport {
				requestBody := `{"model": "` + scenario.requestedModel + `", "messages": []}`
				result, err := ReplaceModelInRequestBody([]byte(requestBody), effectiveModel)
				if err != nil {
					t.Fatalf("ReplaceModelInRequestBody 失败: %v", err)
				}

				actualModel := gjson.GetBytes(result, "model").String()
				if actualModel != scenario.effectiveModel {
					t.Errorf("请求体中的模型 = %q, 期望 %q", actualModel, scenario.effectiveModel)
				}
			}
		})
	}
}

// ==================== 配置验证集成测试 ====================

func TestProviderConfigValidation(t *testing.T) {
	// 场景 1：完美配置
	validProvider := Provider{
		Name: "ValidProvider",
		SupportedModels: map[string]bool{
			"anthropic/claude-sonnet-4": true,
			"anthropic/claude-opus-4":   true,
		},
		ModelMapping: map[string]string{
			"claude-sonnet-4": "anthropic/claude-sonnet-4",
			"claude-opus-4":   "anthropic/claude-opus-4",
		},
	}

	errors := validProvider.ValidateConfiguration()
	if len(errors) != 0 {
		t.Errorf("完美配置不应有错误，但返回了: %v", errors)
	}

	// 场景 2：错误配置 - 映射目标不存在
	invalidProvider := Provider{
		Name: "InvalidProvider",
		SupportedModels: map[string]bool{
			"model-a": true,
		},
		ModelMapping: map[string]string{
			"external": "non-existent-model",
		},
	}

	errors = invalidProvider.ValidateConfiguration()
	if len(errors) == 0 {
		t.Errorf("错误配置应该返回验证错误")
	}

	// 场景 3：通配符配置
	wildcardProvider := Provider{
		Name: "WildcardProvider",
		SupportedModels: map[string]bool{
			"anthropic/claude-*": true,
			"openai/gpt-*":       true,
		},
		ModelMapping: map[string]string{
			"claude-*": "anthropic/claude-*",
			"gpt-*":    "openai/gpt-*",
		},
	}

	errors = wildcardProvider.ValidateConfiguration()
	if len(errors) != 0 {
		t.Errorf("通配符配置不应有错误，但返回了: %v", errors)
	}
}

// ==================== 代理兼容性测试 ====================

func TestProxyHandlerReturnsLastUpstreamError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := newTCP4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" {
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Request-Id", "req_rate_limited")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"message":"rate limit","type":"rate_limit_error"}}`))
	}))
	defer upstream.Close()

	router := setupRelayTestRouter(t, "codex", []Provider{{
		ID:      1,
		Name:    "OpenAI",
		APIURL:  upstream.URL,
		APIKey:  "test-key",
		Enabled: true,
	}})

	req := httptest.NewRequest(http.MethodPost, "/responses", strings.NewReader(`{"model":"gpt-5","input":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusTooManyRequests, rec.Body.String())
	}
	if got := rec.Header().Get("X-Request-Id"); got != "req_rate_limited" {
		t.Fatalf("X-Request-Id = %q, want %q", got, "req_rate_limited")
	}
	if got := gjson.Get(rec.Body.String(), "error.type").String(); got != "rate_limit_error" {
		t.Fatalf("error.type = %q, want %q, body=%s", got, "rate_limit_error", rec.Body.String())
	}
}

func TestProxyHandlerKeepsLastUpstreamErrorWhenLaterProviderHasNoResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := newTCP4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Request-Id", "req_upstream_error")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"bad key","type":"invalid_api_key"}}`))
	}))
	defer upstream.Close()

	router := setupRelayTestRouter(t, "codex", []Provider{
		{
			ID:      1,
			Name:    "OpenAI",
			APIURL:  upstream.URL,
			APIKey:  "test-key",
			Enabled: true,
		},
		{
			ID:      2,
			Name:    "BrokenProvider",
			APIURL:  "://invalid-url",
			APIKey:  "test-key",
			Enabled: true,
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/responses", strings.NewReader(`{"model":"gpt-5","input":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}
	if got := rec.Header().Get("X-Request-Id"); got != "req_upstream_error" {
		t.Fatalf("X-Request-Id = %q, want %q", got, "req_upstream_error")
	}
	if got := gjson.Get(rec.Body.String(), "error.type").String(); got != "invalid_api_key" {
		t.Fatalf("error.type = %q, want %q, body=%s", got, "invalid_api_key", rec.Body.String())
	}
}

func TestRegisterRoutesSupportsOpenAIResponsesPath(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := newTCP4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" {
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_test","object":"response"}`))
	}))
	defer upstream.Close()

	router := setupRelayTestRouter(t, "codex", []Provider{{
		ID:      1,
		Name:    "OpenAI",
		APIURL:  upstream.URL,
		APIKey:  "test-key",
		Enabled: true,
	}})

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-5","input":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := gjson.Get(rec.Body.String(), "id").String(); got != "resp_test" {
		t.Fatalf("id = %q, want %q, body=%s", got, "resp_test", rec.Body.String())
	}
}

func TestEnsureRequestLogPayloadTable(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	NewProviderRelayService(NewProviderService(), ":0")

	db, err := xdb.DB("default")
	if err != nil {
		t.Fatalf("xdb.DB(default) error = %v", err)
	}
	rows, err := db.Query("PRAGMA table_info('request_log_payload')")
	if err != nil {
		t.Fatalf("table_info request_log_payload: %v", err)
	}
	defer rows.Close()

	columns := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			t.Fatalf("scan table_info row: %v", err)
		}
		columns[name] = true
	}
	for _, column := range []string{
		"log_id",
		"request_headers",
		"request_body",
		"response_headers",
		"response_body",
		"upstream_request_body",
		"upstream_response_body",
		"request_truncated",
		"response_truncated",
		"created_at",
	} {
		if !columns[column] {
			t.Fatalf("request_log_payload missing column %q; columns=%v", column, columns)
		}
	}
}

func TestProxyHandlerDoesNotPrintProviderAPIKeyWhenRequestDebugEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := newTCP4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_test","object":"response"}`))
	}))
	defer upstream.Close()

	const secret = "sk-test-sensitive-secret"
	router := setupRelayTestRouter(t, "codex", []Provider{{
		ID:      1,
		Name:    "OpenAI",
		APIURL:  upstream.URL,
		APIKey:  secret,
		Enabled: true,
	}})

	xrequest.SetRequestDebug(true)
	t.Cleanup(func() {
		xrequest.SetRequestDebug(false)
	})

	output := captureStdout(t, func() {
		req := httptest.NewRequest(http.MethodPost, "/responses", strings.NewReader(`{"model":"gpt-5","input":"hi"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
		}
	})

	if strings.Contains(output, secret) {
		t.Fatalf("stdout leaked provider API key: %s", output)
	}
}

func TestProxyHandlerStoresRawPayloadWhenEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := newTCP4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Set-Cookie", "session=upstream-secret")
		_, _ = w.Write([]byte(`{"id":"resp_test","object":"response","usage":{"input_tokens":3,"output_tokens":2}}`))
	}))
	defer upstream.Close()

	router := setupRelayTestRouter(t, "codex", []Provider{{
		ID:      1,
		Name:    "OpenAI",
		APIURL:  upstream.URL,
		APIKey:  "provider-secret",
		Enabled: true,
	}}, AppSettings{
		CaptureRawLogs: true,
		RelayPort:      defaultRelayPort,
		RawLogMaxBytes: defaultRawLogMaxBytes,
	})

	req := httptest.NewRequest(http.MethodPost, "/responses", strings.NewReader(`{"model":"gpt-5","input":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer client-secret")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	db, err := xdb.DB("default")
	if err != nil {
		t.Fatalf("xdb.DB(default) error = %v", err)
	}
	var requestHeaders, requestBody, responseHeaders, responseBody string
	var requestTruncated, responseTruncated int
	if err := db.QueryRow(`
		SELECT request_headers, request_body, response_headers, response_body, request_truncated, response_truncated
		FROM request_log_payload
		ORDER BY log_id DESC
		LIMIT 1
	`).Scan(&requestHeaders, &requestBody, &responseHeaders, &responseBody, &requestTruncated, &responseTruncated); err != nil {
		t.Fatalf("select request_log_payload: %v", err)
	}

	if !strings.Contains(requestBody, `"input":"hi"`) {
		t.Fatalf("request_body = %q, want original request", requestBody)
	}
	if !strings.Contains(responseBody, `"resp_test"`) {
		t.Fatalf("response_body = %q, want upstream response", responseBody)
	}
	for _, leaked := range []string{"client-secret", "provider-secret", "upstream-secret"} {
		if strings.Contains(requestHeaders+requestBody+responseHeaders+responseBody, leaked) {
			t.Fatalf("raw payload leaked %q", leaked)
		}
	}
	if requestTruncated != 0 || responseTruncated != 0 {
		t.Fatalf("truncated flags = request:%d response:%d, want 0/0", requestTruncated, responseTruncated)
	}
}

func TestProxyHandlerStoresOriginalRawRequestBeforeModelMapping(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var upstreamBody string
	upstream := newTCP4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read upstream body: %v", err)
		}
		upstreamBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_test","object":"response"}`))
	}))
	defer upstream.Close()

	router := setupRelayTestRouter(t, "codex", []Provider{{
		ID:      1,
		Name:    "OpenRouter",
		APIURL:  upstream.URL,
		APIKey:  "provider-secret",
		Enabled: true,
		SupportedModels: map[string]bool{
			"openai/gpt-5": true,
		},
		ModelMapping: map[string]string{
			"gpt-5": "openai/gpt-5",
		},
	}}, AppSettings{
		CaptureRawLogs: true,
		RelayPort:      defaultRelayPort,
		RawLogMaxBytes: defaultRawLogMaxBytes,
	})

	req := httptest.NewRequest(http.MethodPost, "/responses", strings.NewReader(`{"model":"gpt-5","input":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := gjson.Get(upstreamBody, "model").String(); got != "openai/gpt-5" {
		t.Fatalf("upstream model = %q, want mapped model; body=%s", got, upstreamBody)
	}

	db, err := xdb.DB("default")
	if err != nil {
		t.Fatalf("xdb.DB(default) error = %v", err)
	}
	var requestBody string
	if err := db.QueryRow(`
		SELECT request_body
		FROM request_log_payload
		ORDER BY log_id DESC
		LIMIT 1
	`).Scan(&requestBody); err != nil {
		t.Fatalf("select request_log_payload: %v", err)
	}

	if got := gjson.Get(requestBody, "model").String(); got != "gpt-5" {
		t.Fatalf("request_body model = %q, want original client model; body=%s", got, requestBody)
	}
}

func TestProxyHandlerStoresRawStreamResponseWithSSESeparators(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const upstreamResponse = "data: {\"response\":{\"usage\":{\"input_tokens\":1,\"output_tokens\":2}}}\n\n"
	upstream := newTCP4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(upstreamResponse))
	}))
	defer upstream.Close()

	router := setupRelayTestRouter(t, "codex", []Provider{{
		ID:      1,
		Name:    "OpenAI",
		APIURL:  upstream.URL,
		APIKey:  "provider-secret",
		Enabled: true,
	}}, AppSettings{
		CaptureRawLogs: true,
		RelayPort:      defaultRelayPort,
		RawLogMaxBytes: defaultRawLogMaxBytes,
	})

	req := httptest.NewRequest(http.MethodPost, "/responses", strings.NewReader(`{"model":"gpt-5","input":"hi","stream":true}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if rec.Body.String() != upstreamResponse {
		t.Fatalf("client response = %q, want upstream SSE response", rec.Body.String())
	}

	db, err := xdb.DB("default")
	if err != nil {
		t.Fatalf("xdb.DB(default) error = %v", err)
	}
	var responseBody string
	if err := db.QueryRow(`
		SELECT response_body
		FROM request_log_payload
		ORDER BY log_id DESC
		LIMIT 1
	`).Scan(&responseBody); err != nil {
		t.Fatalf("select request_log_payload: %v", err)
	}

	if responseBody != upstreamResponse {
		t.Fatalf("response_body = %q, want exact SSE response %q", responseBody, upstreamResponse)
	}
}

func TestLogServiceGetRequestLogPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := newTCP4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_test","object":"response"}`))
	}))
	defer upstream.Close()

	router := setupRelayTestRouter(t, "codex", []Provider{{
		ID:      1,
		Name:    "OpenAI",
		APIURL:  upstream.URL,
		APIKey:  "provider-secret",
		Enabled: true,
	}}, AppSettings{
		CaptureRawLogs: true,
		RelayPort:      defaultRelayPort,
		RawLogMaxBytes: defaultRawLogMaxBytes,
	})

	req := httptest.NewRequest(http.MethodPost, "/responses", strings.NewReader(`{"model":"gpt-5","input":"hi"}`))
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
	var logID int64
	if err := db.QueryRow("SELECT MAX(id) FROM request_log").Scan(&logID); err != nil {
		t.Fatalf("select max request_log id: %v", err)
	}

	payload, err := NewLogService().GetRequestLogPayload(logID)
	if err != nil {
		t.Fatalf("GetRequestLogPayload(%d) error = %v", logID, err)
	}
	if !payload.HasPayload {
		t.Fatal("HasPayload = false, want true")
	}
	if !strings.Contains(payload.RequestBody, `"input":"hi"`) {
		t.Fatalf("RequestBody = %q, want raw request", payload.RequestBody)
	}
	if !strings.Contains(payload.ResponseBody, `"resp_test"`) {
		t.Fatalf("ResponseBody = %q, want raw response", payload.ResponseBody)
	}

	missing, err := NewLogService().GetRequestLogPayload(logID + 1000)
	if err != nil {
		t.Fatalf("GetRequestLogPayload(missing) error = %v", err)
	}
	if missing.HasPayload {
		t.Fatal("missing HasPayload = true, want false")
	}
}

func TestCodexParseTokenUsageFromRootUsage(t *testing.T) {
	usage := &ReqeustLog{}
	CodexParseTokenUsageFromResponse(`{
		"usage": {
			"input_tokens": 12,
			"output_tokens": 3,
			"input_tokens_details": {"cached_tokens": 4},
			"output_tokens_details": {"reasoning_tokens": 2}
		}
	}`, usage)

	if usage.InputTokens != 12 {
		t.Fatalf("InputTokens = %d, want 12", usage.InputTokens)
	}
	if usage.OutputTokens != 3 {
		t.Fatalf("OutputTokens = %d, want 3", usage.OutputTokens)
	}
	if usage.CacheReadTokens != 4 {
		t.Fatalf("CacheReadTokens = %d, want 4", usage.CacheReadTokens)
	}
	if usage.ReasoningTokens != 2 {
		t.Fatalf("ReasoningTokens = %d, want 2", usage.ReasoningTokens)
	}
}

func TestClaudeParseTokenUsageAvoidsCumulativeDoubleCount(t *testing.T) {
	usage := &ReqeustLog{}
	ClaudeCodeParseTokenUsageFromResponse(`{"usage":{"output_tokens":2}}`, usage)
	ClaudeCodeParseTokenUsageFromResponse(`{"usage":{"output_tokens":5}}`, usage)

	if usage.OutputTokens != 5 {
		t.Fatalf("OutputTokens = %d, want 5", usage.OutputTokens)
	}
}

func captureStdout(t *testing.T, fn func()) (output string) {
	t.Helper()

	original := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	os.Stdout = writer

	out := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, reader)
		out <- buf.String()
	}()

	writerClosed := false
	defer func() {
		os.Stdout = original
		if !writerClosed {
			_ = writer.Close()
			output = <-out
		}
		_ = reader.Close()
	}()

	fn()

	if err := writer.Close(); err != nil {
		t.Fatalf("close stdout writer: %v", err)
	}
	writerClosed = true
	output = <-out
	return output
}

func setupRelayTestRouter(t *testing.T, kind string, providers []Provider, settings ...AppSettings) *gin.Engine {
	t.Helper()

	t.Setenv("HOME", t.TempDir())

	providerService := NewProviderService()
	if err := providerService.SaveProviders(kind, providers); err != nil {
		t.Fatalf("save providers: %v", err)
	}

	appSettingsService := NewAppSettingsService(nil)
	if len(settings) > 0 {
		if _, err := appSettingsService.SaveAppSettings(settings[0]); err != nil {
			t.Fatalf("save app settings: %v", err)
		}
	}

	relay := NewProviderRelayService(providerService, ":0", appSettingsService)
	router := gin.New()
	relay.registerRoutes(router)
	return router
}

func newTCP4TestServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()

	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp4 test server: %v", err)
	}

	server := httptest.NewUnstartedServer(handler)
	server.Listener = listener
	server.Start()
	return server
}

// ==================== 性能测试 ====================

func BenchmarkIsModelSupported(b *testing.B) {
	provider := Provider{
		SupportedModels: map[string]bool{
			"claude-sonnet-4": true,
			"claude-opus-4":   true,
			"gpt-4":           true,
			"gpt-4-turbo":     true,
		},
		ModelMapping: map[string]string{
			"claude-*": "anthropic/claude-*",
			"gpt-*":    "openai/gpt-*",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = provider.IsModelSupported("claude-sonnet-4")
	}
}

func BenchmarkGetEffectiveModel(b *testing.B) {
	provider := Provider{
		ModelMapping: map[string]string{
			"claude-*": "anthropic/claude-*",
			"gpt-*":    "openai/gpt-*",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = provider.GetEffectiveModel("claude-sonnet-4")
	}
}

func BenchmarkReplaceModelInRequestBody(b *testing.B) {
	bodyBytes := []byte(`{
		"model": "claude-sonnet-4",
		"messages": [{"role": "user", "content": "Hello"}],
		"temperature": 0.7,
		"max_tokens": 1000
	}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ReplaceModelInRequestBody(bodyBytes, "anthropic/claude-sonnet-4")
	}
}
