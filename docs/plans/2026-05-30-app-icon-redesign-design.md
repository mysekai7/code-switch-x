# App Icon Redesign Design

## Goal

Replace the generic `AI` app icon with a CodeSwitchX-specific icon that communicates local proxy routing, multiple providers, and protocol switching.

## Approved Direction

Use the "routing hub X" concept:

- A macOS-style rounded square base.
- A dark graphite-blue background to distinguish the app from generic AI tools.
- Two crossed route bands forming an abstract `X`.
- Four endpoint nodes connected through a central hub.
- No text in the icon, so it stays readable at small sizes.

## Visual Specification

- Canvas: 1024x1024 RGBA.
- Background: rounded rectangle with transparent corners.
- Palette:
  - Background: `#0f172a` to `#111827`
  - Primary route: `#22d3ee`
  - Secondary route: `#34d399`
  - Highlight: `#e5f9ff`
- Shape language:
  - Route lines use rounded caps and soft glow.
  - Endpoint nodes represent providers.
  - The center hub represents the local proxy and protocol conversion point.

## Asset Strategy

Keep `build/appicon.png` as the Wails icon source. Add a small Go standard-library generator so the icon can be reproduced without new third-party dependencies. The generated source PNG is then used by the existing Wails task to regenerate platform icons.

## Validation

- Verify generated PNG files are 1024x1024 RGBA.
- Verify `wails3 generate icons` refreshes macOS `.icns` and Windows `.ico`.
- Verify the git diff only contains the icon assets, generator, and plan docs.
