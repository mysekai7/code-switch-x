# Claude Thinking Rectifier Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a Claude-only thinking rectifier, default enabled, with an app settings switch to disable it when it causes compatibility issues.

**Architecture:** Borrow ccswitch's reactive rectifier shape: only after a Claude upstream returns a non-2xx thinking-related error, mutate the same request and retry the same provider once. Keep this feature out of Codex and non-Claude proxy paths.

**Tech Stack:** Go, Gin, `gjson`/`sjson` or standard `encoding/json`, existing Wails/Vue settings page, existing Go and frontend test/build commands.

---

### Task 1: App Settings Field

**Files:**
- Modify: `services/appsettings.go`
- Modify: `services/appsettings_test.go`
- Modify: `frontend/src/services/appSettings.ts`
- Modify: `frontend/src/components/General/Index.vue`
- Modify: `frontend/src/locales/zh.json`
- Modify: `frontend/src/locales/en.json`

**Steps:**
1. Write failing Go test that default app settings enable Claude thinking rectifier.
2. Add `ClaudeThinkingRectifier bool json:"claude_thinking_rectifier"` to `AppSettings` and normalize missing persisted files to true.
3. Add frontend type/default state and a settings switch with explanatory copy.
4. Run focused Go test and frontend type check/build.

### Task 2: Rectifier Helpers

**Files:**
- Create: `services/claude_thinking_rectifier.go`
- Create/modify tests: `services/claude_thinking_rectifier_test.go`

**Steps:**
1. Write failing tests for signature error detection and request mutation.
2. Write failing tests for budget error detection and request mutation.
3. Implement minimal helpers:
   - `shouldRectifyClaudeThinkingSignature(errorBody []byte) bool`
   - `rectifyClaudeThinkingSignatureRequest(body []byte) ([]byte, bool, error)`
   - `shouldRectifyClaudeThinkingBudget(errorBody []byte) bool`
   - `rectifyClaudeThinkingBudgetRequest(body []byte) ([]byte, bool, error)`
4. Verify focused tests pass.

### Task 3: Claude Relay Retry

**Files:**
- Modify: `services/providerrelay.go`
- Modify: `services/providerrelay_test.go`

**Steps:**
1. Write failing integration test where first Claude upstream response rejects invalid thinking signature, then retry succeeds with cleaned request.
2. Write failing integration test where app setting disables the rectifier and no retry occurs.
3. Write failing integration test for budget error retry.
4. Implement same-provider one-shot retry only for `kind == "claude"` and only when `AppSettings.ClaudeThinkingRectifier` is true.
5. Preserve existing failover behavior for non-rectified errors.

### Task 4: Verification

**Commands:**
- `go test ./services -run 'TestAppSettings|TestClaudeThinking|TestProxyHandler.*Claude' -count=1`
- `go test ./services -count=1`
- `go test ./... -count=1`
- `go vet ./...`
- `cd frontend && npm run build -q`
- `git diff --check`
