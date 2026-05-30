# Raw Request And Response Logs Design

**Goal:** Add optional raw request and response inspection to the logs page without slowing the existing request log list or persisting sensitive data by default.

**Decision:** Raw payload capture is disabled by default. Users enable it from app settings when they need debugging detail.

## Current State

- `ProviderRelayService` already reads the inbound request body before selecting providers.
- `request_log` stores summary fields only: platform, provider, model, status, token usage, duration, stream flag, and creation time.
- The logs page polls summary records every 30 seconds and renders a paginated table.
- Response forwarding is centralized enough to capture response bytes while streaming them to the client.

## Recommended Architecture

Store raw payloads in a separate `request_log_payload` table keyed by `request_log.id`.

Keep `request_log` lightweight so list queries remain fast. Add a detail API that loads raw payloads only when the user opens a specific log entry.

## Data Model

Create `request_log_payload`:

- `log_id INTEGER PRIMARY KEY`
- `request_headers TEXT`
- `request_body TEXT`
- `response_headers TEXT`
- `response_body TEXT`
- `upstream_request_body TEXT`
- `upstream_response_body TEXT`
- `request_truncated INTEGER DEFAULT 0`
- `response_truncated INTEGER DEFAULT 0`
- `created_at DATETIME DEFAULT CURRENT_TIMESTAMP`

Headers are stored only after redaction. Bodies are stored as captured text with a strict size cap.

## Capture Rules

- Default setting: `capture_raw_logs=false`.
- Maximum stored bytes per body: start with `262144` bytes.
- If a body exceeds the cap, store the prefix and mark the corresponding truncated flag.
- Capture only when the setting is enabled.
- Never store provider API keys.
- Always redact sensitive headers: `Authorization`, `Cookie`, `Set-Cookie`, `X-API-Key`, `api-key`, and case-insensitive equivalents.

## Protocol Adapter Behavior

For normal custom providers:

- `request_body`: inbound client request body.
- `response_body`: client-facing upstream response body.

For DeepSeek Codex conversion:

- `request_body`: original Codex `/responses` request.
- `response_body`: final OpenAI Responses-compatible body returned to Codex.
- `upstream_request_body`: translated DeepSeek `/chat/completions` request.
- `upstream_response_body`: raw DeepSeek response.

This gives users enough information to debug both client compatibility and provider protocol conversion.

## UI

Add a `Details` action on each log row. Opening details loads payload data by log ID and shows a drawer or modal with tabs:

- `Request`: redacted headers and body.
- `Response`: redacted headers and body.
- `Upstream`: shown only when upstream fields exist.

Render JSON bodies with pretty formatting when possible. Fall back to plain text for SSE or invalid JSON. Show a `truncated` badge when any payload is capped.

## Error Handling

- If payload capture fails, the primary proxy request must still succeed.
- If detail payload is missing, show an empty-state message instead of treating it as a fatal UI error.
- If response capture is truncated, continue streaming to the client unchanged.

## Risks

- Raw payloads may contain prompts, file content, tool output, or other private data. This is why capture is opt-in.
- SQLite size can grow quickly. A follow-up retention setting may be needed if users keep capture enabled.
- Streaming responses require bounded capture to avoid unbounded memory growth.

## Testing Strategy

- Unit test DB migration creates `request_log_payload`.
- Unit test raw logging is disabled by default.
- Unit test enabling capture stores request and response bodies with redacted headers.
- Unit test body truncation marks truncation flags.
- Unit test DeepSeek Codex stores both client-facing and upstream payloads.
- Frontend test or type check validates detail fetch typing and rendering state.
