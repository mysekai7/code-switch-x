# Proxy Protocol Compatibility V2 Design

## Goal

Improve Claude and Codex proxy compatibility while keeping this app scoped to Claude native Messages and Codex native Responses plus DeepSeek Chat adaptation.

## Scope

- Claude: add auth strategy handling for Anthropic official `x-api-key` versus relay `Authorization: Bearer`.
- Claude: add route aliases compatible with common proxy base URL layouts.
- Codex: complete DeepSeek Responses <-> Chat Completions conversion.
- Codex: add route aliases and Chat Completions passthrough routes.
- Codex: normalize Chat upstream errors into Responses-style errors when conversion is active.

## Explicit Non-Goals

- No Gemini protocol conversion.
- No Claude OpenAI/Gemini protocol conversion.
- No new third-party dependency.
- No unrelated provider UI redesign beyond fields required to represent auth strategy.

## Design

Claude auth should use an explicit-first strategy:

1. If a provider has an explicit auth mode, use it.
2. If the API URL is clearly Anthropic official, use `x-api-key`.
3. Otherwise keep Bearer auth for relay compatibility.

Codex DeepSeek conversion should align with the current OpenAI Responses and Chat Completions shapes used by Codex:

- Convert request messages, tools, tool choice, reasoning, and supported pass-through fields.
- Preserve `previous_response_id` tool call recovery.
- Support real Chat streaming conversion into Responses SSE.
- Normalize non-2xx Chat errors into Responses-style `error` envelopes.
- For local Chat Completions routes, do not apply Responses-to-Chat conversion; forward Chat requests to Chat upstreams.

Route compatibility should be additive. Existing routes remain unchanged.

## Testing

Use TDD with focused unit/integration tests:

- Claude auth mode selection and header injection.
- Claude/Codex route aliases.
- DeepSeek request pass-through field conversion.
- DeepSeek Chat streaming response conversion.
- DeepSeek Chat error normalization.
- Chat Completions passthrough does not run Responses conversion.
