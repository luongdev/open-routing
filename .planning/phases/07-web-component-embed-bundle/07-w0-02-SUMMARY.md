---
phase: 07-web-component-embed-bundle
plan: w0-02
subsystem: ui-theming
tags: [css, tailwind-v4, oklch, dark-mode, frankenstyle, design-tokens]
dependency_graph:
  requires: [w0-01]
  provides: [EMBER_PALETTE, COMPAT_OR_TOKENS, DARK_MODE_TOGGLE]
  affects:
    - web/packages/ui/src/styles/
    - web/packages/ui/src/themes/
    - web/packages/ui/src/components/shell/catalog-shell.ts
tech_stack:
  added:
    - Tailwind v4 @theme directive (OKLCh design tokens)
    - CSS .dark class-based dark mode (toggled via isDarkClassTheme helper)
  patterns:
    - CSS cascade layering (frankenstyle-kit → tailwindcss preflight → ember theme → compat)
    - Dual-prefix vars (--color-* for Tailwind utilities, bare --* and --uk-* for Frankenstyle internals)
key_files:
  created:
    - web/packages/ui/src/styles/ember-theme.css
    - web/packages/ui/src/styles/compat-or-tokens.css
  modified:
    - web/packages/ui/src/styles/frankenstyle.css
    - web/packages/ui/src/themes/index.ts
    - web/packages/ui/src/components/shell/catalog-shell.ts
decisions_made:
  - "D7-W0-02-A: Define vars in three parallel namespaces (--color-*, bare --*, --uk-*) so Tailwind utilities, Frankenstyle shadcn label fallbacks, and UK component internals all resolve to Ember OKLCh values"
  - "D7-W0-02-B: ember-light/ember-dark use empty inline token maps; dark mode driven by .dark class on <html> via isDarkClassTheme(); avoids duplicating 34 OKLCh values as JS strings"
  - "D7-W0-02-C: Frankenstyle uses --uk-* vars for component colors (not --color-* or bare --primary); --uk-primary/--uk-bg/--uk-card overridden in both :root and .dark blocks to adopt Ember palette"
metrics:
  duration: 11m
  completed_date: "2026-05-18T21:55:00Z"
---

# Phase 07 Plan W0-02: Ember OKLCh Palette via Tailwind v4 @theme Directives Summary

Ember OKLCh color palette defined via Tailwind v4 `@theme` directives with `.dark` class dark-mode toggle; compat layer aliases all `--or-color-*` tokens to new vars so 42 unmigrated components keep rendering.

## Frankenstyle CSS Var Convention

Frankenstyle v0.3.8 uses three parallel naming schemes:
- `--color-*`: Tailwind v4 `@theme`-registered tokens that generate utilities like `bg-primary`, `text-foreground`
- Bare `--primary`, `--background`, etc.: Legacy shadcn-compatibility fallbacks wrapped in `hsl(var(--primary))` — only in label/badge components. Since we use OKLCh values (not HSL channels), these are defined as direct OKLCh values; the `hsl()` wrapper in fallbacks will not produce correct output but these paths are only hit when `--uk-primary` is unset
- `--uk-*`: Frankenstyle's primary component token namespace (`--uk-primary`, `--uk-bg`, `--uk-card`, `--uk-danger`, etc.) — these are what Frankenstyle components actually read

All three namespaces are defined in `ember-theme.css` in both `:root` (light) and `.dark` (dark) blocks.

## Files Created/Modified

| File | Change |
|------|--------|
| `web/packages/ui/src/styles/ember-theme.css` | Created — 17-token Tailwind `@theme` block + `:root` and `.dark` blocks for all three var namespaces |
| `web/packages/ui/src/styles/compat-or-tokens.css` | Created — 23 `--or-color-*` aliases pointing to `--color-*` vars |
| `web/packages/ui/src/styles/frankenstyle.css` | Updated — adds 2 `@import` lines for new files |
| `web/packages/ui/src/themes/index.ts` | Updated — extends `ThemeName` union, adds `isDarkClassTheme()` helper |
| `web/packages/ui/src/components/shell/catalog-shell.ts` | Updated — `_applyTheme()` toggles `.dark` on `<html>` via `isDarkClassTheme()` |

