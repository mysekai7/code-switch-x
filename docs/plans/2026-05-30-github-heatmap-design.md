# GitHub Heatmap Visual Design

## Goal

Update the home page usage heatmap so it visually matches GitHub's contribution graph style while preserving the existing data model and interactions.

## Current Implementation

- `frontend/src/data/usageHeatmap.ts` builds a 14-day hourly heatmap: 3 columns per day and 8 rows per column.
- `frontend/src/components/Main/Index.vue` renders the existing matrix and tooltip.
- `frontend/src/style.css` owns the `.contrib-*` visual treatment.

This is not GitHub's daily calendar data model. GitHub uses a year-scale contribution graph with weekday rows and month labels. This change intentionally keeps CodeSwitchX's existing hourly usage semantics.

## Chosen Approach

Use a GitHub visual skin only:

- Keep the current `UsageHeatmapWeek[]` data structure.
- Keep tooltip metrics, hover events, and backend `HeatmapStats` calls unchanged.
- Replace the current large responsive blocks and neon hover with GitHub-like compact cells.
- Use a bordered card, muted background, compact legend, and GitHub contribution green scale.
- Add a GitHub-style footer row with explanatory text on the left and `Less` / `More` legend on the right.

## Non-Goals

- Do not convert the heatmap to 365-day daily aggregation.
- Do not add month and weekday labels that would imply GitHub's calendar semantics.
- Do not change backend statistics queries.

## Acceptance Criteria

- The home heatmap appears as a compact GitHub-like contribution card.
- Existing heatmap data and tooltip behavior remain intact.
- Small screens can scroll horizontally without breaking the card.
- Tests cover the expected template and CSS structure.
