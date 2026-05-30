# App Identity Isolation Design

**Goal:** Treat this application as the independent `code-switch-x` app, so it does not overwrite data, backups, or autostart entries from any earlier installation on the same Mac.

## Scope

Change application-owned identifiers:
- Runtime data directories: `.code-switch-x` and `.codex-switch-x`.
- SQLite DB path under the application-owned data directory.
- macOS LaunchAgent label and plist filename.
- Windows/Linux autostart names used by this app.
- Build/package identity and display name where it identifies this app itself.
- User-facing text that documents the application-owned data directory.

Keep Claude/Codex target configuration locations compatible:
- Keep official target directories such as `.claude`, `.codex`, and `.claude.json` unchanged.
- Keep Wails Go module binding namespace unchanged to avoid unrelated frontend regeneration risk.

## Design

Use centralized application identity constants in the services package. App-owned runtime directories, package names, desktop entries, LaunchAgent labels, and backup filenames all use the `code-switch-x` identity.

For macOS autostart, the LaunchAgent label becomes `com.codeswitch-x.app` and the plist path becomes `~/Library/LaunchAgents/com.codeswitch-x.app.plist`. This avoids replacing any earlier app LaunchAgent.

For provider, MCP, skill, app settings, and request log storage, the path moves to the `-x` directories. This is the main runtime isolation boundary and prevents the app from writing to an existing installation's application-owned JSON files or SQLite DB.

Build metadata is updated where it identifies this app package itself: product name, bundle identifier, executable/app name, Linux desktop entry, Windows metadata, and packaging names. The Go module and generated frontend binding imports remain unchanged because they are compile-time internals, not runtime storage identity.

Claude/Codex backup filenames, injected proxy provider keys, and placeholder auth token values also use `code-switch-x`. Official Claude/Codex directories remain unchanged because those tools read their configuration from those locations.

## Testing

Add focused service tests that verify:
- Provider config path uses `.code-switch-x`.
- App settings path uses `.codex-switch-x`.
- macOS/Linux autostart paths and labels use the `-x` identifiers.
- Claude/Codex backup names and injected proxy markers use `code-switch-x`.
- Build metadata and release/download surfaces use `code-switch-x`.

Run focused tests first to prove the current implementation fails, then implement the minimal changes and rerun `go test ./services -count=1`.
