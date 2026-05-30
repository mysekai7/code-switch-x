# App Icon Redesign Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the current generic app icon with a CodeSwitchX-specific "routing hub X" icon.

**Architecture:** Generate a 1024x1024 PNG icon with Go standard-library image APIs, then reuse the existing Wails icon generation task to derive macOS and Windows icons. The source PNG remains `build/appicon.png`, matching the current build pipeline.

**Tech Stack:** Go standard library image packages, Wails 3 task runner, existing `build/Taskfile.yml` icon generation task.

---

### Task 1: Add Reproducible Icon Generator

**Files:**
- Create: `scripts/generate_icon.go`
- Modify: `build/appicon.png`
- Modify: `assets/icon.png`
- Modify: `assets/icon-dark.png`

**Step 1: Implement the generator**

Create a Go command that draws:

- Rounded-square dark graphite-blue background.
- Two glowing diagonal route bands forming an `X`.
- Four provider endpoint nodes.
- One central proxy hub node.

Use supersampling and downsampling to reduce jagged edges.

**Step 2: Run the generator**

Run:

```bash
go run ./scripts/generate_icon.go
```

Expected:

- `build/appicon.png` updated.
- `assets/icon.png` updated.
- `assets/icon-dark.png` updated.

### Task 2: Regenerate Platform Icons

**Files:**
- Modify: `build/darwin/icons.icns`
- Modify: `build/windows/icon.ico`

**Step 1: Run the existing Wails icon task**

Run:

```bash
wails3 task common:generate:icons
```

Expected:

- `build/darwin/icons.icns` updated.
- `build/windows/icon.ico` updated.

### Task 3: Verify Assets

**Step 1: Check file types**

Run:

```bash
file assets/icon.png assets/icon-dark.png build/appicon.png build/darwin/icons.icns build/windows/icon.ico
```

Expected:

- PNG files report `1024 x 1024` RGBA.
- `.icns` reports a macOS icon.
- `.ico` reports Windows icon resources.

**Step 2: Check repository diff**

Run:

```bash
git diff --check
git status --short
```

Expected:

- No whitespace errors.
- Only expected files changed.
