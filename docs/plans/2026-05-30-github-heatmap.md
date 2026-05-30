# GitHub Heatmap Visual Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Restyle the home page usage heatmap to look like GitHub's contribution graph without changing the existing data model or backend calls.

**Architecture:** Keep `frontend/src/data/usageHeatmap.ts` and `services/logservice.go` unchanged. Update `frontend/src/components/Main/Index.vue` to use a GitHub-style heatmap header/body/footer structure and update `.contrib-*` CSS in `frontend/src/style.css`.

**Tech Stack:** Vue 3, TypeScript, CSS, Node `node:test`.

---

### Task 1: Add Layout Regression Test

**Files:**
- Modify: `frontend/src/components/Main/mainSettingsVisibility.test.mjs`

**Step 1: Write the failing test**

Add assertions that `Main/Index.vue` renders:

- `contrib-calendar`
- `contrib-grid-shell`
- `contrib-footer`
- `contrib-help`
- `contrib-intensity`

Add assertions that `frontend/src/style.css` contains GitHub-like CSS:

- `.contrib-wall` with compact border radius and a light card background variable.
- `.contrib-cell` fixed `10px` square dimensions.
- `.contrib-cell:hover` without transform scaling.
- `.contrib-footer` using flex layout.
- `.contrib-intensity` aligned to the right.

**Step 2: Run test to verify it fails**

Run:

```bash
cd frontend && npm test
```

Expected: FAIL because the new template/CSS structure does not exist yet.

### Task 2: Update Template Structure

**Files:**
- Modify: `frontend/src/components/Main/Index.vue`

**Step 1: Implement minimal template changes**

Inside `.contrib-wall`:

- Wrap the heatmap grid in `.contrib-calendar`.
- Wrap the grid itself in `.contrib-grid-shell`.
- Move the legend to a bottom `.contrib-footer`.
- Keep the existing `v-for`, `intensityClass`, and tooltip event handlers.

**Step 2: Run test**

Run:

```bash
cd frontend && npm test
```

Expected: still FAIL until CSS is updated.

### Task 3: Update GitHub-Like CSS

**Files:**
- Modify: `frontend/src/style.css`

**Step 1: Implement CSS**

Update `.contrib-*` styles:

- GitHub-like light surface and border.
- Compact 10px cells with 2px radius and 3px gaps.
- No hover scale; use outline or subtle border.
- Footer with help text on the left and Less/More legend on the right.
- Dark theme variables remain readable.

**Step 2: Run test**

Run:

```bash
cd frontend && npm test
```

Expected: PASS.

### Task 4: Verify Build

**Files:**
- No code changes.

**Step 1: Run build**

```bash
cd frontend && npm run build -q
```

Expected: exit 0. Existing Vite chunk warning is acceptable.

**Step 2: Run diff check**

```bash
git diff --check
```

Expected: exit 0.
