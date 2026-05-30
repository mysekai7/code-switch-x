# Raw Request And Response Logs Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add opt-in raw request and response details to the logs page.

**Architecture:** Keep the existing `request_log` table as the summary index. Add a separate `request_log_payload` table and a detail API so the list page stays lightweight. Capture payloads only when the user enables the setting, with header redaction and body size limits.

**Tech Stack:** Go 1.24, Gin, SQLite via `xdb`, Vue 3, Wails v3 bindings, TypeScript.

**Commit Policy:** Do not commit from the main workspace unless the user explicitly asks.

---

### Task 1: Add App Setting For Raw Payload Capture

**Files:**
- Modify: `services/appsettings.go`
- Modify: `services/appsettings_test.go`
- Modify: `frontend/src/services/appSettings.ts`
- Modify: `frontend/src/components/General/Index.vue`
- Modify: `frontend/src/locales/zh.json`
- Modify: `frontend/src/locales/en.json`

**Step 1: Write failing Go test**

Add a test in `services/appsettings_test.go`:

```go
func TestAppSettingsDefaultsRawLogCaptureOff(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	service := NewAppSettingsService(nil)

	settings, err := service.GetAppSettings()
	if err != nil {
		t.Fatalf("GetAppSettings() error = %v", err)
	}
	if settings.CaptureRawLogs {
		t.Fatalf("CaptureRawLogs = true, want false")
	}
	if settings.RawLogMaxBytes != defaultRawLogMaxBytes {
		t.Fatalf("RawLogMaxBytes = %d, want %d", settings.RawLogMaxBytes, defaultRawLogMaxBytes)
	}
}
```

**Step 2: Run test to verify failure**

Run:

```bash
go test ./services -run TestAppSettingsDefaultsRawLogCaptureOff -count=1
```

Expected: FAIL because `CaptureRawLogs` and `RawLogMaxBytes` do not exist.

**Step 3: Implement minimal settings fields**

Add fields:

```go
CaptureRawLogs bool `json:"capture_raw_logs"`
RawLogMaxBytes int  `json:"raw_log_max_bytes"`
```

Add constants:

```go
const defaultRawLogMaxBytes = 262144
```

Default `CaptureRawLogs` to `false` and `RawLogMaxBytes` to `defaultRawLogMaxBytes`.

**Step 4: Add frontend setting controls**

Add a settings toggle labeled `保存原始请求/响应` and a short warning that raw logs may contain private prompts or tool output.

**Step 5: Verify**

Run:

```bash
go test ./services -run TestAppSettingsDefaultsRawLogCaptureOff -count=1
npm test
```

Expected: PASS.

---

### Task 2: Add Payload Storage Table And Model

**Files:**
- Modify: `services/providerrelay.go`
- Test: `services/providerrelay_test.go`

**Step 1: Write failing migration test**

Add a test that initializes the relay DB and verifies `request_log_payload` exists with expected columns.

**Step 2: Run test to verify failure**

Run:

```bash
go test ./services -run TestEnsureRequestLogPayloadTable -count=1
```

Expected: FAIL because the table does not exist.

**Step 3: Implement table creation**

Add `ensureRequestLogPayloadTableWithDB(db)` and call it from relay initialization after `ensureRequestLogTableWithDB`.

Schema:

```sql
CREATE TABLE IF NOT EXISTS request_log_payload (
  log_id INTEGER PRIMARY KEY,
  request_headers TEXT,
  request_body TEXT,
  response_headers TEXT,
  response_body TEXT,
  upstream_request_body TEXT,
  upstream_response_body TEXT,
  request_truncated INTEGER DEFAULT 0,
  response_truncated INTEGER DEFAULT 0,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
)
```

**Step 4: Verify**

Run:

```bash
go test ./services -run TestEnsureRequestLogPayloadTable -count=1
```

Expected: PASS.

---

### Task 3: Add Redaction And Bounded Capture Helpers

**Files:**
- Create: `services/rawlog.go`
- Test: `services/rawlog_test.go`

**Step 1: Write failing tests**

Cover:

- Sensitive headers are redacted case-insensitively.
- Non-sensitive headers remain.
- Body larger than the max byte limit is truncated and marked.
- Invalid max byte values fall back to default.

**Step 2: Run tests to verify failure**

Run:

```bash
go test ./services -run 'TestRawLog|TestRedact' -count=1
```

Expected: FAIL because helpers do not exist.

**Step 3: Implement helpers**

Create helpers:

```go
func redactHeaders(headers http.Header) map[string][]string
func marshalRedactedHeaders(headers http.Header) string
func captureBounded(data []byte, maxBytes int) (body string, truncated bool)
```

Sensitive names:

- `authorization`
- `cookie`
- `set-cookie`
- `x-api-key`
- `api-key`
- `anthropic-api-key`

