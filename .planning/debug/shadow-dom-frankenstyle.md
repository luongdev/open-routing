---
status: fixing
trigger: "Frankenstyle global CSS classes do not apply inside Lit Shadow DOM components"
created: 2026-05-19T00:00:00Z
updated: 2026-05-19T00:00:00Z
---

## Current Focus

hypothesis: Frankenstyle CSS lives at :root in the main document; Shadow DOM in catalog-shell and playground-route blocks it from reaching uk-* class users inside those shadow roots.
test: Adopt constructed CSSStyleSheets (frankenstyle-kit.css + ember-theme vars + compat-or-tokens) into catalog-shell and playground-route shadow roots.
expecting: or-button computed style shows oklch() background, proper padding/height.
next_action: Create src/styles/shadow-sheets.ts with ?inline imports; override createRenderRoot in both shell components.

reasoning_checkpoint:
  hypothesis: "Frankenstyle CSS is scoped to the main document via :root selectors; catalog-shell and playground-route use Shadow DOM (LitElement default), which creates isolated CSS scopes. uk-* light DOM children rendered inside those shadow roots inherit from the shadow root's adopted sheets, not the main document stylesheet."
  confirming_evidence:
    - "parentShadowAdoptedSheets: 1 (only playground-route's own static styles) — confirmed no frankenstyle sheet is adopted"
    - "catalogShellAdoptedSheets: 1 (only catalog-shell own styles)"
    - "or-button uses light DOM (createRenderRoot returns this) but lives inside playground-route shadow DOM, inheriting from that shadow root's sheets only"
    - "frankenstyle.css imports frankenstyle/css/frankenstyle-kit.css — exists in node_modules, 34418 lines"
  falsification_test: "If adopting frankenSheet into catalog-shell shadowRoot changes or-button computed background from rgb(239,239,239) to oklch(...), hypothesis is confirmed"
  fix_rationale: "adoptedStyleSheets on shadowRoot makes those sheets available to all descendants in that shadow tree, including light-DOM children whose createRenderRoot returns themselves (they inherit from parent shadow root)"
  blind_spots: "ember-theme.css uses @theme {} Tailwind directive — cannot be imported ?inline as-is; need a shadow-vars-only CSS file"

## Symptoms

expected: or-button elements show Frankenstyle/Ember styling (oklch bg, proper padding, border-radius)
actual: backgroundColor rgb(239,239,239) browser UA default, height 6px, padding 1px 6px
errors: No errors — CSS simply does not apply
reproduction: Navigate to localhost:5173/playground, inspect any or-button computed style
started: Phase 7 implementation of Shadow DOM components

## Eliminated

- hypothesis: or-button uses Shadow DOM itself (blocking CSS)
  evidence: createRenderRoot() returns this (light DOM) in or-button
  timestamp: 2026-05-19

## Evidence

- timestamp: 2026-05-19
  checked: catalog-shell.ts, playground-route.ts
  found: Both extend LitElement without overriding createRenderRoot — use Shadow DOM by default
  implication: Frankenstyle at :root cannot pierce shadow boundary

- timestamp: 2026-05-19
  checked: frankenstyle package exports
  found: frankenstyle/css/frankenstyle-kit.css exports correctly via package.json exports map
  implication: ?inline import will resolve correctly

- timestamp: 2026-05-19
  checked: ember-theme.css
  found: Contains @theme {} Tailwind v4 directive — cannot be imported ?inline directly
  implication: Need separate shadow-vars.css with only :root/.dark vars (no @theme block)

- timestamp: 2026-05-19
  checked: compat-or-tokens.css
  found: Pure :root/.dark CSS vars — safe to import ?inline
  implication: Can be included in constructed sheet

## Resolution

root_cause: Frankenstyle CSS is only injected into the main document. Shadow DOM roots in catalog-shell and playground-route have no adopted stylesheets for frankenstyle, so uk-* class descendants get only browser UA defaults.
fix: Create src/styles/shadow-sheets.ts that builds constructed CSSStyleSheets from ?inline CSS imports. Override createRenderRoot() in catalog-shell.ts and playground-route.ts to adopt these sheets. Create ember-shadow-vars.css (no @theme directive) for Shadow DOM use.
verification: []
files_changed: [web/packages/ui/src/styles/shadow-sheets.ts, web/packages/ui/src/styles/ember-shadow-vars.css, web/packages/ui/src/components/shell/catalog-shell.ts, web/packages/ui/src/components/shell/playground-route.ts]
