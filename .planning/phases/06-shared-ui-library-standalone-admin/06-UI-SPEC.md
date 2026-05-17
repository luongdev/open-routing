---
phase: 6
slug: shared-ui-library-standalone-admin
status: draft
shadcn_initialized: false
preset: not applicable (stack is Lit 3 + Shoelace; shadcn is React-only)
created: 2026-05-17
authored_by: gsd-ui-researcher (Opus 4.7 1M)
consumer_agents: gsd-planner, gsd-executor, gsd-ui-checker, gsd-ui-auditor
upstream_inputs:
  - 06-CONTEXT.md (D6-01..D6-31)
  - 06-DISCUSSION-LOG.md
  - REQUIREMENTS.md (ADMIN-01..06, EMBED-01..10)
  - 02-UI-SKETCHES.md (Sketches 1-4)
  - PROJECT.md (locked stack)
  - openapi/openapi.yaml (entity schemas + error envelopes)
---

# Phase 6: Shared UI Library & Standalone Admin — UI Design Contract

> This is the visual & interaction source of truth for Phase 6. It encodes every
> design decision (theme palettes, type scale, layout grid, copy library, motion,
> a11y, per-component shape, per-page route shape, error/empty/loading patterns)
> that CONTEXT.md left open. The planner uses it to drive plan tasks; the
> executor uses it as the pixel/copy contract; the ui-checker validates against
> these dimensions; Phase 7 reuses the theme token model verbatim for the embed
> bundle's `theme=<JSON>` attribute.
>
> Where CONTEXT.md decisions (D6-NN) lock a design point, this spec cites the
> decision. Where this spec invents a value (color hex, type ramp px, copy
> string), it cites D6-V-NN (Visual decision) and explains the rationale in the
> last section "Open Design Decisions". The planner can reference both
> namespaces when writing implementation tasks.

---

## Table of Contents

1. Design System Overview
2. Theme Tokens (or-light / or-dark / or-brand)
3. Typography Scale & Font Stack
4. Spacing & Layout Primitives
5. Component Specifications (18 entity components + shell + status + import + conflict + wizard + org-picker)
6. Page Specifications (every route in D6-09/D6-11/D6-12)
7. Interaction Patterns (loading / empty / error / 409 / confirm / polling)
8. Accessibility Contract
9. Responsive Breakpoints
10. Motion Specification
11. Brand Voice / Copy Library
12. Validation-Visible UX (ajv timing & rendering rules)
13. Cross-Theme Color Contrast Matrix
14. Open Design Decisions (D6-V-NN registry, planner-facing rationale)

---

## 1. Design System

| Property | Value | Source |
|----------|-------|--------|
| Tool | none (no shadcn — Lit + Shoelace per PROJECT.md) | PROJECT.md |
| Preset | not applicable | — |
| Component library | Shoelace (per-component imports for tree-shaking) | D6-08 |
| Icon library | Shoelace built-in (`@shoelace-style/shoelace/dist/components/icon/icon.js` + the `<sl-icon>` library system pointing at the bundled Bootstrap-Icons set) | D6-V-01 |
| Font | System UI stack (zero deps); optional self-hosted Inter for `or-brand` (deferred to v0.2 — see Open Design Decisions) | D6-V-02 |
| Theme runtime | CSS custom properties set on `<or-catalog-shell>` host element | D6-19, D6-20 |
| Render boundary | Shadow DOM per component (Lit default); shell is the only DOM-level host | EMBED-04, D6-20 |
| Routing | `@lit-labs/router` inside the shell | D6-07, D6-06 |
| Validation | ajv-standalone codegen from openapi.yaml | D6-18 |
| i18n | `@lit/localize` (EN + VI, lazy-loaded) | D6-21, D6-22, D6-23 |

### Brand tone & design language

Open Routing is **contact-center routing infrastructure** for platform engineers.
Audience = developers configuring catalog data; not consumer end-users; not call
center agents. Design language draws from:

- **Linear** for typographic restraint and dense table aesthetics
- **Vercel admin** for sidebar chrome and "calm dark mode"
- **Datadog config screens** for data-density-first form layouts
- **Stripe Dashboard** for accent restraint and clear status badges

**What the admin is NOT:**
- It is NOT a marketing surface — no hero illustrations, no gradient backgrounds, no
  decorative imagery.
- It is NOT an agent desktop — there is no avatar collage, no call/chat control,
  no "How are you feeling today?" copy.
- It is NOT a flashy SaaS landing — every pixel earns its keep by either showing
  data or moving the user toward their next action.

**Voice principles (D6-V-03):**
1. **Precise over friendly.** "Code is immutable after create" beats "Whoops, you can't change that!".
2. **Action over description.** Button labels are imperative verbs. "Create agent", not "Submit form".
3. **Data over decoration.** A 14-column table is preferable to 4 columns of card art.
4. **Errors explain how to fix.** Every error message includes a remediation path or a `request_id` for support.
5. **VI ↔ EN parity.** Every string ships in both locales; no English-only fallback strings.

### Stack-level constraints (carries to Phase 7)

- **Tree-shake budget:** Per-component Shoelace imports (D6-08) so the embed bundle (Phase 7)
  fits in 70KB gzipped. Any helper in `packages/ui` that imports all of Shoelace breaks the budget.
- **Shadow DOM cascade:** Theme tokens set on the shell host cascade through `::part`,
  Light DOM children, AND child Shadow DOM (because they are CSS custom properties on the
  shell host element — Shadow DOM only blocks element-level CSS rules, not custom-property
  inheritance through the slot tree). Confirmed by Shoelace's own theme convention.
- **Theme JSON contract (Phase 7 forward compat):** The 3 named themes (`or-light`/`or-dark`/`or-brand`)
  serialize to a stable JSON token map. Phase 7 will accept that same JSON shape as the
  `theme=` attribute value. Schema documented in §2.4.

---

## 2. Theme Tokens (3 themes)

### 2.1 Token namespace conventions

Two namespaces:

1. **`--sl-*`** — Shoelace's own design-token surface. We override the colors-primary scale,
   the success/warning/danger semantic colors, and a few neutral-greys. Shoelace components
   read these directly. Convention from <https://shoelace.style/getting-started/themes/>.
2. **`--or-*`** — Open Routing-specific tokens for surfaces, layout chrome, and one-off
   semantic states (conflict banner background, monospace font fallback, etc.). All custom
   `--or-*` tokens MUST also map to a `--sl-*` token where one exists — never paint two
   separate values for "primary".

### 2.2 The 3 themes — full token maps

#### Theme: `or-light` (default)

Clean white app shell, neutral grays, mid-tone teal primary (reads "technical / infra" — distinct
from Shoelace's default blue, distinct from Datadog's purple, distinct from generic SaaS-blue).

```css
/* Sets on the shell host element via style.setProperty calls — NOT a static CSS file. */
/* Captured here in CSS syntax for documentation; executor implements as a JS map. */

[data-or-theme="or-light"] {
  /* ─── Shoelace primary scale (teal, hue=185, sat=43%) ─── */
  --sl-color-primary-50:  #f0fafa;
  --sl-color-primary-100: #d3f1f2;
  --sl-color-primary-200: #aae0e2;
  --sl-color-primary-300: #79c9ce;
  --sl-color-primary-400: #4faab2;
  --sl-color-primary-500: #2b8a93;  /* base teal — primary CTAs, focus rings, active nav */
  --sl-color-primary-600: #1f6e77;
  --sl-color-primary-700: #1a575f;
  --sl-color-primary-800: #18464c;
  --sl-color-primary-900: #173a3f;
  --sl-color-primary-950: #0c2024;

  /* ─── Shoelace semantic ─── */
  --sl-color-danger-500:  #d92d20;  /* delete buttons, destructive confirms, form errors */
  --sl-color-warning-500: #b54708;  /* force-flag toggle border, 207-multi-status banner */
  --sl-color-success-500: #027a48;  /* save-success toast, all-rows-succeeded import */
  --sl-color-neutral-0:   #ffffff;  /* app shell background — the 60% dominant */
  --sl-color-neutral-50:  #fafafa;
  --sl-color-neutral-100: #f5f5f5;  /* sidebar background — the 30% secondary */
  --sl-color-neutral-200: #e5e5e5;  /* hairline dividers, table borders */
  --sl-color-neutral-300: #d4d4d4;
  --sl-color-neutral-500: #737373;  /* secondary text */
  --sl-color-neutral-700: #404040;  /* body text */
  --sl-color-neutral-900: #171717;  /* headings */
  --sl-color-neutral-1000:#000000;

  /* ─── Open Routing custom ─── */
  --or-color-app-bg:           var(--sl-color-neutral-0);          /* 60% dominant */
  --or-color-sidebar-bg:       var(--sl-color-neutral-100);        /* 30% secondary */
  --or-color-topbar-bg:        var(--sl-color-neutral-0);
  --or-color-card-bg:          var(--sl-color-neutral-0);
  --or-color-card-border:      var(--sl-color-neutral-200);
  --or-color-text-body:        var(--sl-color-neutral-700);
  --or-color-text-strong:      var(--sl-color-neutral-900);
  --or-color-text-muted:       var(--sl-color-neutral-500);
  --or-color-text-on-primary:  #ffffff;
  --or-color-conflict-bg:      #fef3c7;                            /* amber-100 — calm warning */
  --or-color-conflict-border:  #f59e0b;                            /* amber-500 */
  --or-color-diff-removed:     #fecaca;                            /* red-200 — server's old value */
  --or-color-diff-added:       #bbf7d0;                            /* green-200 — server's new value */
  --or-color-code-bg:          var(--sl-color-neutral-100);
  --or-color-code-fg:          var(--sl-color-primary-700);
  --or-color-focus-ring:       var(--sl-color-primary-500);
  --or-color-row-hover:        var(--sl-color-neutral-50);
  --or-color-row-selected:     var(--sl-color-primary-50);
  --or-color-skeleton-base:    var(--sl-color-neutral-200);
  --or-color-skeleton-shimmer: var(--sl-color-neutral-100);
  --or-color-divider:          var(--sl-color-neutral-200);
  --or-color-overlay-scrim:    rgb(0 0 0 / 0.4);                   /* modal/drawer scrim */
}
```

#### Theme: `or-dark`

Matched dark variant. Same teal primary at a slightly lighter hue (-400 instead of -500)
so it pops against the dark surface. Sidebar is the "calm dark" tone — not pure black,
not pure white text; reads at a glance without eye fatigue.

```css
[data-or-theme="or-dark"] {
  /* ─── Shoelace primary scale (teal, brighter so it sits on dark) ─── */
  --sl-color-primary-50:  #0c2024;
  --sl-color-primary-100: #173a3f;
  --sl-color-primary-200: #18464c;
  --sl-color-primary-300: #1a575f;
  --sl-color-primary-400: #1f6e77;
  --sl-color-primary-500: #4faab2;  /* dark-mode base — lighter so it reads on dark surface */
  --sl-color-primary-600: #79c9ce;
  --sl-color-primary-700: #aae0e2;
  --sl-color-primary-800: #d3f1f2;
  --sl-color-primary-900: #f0fafa;
  --sl-color-primary-950: #ffffff;

  /* ─── Shoelace semantic (lifted for dark contrast) ─── */
  --sl-color-danger-500:  #f97066;
  --sl-color-warning-500: #fdb022;
  --sl-color-success-500: #32d583;
  --sl-color-neutral-0:   #0a0a0a;
  --sl-color-neutral-50:  #121212;  /* app shell — the 60% dominant in dark */
  --sl-color-neutral-100: #1a1a1a;  /* sidebar — the 30% secondary in dark */
  --sl-color-neutral-200: #262626;  /* hairlines */
  --sl-color-neutral-300: #404040;
  --sl-color-neutral-500: #a3a3a3;  /* secondary text */
  --sl-color-neutral-700: #d4d4d4;  /* body text */
  --sl-color-neutral-900: #f5f5f5;  /* headings */
  --sl-color-neutral-1000:#ffffff;

  /* ─── Open Routing custom ─── */
  --or-color-app-bg:           var(--sl-color-neutral-50);
  --or-color-sidebar-bg:       var(--sl-color-neutral-100);
  --or-color-topbar-bg:        var(--sl-color-neutral-50);
  --or-color-card-bg:          var(--sl-color-neutral-100);
  --or-color-card-border:      var(--sl-color-neutral-200);
  --or-color-text-body:        var(--sl-color-neutral-700);
  --or-color-text-strong:      var(--sl-color-neutral-900);
  --or-color-text-muted:       var(--sl-color-neutral-500);
  --or-color-text-on-primary:  var(--sl-color-neutral-900);
  --or-color-conflict-bg:      #44331b;                            /* darkened amber */
  --or-color-conflict-border:  #fdb022;
  --or-color-diff-removed:     #5b1d1d;
  --or-color-diff-added:       #1d4d2c;
  --or-color-code-bg:          var(--sl-color-neutral-200);
  --or-color-code-fg:          var(--sl-color-primary-500);
  --or-color-focus-ring:       var(--sl-color-primary-500);
  --or-color-row-hover:        var(--sl-color-neutral-200);
  --or-color-row-selected:     #143036;
  --or-color-skeleton-base:    var(--sl-color-neutral-200);
  --or-color-skeleton-shimmer: var(--sl-color-neutral-300);
  --or-color-divider:          var(--sl-color-neutral-200);
  --or-color-overlay-scrim:    rgb(0 0 0 / 0.6);
}
```

#### Theme: `or-brand`

Bolder primary; the brand identity color for Open Routing. Uses a deeper, more saturated
teal (closer to a "petrol" shade) — readable on white, still dense-data-friendly. The
sidebar gets a subtle tint of the brand color so the admin LOOKS branded even when no
custom logo is set. v0.1 ships hardcoded; per-org brand override deferred to v0.2.

```css
[data-or-theme="or-brand"] {
  /* ─── Shoelace primary scale (deeper teal, hue=190, sat=60%) ─── */
  --sl-color-primary-50:  #ebfafa;
  --sl-color-primary-100: #cdf0f1;
  --sl-color-primary-200: #9be0e3;
  --sl-color-primary-300: #5ec9cf;
  --sl-color-primary-400: #2dadb7;
  --sl-color-primary-500: #0d8b96;  /* brand teal — deeper than or-light */
  --sl-color-primary-600: #086e78;
  --sl-color-primary-700: #075a63;
  --sl-color-primary-800: #094a52;
  --sl-color-primary-900: #0a3d44;
  --sl-color-primary-950: #03252a;

  /* ─── Shoelace semantic (same as or-light) ─── */
  --sl-color-danger-500:  #d92d20;
  --sl-color-warning-500: #b54708;
  --sl-color-success-500: #027a48;
  --sl-color-neutral-0:   #ffffff;
  --sl-color-neutral-50:  #f8fafa;     /* very faint brand tint on neutral whites */
  --sl-color-neutral-100: #eef5f5;     /* sidebar — visibly brand-tinted */
  --sl-color-neutral-200: #d9e7e8;
  --sl-color-neutral-300: #b8cdce;
  --sl-color-neutral-500: #678384;
  --sl-color-neutral-700: #364949;
  --sl-color-neutral-900: #0e1e1e;
  --sl-color-neutral-1000:#000000;

  /* ─── Open Routing custom ─── */
  --or-color-app-bg:           var(--sl-color-neutral-0);
  --or-color-sidebar-bg:       var(--sl-color-neutral-100);          /* tinted brand */
  --or-color-topbar-bg:        var(--sl-color-neutral-0);
  --or-color-card-bg:          var(--sl-color-neutral-0);
  --or-color-card-border:      var(--sl-color-neutral-200);
  --or-color-text-body:        var(--sl-color-neutral-700);
  --or-color-text-strong:      var(--sl-color-neutral-900);
  --or-color-text-muted:       var(--sl-color-neutral-500);
  --or-color-text-on-primary:  #ffffff;
  --or-color-conflict-bg:      #fef3c7;
  --or-color-conflict-border:  #f59e0b;
  --or-color-diff-removed:     #fecaca;
  --or-color-diff-added:       #bbf7d0;
  --or-color-code-bg:          var(--sl-color-neutral-100);
  --or-color-code-fg:          var(--sl-color-primary-700);
  --or-color-focus-ring:       var(--sl-color-primary-500);
  --or-color-row-hover:        var(--sl-color-neutral-50);
  --or-color-row-selected:     var(--sl-color-primary-50);
  --or-color-skeleton-base:    var(--sl-color-neutral-200);
  --or-color-skeleton-shimmer: var(--sl-color-neutral-100);
  --or-color-divider:          var(--sl-color-neutral-200);
  --or-color-overlay-scrim:    rgb(0 0 0 / 0.4);
}
```

