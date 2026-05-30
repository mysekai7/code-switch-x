# Claude Upstream Error Normalization Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make Claude Code CLI receive recognizable Anthropic-style errors when Claude upstream providers return non-2xx responses such as 413.

**Architecture:** Keep successful Claude responses unchanged. For Claude upstream errors, normalize the response body to Anthropic error JSON and mark request-shape errors such as 413 as non-retryable so provider fallback does not hide the original error.

**Tech Stack:** Go `net/http`, existing Gin relay tests, existing provider relay flow.

---

### Task 1: Add Regression Tests

**Files:**
- Modify: `services/providerrelay_test.go`

**Step 1: Write the failing tests**

Add tests that verify:

- A Claude upstream `413` plain-text response returns HTTP 413 with Anthropic-style JSON:
  `{"type":"error","error":{"type":"request_too_large","message":"..."}}`.
- The same `413` does not fall back to a second provider.
- Existing Codex upstream error pass-through remains unchanged.

**Step 2: Run tests to verify failure**

Run:

```bash
go test ./services -run 'TestProxyHandler(FormatsClaudePlainText413AsAnthropicError|DoesNotFallbackClaudeNonRetryableUpstreamError)' -count=1
```

Expected: fail before implementation.

### Task 2: Implement Claude Error Normalization

**Files:**
- Modify: `services/providerrelay.go`

**Step 1: Add helper functions**

Add helpers for:

- Detecting Claude non-retryable upstream statuses: `400, 405, 406, 413, 414, 415, 422, 429, 501`.
- Mapping HTTP status to Anthropic error type, with `413 -> request_too_large`.
- Normalizing upstream body into Anthropic error JSON.

**Step 2: Use helpers in the relay loop**

When `kind == "claude"` and an upstream error is non-retryable, return it immediately instead of trying the next provider.

**Step 3: Preserve Codex behavior**

Do not apply Anthropic normalization to Codex responses.

### Task 3: Verify

Run:

```bash
go test ./services -run 'TestProxyHandler(FormatsClaudePlainText413AsAnthropicError|DoesNotFallbackClaudeNonRetryableUpstreamError)' -count=1
go test ./services
go test ./...
git diff --check
```

Expected: all commands exit 0.
