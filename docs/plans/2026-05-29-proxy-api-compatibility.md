# Proxy API Compatibility Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Improve Claude `/v1/messages` and Codex `/responses` compatibility with official API behavior using minimal, backward-compatible changes.

**Architecture:** Keep the existing provider selection and fallback pipeline. Preserve successful response passthrough, add upstream error capture so final failures can return the last upstream status, headers, and body. Add a Codex `/v1/responses` alias and fix usage parsing without changing provider authentication.

**Tech Stack:** Go 1.24, Gin, `github.com/daodao97/xgo/xrequest`, `gjson/sjson`, Go unit tests.

---

### Task 1: Upstream Error Passthrough

**Files:**
- Modify: `services/providerrelay.go`
- Test: `services/providerrelay_test.go`

**Step 1: Write failing tests**

Add tests for:
- A final upstream `429` response is written back as `429`, not converted to local `400`.
- Upstream response headers and JSON body are preserved.
- A later provider with no HTTP response does not discard the last upstream HTTP error response.

**Step 2: Verify RED**

Run:

```bash
GOPROXY=https://proxy.golang.org,direct GOMODCACHE=/private/tmp/codex-gomodcache GOCACHE=/private/tmp/codex-gocache go test ./services -run 'TestProxyHandlerReturnsLastUpstreamError|TestProxyHandlerKeepsLastUpstreamErrorWhenLaterProviderHasNoResponse' -count=1
```

Expected: tests fail because current code returns local `400` after provider failures.

**Step 3: Implement minimal code**

Add a small upstream error carrier used by `forwardRequest`. For non-2xx responses, read the upstream body and clone response headers. After all providers fail, if the last failure has an upstream response, write its headers, status, and body to `gin.Context.Writer`.

**Step 4: Verify GREEN**

Run the same focused test command. Expected: PASS.

### Task 2: Codex `/v1/responses` Alias

**Files:**
- Modify: `services/providerrelay.go`
- Test: `services/providerrelay_test.go`

**Step 1: Write failing test**

Add a route registration test proving `POST /v1/responses` reaches the same Codex proxy path as `POST /responses`.

**Step 2: Verify RED**

Run:

```bash
GOPROXY=https://proxy.golang.org,direct GOMODCACHE=/private/tmp/codex-gomodcache GOCACHE=/private/tmp/codex-gocache go test ./services -run TestRegisterRoutesSupportsOpenAIResponsesPath -count=1
```

Expected: FAIL with `404` before route alias exists.

**Step 3: Implement minimal code**

Register `router.POST("/v1/responses", prs.proxyHandler("codex", "/responses"))` while keeping existing `/responses`.

**Step 4: Verify GREEN**

Run the same focused test command. Expected: PASS.

### Task 3: Usage Parser Corrections

**Files:**
- Modify: `services/providerrelay.go`
- Test: `services/providerrelay_test.go`

**Step 1: Write failing tests**

Add tests for:
- Codex non-streaming response parses root-level `usage`.
- Claude cumulative `usage.output_tokens` does not double count repeated deltas.

**Step 2: Verify RED**

Run:

```bash
GOPROXY=https://proxy.golang.org,direct GOMODCACHE=/private/tmp/codex-gomodcache GOCACHE=/private/tmp/codex-gocache go test ./services -run 'TestCodexParseTokenUsageFromRootUsage|TestClaudeParseTokenUsageAvoidsCumulativeDoubleCount' -count=1
```

Expected: FAIL under current parser behavior.

**Step 3: Implement minimal code**

Read both `usage.*` and `response.usage.*` for Codex without double counting the same event. For Claude, keep additive parsing for message-level final usage but treat top-level cumulative output usage as max/overwrite semantics.

**Step 4: Verify GREEN**

Run focused tests, then `go test ./services -count=1` if dependency compilation completes in time.

### Task 4: Final Checks

**Files:**
- Verify: `services/providerrelay.go`
- Verify: `services/providerrelay_test.go`

**Step 1: Format**

Run:

```bash
gofmt -w services/providerrelay.go services/providerrelay_test.go
```

**Step 2: Test**

Run:

```bash
GOPROXY=https://proxy.golang.org,direct GOMODCACHE=/private/tmp/codex-gomodcache GOCACHE=/private/tmp/codex-gocache go test ./services -count=1
```

Expected: PASS. If it times out or fails due environment, report exact status.

**Step 3: Commit**

Do not commit unless the user explicitly asks. Repository instructions prohibit autonomous commits on the main workspace.
