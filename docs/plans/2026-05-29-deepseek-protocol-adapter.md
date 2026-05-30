# DeepSeek Protocol Adapter Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a DeepSeek provider type that adapts Codex OpenAI Responses API requests to DeepSeek Chat Completions while preserving custom providers as pass-through.

**Architecture:** Keep provider selection in `ProviderRelayService`, add a small adapter boundary selected by `kind + providerType`. Phase 1 implements Codex + DeepSeek only: Responses request to Chat Completions request, and Chat Completions response or SSE back to Responses-compatible output/events. Claude + DeepSeek remains pass-through to DeepSeek's official Anthropic-compatible endpoint.

**Tech Stack:** Go standard library, existing Gin relay, `gjson`/`sjson`, existing provider JSON persistence, current Vue provider type UI.

---

### Task 1: Persist Provider Type Separately

**Files:**
- Modify: `services/providerservice.go`
- Modify: `frontend/src/data/cards.ts`
- Modify: `frontend/src/components/Main/Index.vue`
- Test: `services/providerservice_test.go`
- Test: `frontend/src/data/providerTypes.test.mjs`

**Steps:**
1. Write failing tests that `Provider.ProviderType` falls back to legacy `Icon` and defaults to `custom`.
2. Add `providerType,omitempty` to `Provider` and frontend card/form types.
3. Keep existing `icon` for display compatibility; use `providerType` for protocol behavior.
4. Re-run focused tests.

### Task 2: Add DeepSeek Responses Adapter Unit Tests

**Files:**
- Create: `services/deepseek_adapter.go`
- Create: `services/deepseek_adapter_test.go`

**Steps:**
1. Write failing tests for non-stream Responses request to DeepSeek chat request.
2. Write failing tests for DeepSeek non-stream chat response to Responses response.
3. Write failing tests for DeepSeek SSE chunks to Responses SSE events.
4. Implement minimal adapter functions.

### Task 3: Route Codex DeepSeek Providers Through Adapter

**Files:**
- Modify: `services/providerrelay.go`
- Test: `services/providerrelay_test.go`

**Steps:**
1. Write failing relay test proving a `deepseek` Codex provider receives `/chat/completions`, not `/responses`.
2. Write failing relay test proving the client receives Responses-shaped JSON/SSE.
3. Add adapter branch in `forwardRequest` only for `kind == "codex" && provider.ProviderKind() == "deepseek"`.
4. Keep `custom` pass-through unchanged.

### Task 4: Verification

**Commands:**
- `go test ./services -run 'TestDeepSeek|TestProvider' -count=1`
- `go test ./... -count=1`
- `go vet ./...`
- `cd frontend && npm test`
- `cd frontend && npm run build`
- `git diff --check`