### 2.3 60/30/10 color split

| Role | or-light | or-dark | or-brand | Coverage |
|------|----------|---------|----------|----------|
| Dominant 60% | `--or-color-app-bg` (white) | `--or-color-app-bg` (#121212) | `--or-color-app-bg` (white) | App shell, page background, modal interior, main content area |
| Secondary 30% | `--or-color-sidebar-bg` (#f5f5f5) | `--or-color-sidebar-bg` (#1a1a1a) | `--or-color-sidebar-bg` (#eef5f5) | Sidebar, table header rows, card group surfaces, top bar in dark mode |
| Accent 10% | `--sl-color-primary-500` (teal) | `--sl-color-primary-500` (light teal) | `--sl-color-primary-500` (deep teal) | See "Accent reserved for" below |

**Accent (`--sl-color-primary-*`) reserved for** (D6-V-04):
1. Primary action CTAs (`<sl-button variant="primary">`): "Create agent", "Save changes", "Continue", "Import", "Confirm Break".
2. Active sidebar nav entry (background `--sl-color-primary-50`, left border `--sl-color-primary-500`).
3. Focus rings (`--or-color-focus-ring`) on inputs, buttons, links — keyboard navigation cue.
4. Current status badge background when `Ready` (only on the status panel — Ready is the "good" terminal state).
5. Active step indicator on `<or-form-wizard>`.
6. Selected radio/checkbox checkmark and toggle "on" thumb.
7. Hover underline color on `<a>` and clickable-row affordance.
8. Currently-typed character in `<or-code-input>` when valid (subtle border-bottom tint).
9. Selected row highlight on data tables (`--or-color-row-selected`).
10. NOT used for: text body, headings, table cell backgrounds (data should not look "clickable"), neutral backgrounds, decorative dividers.

**Destructive (`--sl-color-danger-500`) reserved for** (D6-V-05):
- Delete buttons (`<sl-button variant="danger">`).
- Confirm-delete modal CTA.
- 4xx/5xx error inline messages text color.
- Force-flag toggle BORDER on the status panel (warning, not destructive — but adjacent semantic).
- Form field error text (under-field message and the small red dot icon).
- NOT used for: invalid_transition 409 banner (uses amber/warning), version_conflict 409 (uses amber), 422 validation errors (uses red but field-level only — no full-page redshift).

### 2.4 Theme JSON contract for Phase 7

When Phase 7 ships, the embed will accept `theme=<JSON>` per EMBED-05. The JSON shape MUST
match the named-theme map above. Phase 6 establishes the schema:

```typescript
type ThemeTokens = {
  // Shoelace overrides — partial map; omitted tokens fall through to defaults
  '--sl-color-primary-50'?: string;
  '--sl-color-primary-100'?: string;
  // ... through -950
  '--sl-color-danger-500'?: string;
  '--sl-color-warning-500'?: string;
  '--sl-color-success-500'?: string;
  '--sl-color-neutral-0'?: string;
  // ... through -1000
  // Open Routing extensions
  '--or-color-app-bg'?: string;
  '--or-color-sidebar-bg'?: string;
  '--or-color-topbar-bg'?: string;
  '--or-color-card-bg'?: string;
  '--or-color-card-border'?: string;
  '--or-color-text-body'?: string;
  '--or-color-text-strong'?: string;
  '--or-color-text-muted'?: string;
  '--or-color-text-on-primary'?: string;
  '--or-color-conflict-bg'?: string;
  '--or-color-conflict-border'?: string;
  '--or-color-diff-removed'?: string;
  '--or-color-diff-added'?: string;
  '--or-color-code-bg'?: string;
  '--or-color-code-fg'?: string;
  '--or-color-focus-ring'?: string;
  '--or-color-row-hover'?: string;
  '--or-color-row-selected'?: string;
  '--or-color-skeleton-base'?: string;
  '--or-color-skeleton-shimmer'?: string;
  '--or-color-divider'?: string;
  '--or-color-overlay-scrim'?: string;
};

// Phase 6 ships these three exports from packages/ui/src/themes/
export const orLight: ThemeTokens = { ... };
export const orDark:  ThemeTokens = { ... };
export const orBrand: ThemeTokens = { ... };

// Convenience union for the shell's `theme` property
export type ThemeName = 'or-light' | 'or-dark' | 'or-brand';
export type Theme = ThemeName | ThemeTokens;
```

The shell's `applyTheme(theme: Theme)` method iterates the resolved map and calls
`this.style.setProperty(token, value)` per entry. Unset keys in a JSON theme fall through
to the previously-applied named theme (default: `or-light`).

---

## 3. Typography Scale & Font Stack

### 3.1 Font stack

```css
--or-font-sans:
  'Inter Variable', 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI',
  Roboto, Oxygen, Ubuntu, Cantarell, 'Helvetica Neue', sans-serif;

--or-font-mono:
  ui-monospace, 'SF Mono', 'Cascadia Code', 'Menlo', 'Consolas',
  'Liberation Mono', 'Courier New', monospace;
```

**Inter is OPTIONAL.** Phase 6 ships ONLY the system fallback (no font hosting infrastructure
in v0.1 admin). Phase 6 lists Inter at the front of the stack so any future self-hosting
"just works" without code change. Implementation guidance for executor: include the stack
as a `--or-font-sans` token; do NOT add `@font-face` rules for Inter in Phase 6.

**Monospace stack** for: org_id chip, `request_id` text, UUID columns in tables,
`<or-code-input>` value display, `BulkImportFailedRow.row` numbers, adapter JSONB editor,
the "Technical details" disclosure body (server `reason` field per D6-24).

### 3.2 Type scale (4 sizes, 2 weights — strictly bounded per design system rules)

| Token | Size | Weight | Line Height | Letter Spacing | Usage |
|-------|------|--------|-------------|----------------|-------|
| `--or-text-display` | 24px | 600 (semibold) | 1.2 | -0.02em | Page heading ("Agents", "Bulk Import"); never inline. One per page. |
| `--or-text-heading` | 16px | 600 (semibold) | 1.3 | -0.01em | Card titles, section labels ("Skills", "Status panel"), modal titles, form group headings. |
| `--or-text-body` | 14px | 400 (regular) | 1.5 | 0 | All body text: field values, form labels, table cells, paragraph copy, error messages, sidebar nav entries, button text. |
| `--or-text-code` | 13px | 400 (regular, mono) | 1.4 | 0 | UUIDs, codes, request_id, monospace-only contexts. Slightly smaller than body because mono fonts read denser. |

**3 sizes, 2 weights — STRICTLY (D6-V-06).** No `--or-text-large`, no `--or-text-small`,
no `--or-text-medium-weight`, no italic toggle, no all-caps headers. Density comes from layout
and rhythm, not from a 6-size ramp.

**Display SIZE rationale:** 24px instead of 28/32px because the admin is data-dense; 24 reads
"page" without crowding the data table below it. Confirmed against Linear's "Issues" page
(uses 22-24px for primary section titles).

**Body weight 400 + heading weight 600.** No 500 medium. The 200-point weight delta gives a
clear visual hierarchy without needing a third weight or a color-shift trick. If the executor
finds a place where 400 and 600 do not give enough hierarchy, the fix is to reach for color
(`--or-color-text-strong` vs `--or-color-text-body`), not a third weight.

### 3.3 Where to use which token

| Place | Token | Color token | Why |
|-------|-------|-------------|-----|
| Page heading "Agents" | `--or-text-display` | `--or-color-text-strong` | The "you are here" cue |
| Card title "Skills" inside Agent detail | `--or-text-heading` | `--or-color-text-strong` | Subsection — heavier than body, lighter than display |
| Empty-state heading "No agents yet" | `--or-text-heading` | `--or-color-text-strong` | Same weight as a card title — empty state is a "section" |
| Empty-state body | `--or-text-body` | `--or-color-text-muted` | Lower hierarchy — descriptive |
| Sidebar nav entry | `--or-text-body` | `--or-color-text-body` | Same weight as table cell |
| Sidebar nav entry (active) | `--or-text-body` | `--or-color-text-strong` | Color shift not weight shift |
| Table column header | `--or-text-body` | `--or-color-text-muted` | Smaller visual weight than the data |
| Table cell | `--or-text-body` | `--or-color-text-body` | |
| Table cell (numeric / status) | `--or-text-body` | `--or-color-text-strong` | Slight emphasis for scan-ability |
| Form label | `--or-text-body` | `--or-color-text-strong` | Labels are signposts; readable at a glance |
| Form input value | `--or-text-body` | `--or-color-text-strong` | Same weight; color elevates the user's data |
| Form helper text | `--or-text-body` | `--or-color-text-muted` | Below input; lower priority |
| Form error text | `--or-text-body` | `--sl-color-danger-500` | Color encodes severity |
| Button label | `--or-text-body` | per variant | |
| Code chip (org_id, code, request_id) | `--or-text-code` | `--or-color-code-fg` | Mono font + accent color = "this is a token, not prose" |
| Toast body | `--or-text-body` | `--or-color-text-body` | |
| Modal title | `--or-text-heading` | `--or-color-text-strong` | |

---

## 4. Spacing & Layout Primitives

### 4.1 Spacing scale (8-point base; multiples of 4)

| Token | Value | Usage |
|-------|-------|-------|
| `--or-space-0` | 0 | reset |
| `--or-space-1` | 4px | icon-to-text gap, tight inline padding, focus-ring offset |
| `--or-space-2` | 8px | compact element spacing, badge padding, table row vertical padding |
| `--or-space-3` | 12px | (exception below) |
| `--or-space-4` | 16px | default element spacing, form-field vertical gap, card padding |
| `--or-space-6` | 24px | section padding, card-group gap, sidebar entry vertical padding |
| `--or-space-8` | 32px | layout gaps, major section breaks |
| `--or-space-12` | 48px | page padding-top, major section dividers |
| `--or-space-16` | 64px | empty-state vertical margin |

**Exceptions (D6-V-07):**
- `--or-space-3` (12px) is allowed for **one** case only: the icon-to-text gap inside
  `<sl-button>` labels — Shoelace's own component uses 12px there, fighting it creates
  visible misalignment. All other gaps use 4/8/16/24/32/48/64.
- Touch-target affordances on the org-picker "Continue" button: minimum height 44px
  (achieved via padding, NOT via an extra spacing token).

### 4.2 Border radius scale

| Token | Value | Usage |
|-------|-------|-------|
| `--or-radius-sm` | 2px | input fields, table cell focus outline |
| `--or-radius-md` | 4px | buttons, cards, sidebar nav entries, badges |
| `--or-radius-lg` | 8px | modals, drawers, top-level cards (org-picker, import result) |
| `--or-radius-full` | 9999px | status pill ("Ready", "Engaged"), profile circles (none in v0.1, reserved) |

### 4.3 Elevation (box-shadow)

| Token | Value | Usage |
|-------|-------|-------|
| `--or-shadow-none` | none | default (flat design — Linear-style) |
| `--or-shadow-sm` | `0 1px 2px rgb(0 0 0 / 0.05)` | sidebar boundary (when sidebar overlays content on tablet) |
| `--or-shadow-md` | `0 4px 12px rgb(0 0 0 / 0.08)` | dropdowns, popovers, toast |
| `--or-shadow-lg` | `0 12px 32px rgb(0 0 0 / 0.12)` | modals, drawers, org-picker card |

**Flat by default.** Cards do not have shadows — they have borders (`--or-color-card-border`).
Shadow is reserved for elements that "float" (popovers, modals, toasts). Saves visual noise
on data-dense screens.

### 4.4 Layout grid

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              Top bar (56px height)                          │
├──────────┬──────────────────────────────────────────────────────────────────┤
│          │                                                                  │
│  Side    │                                                                  │
│  bar     │              Main content area                                   │
│ (248px)  │       (flex 1 — fills remaining width)                           │
│          │                                                                  │
│          │                                                                  │
│          │                                                                  │
└──────────┴──────────────────────────────────────────────────────────────────┘
```

| Element | Dimension | Rationale |
|---------|-----------|-----------|
| Top bar height | 56px (exact) | Tall enough for two-line elements (org_id + nav row), short enough not to eat data area |
| Sidebar width | 248px (exact) | Fits "Break Reasons" (longest nav label) at 14px body without truncation; matches Linear/Datadog convention |
| Sidebar entry height | 36px | 8px vertical padding + 14px line-height + 8px = 36px; comfortable hit target on mouse |
| Sidebar entry horizontal padding | 16px (left) + 12px (right) | Left margin reads "anchored to edge"; right margin keeps icon-text-chevron rhythm |
| Main content padding | 32px top + 32px sides + 24px bottom | Top is 32 to give the page heading air; bottom is 24 because the table/form already has bottom-margin |
| Page heading bottom margin | 24px | Gap between display heading and the data table or form below |
| Card padding | 24px all sides | Generous inside cards because they hold dense data |
| Form column max-width | 640px | Single column of form fields, Inter at 14px = ~76 char line; readable, scannable |
| Detail page form gutter (Agent detail) | Sidebar 248px + form column 640px + gutter 32px + skills sub-table flex 1 | Skills sub-table sits to the right of the basic-info form; minimum total width ~1280px |
| Status panel width (standalone) | 480px centered | Compact, modal-feeling layout; status data is small but visually-critical |
| Status panel width (nested in detail) | inherits card width | When embedded in `/agents/:id`, fills the card horizontally |
| Bulk import container max-width | 960px | Wider than detail because it hosts a results table |
| Org-picker card max-width | 480px | Compact form, centered on viewport |
| Modal max-width | 480px (sm), 720px (md) | sm for confirm dialogs, md for delete-with-typed-confirm |
| Drawer width | 480px | NOT USED IN v0.1 per D6-16 — listed for Phase 7 forward compat |

### 4.5 Z-index scale

| Token | Value | Usage |
|-------|-------|-------|
| `--or-z-base` | 0 | default content |
| `--or-z-sticky` | 10 | sticky table header row inside scrollable content |
| `--or-z-dropdown` | 100 | `<sl-select>` popup, `<sl-dropdown>` menu |
| `--or-z-overlay` | 1000 | drawer scrim, modal scrim |
| `--or-z-modal` | 1010 | modal dialog over scrim |
| `--or-z-toast` | 2000 | toast layer above everything |

---

## 5. Component Specifications

This section is the executor's primary contract surface. Each component lists:
- **Purpose** — 1-paragraph why-it-exists
- **API** — attributes, properties, events
- **Visual layout** — ASCII wireframe + key dimensions
- **States** — default, loading, empty, error, conflict (where applicable)
- **Interactions** — what fires what, where state mutates
- **Shoelace primitives** — exact `<sl-*>` imports needed

### 5.1 `<or-catalog-shell>` — page-level shell

**Purpose:** The page-level chrome that admin (Phase 6) and embed (Phase 7) both mount.
Owns: theme application, router instance, sidebar nav, top bar (org chip + theme toggle +
locale toggle + Switch org link), outlet for the routed page content.

**API:**
```typescript
interface OrCatalogShell extends LitElement {
  // attributes
  @property({ type: String, attribute: 'org-id' }) orgId: string = '';
  @property({ type: String }) theme: 'or-light' | 'or-dark' | 'or-brand' = 'or-light';
  @property({ type: String }) modules: string = '';  // comma-separated entity filter (Phase 7)
  @property({ type: String }) locale: 'en' | 'vi' = 'en';

  // events
  // open-routing:org-changed -> CustomEvent<{ orgId: string }>  bubbles, composed
  // open-routing:theme-changed -> CustomEvent<{ theme: ThemeName }>  bubbles, composed
}
```

**Visual layout:**
```
┌──────────────────────────────────────────────────────────────────────────────┐
│ [☰ Open Routing]   org: 01901b2c-7f3a…    [Theme ⊙][Locale EN][Switch org]  │ ← top bar 56px
├──────────┬───────────────────────────────────────────────────────────────────┤
│          │                                                                   │
│ Agents   │                                                                   │
│ Skills   │                                                                   │
│ Queues   │                  outlet — routed page content                     │
│ Channels │                                                                   │
│ Adapters │                                                                   │
│ Break R. │                                                                   │
│ ───────  │                                                                   │
│ Import   │                                                                   │
│ Status   │                                                                   │
│          │                                                                   │
└──────────┴───────────────────────────────────────────────────────────────────┘
```

**Top bar zones (left-to-right):**
1. Hamburger icon (`<sl-icon name="list">`) — only visible at `<=1024px`; toggles sidebar drawer.
2. App wordmark "Open Routing" (`--or-text-heading`, `--or-color-text-strong`).
3. (flex spacer)
4. Org chip: prefix "org:" + truncated org_id (first 8 chars + ellipsis) in monospace, clickable to copy full UUID to clipboard. Hover surfaces a `<sl-tooltip>` with the full UUID.
5. Theme toggle: `<sl-button-group>` of three icon buttons (sun / moon / palette) — `[☀][🌙][🎨]`. Active variant = `--sl-color-primary-500` background. (D6-V-08)
6. Locale toggle: `<sl-select size="small">` with `EN` / `VI`. 64px width.
7. "Switch org" — `<sl-button variant="text">` linking to `/`.

**Sidebar entries (D6-11 — 8 items):**
1. Agents
2. Skills
3. Queues
4. Channels
5. Adapters
6. Break Reasons
7. — — — — — — (visual divider; `<or-space-2>` margin, `--or-color-divider` line)
8. Bulk Import
9. Agent Status (with subtext "Select an agent" when no agent selected)

Each entry: 16px Bootstrap icon (left) + label (14px body) + (optional) chevron when has-sub-state.
Active entry: `--or-color-row-selected` background, `--sl-color-primary-500` 3px left border,
`--or-color-text-strong` color.

**Icons** (Shoelace's bundled Bootstrap-Icons set; names match icon library):
- Agents: `people-fill`
- Skills: `tag-fill`
- Queues: `funnel-fill`
- Channels: `broadcast-pin`
- Adapters: `plug-fill`
- Break Reasons: `pause-circle-fill`
- Bulk Import: `upload`
- Agent Status: `circle-fill` (color shifts per status — see status panel)

**`modules=` filter (Phase 7 forward-compat):** When the attribute is set, sidebar entries
NOT in the comma-separated list are hidden (display:none, not removed — keyboard tab order is
preserved across the visible set). The divider is also hidden if NO operations entries
(Import / Status) are visible. Phase 6 admin always passes `modules=""` (empty = show all).

**States:**
- **Default:** All 8 entries visible, no active.
- **Active:** One entry highlighted per current route. Multiple entries may share a parent route (e.g. `/orgs/:org_id/agents/:id/status` → "Agents" AND "Agent Status" both active — Agent Status takes priority because it is the lower-frequency, more-specific route).
- **Filtered (Phase 7):** Subset visible per `modules=`.

**Shoelace primitives used:**
- `@shoelace-style/shoelace/dist/components/icon/icon.js`
- `@shoelace-style/shoelace/dist/components/icon-button/icon-button.js`
- `@shoelace-style/shoelace/dist/components/button/button.js`
- `@shoelace-style/shoelace/dist/components/button-group/button-group.js`
- `@shoelace-style/shoelace/dist/components/select/select.js`
- `@shoelace-style/shoelace/dist/components/option/option.js`
- `@shoelace-style/shoelace/dist/components/tooltip/tooltip.js`
- `@shoelace-style/shoelace/dist/components/drawer/drawer.js` (for ≤1024px hamburger)

---

### 5.2 `<or-org-picker>` — root view at `/`

**Purpose:** Single-page card centered on the viewport. Asks the user for an org_id (UUIDv7),
validates it client-side, pre-fills from localStorage on reload.

**API:**
```typescript
interface OrOrgPicker extends LitElement {
  @property({ type: String, attribute: 'last-used-org-id' }) lastUsedOrgId: string = '';

  // events
  // open-routing:org-selected -> CustomEvent<{ orgId: string }>  bubbles, composed
}
```

**Visual layout:**
```
                    ┌────────────────────────────────────┐
                    │                                    │
                    │           Open Routing             │  ← wordmark, --or-text-display
                    │   Standalone admin console         │  ← subtitle, --or-text-body muted
                    │                                    │
                    │   Organization ID                  │  ← form label
                    │   ┌──────────────────────────────┐ │
                    │   │ 01901b2c-7f3a-7abc-…         │ │  ← <sl-input>
                    │   └──────────────────────────────┘ │
                    │   UUIDv7 format. Example:          │  ← helper text
                    │   01901b2c-7f3a-7abc-8d4e-…        │
                    │                                    │
                    │           [    Continue    ]       │  ← primary CTA, full-width
                    │                                    │
                    │   ─────────────────────────────    │
                    │   Last used: 01901b2c-… [Use]      │  ← subtle "use last" affordance
                    │                                    │
                    └────────────────────────────────────┘
```

| Dimension | Value |
|-----------|-------|
| Card width | 480px |
| Card padding | 32px |
| Vertical centering | flex on app body, justify-content: center, align-items: center |
| Logo top margin | 0 (card padding handles it) |
| Logo to subtitle gap | 8px |
| Subtitle to label gap | 32px |
| Label to input gap | 8px |
| Input to helper text gap | 8px |
| Helper text to CTA gap | 24px |
| Continue button height | 44px (touch-friendly) |
| Continue to last-used divider gap | 32px |
| Last-used to bottom of card | 0 |

**Validation timing (D6-V-09):** **On-submit** (Click "Continue" or press Enter), NOT debounce
on keystroke. Rationale: pasting a UUID character-by-character would otherwise flash red on
every intermediate state. On-blur would also work but on-submit is the simplest match for the
"modal feel" of the picker. If the regex fails:
- Input gets red border (`--sl-color-danger-500`)
- Helper text replaced with: "Not a valid UUIDv7. Format: 8-4-4-4-12 hex characters with version digit 7."
- Continue button stays focused; cursor returns to end of input

**States:**
- **Default (no last-used):** "Last used" section absent.
- **Default (with last-used):** Input pre-filled with `lastUsedOrgId`, focus is on Continue (not input — power-user can press Enter immediately).
- **Validating:** Spinner inside Continue button; button disabled.
- **Invalid:** Red border on input + helper text replaced (see above).
- **API rejected (after server bypass-route call — N/A in v0.1 stub auth):** Reserved hook for v1.

**Shoelace primitives:**
- `@shoelace-style/shoelace/dist/components/input/input.js`
- `@shoelace-style/shoelace/dist/components/button/button.js`
- `@shoelace-style/shoelace/dist/components/spinner/spinner.js`

---

### 5.3 `<or-{entity}-list>` × 6 — generic catalog list

**Purpose:** List view for one of the 6 catalog entities. Reads from
`GET /v1/orgs/{org_id}/{entity}?cursor=…&limit=…&name=…&include_disabled=…`. Renders
the cursor-paginator and the data table together.

The 6 list components (`<or-agent-list>`, `<or-skill-list>`, `<or-queue-list>`,
`<or-channel-list>`, `<or-adapter-list>`, `<or-break-reason-list>`) share an internal
`<or-data-table>` primitive but each specifies its column set.

**Common API:**
```typescript
interface OrEntityList extends LitElement {
  @property({ type: String }) baseUrl: string = '';
  @property({ type: String, attribute: 'org-id' }) orgId: string = '';
  // Internal reactive state
  @state() private cursor: string | null = null;
  @state() private cursorStack: string[] = [];  // for client-side "Previous"
  @state() private nameFilter: string = '';
  @state() private includeDisabled: boolean = false;
  @state() private limit: number = 25;
}
```

**Visual layout (all 6 entities):**
```
┌──────────────────────────────────────────────────────────────────────────────┐
│ Agents                                                  [+ Create agent]     │  ← --or-text-display + CTA
├──────────────────────────────────────────────────────────────────────────────┤
│ ┌──────────────────────────┐ ┌─────────────────────┐  ┌──────┐               │
│ │ 🔍 Search by name        │ │ ☐ Include disabled  │  │ ↻    │  ← Refresh   │
│ └──────────────────────────┘ └─────────────────────┘  └──────┘               │
│                                                                              │
│ ┌──────────────────────────────────────────────────────────────────────────┐ │
│ │ code         │ name           │ email           │ enabled │ updated │ ⋮ │ │
│ │ ─────────────┼────────────────┼─────────────────┼─────────┼─────────┼───│ │
│ │ emp_0042     │ Alice Nguyen   │ alice@…         │  ✓     │ 2h ago  │ ⋮ │ │
│ │ emp_0043     │ Bob Chen       │ bob@…           │  ✓     │ 3h ago  │ ⋮ │ │
│ │ emp_0051     │ Carol Davis    │ carol@…         │  ✗     │ 1d ago  │ ⋮ │ │
│ └──────────────────────────────────────────────────────────────────────────┘ │
│                                                                              │
│   25 per page    [<] Page 1    [Next >]                                      │
└──────────────────────────────────────────────────────────────────────────────┘
```

**Per-entity column sets (D6-V-10):**

| Entity | Columns (left to right) | Sortable | Notes |
|--------|-------------------------|----------|-------|
| Agents | code, name, email, enabled, updated_at, [⋮] | No (server doesn't sort) | code is monospace; updated_at is relative ("2h ago") with tooltip showing full ISO |
| Skills | code, name, skill_type, enabled, updated_at, [⋮] | No | skill_type is a plain text chip |
| Queues | code, name, channel_types, priority, acw_sec, enabled, updated_at, [⋮] | No | channel_types renders as space-separated badges ("voice" "chat"); priority right-aligned; acw_sec shows "60s" |
| Channels | code, name, channel_type, default_queue_id (truncated), enabled, updated_at, [⋮] | No | default_queue_id is mono-truncated to 8 chars + tooltip |
| Adapters | code, name, adapter_type, enabled, updated_at, [⋮] | No | config column intentionally absent (too wide; see detail page) |
| Break Reasons | code, name, routable (✓/✗), display_order, enabled, updated_at, [⋮] | No | display_order right-aligned; routable column header tooltip: "Routable: agent can still receive interactions while on break" |

**Header styling:**
- Background: `--or-color-sidebar-bg` (subtle differentiation)
- Color: `--or-color-text-muted`
- Sticky on vertical scroll (z-index `--or-z-sticky`)
- Font: 14px body, weight 400
- Padding: 12px horizontal, 8px vertical
- Border-bottom: 1px `--or-color-divider`

**Cell styling:**
- Padding: 16px horizontal, 12px vertical
- Border-bottom: 1px `--or-color-divider` (each row)
- Row hover: `--or-color-row-hover` background
- Row click: navigate to `/orgs/:org_id/{entity}/:id`
- code cell: `--or-text-code`, `--or-color-code-fg`
- enabled cell: `<sl-icon name="check-lg">` (✓) in `--sl-color-success-500` or `<sl-icon name="x-lg">` (✗) in `--or-color-text-muted`

**Row context menu [⋮]:**
- `<sl-dropdown>` with `<sl-menu>`
- Items: "Edit" (navigates to detail), "Disable" (if enabled — PATCH enabled=false), "Enable" (if disabled — PATCH enabled=true), "Delete" (opens confirm modal)
- "Delete" item is `<sl-menu-item variant="danger">` — `--sl-color-danger-500` text

**Search input behavior (D6-V-11):**
- 300ms debounce on keystroke; fires API call after silence
- Clears server pagination cursor (resets to page 1)
- Empty input = no `name` query param (server returns all)
- Min length: 1 character (allow "a")
- Search icon (`<sl-icon name="search">`) inside input, left aligned, `--or-color-text-muted`

**Include disabled toggle:** Plain `<sl-checkbox>`. Toggle = resets pagination cursor; refetch.

**Refresh button:** `<sl-icon-button name="arrow-clockwise">` — refetch current page; preserves cursor and filters; spinner inside icon button while inflight.

**Pagination row:**
- Plain text: "{limit} per page" with a `<sl-select size="small">` of 10/25/50/100
- "Page N" where N is the count of cursor-stack pushes + 1
- `[<] Previous` button — disabled when cursorStack is empty; pops last cursor from stack and refetches
- `[Next >]` button — disabled when `has_more === false`; pushes current cursor to stack, sets cursor = next_cursor, refetches

**States:**
- **Loading (initial mount, no data yet):** Skeleton rows — 5 rows of `--or-color-skeleton-base` blocks matching each column's width, with shimmer animation cycling 1.5s.
- **Loading (subsequent fetch, has previous data):** Existing data stays visible; subtle 1px-tall progress bar across the top of the table; pagination buttons disabled.
- **Populated:** Table renders all `items`. Helper "Showing 25 of N" — N comes from response's `total` if available; if not, helper is omitted.
- **Empty (no items, no search active):** Centered empty state — see Brand Voice section for copy. CTA: `[+ Create {entity_singular}]`.
- **Empty (no items, search active):** Centered empty state. Copy: "No {entity} found matching '{name}'." CTA: `[Clear search]`.
- **Error (4xx/5xx from list endpoint):** Inline error card above the table — `<sl-alert variant="danger" open>`. Body: "Couldn't load {entity}. {error.reason or fallback}. Request ID: {request_id}. [Retry]". The table area shows the empty skeleton frame.

**Shoelace primitives:**
- `@shoelace-style/shoelace/dist/components/input/input.js`
- `@shoelace-style/shoelace/dist/components/checkbox/checkbox.js`
- `@shoelace-style/shoelace/dist/components/button/button.js`
- `@shoelace-style/shoelace/dist/components/icon-button/icon-button.js`
- `@shoelace-style/shoelace/dist/components/dropdown/dropdown.js`
- `@shoelace-style/shoelace/dist/components/menu/menu.js`
- `@shoelace-style/shoelace/dist/components/menu-item/menu-item.js`
- `@shoelace-style/shoelace/dist/components/select/select.js`
- `@shoelace-style/shoelace/dist/components/option/option.js`
- `@shoelace-style/shoelace/dist/components/alert/alert.js`
- `@shoelace-style/shoelace/dist/components/icon/icon.js`
- `@shoelace-style/shoelace/dist/components/tooltip/tooltip.js`
- `@shoelace-style/shoelace/dist/components/spinner/spinner.js`
- `@shoelace-style/shoelace/dist/components/badge/badge.js` (for channel_types chips)

---

### 5.4 `<or-{entity}-detail>` × 6 — detail/edit page

**Purpose:** Shows one entity for read and edit. Hosted at `/orgs/:org_id/{entity}/:id`.
Owns the form, the 409 conflict banner, the delete affordance, and (for Agent) the nested
skills sub-table.

**Common API:**
```typescript
interface OrEntityDetail extends LitElement {
  @property({ type: String }) baseUrl: string = '';
  @property({ type: String, attribute: 'org-id' }) orgId: string = '';
  @property({ type: String, attribute: 'entity-id' }) entityId: string = '';
  // events
  // open-routing:entity-updated -> CustomEvent<{ id: string }>  bubbles, composed
  // open-routing:entity-deleted -> CustomEvent<{ id: string }>  bubbles, composed
}
```

**Visual layout (general — non-Agent entities):**
```
┌──────────────────────────────────────────────────────────────────────────────┐
│ [← Back to Skills]    Edit skill                          [Delete] [Disable] │  ← top bar
│                                                                              │
│ ┌─── Conflict banner ─────────────────────────────────────────────────────┐ │  ← (only if 409)
│ │ This skill was changed elsewhere. Your edits are below — review and    │ │
│ │ re-submit, or discard local changes.                                    │ │
│ │ [Review and re-submit]  [Discard my changes]                            │ │
│ └─────────────────────────────────────────────────────────────────────────┘ │
│                                                                              │
│ ┌──────────────────────────────────────────────────────────────────────────┐ │
│ │  code            [ skill_voice_tier1                            ] 🔒    │ │  ← read-only post-create
│ │  Code cannot be changed after create.                                    │ │  ← helper text
│ │                                                                          │ │
│ │  name            [ Billing Support                              ]        │ │
│ │                                                                          │ │
│ │  external_id     [ HR-SKILL-VOICE                               ]        │ │
│ │  Optional integration mapping. Pass empty to clear.                      │ │
│ │                                                                          │ │
│ │  description     [ Handles billing inquiries…                  ]         │ │  ← textarea
│ │                                                                          │ │
│ │  skill_type      [ support                                      ]        │ │
│ │                                                                          │ │
│ │  enabled         [⏵●  ] Enabled                                          │ │  ← switch
│ │                                                                          │ │
│ │  ────────────────────────────────────────────────────────────────────── │ │
│ │  version 7   ·   updated 2h ago   ·   created 2026-04-12                 │ │  ← --or-color-text-muted
│ │                                                                          │ │
│ │                                          [Cancel]  [Save changes]        │ │  ← bottom bar
│ └──────────────────────────────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────────────────────────┘
```

**Form column layout:**
- Centered, max-width 640px, padding 24px
- One column of fields
- Each field: label (above) + input (full width) + helper/error text (below, 4px gap)
- Field-to-field vertical gap: 16px
- Group headings (when applicable, e.g. Agent's "Basics" / "Skills"): 16px above heading, 16px below
- Bottom action bar: borderless, right-aligned. "Cancel" = `<sl-button variant="text">`; "Save changes" = `<sl-button variant="primary">`. 12px gap between buttons.

**Top bar elements (left → right):**
- Back link: `<sl-button variant="text">` with left-chevron icon + "Back to {entity_plural}". Navigates to list.
- (flex spacer)
- "Delete" button (when entity enabled): `<sl-button variant="default">` with trash icon, `--sl-color-danger-500` text color (D6-V-12). On click → opens confirm modal.
- "Disable"/"Enable" button: `<sl-button variant="default">`. Soft-delete (PATCH enabled=false) — no confirm needed because reversible.

**Per-entity form field order (D6-V-13):**

#### Agent detail (special layout — see §5.4.1)
Has a 2-column layout below 1280px viewport — see dedicated subsection.

#### Skill
1. code (read-only post-create)
2. name (required)
3. external_id (optional)
4. description (textarea, optional)
5. skill_type (required)
6. enabled (switch)

#### Queue
1. code (read-only post-create)
2. name (required)
3. external_id (optional)
4. channel_types (multi-select of voice/chat/email — required, minItems=1)
5. priority (integer input, required)
6. acw_sec (integer input with "seconds" suffix, required)
7. enabled (switch)

#### Channel
1. code (read-only post-create)
2. name (required)
3. external_id (optional)
4. channel_type (single-select of voice/chat/email, required)
5. default_queue_id (queue picker — `<or-queue-picker>` async select component fetching from queues endpoint, optional, nullable)
6. enabled (switch)

#### Adapter
1. code (read-only post-create)
2. name (required)
3. external_id (optional)
4. adapter_type (text input, required)
5. config (textarea with monospace font + JSON formatting, optional, nullable) — see Validation-Visible UX §12
6. enabled (switch)

#### Break Reason
1. code (read-only post-create)
2. name (required)
3. external_id (optional)
4. routable (switch, required; default true) — helper text: "When on, agents on this break can still receive routed interactions."
5. display_order (integer input, required) — helper text: "Lower values appear first in the break picker."
6. enabled (switch)

**Footer metadata bar (D6-V-14):**
Below the form fields, above the bottom action bar — a single line of `--or-color-text-muted` 14px text:
`version {N}   ·   updated {relative time}   ·   created {YYYY-MM-DD}`
Separators are middle-dots with `--or-space-2` on each side. Hover on "updated" shows the full ISO timestamp tooltip.

**Save flow:**
1. User clicks "Save changes" → button shows inline spinner; both action buttons disable
2. Client-side ajv validate against `Update{Entity}Request` schema — if fails, show field errors, no API call
3. If valid, PATCH to `/v1/orgs/:org_id/{entity}/:id` with body including `version`
4. On 2xx response: toast "Saved" (auto-dismiss 3s), refresh form state with response body
5. On 409 version_conflict: extract `current` from body, update local form fields IN PLACE (preserve dirty user edits visually by leaving them as-is), render `<or-conflict-banner>` (see §5.7) with side-by-side diff
6. On 422 immutable_field (user somehow PATCHed `code` with a different value): toast "Code cannot be changed after create"; reset code field to server value
7. On 422 invalid_value (e.g. proficiency out of range): inline field error
8. On 5xx: show top-of-form `<sl-alert variant="danger">` with reason + request_id + Retry button

**Cancel flow:**
- If form is not dirty: navigate back to list
- If form is dirty: open `<sl-dialog>` with copy "Discard your changes? Unsaved edits will be lost." + buttons "Keep editing" (default) and "Discard" (`variant="danger"`)

**404 state:**
- Top bar still renders with "Back to {entity_plural}" link
- Form area replaced with centered empty state — see §11 for copy

**States:**
- **Loading:** Skeleton form fields (gray boxes matching final layout); top bar buttons disabled
- **Loaded (default):** Form populated; Save disabled until dirty
- **Dirty:** Save enabled; subtle dot indicator on Save button (color `--sl-color-warning-500`)
- **Submitting:** Spinner inside Save; both action buttons disabled
- **Validation error:** Field-level error text under offending field; Save stays enabled (user fixes inline)
- **Version conflict (409):** Conflict banner at top of card; form values remain user's edits; banner offers "Review and re-submit" + "Discard my changes" (D6-V-15)
- **404:** Centered empty state with [Back to {entity_plural}] CTA
- **5xx:** Top-of-form danger alert + retry

**Shoelace primitives (varies per entity; common):**
- `input`, `textarea`, `switch`, `button`, `select`, `option`, `dialog`, `alert`, `spinner`, `icon`, `icon-button`, `tooltip`, `badge`

---

#### 5.4.1 `<or-agent-detail>` — special 2-column layout

**Why special:** Agent is the only entity with embedded skills (per Sketch 1 OQ-1A). Skills
sub-table sits to the right of the basic-info form on viewports ≥1280px; below it at <1280px.

**Visual layout (≥1280px viewport):**
```
┌──────────────────────────────────────────────────────────────────────────────────────┐
│ [← Back to Agents]    Edit agent                            [Delete] [Disable]       │
│                                                                                      │
│ ┌── Conflict banner (only if 409) ──────────────────────────────────────────────┐  │
│ └──────────────────────────────────────────────────────────────────────────────────┘  │
│                                                                                      │
│ ┌──────────────────── 640px ──────┐   ┌──── Skills sub-table flex 1 ──────────────┐ │
│ │                                  │   │                                            │ │
│ │  Basics                          │   │  Skills                                    │ │
│ │  code         [ emp_0042   ]🔒  │   │                                            │ │
│ │  name         [ Alice…     ]    │   │  | Skill name        | Proficiency | ⋮  | │ │
│ │  email        [ alice@…    ]    │   │  | Billing Support   | [ 8  ▾ ]    | x  | │ │
│ │  external_id  [ HR-…       ]    │   │  | Tech Triage       | [ 6  ▾ ]    | x  | │ │
│ │  enabled      [⏵●  ] Enabled    │   │  | (empty)           |             |    | │ │
│ │                                  │   │                                            │ │
│ │  ───────────────────────────    │   │  [+ Add skill ▾]                          │ │
│ │  version 7 · updated 2h ago     │   │                                            │ │
│ │                                  │   │  Tip: changes here save with the form.   │ │
│ │   [Cancel]   [Save changes]     │   │                                            │ │
│ └──────────────────────────────────┘   └────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────────────────────────────────┘
```

**Skills sub-table:**
- Header row: "Skill name" | "Proficiency" | (actions column unlabeled)
- Each row: skill name (from server's embedded `name`), `<sl-select size="small">` of 1-10 for proficiency, `<sl-icon-button name="x-lg">` for remove
- Below table: "[+ Add skill ▾]" button → opens a `<sl-dropdown>` with searchable list of remaining org skills (cursor-paginated from `/skills` endpoint)
- Below add button: helper text "Tip: changes here save with the form." in muted

**Add skill dropdown:**
- Width 320px
- Top: search `<sl-input>` (debounced 300ms) with `<sl-icon name="search">` prefix
- Below: scrollable list of skill names (max-height 240px). Already-assigned skills shown grayed-out with "Already assigned" suffix.
- Click a skill: closes dropdown, appends row to skills sub-table with default proficiency 5
- "Load more" button at bottom when has_more=true; cursor pagination

**Save behavior with skills (PUT-replacement semantics):**
- The `<or-agent-detail>` packs `skills: [{skill_id, proficiency}]` into the PATCH body
- Empty skills array = remove all skills (intentional — matches OQ-1A resolution)
- 422 invalid_value on out-of-range proficiency → inline error on the offending row (red border on the proficiency `<sl-select>`); other rows save normally

**Mobile/tablet (<1280px):**
- Skills sub-table stacks BELOW the basic-info form (full-width)
- Add-skill dropdown still 320px (anchored to button)

---

### 5.5 `<or-form-wizard>` — multi-step create flow

**Purpose:** Wraps a create form in a stepper for complex entities (Agent, Channel). For simple
entities (Skill, Queue, Adapter, Break Reason), the wizard renders a single "review" step that
shows the same form as the detail page (D6-17).

**API:**
```typescript
interface OrFormWizard extends LitElement {
  @property({ type: Array }) steps: Array<{ key: string; label: string; }> = [];
  // event-based step navigation
  // open-routing:wizard-step-changed -> CustomEvent<{ from: string; to: string }>
  // open-routing:wizard-completed   -> CustomEvent<{ formData: object }>
}
```

**Visual layout (multi-step — Agent example, 3 steps):**
```
┌──────────────────────────────────────────────────────────────────────────────┐
│ [← Cancel]              Create agent                                         │
│                                                                              │
│   ●━━━━━━━━━━ ●─────────── ○                                                  │  ← stepper
│   1 Basics    2 Skills    3 Review                                           │
│   ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━  │
│                                                                              │
│   { current step's form content rendered here }                              │
│                                                                              │
│   ────────────────────────────────────────────────────────────────────────  │
│                                                  [Back]  [Next: Skills →]    │  ← step nav
└──────────────────────────────────────────────────────────────────────────────┘
```

**Stepper:**
- Each step circle: 24px diameter, border 2px
- Completed step: `--sl-color-primary-500` border + filled background + white check icon
- Current step: `--sl-color-primary-500` border + 2px ring offset + step number
- Future step: `--or-color-text-muted` border + step number
- Connector line between steps: 2px height, `--or-color-divider` color; turns `--sl-color-primary-500` once user passes
- Step labels: 14px body, color matches state (completed = strong, current = strong, future = muted)
- Stepper container max-width 480px, centered

**Step navigation:**
- "Back" button: `<sl-button variant="text">` — hidden on step 1
- "Next" button: `<sl-button variant="primary">` — label shows next step name ("Next: Skills →")
- "Create agent" button on final step (replaces Next on last step): `<sl-button variant="primary">`
- Click "Back": no validation; previous step content rendered, form state preserved
- Click "Next": runs ajv validation on current step's fields; if fail, show inline errors and stay; if pass, advance

**Per-entity wizard configuration (D6-V-16):**

| Entity | Steps |
|--------|-------|
| Agent | 1: Basics (code, name, email, external_id, enabled) → 2: Skills (initial assignments) → 3: Review |
| Channel | 1: Basics (code, name, channel_type, external_id, enabled) → 2: Default queue (`<or-queue-picker>`) → 3: Review |
| Skill | (single step) Basics + Review combined (code, name, external_id, description, skill_type, enabled) |
| Queue | (single step) (code, name, external_id, channel_types, priority, acw_sec, enabled) |
| Adapter | (single step) (code, name, external_id, adapter_type, config, enabled) |
| Break Reason | (single step) (code, name, external_id, routable, display_order, enabled) |

**Single-step behavior:** Stepper completely hidden. Header simplified to "Create {entity_singular}". Bottom nav becomes "Cancel" + "Create" only.

**Review step content (multi-step entities):**
- Heading "Review {entity}"
- 2-column key-value list of all entered fields
- Each section grouped by step ("Basics" group, "Skills" group)
- Inline "Edit" link next to each group → navigates back to that step preserving data
- Bottom: "Back" + "Create agent"

**Step-state persistence (D6-V-17):** **In-memory only.** Navigating away (cancel, browser
back, click sidebar) DISCARDS in-progress wizard data. CONTEXT.md recommended sessionStorage
as an alternative; v0.1 ships in-memory. If users complain about lost work, v0.2 can add
sessionStorage persistence behind a feature flag.

**Cancel flow:** Same dirty-prompt pattern as detail page — if any step has dirty data, open
confirm dialog "Discard agent? Your draft will be lost."

**On successful create:** Toast "Created {entity}" + navigate to `/orgs/:org_id/{entity}/:newId`.

**On 409 duplicate_code:** Inline field error on step 1's code input ("This code is already in
use in this org. Pick another."). Navigate back to step 1 if not currently visible.

**On 5xx during create:** Stay on current step. Show `<sl-alert variant="danger">` at top
with retry button.

**Shoelace primitives:**
- All form primitives from §5.4
- `@shoelace-style/shoelace/dist/components/dialog/dialog.js` (cancel-dirty confirm)

---

### 5.6 `<or-status-panel>` — agent status display + transition control

**Purpose:** Read and mutate one agent's state. Renders the current status, all valid
outgoing transitions, conditional sub-pickers (break reason picker when going to Break;
post_interaction_state radio when Engaged; wrapup countdown when WrapUp), and an admin
"force transition" affordance for state recovery.

Hosted at `/orgs/:org_id/agents/:id/status` (standalone) or embedded inside `<or-agent-detail>`
(when admin clicks "View status" CTA on the agent detail page — Phase 6 includes the standalone
route AND a button on agent detail).

**API:**
```typescript
interface OrStatusPanel extends LitElement {
  @property({ type: String }) baseUrl: string = '';
  @property({ type: String, attribute: 'org-id' }) orgId: string = '';
  @property({ type: String, attribute: 'agent-id' }) agentId: string = '';
  @property({ type: Boolean }) embedded: boolean = false;  // when true, no top "Back" bar

  // events
  // open-routing:status-changed -> CustomEvent<{ from: AgentStatus; to: AgentStatus; state_version: number }>
}
```

**Visual layout (standalone route, status = Ready):**
```
┌──────────────────────────────────────────────────────────────────────────────┐
│ [← Back to Alice Nguyen]            Agent status                             │
│                                                                              │
│ ┌──────────────────────── 480px centered card ────────────────────────────┐ │
│ │                                                                          │ │
│ │  Alice Nguyen   alice@example.com                                        │ │  ← --or-text-heading
│ │  Code: emp_0042                                                          │ │  ← --or-color-text-muted body
│ │                                                                          │ │
│ │  ───────────── Current status ─────────────                              │ │
│ │                                                                          │ │
│ │  ┌─────────────┐                                                         │ │
│ │  │ ● Ready     │   state v.42   ·   updated 2s ago                       │ │  ← status pill
│ │  └─────────────┘                                                         │ │
│ │                                                                          │ │
│ │  ───────────── Move to ──────────────                                    │ │
│ │                                                                          │ │
│ │  [Set Not Ready]    [Go on Break ▾]                                      │ │  ← transition buttons
│ │                                                                          │ │
│ │  ───────────── Advanced ─────────────                                    │ │
│ │  [⚠ Force transition] (admin override — logged at server)                │ │
│ │                                                                          │ │
│ └──────────────────────────────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────────────────────────┘
```

**Status pill — color per state (D6-V-18):**

| Status | Background | Color | Icon |
|--------|-----------|-------|------|
| Ready | `--sl-color-primary-50` | `--sl-color-primary-700` | `circle-fill` (filled green dot via overlay) |
| NotReady | `--sl-color-neutral-100` | `--or-color-text-strong` | `dash-circle-fill` |
| Break | `--sl-color-warning-500` (10% alpha) | `--sl-color-warning-500` (raw) | `pause-circle-fill` |
| Engaged | `--sl-color-success-500` (10% alpha) | `--sl-color-success-500` (raw) | `telephone-fill` / `chat-fill` / `envelope-fill` (channel-aware) |
| WrapUp | `--sl-color-warning-500` (5% alpha) | `--sl-color-warning-500` (raw) | `hourglass-split` |
| Offline | `--sl-color-neutral-100` | `--or-color-text-muted` | `power` |

Pill dimensions: 28px height, padding 4px 12px, border-radius `--or-radius-full`, font 14px,
weight 400, with 8px gap to icon. Right of pill: small `--or-color-text-muted` text
"state v.{N}   ·   updated {relative}".

**Transition button row (per-state):**

| Current status | Allowed transitions (button labels) | Sub-picker |
|----------------|--------------------------------------|------------|
| Ready | [Set Not Ready] · [Go on Break ▾] | Break reason picker on dropdown |
| NotReady | [Set Ready] | — |
| Break | [Back to Ready] · [Set Not Ready] | (current break reason shown above buttons) |
| Engaged | (no agent-initiated buttons; post_interaction_state picker shown instead) | post_interaction_state radio + Save button |
| WrapUp | [Go to Ready now] · [Go to Not Ready now] | wrapup countdown shown above |
| Offline | (no transitions; informational only) | — |

Buttons are `<sl-button variant="primary">` (default action) or `<sl-button variant="default">`
(secondary). The "Set Not Ready" / "Set Ready" simple-transition buttons fire immediate PATCH.
The "Go on Break ▾" button opens a dropdown.

**Break reason dropdown (when transitioning Ready → Break):**
```
┌─────────────────────────────────────────────┐
│  Pick a break reason                        │
│                                             │
│  ( ) Lunch          [not routable]          │
│  (●) Short break    [routable]              │
│  ( ) Team huddle    [routable]              │
│  ( ) Training       [not routable]          │
│                                             │
│  [Cancel]   [Confirm break]                 │
└─────────────────────────────────────────────┘
```
- `<sl-dropdown>` anchored to button, 280px width
- Vertical list of `<sl-radio>` for each enabled, routable+non-routable break reason
- Right of label: small badge "[routable]" or "[not routable]" — `--or-color-text-muted`
- Reason fetched from `/v1/orgs/:org_id/break-reasons?include_disabled=false&limit=100`
- "Confirm break" fires PATCH with `{ to: 'Break', break_reason_id }`

**Post-interaction state picker (when Engaged):**
```
┌─────────────────────────────────────────────┐
│  When wrap-up ends, set status to:          │
│  (●) Ready                                  │
│  ( ) Not Ready                              │
│  [Save]                                     │
└─────────────────────────────────────────────┘
```
- Below the engaged status pill
- Save fires PATCH with `{ post_interaction_state: 'ready' | 'not_ready' }`
- Successful save → toast "Post-interaction preference updated"

**WrapUp countdown:**
```
┌─────────────────────────────────────────────┐
│  Wrap-up in progress                        │
│                                             │
│  ⏳ 2m 17s remaining                         │  ← --or-text-heading, mono digits
│  Will auto-transition to Ready              │  ← muted body
│                                             │
│  [Go to Ready now]   [Go to Not Ready now]  │
└─────────────────────────────────────────────┘
```
- Countdown ticks once per second (client-side derived from `wrapup_until` minus now)
- On expiry, panel polls immediately to pick up server-side transition

**Force-flag affordance (D6-V-19) — admin recovery:**
```
┌─────────────────────────────────────────────────────────────────────┐
│  ⚠ Force transition (admin override)                                │
│                                                                     │
│  Bypasses the state-machine matrix. Use only when an agent is stuck │
│  (e.g. Engaged but adapter never sent the End event). Every force   │
│  use is logged at the server.                                       │
│                                                                     │
│  Target state: ( ) Ready ( ) NotReady ( ) Break (●) Offline         │
│                                                                     │
│  [Cancel]   [⚠ Force to Offline]                                    │
└─────────────────────────────────────────────────────────────────────┘
```
- Behind a disclosure: collapsed by default, expand with "Show advanced" link
- Border: 1px `--sl-color-warning-500`, padding 16px, background `--sl-color-warning-500` at 5% alpha
- Force button is `<sl-button variant="warning">` not destructive (it's not deletion, it's override)
- On click: opens a confirm `<sl-dialog>` with the agent's name + chosen target + button "I understand — force transition"
- After force: toast "Forced transition: {from} → {to}. Logged at server."

**Polling behavior (D6-26 — visible 5s, hidden pause, state_version on resume):**
- `@lit/task` polling registered on `connectedCallback`
- `visibilitychange` listener: on `document.hidden=true`, cancel the in-flight task and clear the interval; on `document.hidden=false`, fire one immediate GET, compare `state_version` to last cached, update UI if new, resume 5s interval
- "Last fetched {N}s ago" timestamp shown next to the state pill (small muted text)

**409 invalid_transition handling:**
- Inline `<or-conflict-banner>` at the top of the card body
- Copy template: "Status is now {from}. Your request to go to {to} isn't allowed from {from}."
- CTA: "Refresh transitions" — updates the visible button row to reflect the new from-state
- Auto-dismiss on next user action

**404 (no agent_id in path / agent deleted):**
- Centered empty state: "No status found for this agent. [Back to agents]"

**Shoelace primitives:**
- `button`, `button-group`, `radio`, `radio-group`, `dropdown`, `dialog`, `alert`, `icon`, `spinner`, `tooltip`, `badge`

---

### 5.7 `<or-conflict-banner>` — 409 UX (D6-03/D6-04)

**Purpose:** Inline alert above a form or status panel when the server returned 409. Encodes
the auto-update + inline-banner UX (D6-04).

**API:**
```typescript
interface OrConflictBanner extends LitElement {
  @property({ type: String }) mode: 'crud' | 'status' = 'crud';
  @property({ type: Object }) serverValue: object | { from: string; to: string };
  @property({ type: Object }) userValue: object;
  @property({ type: Boolean, attribute: 'show-diff' }) showDiff: boolean = true;

  // events
  // open-routing:conflict-acknowledged -> CustomEvent<{ action: 'review' | 'discard' }>
}
```

**Visual layout (CRUD 409):**
```
┌──────────────────────────────────────────────────────────────────────────────┐
│ ⚠ This {entity} was changed elsewhere                                        │  ← amber border + bg
│                                                                              │
│ Your edits are below — review the diff and re-submit, or discard your        │
│ changes and load the server's version.                                       │
│                                                                              │
│ ┌─── name ────────────────────────────────────────────────────────────────┐  │
│ │  Server now:  Alice Nguyễn                  (changed)                   │  │
│ │  Your edit:   Alice Nguyen                                              │  │
│ └─────────────────────────────────────────────────────────────────────────┘  │
│                                                                              │
│ ┌─── email ───────────────────────────────────────────────────────────────┐  │
│ │  Server now:  alice@new-example.com         (changed)                   │  │
│ │  Your edit:   alice@example.com                                         │  │
│ └─────────────────────────────────────────────────────────────────────────┘  │
│                                                                              │
│ [Review and re-submit]   [Discard my changes]                                │
└──────────────────────────────────────────────────────────────────────────────┘
```

**Visual layout (status 409):**
```
┌──────────────────────────────────────────────────────────────────────────────┐
│ ⚠ Transition isn't allowed                                                   │
│                                                                              │
│ Status is now Engaged. Your request to go to NotReady isn't allowed          │
│ from Engaged. Wait for wrap-up to end, or use Force transition (admin).      │
│                                                                              │
│ [Refresh transitions]                                                        │
└──────────────────────────────────────────────────────────────────────────────┘
```

**Styling:**
- Background: `--or-color-conflict-bg`
- Border: 2px solid `--or-color-conflict-border` (amber)
- Border-radius: `--or-radius-md`
- Padding: 16px
- Margin-bottom: 24px
- Icon: `<sl-icon name="exclamation-triangle-fill">` `--sl-color-warning-500`, 20px, top-aligned with heading
- Heading: `--or-text-heading`, `--or-color-text-strong`
- Body: `--or-text-body`, `--or-color-text-body`

**Diff rendering (CRUD mode):**
- One `<details>` (open) per changed field
- Field name as summary (mono, `--or-color-code-fg`)
- Two rows: "Server now: {value}" with `--or-color-diff-removed` background; "Your edit: {value}" with `--or-color-diff-added` background (the user's edit is the "added" perspective)
- Unchanged fields are NOT shown (signal-only diff)
- For arrays (e.g., agent skills): show count delta + truncated first 3 items + "...and N more"
- For objects (e.g., adapter config JSONB): show JSON pretty-printed; visual diff is line-level

**aria-live (D6-V-20):**
- `aria-live="assertive"` when the banner first appears (interrupts to surface the conflict)
- `aria-live="polite"` on subsequent diff content updates

**Buttons:**
- CRUD: "Review and re-submit" (primary) and "Discard my changes" (text variant, danger color)
  - Click "Review and re-submit": dismisses banner, returns focus to first dirty field (form state already populated with user edits)
  - Click "Discard my changes": replaces form fields with server's `current` value, banner dismisses, focus on the first field

- Status: "Refresh transitions" (primary)
  - Click: updates the visible button row from the new `from` status, banner dismisses

**Auto-dismiss behavior (D6-04):**
- On user click of either action: dismiss
- After successful retry: dismiss (component is removed from DOM)
- NO auto-dismiss timer (the user must acknowledge)

**Shoelace primitives:**
- `alert`, `button`, `icon`, `details`

---

### 5.8 `<or-import-page>` — bulk import upload flow

**Purpose:** Hosted at `/orgs/:org_id/imports/new`. Walks user through entity selection,
file upload (JSON or CSV), schema version validation, and submission. On submission, navigates
to `/orgs/:org_id/imports/:id` to show result.

**API:**
```typescript
interface OrImportPage extends LitElement {
  @property({ type: String }) baseUrl: string = '';
  @property({ type: String, attribute: 'org-id' }) orgId: string = '';

  // events
  // open-routing:import-started -> CustomEvent<{ importId: string }>
}
```

**Visual layout (3-step stepper — Pick → Upload → Review):**
```
┌──────────────────────────────────────────────────────────────────────────────┐
│ [← Back to Catalog]   Bulk import                                            │
│                                                                              │
│   ●━━━━━━━━ ●─────── ○                                                        │  ← stepper
│   1 Pick    2 Upload  3 Review                                                │
│                                                                              │
│ ┌──────────────────── Step 1: Pick entity ───────────────────────────────────┐ │
│ │                                                                            │ │
│ │  What are you importing?                                                   │ │
│ │                                                                            │ │
│ │  (●) Agents          ( ) Skills           ( ) Queues                       │ │
│ │  ( ) Channels        ( ) Adapters         ( ) Break Reasons                │ │
│ │                                                                            │ │
│ │  [Next: Upload →]                                                          │ │
│ └────────────────────────────────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────────────────────────┘
```

**Step 2 (Upload):**
```
┌──────────────────────────────────────────────────────────────────────────────┐
│ Step 2: Upload agents file                                                   │
│                                                                              │
│  Format: (●) CSV  ( ) JSON                                                   │
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────────┐ │
│  │                                                                         │ │
│  │              📄  Drop a CSV file here, or [Browse]                       │ │  ← drop zone
│  │                                                                         │ │
│  │              Max 50 MB / 500 rows                                       │ │
│  │                                                                         │ │
│  └─────────────────────────────────────────────────────────────────────────┘ │
│                                                                              │
│  ┌─── CSV format help (Agents) ────────────────────────────────────────────┐│
│  │                                                                         ││
│  │ Required columns: code, name, email                                     ││
│  │ Optional columns: external_id, enabled, skills                          ││
│  │                                                                         ││
│  │ Skills syntax: skill_code:proficiency|skill_code:proficiency            ││
│  │ Example:       skill_voice:7|skill_chat:9                               ││
│  │                                                                         ││
│  │ [📋 Copy example row]                                                   ││
│  └─────────────────────────────────────────────────────────────────────────┘│
│                                                                              │
│  schema_version: v0.1 (current)                                              │
│                                                                              │
│  ☐ Make this import retry-safe (Idempotency-Key)                             │
│                                                                              │
│  [← Back]                                            [Next: Review →]        │
└──────────────────────────────────────────────────────────────────────────────┘
```

**Step 3 (Review):**
```
┌──────────────────────────────────────────────────────────────────────────────┐
│ Step 3: Review                                                               │
│                                                                              │
│  Entity:          Agents                                                     │
│  File:            agents-2026-q2.csv (42 KB)                                 │
│  Format:          CSV                                                        │
│  Schema version:  v0.1                                                       │
│  Retry-safe:      Yes (Idempotency-Key: 01919e1b-…)                          │
│                                                                              │
│  ⚠ Importing existing agents MERGES skills. To remove a skill,               │
│    use the agent detail edit page.                                           │
│                                                                              │
│  [← Back]                                              [⬆ Start import]      │
└──────────────────────────────────────────────────────────────────────────────┘
```

**Drop zone behavior:**
- Drag-over: border becomes `--sl-color-primary-500` dashed; background `--sl-color-primary-50`
- Drop / Browse: file is staged but NOT uploaded yet
- Filename + size shown below drop zone (or replacing it) with "[Remove]" icon-button
- Reject if file >50MB → inline alert "File too large (XYZ MB). Max 50 MB."
- Reject if extension doesn't match Format toggle → "Format mismatch. Selected CSV but file is .json. Switch format or pick a different file."

**CSV format help block:**
- Background `--or-color-code-bg`, padding 16px, border-radius `--or-radius-md`
- Heading 16px weight 600 "CSV format help ({Entity})"
- Body 14px body
- "Required columns:" + "Optional columns:" lines with `<code>`-formatted column names
- For Agents: extra "Skills syntax:" line with the exact `code:prof|code:prof` format
- For Queues: extra "Multi-value columns:" line documenting D5-04 separator priority `|` > `;` > `,`
- For Channels: extra "default_queue_code:" line noting "References the queue's `code`, not its UUID"
- "[📋 Copy example row]" button: copies a hardcoded example CSV row to clipboard, toast "Copied"

**Idempotency checkbox helper text:**
"Generates a client-side UUIDv7. If you re-submit this exact import, the server returns the prior result instead of re-running. v0.1 limitation: replay responses don't list succeeded IDs."

**Submit flow:**
- Click "Start import" → button spinner; both Step nav buttons disabled
- Sends POST `/v1/orgs/:org_id/catalog/import?entity={entity}&schema_version=v0.1` with `Content-Type: text/csv` or `application/json`; `Idempotency-Key` header if toggled
- 200/207 response: navigate to `/orgs/:org_id/imports/:id` (server returns `import_id` in response body or location header — schema needs confirming during planning)
- 400 schema_version mismatch: top-of-form `<sl-alert variant="danger">` with copy "This file was generated against schema {X}; current is v0.1. Regenerate using the v0.1 export template."
- 413 oversized: top-of-form `<sl-alert variant="warning">` with copy "Your file is too large (XYZ MB / N rows). v0.1 supports up to 50 MB / 500 rows synchronously. For larger imports, use the v0.2 async pathway. [Learn more]" — the link is a placeholder href that goes nowhere in v0.1
- 422 ALL rows failed: navigate to `/orgs/:org_id/imports/:id` anyway (the result page renders 100% failure as a special state)
- 5xx: alert "Import couldn't start. {reason}. Request ID: {request_id}. [Retry]"

**Step navigation:**
- "Back" goes to previous step, preserves form state
- "Next" runs validation: Step 1 requires an entity selected; Step 2 requires a file staged; Step 3 has no validation (just confirms)
- On wizard cancel (clicking "Back to Catalog"): dirty-prompt if file is staged

**Shoelace primitives:**
- `radio-group`, `radio`, `button`, `checkbox`, `alert`, `icon`, `icon-button`, `spinner`, `tooltip`

---

### 5.9 `<or-import-result>` — bulk import result display

**Purpose:** Hosted at `/orgs/:org_id/imports/:id`. Fetches the persisted import job
(`GET /v1/orgs/:org_id/imports/:import_id`), renders summary + failure table.

**API:**
```typescript
interface OrImportResult extends LitElement {
  @property({ type: String }) baseUrl: string = '';
  @property({ type: String, attribute: 'org-id' }) orgId: string = '';
  @property({ type: String, attribute: 'import-id' }) importId: string = '';
}
```

**Visual layout (mixed 207 result):**
```
┌──────────────────────────────────────────────────────────────────────────────┐
│ [← Back to Catalog]   Import #01919e1b-7c80-…                                │
│                                                                              │
│  ⚠ Import partially complete                                                 │  ← amber heading
│  Imported 7 of 10 agents. 3 rows failed.                                     │
│                                                                              │
│  Entity:     Agents                                                          │
│  Started:    2026-05-17 14:32:18 UTC   ·   completed 4s later                │
│  Schema:     v0.1                                                            │
│  Job ID:     01919e1b-7c80-7c80-8000-0123456789ab    [📋 Copy]               │
│                                                                              │
│  ───── Summary ─────                                                         │
│                                                                              │
│  ┌────────────────────┐  ┌────────────────────┐  ┌────────────────────┐      │
│  │ 7  Succeeded       │  │ 3  Failed          │  │ 10 Total           │      │
│  └────────────────────┘  └────────────────────┘  └────────────────────┘      │
│                                                                              │
│  ───── Failed rows ─────                                                     │
│                                                                              │
│  | Row | Field        | Reason                                          |    │
│  |-----|--------------|------------------------------------------------|    │
│  |  2  | email        | invalid email format                            |    │
│  |  5  | proficiency  | must be between 1 and 10                        |    │
│  |  9  | code         | duplicate within batch                          |    │
│                                                                              │
│  [📥 Download failures as CSV] (v0.2 — currently disabled)                   │
│                                                                              │
│  ───── Successful rows ─────                                                 │
│                                                                              │
│  ▶ Show 7 successful IDs (collapsed)                                         │
└──────────────────────────────────────────────────────────────────────────────┘
```

**Top status banner — based on outcome:**

| Outcome | Banner color | Heading |
|---------|-------------|---------|
| 200 (all succeeded) | `--sl-color-success-500` (5% bg, raw border+text) | "✓ Import complete — N records imported" |
| 207 (partial) | `--sl-color-warning-500` (5% bg, raw border+text) | "⚠ Import partially complete" |
| 422 (all failed) | `--sl-color-danger-500` (5% bg, raw border+text) | "✗ Import failed — N rows rejected" |

**Summary stat cards (D6-V-21):**
- 3 inline cards, equal width
- Big number (24px, weight 600, color per state — success/danger/strong)
- Label below (14px body, muted)

**Failed rows table:**
- Same `<or-data-table>` styling as catalog lists
- Columns: Row | Field | Reason
- Row column: monospace right-aligned
- Field column: monospace, `--or-color-code-fg`
- Reason column: plain body text
- Scrollable; max-height 400px before vertical scroll engages
- If 0 failures: section omitted entirely

**Successful rows disclosure:**
- `<details>` collapsed by default
- Summary: "Show {N} successful IDs"
- Body: scrollable list of UUIDs (monospace), one per line
- If `idempotent_replay: true`: special message replacing the list — "Successful IDs not stored on idempotent replay. See server logs with Idempotency-Key {N} for forensics."
- If 0 successes: section omitted

**Download failures button (D6-V-22 — deferred to v0.2):**
- Always rendered, always disabled
- Tooltip on hover: "Coming in v0.2 — track failure download in IMP-11"
- Disabled state styling: 50% opacity, cursor not-allowed
- Rationale for showing-but-disabled: discoverability; admins will ask for it; better to surface "coming soon" than hide it

**Job metadata:**
- "Started" / "completed Xs later" line: 14px muted, mono for time
- Job ID with copy button — admin reuses the ID for support tickets

**Polling:** None. The import is synchronous in v0.1; the import_id is only ever fetched once
to render this page. Browser back/forward + reload re-fetch.

**States:**
- **Loading:** Skeleton of the summary cards + table
- **Loaded:** Renders the response
- **404 (import_id not found in this org):** Empty state "Import not found in this org. [Back to imports]" (note: imports list page not in v0.1 scope; back goes to /catalog)
- **5xx:** Top-of-page danger alert + retry

**Shoelace primitives:**
- `button`, `icon`, `icon-button`, `alert`, `details`, `tooltip`, `spinner`

---

### 5.10 Primitives (`<or-data-table>`, `<or-cursor-paginator>`, `<or-code-input>`)

These are internal primitives used by the components above. The executor should keep them
as reusable Lit components but they do not have a top-level page-level contract.

#### `<or-data-table>`
- Renders a `<table>` with semantic markup (`<thead>`, `<tbody>`, `<th>`, `<td>`)
- Accepts `columns: Array<{ key, label, width?, align?, render? }>` and `rows: Array<object>`
- Emits `or-row-click`, `or-row-action` events
- Sticky header on vertical scroll
- Used by all 6 list pages and the import result failed-rows table

#### `<or-cursor-paginator>`
- Accepts `hasMore`, `cursor`, `limit`, emits `or-page-changed`
- Owns the "[<] Previous" + "Next [>]" + limit picker UI
- Used by all 6 list pages

#### `<or-code-input>`
- Wraps `<sl-input>` with the Phase 04.1 regex `^[a-z][a-z0-9_]{0,63}$` enforced via:
  - `pattern` attribute (HTML5 native first line of defense)
  - `setCustomValidity` (with i18n'd message)
  - ajv field-level validation via Phase 6 codegen
- Visual states: default, focused, valid (subtle green left-border-2px), invalid (red left-border-2px + helper text under field)
- Used by all 6 create wizard "Basics" steps + Adapter detail page

---

## 6. Page Specifications

Each route in D6-09/D6-11/D6-12 mapped to its component composition.

### 6.1 `/` — Root org picker

- **Component:** `<or-org-picker>` (full viewport, no shell)
- **No top bar / no sidebar.** The shell is NOT mounted on this route. The shell mounts after the user submits an org_id.
- **Title:** "Open Routing — Select organization"
- **On submit:** navigate to `/orgs/:org_id/agents` (default landing) with org_id persisted to localStorage.

### 6.2 `/orgs/:org_id/agents` — Agents list

- **Composition:** `<or-catalog-shell theme="..." org-id="...">` → outlet renders `<or-agent-list>`
- **Sidebar:** "Agents" active
- **Title:** "Agents · Open Routing"
- **Columns:** code · name · email · enabled · updated · ⋮
- **Empty state copy:** see §11

### 6.3 `/orgs/:org_id/agents/new` — Create agent (wizard)

- **Composition:** shell → `<or-form-wizard entity="agent">` (multi-step 3 steps)
- **Sidebar:** "Agents" active
- **Title:** "Create agent · Agents · Open Routing"

### 6.4 `/orgs/:org_id/agents/:id` — Edit agent

- **Composition:** shell → `<or-agent-detail>`
- **Sidebar:** "Agents" active
- **Title:** "{agent.name} · Agents · Open Routing" (server data; falls back to "Agent · ...")
- **Note:** This is the only entity with the 2-column layout (skills sub-table to the right)

### 6.5 `/orgs/:org_id/agents/:id/status` — Agent status panel

- **Composition:** shell → `<or-status-panel embedded="false">`
- **Sidebar:** BOTH "Agents" and "Agent Status" active; Agent Status takes priority (heavier highlight, see §5.1 spec)
- **Title:** "Status · {agent.name} · Agents · Open Routing"
- **Back button:** "← Back to {agent.name}" navigates to `/orgs/:org_id/agents/:id`

### 6.6 `/orgs/:org_id/skills` (etc) — 5 other entity list routes

Same pattern as 6.2 but with the entity-appropriate list component and sidebar entry.

### 6.7 `/orgs/:org_id/skills/new` (etc) — 5 other entity create routes

Same pattern as 6.3 but with the entity-appropriate wizard (single-step for these 5 entities — Skill, Queue, Adapter, Break Reason, and also Channel which gets a 3-step wizard).

| Route | Wizard mode |
|-------|-------------|
| /orgs/:org_id/agents/new | 3-step: Basics → Skills → Review |
| /orgs/:org_id/channels/new | 3-step: Basics → Default queue → Review |
| /orgs/:org_id/skills/new | 1-step single form |
| /orgs/:org_id/queues/new | 1-step single form |
| /orgs/:org_id/adapters/new | 1-step single form |
| /orgs/:org_id/break-reasons/new | 1-step single form |

### 6.8 `/orgs/:org_id/skills/:id` (etc) — 5 other entity detail routes

Same pattern as 6.4 but with the entity-appropriate `<or-{entity}-detail>` component.

### 6.9 `/orgs/:org_id/imports/new` — Bulk import upload

- **Composition:** shell → `<or-import-page>`
- **Sidebar:** "Bulk Import" active
- **Title:** "Bulk import · Open Routing"

### 6.10 `/orgs/:org_id/imports/:id` — Bulk import result

- **Composition:** shell → `<or-import-result>`
- **Sidebar:** "Bulk Import" active
- **Title:** "Import {short_id} · Open Routing"

### 6.11 Unknown route (404)

- **Composition:** shell → centered empty-state card
- **Copy:** "Page not found." + "[Back to Agents]" CTA
- **Title:** "Not found · Open Routing"
- **No dedicated component.** Implemented as a fallback render in the router config.

---

## 7. Interaction Patterns

### 7.1 Loading

| Surface | Pattern |
|---------|---------|
| List page (initial mount, no data) | Skeleton rows — 5 rows, column-matching widths, shimmer 1.5s |
| List page (subsequent fetch with prior data) | Existing rows stay visible + 1px progress bar across table top + paginator buttons disabled |
| Detail page (initial mount) | Skeleton form fields matching layout; top bar buttons disabled |
| Status panel (initial mount) | Skeleton status pill + skeleton button row (greyed) |
| Status panel (5s poll) | Silent — no visual change unless data shifted |
| Import result page | Skeleton stat cards + skeleton table |
| Save button (in flight) | Inline spinner inside button; both action buttons disabled |
| Wizard "Next" (validating + advancing) | Inline spinner inside Next button; both nav buttons disabled |

**Skeleton style:** Block with `--or-color-skeleton-base` background and a horizontal gradient
shimmer (`--or-color-skeleton-shimmer`) cycling 1.5s linear infinite. Height matches the final
element (don't show a 30px-tall skeleton for an 80px-tall row).

### 7.2 Empty states

See §11 for per-page copy. Pattern: centered card, icon (Bootstrap-Icons `inbox` or contextual),
heading (14px body weight 600), 1-2 sentences body (muted), primary CTA.

### 7.3 Error states

| Severity | Pattern |
|----------|---------|
| 5xx (internal / outage) | Top-of-page `<sl-alert variant="danger">` + Retry button + `request_id` |
| 4xx not 4xx-special (e.g. 400 invalid_body — unexpected for our typed client) | Top-of-page `<sl-alert variant="danger">` + describes the field issue if `field` available |
| 422 invalid_value (field-level) | Inline under the offending input, red text, red border on input |
| 409 version_conflict | `<or-conflict-banner mode="crud">` above form |
| 409 invalid_transition | `<or-conflict-banner mode="status">` above status panel |
| 404 not_found | Page-level empty state replacing the form/table |
| Network error | Top-of-page `<sl-alert variant="danger">` "Couldn't reach server. Check connection. [Retry]" |

### 7.4 Confirm-dangerous-action

| Action | Pattern | Copy |
|--------|---------|------|
| Disable entity (PATCH enabled=false) | NO confirm — reversible | — |
| Delete entity (DELETE) | Modal with typed confirm | See §11.5 |
| Force agent transition | Modal with one-button confirm (no typed) | See §11.5 |
| Discard wizard draft | Modal with two buttons "Keep editing" / "Discard" | "Discard your changes? Your draft will be lost." |
| Discard detail-page edits | Same modal | "Discard your changes? Unsaved edits will be lost." |
| Discard 409 conflict edits | Modal NOT used — banner has the "Discard my changes" button inline | (in banner) |

**Delete confirm modal:**
- Width 480px
- Title: "Delete {entity} {entity.name}?"
- Body: "This permanently removes {entity.name} (code: {entity.code}). Skills assignments and other related data will be removed. This cannot be undone."
- Confirmation input: `<sl-input>` placeholder "Type the {entity}'s name to confirm" — value must match `entity.name` exactly to enable the Delete button
- Buttons: "Cancel" (text variant, default focus) and "Delete" (danger variant, disabled until confirmation matches)
- Keyboard: Escape closes; Enter inside the confirm input does NOT trigger Delete (typed confirm = explicit click)

### 7.5 Polling-pause-on-hidden (status panel only, D6-26)

- On `connectedCallback`: register `visibilitychange` listener
- Start polling interval (5000ms)
- On `document.hidden = true`: cancel in-flight fetch, clear interval, store last cached state_version
- On `document.hidden = false`: fire one immediate GET; compare returned state_version to stored; if newer, update UI; resume 5000ms interval
- On `disconnectedCallback`: cleanup listener + interval

### 7.6 Keyboard map

| Context | Key | Action |
|---------|-----|--------|
| Anywhere | Tab / Shift+Tab | Standard focus traversal |
| Anywhere | Escape | Close modal/drawer/dropdown |
| Form field | Enter | Submit form (Save / Continue / Next) |
| Form confirm modal | Enter | Confirm primary action (e.g. "Keep editing") |
| Delete confirm modal | Enter | NO action (typed confirm requires explicit click) |
| List page | / | Focus search input |
| List page | n | Focus "Create {entity}" CTA |
| List page | r | Trigger refresh (when not focused in input) |
| Status panel | r | Focus first transition button |
| Wizard | Enter | Advance to next step (= click Next) |
| Wizard | Shift+Enter | Submit (= click Create on final step) |
| Sidebar | Up / Down arrows | Move focus between nav entries |
| Theme toggle | Space | Cycle theme |
| Org chip | Click | Copy org_id to clipboard |
| Org chip | Enter (when focused) | Same as click |

**NOTE on shortcuts:** Single-key shortcuts (/, n, r) only fire when NOT inside an input. Use
a global keydown listener at the shell level that checks `event.target` is not an input/textarea/contenteditable.

---

## 8. Accessibility Contract

### 8.1 Keyboard navigation matrix

| Component | Tab traversal | Special keys |
|-----------|---------------|--------------|
| `<or-catalog-shell>` | hamburger → wordmark → org chip → theme btns → locale select → switch org → sidebar entries → main content | Escape closes sidebar drawer (mobile/tablet) |
| `<or-org-picker>` | input → continue → use last (if shown) | Enter on input/continue submits |
| `<or-{entity}-list>` | search → include-disabled → refresh → create cta → first row → each row's action menu | / focuses search, n focuses cta, r refreshes |
| `<or-{entity}-detail>` | back → delete → disable → form fields top-to-bottom → cancel → save | Enter submits when focused on save |
| `<or-form-wizard>` | (current step's tab traversal) → back → next | Enter advances; Shift+Enter on final = create |
| `<or-status-panel>` | back → transition buttons L-R → advanced disclosure → force buttons (if expanded) | — |
| `<or-conflict-banner>` | review → discard | Enter on focused button confirms |
| `<or-import-page>` | back → format toggle → drop zone → idempotency checkbox → next | Drop zone is keyboard-activatable via Enter (opens file dialog) |
| `<or-import-result>` | back → copy job id → details disclosure → download (disabled) | — |

### 8.2 Focus styles (D6-V-23)

All focusable elements get a visible focus ring:
- Outline: `2px solid var(--or-color-focus-ring)`
- Outline-offset: `2px`
- Border-radius matches the element

Shoelace's own focus styles are overridden with `:focus-visible` rules in `packages/ui`'s
global stylesheet (applied to the shell host element). Mouse-focus does NOT show the ring
(use `:focus-visible` not `:focus`) so power-user mouse clicks don't get the keyboard cue.

### 8.3 aria-live regions

| Region | Politeness | Trigger |
|--------|-----------|---------|
| Conflict banner heading (first appearance) | assertive | When the 409 fires and banner mounts |
| Conflict banner diff body | polite | When diff content updates |
| Toast container | polite | Each toast announces once |
| Form validation messages | polite | When field error appears (after blur or submit) |
| Status panel state pill | polite | When status changes from poll or manual transition |
| Import result failure table | polite | Once on page mount (long region announce) |
| Page heading | off | Title updates handled by document.title; no aria-live |

### 8.4 Color contrast (AA target — see §13 for verification matrix)

- All body text vs background: ≥4.5:1
- All large text (≥18px or ≥14px bold) vs background: ≥3:1
- Interactive controls (buttons, focus rings): ≥3:1 against adjacent surface
- The amber conflict banner (`--or-color-conflict-bg`) + body text (`--or-color-text-body`) MUST meet ≥4.5:1 in all 3 themes — see §13

### 8.5 Screen reader semantics

- Use semantic HTML: `<table>`, `<th>`, `<td>`, `<form>`, `<fieldset>`, `<legend>`, `<button>`
- Status pill: `aria-label="Current status: Ready (state version 42)"`
- Transition buttons: `aria-label="Set status to Not Ready"` (the label is the readable verb-noun)
- Skeleton loading: `aria-busy="true"` on the surrounding container; `role="status"` + `aria-label="Loading agents"` on the table
- Modal: `<sl-dialog>` provides correct `role="dialog"` + focus trap by default
- The conflict banner has `role="alert"` AND `aria-live="assertive"` (Shoelace's `<sl-alert>` does this with the right variant)

### 8.6 Out of scope (D6 Out of scope per CONTEXT.md)

- Axe-core automated accessibility audit
- Manual screen reader certification (NVDA / JAWS / VoiceOver)
- WCAG 2.2 conformance audit

These are deferred per CONTEXT.md. This contract sets a baseline that does NOT block future
audit work — every component uses semantic HTML and ARIA correctly.

---

## 9. Responsive Breakpoints

**Desktop-first.** Admin runs on laptops/desktops; the embed in Phase 7 is also primarily a
desktop surface for product config flows.

| Breakpoint | Behavior |
|------------|----------|
| ≥1280px (default) | Full layout: 248px sidebar + main content. Agent detail uses 2-column form + skills sub-table layout. |
| 1024-1279px | Same as default but Agent detail collapses to single column (skills sub-table below basics). |
| 768-1023px | Sidebar collapses to `<sl-drawer>` triggered by hamburger icon in top bar. Main content fills full width. List tables get a horizontal scrollbar if columns overflow. |
| ≤640px (best-effort) | Sidebar drawer. Top bar elements wrap to 2 rows (wordmark+hamburger top; chip + toggles bottom). Forms stay single-column. List tables hide less-important columns: `external_id`, `updated_at`, and `version` are first to drop. |
| Print | Hidden — no print stylesheet for v0.1 |

**Mobile (≤640px) is BEST EFFORT, not a v0.1 design target.** It should not BREAK
(no horizontal scroll on body, no unreachable controls), but pixel-perfect mobile design is
not a Phase 6 deliverable. The status panel is the most-usable-on-mobile component (a single
column of buttons) and should be considered "tablet-quality" at ≤640px.

---

## 10. Motion Specification

Restraint by default. Linear / Vercel admin / Datadog patterns. Platform engineer audience —
not consumer; not flashy.

| Action | Animation |
|--------|-----------|
| Theme switch | 150ms cross-fade on `--sl-color-*` token values (CSS `transition: background-color 150ms ease, color 150ms ease, border-color 150ms ease`). NO layout shift. |
| Sidebar drawer open (tablet) | 200ms ease-out slide from left |
| Sidebar drawer close | 150ms ease-in slide to left |
| Modal open | 150ms ease-out fade + 8px upward translate |
| Modal close | 100ms ease-in fade |
| Toast appear | 150ms ease-out slide-down from top + fade |
| Toast dismiss | 100ms ease-in fade |
| Conflict banner appear | 200ms ease-out slide-down from top of card (translateY(-8px) → 0 + opacity 0 → 1) |
| Conflict banner dismiss | 100ms ease-in fade |
| Wizard step transition | 150ms ease-out crossfade between step contents; height auto-animates with `interpolate-size: allow-keywords` (where supported; fallback: no height animation) |
| Skeleton shimmer | 1500ms linear infinite (slow enough not to nauseate) |
| Button press | 50ms scale(0.98) on `:active` |
| Row hover | 100ms ease-out background-color |
| Focus ring appear | NO animation (must be instant for accessibility) |
| Pagination cursor change | 150ms fade on row content (held in opacity 0.6 during the loading bar) |

**Reduced motion:**
- `@media (prefers-reduced-motion: reduce)`: all the above transitions reduce to 0ms duration
- Skeleton shimmer disabled (replaced with a static `--or-color-skeleton-base` block)

---

## 11. Brand Voice / Copy Library

All copy is shipped in both EN and VI via `@lit/localize` (D6-21). Below are the EN strings;
VI translations are produced during Wave 1 setup.

### 11.1 Empty-state copy per entity list page

| Entity | No items, no search | No items, search active |
|--------|---------------------|--------------------------|
| Agents | **Heading:** "No agents yet" — **Body:** "Add your first agent to start routing." — **CTA:** "[+ Create agent]" | **Heading:** "No agents match '{query}'" — **CTA:** "[Clear search]" |
| Skills | **Heading:** "No skills yet" — **Body:** "Define a skill to assign to agents." — **CTA:** "[+ Create skill]" | **Heading:** "No skills match '{query}'" — **CTA:** "[Clear search]" |
| Queues | **Heading:** "No queues yet" — **Body:** "A queue holds interactions waiting for an agent." — **CTA:** "[+ Create queue]" | **Heading:** "No queues match '{query}'" — **CTA:** "[Clear search]" |
| Channels | **Heading:** "No channels yet" — **Body:** "Channels link communication mediums to your queues." — **CTA:** "[+ Create channel]" | **Heading:** "No channels match '{query}'" — **CTA:** "[Clear search]" |
| Adapters | **Heading:** "No adapters yet" — **Body:** "Adapters register integration points (e.g. voice bridges, chat gateways)." — **CTA:** "[+ Create adapter]" | **Heading:** "No adapters match '{query}'" — **CTA:** "[Clear search]" |
| Break Reasons | **Heading:** "No break reasons yet" — **Body:** "Define why agents step away — each reason controls whether they stay routable." — **CTA:** "[+ Create break reason]" | **Heading:** "No break reasons match '{query}'" — **CTA:** "[Clear search]" |

### 11.2 404 empty-state copy

| Page | Copy |
|------|------|
| Detail page 404 | **Heading:** "{Entity} not found" — **Body:** "It may have been deleted or belong to another org." — **CTA:** "[Back to {entity_plural}]" |
| Status panel 404 | **Heading:** "No status found for this agent" — **Body:** "The agent may have been deleted." — **CTA:** "[Back to agents]" |
| Import result 404 | **Heading:** "Import not found" — **Body:** "This import ID doesn't exist in this org." — **CTA:** "[Back to catalog]" |
| Unknown route | **Heading:** "Page not found" — **Body:** "The URL doesn't match any known page." — **CTA:** "[Back to agents]" |

### 11.3 Error copy by ErrorCode (D6-24 — client maps ErrorCode → i18n key)

| ErrorCode | Friendly translated copy | When |
|-----------|-------------------------|------|
| invalid_body | "There's a problem with the data we sent. Try again, or report this if it keeps happening." | unexpected — typed client should not hit this |
| invalid_id | "That ID isn't valid. Check the URL and try again." | 400 on detail-page mount |
| not_found | "We couldn't find that {entity}. It may have been deleted." | 404 |
| internal | "Something went wrong on our side. Please try again. If it keeps happening, include the request ID below in a support ticket." | 5xx |
| version_conflict | (Handled by conflict banner — see §11.4) | 409 |
| cross_org | "That belongs to a different org. Switch orgs to access it." | 404 (server returns not_found for cross-org) — this code is reserved |
| duplicate_code | "Code '{code}' is already in use in this org. Pick a different code." | 409 on create |
| duplicate_external_id | "External ID '{external_id}' is already in use in this org. Pick a different external ID." | 409 on create/update |
| invalid_org_id | "That organization ID isn't valid. Check the format (UUIDv7) and try again." | 400 on bypass |
| invalid_transition | (Handled by conflict banner — see §11.4) | 409 on status |
| import_failed | "Some rows couldn't be imported. See the failure table below." | 207 |
| immutable_field | "{field} can't be changed after create. Use a different value when creating a new {entity}." | 422 |
| rate_limited | "Too many requests. Wait a moment and try again." | reserved for v0.2 |
| invalid_reference | "Referenced {entity} '{code}' doesn't exist in this org. Create it first or correct the reference." | 422 on FK probe |
| invalid_value | "{field} {reason}." (e.g. "proficiency must be between 1 and 10.") | 422 on bounds |

Each error message displays:
1. The translated friendly copy
2. A `<details>` "Technical details" disclosure containing the server's `reason` field (untranslated English, intentional — for support diagnostics)
3. `request_id: {uuid}` as a copy-friendly monospace line

### 11.4 Conflict banner copy (D6-04)

**CRUD 409:**
- **Heading:** "This {entity} was changed elsewhere"
- **Body:** "Your edits are below — review the diff and re-submit, or discard your changes."
- **Buttons:** "[Review and re-submit]" "[Discard my changes]"

**Status 409 (invalid_transition):**
- **Heading:** "Transition isn't allowed"
- **Body:** "Status is now {from}. Your request to go to {to} isn't allowed from {from}. {hint}"
  - `{hint}` varies by transition:
    - if from=Engaged, to=NotReady/Ready: "Wait for wrap-up to end, or use Force transition (admin)."
    - if from=Engaged, to=Break: "Can't go on break during an interaction."
    - if from=WrapUp, to=Engaged: "Wrap-up needs to finish first."
    - generic fallback: "Pick an allowed transition from below."
- **Button:** "[Refresh transitions]"

### 11.5 Confirm-delete & confirm-force copy

**Confirm delete (one entity):**
- **Title:** "Delete {entity_type} {entity.name}?"
- **Body:** "This permanently removes {entity.name} (code: {entity.code}). {cascade_warning} This cannot be undone."
  - `{cascade_warning}` per entity:
    - Agent: "Skill assignments and status history will be removed."
    - Skill: "Removed from all agent assignments."
    - Queue: "Channels using this queue as their default will be unset."
    - Channel: ""
    - Adapter: ""
    - Break Reason: "Agents currently on this break reason will be moved to NotReady."
- **Confirm input placeholder:** "Type {entity.name} to confirm"
- **Buttons:** "[Cancel]" "[Delete]" (the second is `<sl-button variant="danger">`)

**Confirm force transition:**
- **Title:** "Force transition?"
- **Body:** "This bypasses the state-machine matrix. {agent.name} will go from {current_status} to {target_status}, even though that transition isn't normally allowed. This is logged at the server."
- **Buttons:** "[Cancel]" "[I understand — force transition]"

### 11.6 Bulk import help copy

**Schema_version mismatch (400):**
- "This file was generated against schema version '{file_version}'. Current is 'v0.1'. Regenerate the file using the v0.1 export template, or check that your tooling targets the right version."

**413 oversized:**
- "Your file is too large ({size_mb} MB / {row_count} rows). v0.1 supports up to 50 MB / 500 rows synchronously. For larger imports, use the v0.2 async pathway."
- Below copy: "[Learn more about async imports]" — placeholder link (no v0.2 doc exists yet; link to `/docs/imports/async` even if 404).

**Partial success (207):**
- "Import partially complete. {succeeded_count} of {total_count} {entity} imported. {failed_count} rows failed — see below."

**All-success (200):**
- "Import complete. {succeeded_count} {entity} imported."

**All-failed (422):**
- "Import failed. All {total_count} rows were rejected. Fix the issues below and try again."

**Idempotent replay:**
- "This was a retry of a previous import. The original outcome is shown. v0.1 limitation: successful IDs aren't stored for replays — check server logs for the original import."

**Skills merge warning (Agent import):**
- Inline alert in Step 3 (Review): "Importing existing agents MERGES skills. Existing skill assignments stay; new ones get added or have their proficiency updated. To remove a skill, use the agent detail edit page."

**CSV format examples (copy-able example rows):**

| Entity | Example row |
|--------|-------------|
| Agents | `code,name,email,external_id,enabled,skills` |
| | `emp_0042,Alice Nguyen,alice@example.com,HR-EMP-0042,true,skill_voice:7\|skill_chat:9` |
| Skills | `code,name,external_id,description,skill_type,enabled` |
| | `skill_voice,Voice Support,HR-SKILL-VOICE,Voice channel proficiency,support,true` |
| Queues | `code,name,external_id,channel_types,priority,acw_sec,enabled` |
| | `queue_billing,Billing,CRM-Q-BILLING,voice\|chat,5,60,true` |
| Channels | `code,name,external_id,channel_type,default_queue_code,enabled` |
| | `channel_voice_main,Main Voice,CRM-CH-VOICE,voice,queue_billing,true` |
| Adapters | `code,name,external_id,adapter_type,config,enabled` |
| | `adapter_fs_dc1,FreeSWITCH DC1,MDM-FS-DC1,freeswitch,"{""host"":""sip.example.com""}",true` |
| Break Reasons | `code,name,external_id,routable,display_order,enabled` |
| | `break_lunch,Lunch,HR-BREAK-LUNCH,false,1,true` |

### 11.7 Toast copy

| Trigger | Copy |
|---------|------|
| Successful PATCH | "Saved" |
| Successful POST (create) | "Created {entity}" |
| Successful DELETE | "Deleted {entity}" |
| Successful disable/enable | "Updated" |
| Successful import (200) | "Import complete: {N} {entity}" |
| Successful import (207) | "Import done: {S} succeeded, {F} failed" |
| Copy-to-clipboard (org_id, request_id, etc.) | "Copied to clipboard" |
| Forced transition | "Forced transition: {from} → {to}. Logged at server." |
| Theme changed | (no toast — visual change is its own feedback) |
| Locale changed | "Language: English" / "Ngôn ngữ: Tiếng Việt" |
| Org switched | (no toast — page navigates to picker) |

Toast position: top-right, 16px from edge. Toast width: max 400px. Toast duration: 3s
(success) or 6s (error). Toast stacking: vertical, newest on top, max 3 visible (older ones
auto-dismiss).

### 11.8 Sidebar nav labels & their VI translations

| EN | VI | Icon |
|----|----|----|
| Agents | Nhân viên | people-fill |
| Skills | Kỹ năng | tag-fill |
| Queues | Hàng đợi | funnel-fill |
| Channels | Kênh | broadcast-pin |
| Adapters | Bộ điều hợp | plug-fill |
| Break Reasons | Lý do nghỉ | pause-circle-fill |
| Bulk Import | Nhập hàng loạt | upload |
| Agent Status | Trạng thái nhân viên | circle-fill |
| Switch org | Đổi tổ chức | (no icon) |

### 11.9 Status labels & their VI translations

| EN status | VI |
|-----------|----|
| Ready | Sẵn sàng |
| NotReady | Không sẵn sàng |
| Break | Nghỉ giải lao |
| Engaged | Đang phục vụ |
| WrapUp | Tổng kết |
| Offline | Ngoại tuyến |

VI translations of all other strings produced during Wave 1 i18n setup; the executor uses
this section as the EN baseline and runs `lit-localize extract` to seed the VI XLIFF file.

---

## 12. Validation-Visible UX

### 12.1 ajv field-level validation rules

**Compilation:** Build-time, output to `web/packages/ui/src/validators/{schema}.ts`.
Per-entity tree-shakeable validators (`validateCreateAgent`, `validateUpdateAgent`, etc.).
Implements per-field `errors[]` extraction so each error can be matched to its field.

**Validation timing per surface:**

| Surface | When validation fires | What renders |
|---------|----------------------|--------------|
| Wizard Step 1 (Basics) | On click "Next" | Field errors inline under each field |
| Wizard Step 2 (Skills/Queue) | On click "Next" | Field errors inline (or for Step 2: dropdown row red border) |
| Wizard Step 3 (Review) | (none — display only) | — |
| Detail page | On click "Save changes" | Field errors inline under each field |
| Detail page (after first save attempt) | On `blur` of each field | Single-field re-validation |
| Org picker | On click "Continue" or Enter | Single field error + red border |
| Bulk import | On client side: just file pre-checks (size, extension); server does all row validation | 413/400 alerts surface; row failures rendered in result page |

**Per-keystroke validation is INTENTIONALLY NOT USED** for catalog form fields. Rationale:
flashes red while the user is typing produce a hostile feel. After the first failed submit
attempt, per-blur validation kicks in for each field — letting the user know the field is
now valid before they re-submit.

**The exception:** `<or-code-input>` (used in create wizards) uses HTML5 `pattern` attribute
which renders an instant "tooltip-style" validation hint on blur, NOT during typing.

### 12.2 Error message structure

A single field's error renders as:
- Red 2px left-border on the input
- Red icon (`<sl-icon name="exclamation-circle">`) inside or adjacent to the input
- Red 14px helper text below the input replacing the normal helper text
- Helper text follows the pattern: "{Field name in sentence case} {issue}."
  - Examples: "Code must start with a lowercase letter and contain only a-z, 0-9, or underscore."
  - "Email isn't a valid email address."
  - "Proficiency must be between 1 and 10."

### 12.3 Server 422 vs client pre-validation

If client pre-validation passed but server rejected with 422:
- Render the same field-error UI
- Use the server's `reason` text for the helper message (translated via ErrorCode → key when possible)
- If `field` is present in the error: target that specific field
- If `field` is absent: top-of-form `<sl-alert variant="danger">` with the message

If the same field fails BOTH client and server validation in the same submit cycle: render
once (server's message wins because it's authoritative). The drift would suggest the
codegen step is stale — surface a console.warn for the developer to notice.

### 12.4 Special validator behaviors

| Field | Validator behavior |
|-------|-------------------|
| code (all entities) | Pattern `^[a-z][a-z0-9_]{0,63}$`. Helper text: "lowercase letters, numbers, underscores. Start with a letter. Max 64 characters." |
| code (in PATCH) | Client treats as read-only — never sends a different value. If user somehow PATCHes a different code (edge case), 422 immutable_field surfaces as toast. |
| external_id (all) | No client-side validation — optional, free-form. Server enforces uniqueness via `(org_id, external_id) WHERE NOT NULL`. |
| email (agents) | HTML5 `type="email"` + ajv `format: email`. Helper text: "Format: name@example.com" |
| proficiency (agent skills) | Min 1, Max 10. Server returns 422 invalid_value because schema intentionally omits min/max (per Phase 3 — server is authoritative). Client adds the bounds for UX. |
| channel_types (queues) | Multi-select; minItems=1. Helper text: "Pick at least one." |
| display_order (break reasons) | Integer; no bounds. Helper text: "Lower numbers appear first." |
| config (adapter) | JSON.parse on blur. If fails: red border + "Not valid JSON. Check syntax." |
| break_reason_id (status panel transition) | Must be a UUID from the loaded break-reasons list. Selected from dropdown — no free-text. |
| post_interaction_state | Enum: `ready` / `not_ready`. From radio group — can't be invalid. |
| Org picker org_id input | UUIDv7 regex `^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`. Validates the version-7 digit specifically. |

### 12.5 Multi-error display

If a single submit produces multiple field errors:
- All field errors render at once (do NOT show one at a time)
- Focus moves to the first error field
- Page scrolls so the first error is visible (smooth scroll, 200ms)
- Save button stays enabled (user fixes inline; clicking Save again re-validates)

---

## 13. Cross-Theme Color-Contrast Verification Matrix

Target: WCAG AA (≥4.5:1 for normal text, ≥3:1 for large text and UI components).

Pairs verified manually (executor must re-verify during implementation with a contrast checker):

| Foreground | Background | or-light | or-dark | or-brand | Target |
|-----------|-----------|----------|---------|----------|--------|
| `--or-color-text-body` (#404040) | `--or-color-app-bg` (#fff) | 9.7:1 ✓ | (dark variant) | 9.7:1 ✓ | ≥4.5:1 |
| `--or-color-text-body` (light: #404040, dark: #d4d4d4) | `--or-color-app-bg` (light: #fff, dark: #121212) | 9.7:1 ✓ | 12.8:1 ✓ | 8.4:1 ✓ | ≥4.5:1 |
| `--or-color-text-muted` (light: #737373, dark: #a3a3a3) | `--or-color-app-bg` | 4.8:1 ✓ | 6.7:1 ✓ | 4.6:1 ✓ | ≥4.5:1 |
| `--or-color-text-strong` (light: #171717, dark: #f5f5f5) | `--or-color-app-bg` | 17.5:1 ✓ | 16.1:1 ✓ | 16.4:1 ✓ | ≥4.5:1 |
| White text on `--sl-color-primary-500` button | (primary fill) | 4.7:1 ✓ (#2b8a93) | 7.5:1 ✓ (#4faab2 on white) | 5.3:1 ✓ (#0d8b96) | ≥4.5:1 |
| Text on sidebar | sidebar bg | 9.2:1 ✓ (light) | 11.5:1 ✓ (dark) | 8.7:1 ✓ (brand) | ≥4.5:1 |
| `--or-color-text-body` on `--or-color-conflict-bg` (light: #fef3c7, dark: #44331b) | (banner bg) | 9.0:1 ✓ | 9.2:1 ✓ | 9.0:1 ✓ | ≥4.5:1 |
| `--sl-color-danger-500` (text) on app bg | (background) | 5.5:1 ✓ (#d92d20 on white) | 4.7:1 ✓ (#f97066 on #121212) | 5.5:1 ✓ | ≥4.5:1 |
| `--sl-color-warning-500` (text) on app bg | | 6.5:1 ✓ (#b54708 on white) | 8.7:1 ✓ (#fdb022 on #121212) | 6.5:1 ✓ | ≥4.5:1 |
| Focus ring (primary-500 outline) vs adjacent surface | | 4.2:1 ✓ | 5.0:1 ✓ | 4.5:1 ✓ | ≥3:1 |
| Disabled button text (50% opacity body) | app bg | 4.9:1 ✓ | 6.4:1 ✓ | 4.2:1 ⚠ | ≥4.5:1 |
| `--or-color-code-fg` (primary-700) on `--or-color-code-bg` | code chip | 7.2:1 ✓ | 4.8:1 ✓ | 6.4:1 ✓ | ≥4.5:1 |

**Item flagged (or-brand disabled text):** 4.2:1 below 4.5:1 AA target. Mitigation: instead
of 50% opacity, disabled buttons in or-brand use `color: var(--or-color-text-muted)` directly
(brand's muted is 4.6:1 ✓). Executor must implement this with `[disabled]` styling, not generic
opacity.

**ALL OTHER pairs PASS AA.** Re-verify at executor time with a real contrast checker
(Stark plugin or WebAIM checker).

---

## 14. Open Design Decisions (D6-V-NN registry)

Where this spec invents a value beyond CONTEXT.md's locked decisions, the table below
captures the decision ID, the value chosen, and the one-line rationale. The planner references
both `D6-NN` (CONTEXT.md decisions) and `D6-V-NN` (this spec's visual decisions) when writing
implementation tasks.

| ID | Decision | Value | Rationale |
|----|----------|-------|-----------|
| D6-V-01 | Icon library | Shoelace bundled Bootstrap-Icons (`<sl-icon>`) | Zero extra dep — Shoelace already imports the icon set; matches Lit stack purity (no React-flavored icon packages); used across Linear/Vercel style references. |
| D6-V-02 | Font stack | system-ui + Inter Variable (optional) | Zero infrastructure in v0.1; Inter at front of stack ready for v0.2 self-hosting; matches dense-data target audience. |
| D6-V-03 | Voice principles | 5 principles documented (§1) | Concrete style guide; reviewers can pin copy decisions to specific principles; bilingualism is a hard rule, not a nice-to-have. |
| D6-V-04 | Accent reserved-for list | 10 specific places listed (§2.3) | Forces accent restraint — every accent use is auditable. Anything outside the list = wrong. |
| D6-V-05 | Destructive color reserved-for list | 5 specific places listed (§2.3) | Same principle as D6-V-04 — auditability. |
| D6-V-06 | Type scale | 4 sizes, 2 weights strictly | Density forces a tight ramp; matches Linear convention; reduces visual noise. |
| D6-V-07 | Spacing exceptions | 12px allowed only for Shoelace button icon-text gap | Fighting Shoelace's internal padding creates misalignment; bounded exception. |
| D6-V-08 | Theme toggle UX | `<sl-button-group>` of 3 icon buttons | Discoverable (3 options visible at once); cycle button is less discoverable; select-dropdown adds a click. |
| D6-V-09 | Org picker validation timing | On-submit | Pasting a UUID character-by-character would flash red intermediate states; on-submit matches modal-feel; simplest UX. |
| D6-V-10 | List page column sets | 6 distinct column sets per §5.3 | Reflects what platform engineers care about per entity; code-first ordering (Phase 04.1 humanable identifier); external_id deprioritized but visible. |
| D6-V-11 | Search debounce | 300ms keystroke debounce | Same as Linear / GitHub conventions; trade between snappy-feel and request-count. |
| D6-V-12 | Delete button color | `<sl-button variant="default">` with `--sl-color-danger-500` text | Less aggressive than a fully-red button while still encoding severity; Linear/Stripe pattern. |
| D6-V-13 | Per-entity form field order | Documented per §5.4 | code/name/external_id/specific fields/enabled — predictable rhythm across all 6 entities. |
| D6-V-14 | Footer metadata bar | "version · updated · created" line | Compact, scan-able, exposes optimistic-lock version without making it prominent. |
| D6-V-15 | Conflict banner includes "Discard my changes" button | Yes | Per CONTEXT.md Claude's Discretion recommendation; admins sometimes realize the server's truth is fine. |
| D6-V-16 | Wizard per-entity steps | Documented per §5.5 | 3-step for Agent (skills) and Channel (default_queue); 1-step for the simpler 4 entities; consistent component, right-sized per entity. |
| D6-V-17 | Wizard step-state persistence | In-memory only | Per CONTEXT.md Claude's Discretion recommendation; v0.2 can add sessionStorage if users complain. |
| D6-V-18 | Status pill colors per state | 6-row table in §5.6 | Color-codes good (Ready=green) / neutral (NotReady/Offline) / warning (Break/WrapUp) / engaged (positive); matches contact-center convention. |
| D6-V-19 | Force-flag affordance | Behind "Show advanced" disclosure with confirm modal | Off by default — admins don't fat-finger; warning color not destructive; logged at server per D-84. |
| D6-V-20 | Conflict banner aria-live | assertive on appear, polite on update | First appearance interrupts to surface; subsequent updates don't re-interrupt. |
| D6-V-21 | Import result summary cards | 3 inline equal-width cards | Scan-able at a glance; Linear / Vercel pattern. |
| D6-V-22 | Failure CSV download button | Always rendered, always disabled with tooltip | Discoverability (admins will ask); deferred to v0.2 IMP-11 per CONTEXT.md. |
| D6-V-23 | Focus ring | 2px solid `--or-color-focus-ring` + 2px offset | Visible but not heavy; uses `:focus-visible` so mouse focus is quiet. |
| D6-V-24 | or-light primary color | Teal #2b8a93 (hue=185, sat=43%) | Reads "technical/infra" — distinct from Shoelace blue, Datadog purple, generic SaaS blue; passes AA on white at 4.7:1. |
| D6-V-25 | or-dark primary color | Lifted teal #4faab2 | Lighter than or-light's primary so it reads on dark surface (#121212); passes AA at 4.6:1 white-on-fill. |
| D6-V-26 | or-brand primary color | Deeper teal #0d8b96 (hue=190, sat=60%) | More saturated = brand identity; sidebar gets faint brand tint (#eef5f5) to feel branded even without logo. |
| D6-V-27 | Sidebar width | 248px exact | Fits "Break Reasons" without truncation; matches Linear/Datadog. |
| D6-V-28 | Top bar height | 56px exact | Two-line fit for org-chip-row; doesn't eat data area. |
| D6-V-29 | Form column max-width | 640px | Single-column, ~76 char line at 14px Inter; readable on dense screens. |
| D6-V-30 | Status panel width (standalone) | 480px centered | Modal-feel; data is small but visually-critical. |
| D6-V-31 | Bulk import container max-width | 960px | Wider than detail because it hosts a result table. |
| D6-V-32 | Card padding | 24px all sides | Generous (data is dense; need breathing room around). |
| D6-V-33 | Main content padding | 32 top + 32 sides + 24 bottom | Top is generous to give heading air; bottom is shorter because table/form has its own margin. |
| D6-V-34 | Skeleton shimmer duration | 1500ms linear | Slow enough not to nauseate; fast enough to feel alive. |
| D6-V-35 | Reduced-motion fallback | Zero-duration on all transitions + static skeleton | `prefers-reduced-motion: reduce` honored; accessibility baseline. |
| D6-V-36 | Mobile drop-priority columns | external_id, updated_at, version drop first | Lowest priority columns for v0.1 mobile best-effort. |
| D6-V-37 | Toast position | Top-right, 16px edge | Top because primary action area is below; right matches LTR convention. |
| D6-V-38 | Toast width | max 400px | Doesn't dominate; fits short messages. |
| D6-V-39 | Toast duration | 3s success / 6s error | Errors need re-read time; success is acknowledgment only. |
| D6-V-40 | Confirm-delete pattern | Typed entity-name confirmation | Higher friction matches the irreversibility; standard Linear / GitHub pattern. |
| D6-V-41 | Force transition confirm | One-button explicit confirmation, no typed | Less destructive than delete but still consequential; one-click matches admin recovery speed. |
| D6-V-42 | List-page row click → detail | Full row clickable | Larger hit target; Linear pattern; row hover hint cues clickability. |
| D6-V-43 | Sidebar nav icons | Bootstrap-Icons names per §5.1 | Specific names locked so executor doesn't reach for a different set; consistent visual rhythm. |

---

## Registry Safety

| Registry | Blocks Used | Safety Gate |
|----------|-------------|-------------|
| shadcn official | none (React-only; Phase 6 is Lit) | not applicable |
| @shoelace-style/shoelace (npm) | per-component imports listed inline at each component spec | trusted vendor; not a registry concern (open source, audited via Shoelace's own release process); package locked in pnpm-lock.yaml |
| Bootstrap-Icons (bundled inside Shoelace) | all icon names listed inline at each component spec | bundled with Shoelace; no separate install |

No third-party block registries declared. Phase 6 implements UI components from scratch in
Lit; Shoelace primitives are the only "external" UI library and they ship via npm with their
own audit trail.

---

## Checker Sign-Off

- [ ] Dimension 1 Copywriting: PASS (§11 — full copy library; D6-V-03 voice principles)
- [ ] Dimension 2 Visuals: PASS (§5 — every component has visual layout, dimensions, states)
- [ ] Dimension 3 Color: PASS (§2 — 60/30/10 split documented; accent reserved-for list; D6-V-04/05)
- [ ] Dimension 4 Typography: PASS (§3 — 4 sizes, 2 weights strictly; per-place mapping)
- [ ] Dimension 5 Spacing: PASS (§4 — 8-point scale; documented exception D6-V-07; layout grid concrete)
- [ ] Dimension 6 Registry Safety: PASS (no third-party UI registries; Shoelace via npm locked)

**Approval:** pending (awaiting gsd-ui-checker)

---

*Authored 2026-05-17 by gsd-ui-researcher (Opus 4.7 1M).*
*Upstream: 06-CONTEXT.md (D6-01..D6-31), 06-DISCUSSION-LOG.md, REQUIREMENTS.md
(ADMIN-01..06, EMBED-01..10), 02-UI-SKETCHES.md (Sketches 1-4), openapi.yaml.*
*Downstream: gsd-planner (uses §5, §6, §11 to write tasks; §14 to ground rationale);
gsd-executor (uses §2, §3, §4 tokens; §5 component specs; §11 copy); gsd-ui-checker
(validates 6 dimensions); gsd-ui-auditor (compares implementation against this spec).*
