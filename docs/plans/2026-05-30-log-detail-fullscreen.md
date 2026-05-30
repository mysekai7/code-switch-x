# Log Detail Fullscreen Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the constrained raw log drawer with a fullscreen modal-style detail view that gives request, response, and upstream payloads enough reading space.

**Architecture:** Keep the existing `Logs/Index.vue` data flow and payload loading functions. Change only the detail view markup and global CSS classes so the same state opens a fullscreen overlay with aligned header, tabs, and content grid.

**Tech Stack:** Vue 3 SFC, existing i18n strings, existing `BaseButton`, global CSS in `frontend/src/style.css`, Node test runner for source-level frontend tests.

---

### Task 1: Add Layout Regression Test

**Files:**
- Modify: `frontend/src/data/providerTypes.test.mjs`
- Test target: `frontend/src/components/Logs/Index.vue`, `frontend/src/style.css`

**Step 1: Write failing test**

Add a test that reads the logs component and global CSS, asserting:
- details container uses `raw-log-modal` instead of `raw-log-drawer`
- CSS contains fullscreen sizing through `width: min(1440px, calc(100vw - 32px))`
- CSS contains `grid-template-rows: auto auto 1fr`
- raw payload `pre` no longer uses `max-height: 360px`

**Step 2: Run test to verify it fails**

Run: `cd frontend && npm test`
Expected: FAIL because current implementation still uses `raw-log-drawer` and drawer CSS.

### Task 2: Implement Fullscreen Detail View

**Files:**
- Modify: `frontend/src/components/Logs/Index.vue`
- Modify: `frontend/src/style.css`

**Step 1: Replace drawer markup**

Change `<aside class="raw-log-drawer">` to `<section class="raw-log-modal">` and wrap body states in a `raw-log-body` region so header, tabs, and content align consistently.

**Step 2: Update CSS**

Make `.raw-log-backdrop` center the modal. Make `.raw-log-modal` near-fullscreen, use `display: grid`, and align sections with `grid-template-rows: auto auto 1fr`. Make `.raw-log-content` fill remaining height and scroll as a single area. Remove the small `pre` max-height.

**Step 3: Mobile behavior**

At narrow widths, use full viewport with no outer margin and single-column payload blocks.

### Task 3: Verify

Run:
- `cd frontend && npm test`
- `cd frontend && npm run build -q`
- `git diff --check`

Expected: tests and build exit 0; Vite chunk-size warning is acceptable.
