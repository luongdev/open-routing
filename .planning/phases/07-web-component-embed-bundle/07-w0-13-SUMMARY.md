---
phase: 07-web-component-embed-bundle
plan: w0-13
subsystem: ui-primitives
tags: [frankenstyle, light-dom, or-card, playground, stat-card]
dependency_graph:
  requires: [w0-03]
  provides: [or-card primitive, cardSlot playground content]
  affects: [primitives/index.ts, playground-route.ts]
tech_stack:
  added: []
  patterns: [TC39 accessors with @property, light DOM createRenderRoot, uk-card Frankenstyle]
key_files:
  created:
    - web/packages/ui/src/components/primitives/or-card.ts
    - web/packages/ui/src/components/primitives/or-card.test.ts
    - web/packages/ui/src/components/shell/slots/card.ts
  modified:
    - web/packages/ui/src/components/primitives/index.ts
    - web/packages/ui/src/components/shell/playground-route.ts
decisions:
  - Always render header/body/footer containers (not conditional on querySelector) — light DOM safe
  - Slot files at web/packages/ui/src/components/shell/slots/ per W0.0-03 deviation
  - TC39 accessor syntax for @property fields (same pattern as or-select, data-table)
metrics:
  duration: ~10min
  completed: 2026-05-18
  tasks_completed: 1
  files_created: 3
  files_modified: 2
---

# Phase 7 Plan w0-13: or-card Summary

or-card Frankenstyle primitive wrapping `<div class="uk-card uk-card-{variant}">` with named header/body/footer slots, light DOM, Ember stat-card playground pattern, and 5 passing unit tests.

## What Was Built

`<or-card>` is a light DOM Lit component wrapping Frankenstyle card markup. Props: `hoverable` (boolean), `variant` (default/primary/secondary), `padding` (sm/md/lg). Always renders three slot containers: `uk-card-header` (named slot "header"), `uk-card-body` (default slot), `uk-card-footer` (named slot "footer").

Playground slot shows 4 card patterns: simple card, card with header+footer, hoverable card, and 4 Ember-style stat cards (Today's Total, In-Person, Telehealth, Cancelled) matching the Ember Appointments dashboard layout.

## Deviations from Plan

**1. [Rule 1 - Bug] Always render all slot containers (dropped conditional querySelector)**
- Plan: `const hasHeader = !!this.querySelector('[slot="header"]')` guards conditional rendering
- Actual: Always render `uk-card-header`, `uk-card-body`, `uk-card-footer` containers
- Reason: In light DOM, `render()` replaces `this` children. `querySelector` for `[slot="header"]` finds nothing because the prior render cycle already replaced children. Always rendering containers is the correct light DOM pattern; empty containers are harmless.

**2. [Rule 2 - Deviation] Slot location adjusted per prompt instructions**
- Plan: `web/apps/admin/src/playground/slots/card.ts`
- Actual: `web/packages/ui/src/components/shell/slots/card.ts`
- Reason: W0.0-03 deviation (same as w0-12)

## Test Results

5 tests pass: default markup, primary variant, sm padding, hover class, header/body/footer containers rendered.
Total suite: 175 tests (from 170 after w0-12).

## Cross-AI Peer Review

Both Codex and Gemini returned READY WITH FIXES. No concerns raised for or-select or or-card specifically. HIGH concern (Gemini) was about or-input's `_inputId()` re-evaluation on render — that is in W0.0-11 (prior wave, not this plan). MED concern (Codex) was about or-code-input stale validity — pre-existing issue from Phase 6.

## Self-Check: PASSED

- `/Users/luong/workspace/dev/open-solutions/.claude/worktrees/agent-aa8c66dfc860fd11a/web/packages/ui/src/components/primitives/or-card.ts` EXISTS
- `/Users/luong/workspace/dev/open-solutions/.claude/worktrees/agent-aa8c66dfc860fd11a/web/packages/ui/src/components/primitives/or-card.test.ts` EXISTS
- `/Users/luong/workspace/dev/open-solutions/.claude/worktrees/agent-aa8c66dfc860fd11a/web/packages/ui/src/components/shell/slots/card.ts` EXISTS
- Commit `9285855` EXISTS
