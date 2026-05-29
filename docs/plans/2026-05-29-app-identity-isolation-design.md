# App Identity Isolation Design

**Goal:** Change only this application's own runtime and packaging identifiers to the `-x` variant, so it does not overwrite data or autostart entries from an existing Code Switch installation on the same Mac.

## Scope

Change application-owned identifiers:
- Runtime data directories: `.code-switch-x` and `.codex-switch-x`.
- SQLite DB path under the application-owned data directory.
- macOS LaunchAgent label and plist filename.
- Windows/Linux autostart names used by this app.
- Build/package identity and display name where it identifies this app itself.
- User-facing text that documents the application-owned data directory.

Do not change Claude/Codex target configuration behavior:
- Keep official target directories such as `.claude`, `.codex`, and `.claude.json` unchanged.
- Keep Claude/Codex proxy provider keys, auth token values, and backup filenames unchanged.
- Keep Wails Go module binding namespace unchanged to avoid unrelated frontend regeneration risk.

## Design

Introduce centralized application identity constants in the services package. Existing services that currently hard-code `.code-switch`, `.codex-switch`, `CodeSwitch`, `codeswitch.desktop`, or `com.codeswitch.app` will consume those constants.

For macOS autostart, the LaunchAgent label becomes `com.codeswitch-x.app` and the plist path becomes `~/Library/LaunchAgents/com.codeswitch-x.app.plist`. This is enough to avoid replacing an existing `com.codeswitch.app.plist`.

For provider, MCP, skill, app settings, and request log storage, the path moves to the `-x` directories. This is the main runtime isolation boundary and prevents the app from writing to an existing installation's application-owned JSON files or SQLite DB.

Build metadata is updated only where it identifies this app package itself: product name, bundle identifier, executable/app name, Linux desktop entry, Windows metadata, and packaging names. The Go module and generated frontend binding imports remain `codeswitch/...` because they are compile-time internals, not runtime storage identity.

## Testing

Add focused service tests that verify:
- Provider config path uses `.code-switch-x` and not `.code-switch`.
- App settings path uses `.codex-switch-x` and not `.codex-switch`.
- macOS/Linux autostart paths and labels use the `-x` identifiers.
- Build metadata no longer contains the old package identifiers.

Run focused tests first to prove the current implementation fails, then implement the minimal changes and rerun `go test ./services -count=1`.
