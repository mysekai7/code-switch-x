# App Identity Isolation Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Treat the app as `code-switch-x` for all app-owned runtime, backup, and package identifiers while keeping official Claude/Codex config locations compatible.

**Architecture:** Use shared application identity constants in `services`, replace app-owned hard-coded identifiers with those constants, and update build/package metadata. Keep official Claude/Codex settings directories and Wails module bindings unchanged.

**Tech Stack:** Go services, Wails build metadata, TOML/YAML/JSON/plist packaging files, Go unit tests.

---

### Task 1: Add failing tests for application-owned runtime identifiers

**Files:**
- Create: `services/appidentity_test.go`
- Modify: none

**Step 1: Write failing tests**

Add tests in package `services` that assert:
- `providerFilePath("claude")` returns a path containing `.code-switch-x`.
- `NewAppSettingsService(nil).path` contains `.codex-switch-x`.
- `NewAutoStartService().getDarwinPlistPath()` ends with `Library/LaunchAgents/com.codeswitch-x.app.plist`.
- macOS LaunchAgent content helper, if introduced, contains `<string>com.codeswitch-x.app</string>`.
- `getLinuxDesktopPath()` ends with `autostart/codeswitch-x.desktop`.

**Step 2: Run tests to verify they fail**

Run: `go test ./services -run 'TestAppIdentity|TestProviderFilePath|TestAppSettingsPath|TestAutoStart' -count=1`

Expected: FAIL until every app-owned runtime path and autostart identifier uses `code-switch-x`.

### Task 2: Implement centralized application identity constants

**Files:**
- Create: `services/appidentity.go`
- Modify: `services/providerservice.go`
- Modify: `services/providerrelay.go`
- Modify: `services/appsettings.go`
- Modify: `services/mcpservice.go`
- Modify: `services/skillservice.go`
- Modify: `services/autostartservice.go`

**Step 1: Add constants**

Define constants for:
- `appDataDirName = ".code-switch-x"`
- `appSettingsDirName = ".codex-switch-x"`
- app executable name `CodeSwitchX`
- app display name `Code Switch X`
- app bundle identifier `com.codeswitch-x.app`
- Linux desktop filename `codeswitch-x.desktop`

**Step 2: Replace hard-coded application-owned identifiers**

Use the constants in provider storage, relay DB storage, MCP/skill store dirs, app settings path, and autostart registry/plist/desktop paths and contents.

**Step 3: Run tests to verify they pass**

Run: `go test ./services -run 'TestAppIdentity|TestProviderFilePath|TestAppSettingsPath|TestAutoStart' -count=1`

Expected: PASS.

### Task 3: Update build/package metadata

**Files:**
- Modify: `Taskfile.yml`
- Modify: `build/config.yml`
- Modify: `build/darwin/Info.plist`
- Modify: `build/darwin/Info.dev.plist`
- Modify: Linux desktop metadata file
- Modify: `build/linux/nfpm/nfpm.yaml`
- Modify: `build/windows/info.json`
- Modify: `build/windows/nsis/wails_tools.nsh`
- Modify: `build/windows/wails.exe.manifest`
- Modify: `frontend/src/locales/en.json`
- Modify: `frontend/src/locales/zh.json`

**Step 1: Update app-owned names and IDs**

Replace package/display identifiers with `CodeSwitchX`, `Code Switch X`, `com.codeswitch-x.app`, and `codeswitch-x.desktop` as appropriate.

**Step 2: Keep compile-time bindings unchanged**

Do not change `go.mod`, generated frontend bindings, or Wails `Call.ByName` service names.

### Task 4: Verify Claude/Codex config markers use app identity

**Files:**
- Read: `services/claudesettings.go`
- Read: `services/codexsettings.go`

**Step 1: Check constants use `code-switch-x`**

Verify these use `code-switch-x`:
- Claude backup filename and auth token value.
- Codex backup filenames, provider key, and auth token value.

**Step 2: Run grep checks**

Run: `rg -n 'code-switch-x|codeswitch-x|CodeSwitchX|Code Switch X|com\.codeswitch-x' services/claudesettings.go services/codexsettings.go`

Expected: only Claude/Codex proxy marker and backup constants.

### Task 5: Full verification and review

**Files:**
- All changed files

**Step 1: Run service tests**

Run: `go test ./services -count=1`

Expected: PASS.

**Step 2: Run formatting/checks**

Run: `gofmt -w services/appidentity.go services/appidentity_test.go services/providerservice.go services/providerrelay.go services/appsettings.go services/mcpservice.go services/skillservice.go services/autostartservice.go`

Run: `git diff --check`

Expected: no output/errors.

**Step 3: Review diff**

Run: `git diff --stat` and inspect changed files for unintended Claude/Codex config changes.