**Step 4: Verify**

Run:

```bash
go test ./services -run 'TestRawLog|TestRedact' -count=1
```

Expected: PASS.

---

### Task 4: Capture Custom Provider Request And Response Payloads

**Files:**
- Modify: `services/providerrelay.go`
- Modify: `services/upstream_http.go`
- Test: `services/providerrelay_test.go`

**Step 1: Write failing test**

Create a test that:

- Enables `CaptureRawLogs`.
- Sends a Codex `/responses` request through the relay.
- Asserts `request_log_payload` stores request body and response body.
- Asserts `Authorization` is not stored.

**Step 2: Run test to verify failure**

Run:

```bash
go test ./services -run TestProxyHandlerStoresRawPayloadWhenEnabled -count=1
```

Expected: FAIL because no payload is inserted.

**Step 3: Implement capture plumbing**

Introduce a request-scoped `rawLogCapture` attached to `ReqeustLog`.

Capture:

- inbound request headers/body before model replacement
- client-facing response headers/body in `writeUpstreamResponse`
- upstream error body on non-2xx responses

Insert payload only after summary log insert succeeds and returns `log_id`.

**Step 4: Verify**

Run:

```bash
go test ./services -run TestProxyHandlerStoresRawPayloadWhenEnabled -count=1
```

Expected: PASS.

---

### Task 5: Capture DeepSeek Adapter Upstream Payloads

**Files:**
- Modify: `services/deepseek_adapter.go`
- Test: `services/deepseek_adapter_test.go`

**Step 1: Write failing test**

Create a test that:

- Enables `CaptureRawLogs`.
- Sends a Codex `/responses` request to a DeepSeek provider.
- Asserts payload contains:
  - original Codex request body
  - final OpenAI Responses-compatible response body
  - translated DeepSeek request body
  - raw DeepSeek response body

**Step 2: Run test to verify failure**

Run:

```bash
go test ./services -run TestProviderRelayStoresDeepSeekRawPayloads -count=1
```

Expected: FAIL until adapter capture is implemented.

**Step 3: Implement adapter capture**

In `forwardDeepSeekCodexRequest`, capture `translatedBody`, upstream response body, and translated response body when raw log capture is enabled.

**Step 4: Verify**

Run:

```bash
go test ./services -run TestProviderRelayStoresDeepSeekRawPayloads -count=1
```

Expected: PASS.

---

### Task 6: Add Log Detail API

**Files:**
- Modify: `services/logservice.go`
- Test: `services/logservice_test.go` or `services/providerrelay_test.go`
- Modify: `frontend/src/services/logs.ts`

**Step 1: Write failing API test**

Add a test for:

```go
LogService.GetRequestLogPayload(id int64)
```

Expected behavior:

- Existing payload returns detail object.
- Missing payload returns empty detail without fatal error.

**Step 2: Run test to verify failure**

Run:

```bash
go test ./services -run TestLogServiceGetRequestLogPayload -count=1
```

Expected: FAIL because API does not exist.

**Step 3: Implement API**

Add `RequestLogPayload` DTO and `GetRequestLogPayload`.

**Step 4: Add TypeScript client**

Add `RequestLogPayload` type and `fetchRequestLogPayload(id)`.

**Step 5: Verify**

Run:

```bash
go test ./services -run TestLogServiceGetRequestLogPayload -count=1
npm test
```

Expected: PASS.

---

### Task 7: Add Logs Page Detail Drawer

**Files:**
- Modify: `frontend/src/components/Logs/Index.vue`
- Modify: `frontend/src/style.css`
- Modify: `frontend/src/locales/zh.json`
- Modify: `frontend/src/locales/en.json`

**Step 1: Add UI state**

Track:

- selected log ID
- payload loading state
- loaded payload detail
- active tab: request, response, upstream

**Step 2: Add row action**

Add a `详情` button to each row. Clicking it calls `fetchRequestLogPayload(id)` and opens a drawer.

**Step 3: Render payloads**

Render pretty JSON when parsing succeeds. Render plain text otherwise. Show `已截断` when truncation flags are set.

**Step 4: Verify**

Run:

```bash
npm test
npm run build
```

Expected: PASS.

---

### Task 8: Full Verification

Run:

```bash
go test ./... -count=1
go vet ./...
npm test
npm run build
git diff --check
```

Expected:

- All commands exit 0.
- Existing macOS linker warnings are acceptable.
- Existing Vite chunk-size warning is acceptable.

### Task 9: Manual Smoke Test

Run the app in dev mode and confirm:

- Raw capture is off by default.
- After enabling raw capture and restarting if needed, new requests show detail payloads.
- Headers are redacted.
- Large responses are truncated and marked.
- DeepSeek Codex logs show upstream conversion payloads.

Do not use a real API key in screenshots, logs, or final output.