## Tests Run + Results

```
Test Files  29 passed (29)
Tests       165 passed (165)
Duration    1.76s
```

All 165 pre-existing tests pass. TypeScript typecheck: 0 errors.

## Dark Mode Toggle Verification

Build output (`web/apps/admin/dist/assets/index-C16wthKx.css`) confirms:

- Light (`:root`): `--color-background: oklch(98.5% .005 60)` at byte ~730500
- Dark (`.dark`): `--color-background: oklch(10% .005 60)` in `.dark` block at byte 730705
- Our `.dark` block comes **after** Frankenstyle's `.dark` block (byte 20367) — cascade order correct, our values win
- `--or-color-app-bg: var(--color-background)` confirms compat alias resolves to Ember OKLCh value

Toggling `document.documentElement.classList.toggle('dark')` will instantly flip all 17 tokens in all three namespaces via CSS cascade.

## Compat Layer Status

All 23 `--or-color-*` tokens from existing `or-light`/`or-dark`/`or-brand` themes are aliased in `compat-or-tokens.css`. The `:root, .dark` selector applies at document root level. Components inside the shell's Shadow DOM that were previously getting inline token values via `_applyTheme()` will continue to do so for `or-light/or-dark/or-brand` (inline styles on the host element have higher specificity than `:root` compat aliases). The compat layer acts as a document-level fallback for any components rendered outside the shell.

## Deviations from Plan

### Auto-additions (Rule 2)

**1. [Rule 2 - Missing Critical Functionality] Added --uk-* namespace overrides**
- **Found during:** Task 1 (Frankenstyle CSS var analysis)
- **Issue:** Plan template only specified `--color-*` vars in `@theme`. Frankenstyle's actual components read `--uk-primary`, `--uk-bg`, `--uk-card`, `--uk-danger` — not `--color-*`. Without `--uk-*` overrides, Frankenstyle buttons/cards/badges would render in Frankenstyle's default dark gray palette, not Ember coral.
- **Fix:** Added `--uk-bg`, `--uk-bg-f`, `--uk-card`, `--uk-card-f`, `--uk-drop`, `--uk-drop-f`, `--uk-primary`, `--uk-primary-f`, `--uk-danger`, `--uk-danger-f` to both `:root` and `.dark` blocks in `ember-theme.css`
- **Files modified:** `web/packages/ui/src/styles/ember-theme.css`

**2. [Rule 2 - Missing Critical Functionality] Added isDarkClassTheme() + wired into catalog-shell**
- **Found during:** Task 4 (themes/index.ts update)
- **Issue:** Plan specified adding `ember-dark` to `ThemeName` but did not specify the mechanism for toggling `.dark` on `<html>`. Without this wiring, switching to `ember-dark` would do nothing (empty token map + no class toggle = no visual change).
- **Fix:** Added `isDarkClassTheme()` helper to `themes/index.ts` and wired `document.documentElement.classList.toggle('dark', isDarkClassTheme(this.theme))` into `catalog-shell.ts` `_applyTheme()`.
- **Files modified:** `web/packages/ui/src/themes/index.ts`, `web/packages/ui/src/components/shell/catalog-shell.ts`

## Known Stubs

None — all tokens are fully defined with OKLCh values. No placeholder text or empty data sources.

## Self-Check: PASSED

Files created/exist:
- `web/packages/ui/src/styles/ember-theme.css` — FOUND
- `web/packages/ui/src/styles/compat-or-tokens.css` — FOUND

Commit exists:
- `e833167` — FOUND (feat(ui): Ember OKLCh palette via Tailwind v4 @theme directives)
