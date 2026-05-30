# Proxy Protocol Compatibility V2 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Bring Claude auth and Codex DeepSeek protocol adaptation closer to current Claude/Codex proxy behavior without adding Gemini or broad Claude protocol conversion.

**Architecture:** Add explicit auth strategy metadata to providers, route endpoints through an endpoint kind, and expand the DeepSeek adapter into a full Responses/Chat conversion layer. Keep native Claude and native Codex providers as passthrough.

**Tech Stack:** Go, Gin, `gjson`, `sjson`, standard library HTTP/SSE handling, existing `go test` suite.

---

### Task 1: Claude Auth Strategy

**Files:**
- Modify: `services/providerservice.go`
- Modify: `services/providerrelay.go`
- Test: `services/providerrelay_test.go`

**Step 1: Write failing tests**

Add tests for:
- Anthropic official URL sends `x-api-key`.
- Relay URL sends `Authorization: Bearer`.
- Explicit `authMode: bearer` overrides Anthropic URL.
- Explicit `authMode: anthropic` overrides relay URL.

**Step 2: Run red tests**

Run: `go test ./services -run 'TestProviderRelay(ClaudeAuth|Forward)' -count=1`

Expected: FAIL because `authMode` and Claude header selection do not exist.

**Step 3: Implement**

- Add `AuthMode string json:"authMode,omitempty"` to `Provider`.
- Add method resolving `anthropic` / `bearer` / `auto`.
- In `forwardRequest`, set Claude headers via the resolved mode.
- Preserve client `anthropic-version` if provided; add a safe default only when missing and auth mode is Anthropic.

**Step 4: Verify green**

Run: `go test ./services -run 'TestProviderRelay(ClaudeAuth|Forward)' -count=1`

Expected: PASS.

### Task 2: Route Compatibility

**Files:**
- Modify: `services/providerrelay.go`
- Test: `services/providerrelay_test.go`
- Test: `services/deepseek_adapter_test.go`

**Step 1: Write failing tests**

Add tests for:
- `/claude/v1/messages` reaches Claude upstream `/v1/messages`.
- `/codex/v1/responses` and `/v1/v1/responses` reach Responses handling.
- `/codex/v1/responses/compact` and `/v1/v1/responses/compact` reach compact handling.
- `/chat/completions`, `/v1/chat/completions`, `/codex/v1/chat/completions` are registered.
- DeepSeek Chat route forwards to `/chat/completions` without trying to parse Responses input.

**Step 2: Run red tests**

Run: `go test ./services -run 'TestProviderRelay.*Route|TestProviderRelay.*Chat' -count=1`

Expected: FAIL on missing routes / wrong DeepSeek routing.

**Step 3: Implement**

- Register additive aliases.
- Add endpoint mode to `forwardRequest` or infer it from endpoint.
- Only run `forwardDeepSeekCodexRequest` for Responses endpoints.
- Forward Chat endpoints directly to `/chat/completions`.

**Step 4: Verify green**

Run: `go test ./services -run 'TestProviderRelay.*Route|TestProviderRelay.*Chat' -count=1`

Expected: PASS.

### Task 3: DeepSeek Responses -> Chat Request Conversion

**Files:**
- Modify: `services/deepseek_adapter.go`
- Test: `services/deepseek_adapter_test.go`

**Step 1: Write failing tests**

Add tests for:
- `top_p`, `stop`, `metadata`, `user`, `parallel_tool_calls`, `response_format`, `seed`, `service_tier`, and `stream_options` pass through.
- `developer` and `system` messages collapse into system-compatible Chat messages.
- `latest_reminder` maps to user.
- `input_image` maps to Chat `image_url` when present.
- Unsupported content is not silently lost; it is either converted to text JSON or rejected with a clear error.

**Step 2: Run red tests**

Run: `go test ./services -run 'TestDeepSeek.*Request|TestTranslateResponses' -count=1`

Expected: FAIL on missing pass-through fields and content handling.

**Step 3: Implement**

- Expand request struct or switch to map-based request assembly for flexible field pass-through.
- Canonicalize tool arguments.
- Convert content parts to string or Chat multimodal arrays where supported.
- Keep DeepSeek-specific reasoning handling.

**Step 4: Verify green**

Run: `go test ./services -run 'TestDeepSeek.*Request|TestTranslateResponses' -count=1`

Expected: PASS.

### Task 4: DeepSeek Chat -> Responses Response and Error Conversion

**Files:**
- Modify: `services/deepseek_adapter.go`
- Test: `services/deepseek_adapter_test.go`

**Step 1: Write failing tests**

Add tests for:
- `finish_reason: length` maps to `status: incomplete` with `incomplete_details`.
- `created` maps to `created_at`.
- `refusal` content maps to Responses refusal part.
- `usage.prompt_tokens_details.cached_tokens` maps to `input_tokens_details.cached_tokens`.
- Non-standard error body maps to `{"error":{"message","type","code","param"}}`.

**Step 2: Run red tests**

Run: `go test ./services -run 'TestDeepSeek.*Response|TestDeepSeek.*Error' -count=1`

Expected: FAIL.

**Step 3: Implement**

- Complete response field mapping.
- Add `chatErrorToResponsesError`.
- Use error normalization in `forwardDeepSeekCodexRequest`.

**Step 4: Verify green**

Run: `go test ./services -run 'TestDeepSeek.*Response|TestDeepSeek.*Error' -count=1`

Expected: PASS.

### Task 5: Real Chat SSE -> Responses SSE Conversion

**Files:**
- Modify: `services/deepseek_adapter.go`
- Test: `services/deepseek_adapter_test.go`
- Possibly modify: `services/upstream_http.go`

**Step 1: Write failing tests**

Add tests for:
- Chat SSE text deltas become `response.output_text.delta`.
- Chat SSE reasoning deltas become reasoning summary deltas.
- Chat SSE tool call argument deltas become `response.function_call_arguments.delta`.
- Final usage chunk is reflected in `response.completed.response.usage`.

**Step 2: Run red tests**

Run: `go test ./services -run 'TestDeepSeek.*SSE|TestProviderRelaySynthesizesResponsesStreamForDeepSeekCodex' -count=1`

Expected: FAIL for real Chat SSE conversion cases.

**Step 3: Implement**

- Send upstream `stream: true` when client requests stream.
- Parse Chat SSE chunks.
- Emit Responses SSE events with stable item IDs and final completed event.
- Keep existing synthetic fallback for non-SSE upstreams if needed.

**Step 4: Verify green**

Run: `go test ./services -run 'TestDeepSeek.*SSE|TestProviderRelaySynthesizesResponsesStreamForDeepSeekCodex' -count=1`

Expected: PASS.

### Task 6: Full Verification

**Files:**
- No new production files expected.

**Step 1: Run service tests**

Run: `go test ./services -count=1`

Expected: PASS.

**Step 2: Run full Go tests**

Run: `go test ./... -count=1`

Expected: PASS.

**Step 3: Static checks**

Run: `go vet ./...`

Expected: PASS.

**Step 4: Whitespace check**

Run: `git diff --check`

Expected: no output, exit 0.
