---
phase: 07-web-component-embed-bundle
plan: w0-12
subsystem: ui-primitives
tags: [frankenstyle, light-dom, or-select, playground]
dependency_graph:
  requires: [w0-03]
  provides: [or-select primitive, selectSlot playground content]
  affects: [primitives/index.ts, playground-route.ts]
tech_stack:
  added: []
  patterns: [TC39 accessors with @property, light DOM createRenderRoot, uk-select Frankenstyle]
key_files:
  created:
    - web/packages/ui/src/components/primitives/or-select.ts
    - web/packages/ui/src/components/primitives/or-select.test.ts
    - web/packages/ui/src/components/shell/slots/select.ts
  modified:
    - web/packages/ui/src/components/primitives/index.ts
    - web/packages/ui/src/components/shell/playground-route.ts
decisions:
  - Slot files placed at web/packages/ui/src/components/shell/slots/ per W0.0-03 deviation (not apps/admin)
  - Used TC39 accessor syntax (@property() accessor) to match project decorator convention
  - Playground SLOT_CONTENT map approach allows per-id slot wiring without restructuring SLOTS array
metrics:
  duration: ~15min
  completed: 2026-05-18
  tasks_completed: 1
  files_created: 3
  files_modified: 2
---

# Phase 7 Plan w0-12: or-select Summary

or-select Frankenstyle primitive wrapping `<select class="uk-select">` with light DOM, 6-state playground slot, and 6 passing unit tests.

## What Was Built

`<or-select>` is a light DOM Lit component wrapping a native `<select class="uk-select">`. Props: `value`, `name`, `disabled`, `required`, `label`, `helperText`/`helper-text`, `errorText`/`error-text`, `placeholder`, `size` (sm/md/lg), `options` array. Emits `or-change` CustomEvent with `detail.value`.

Playground wired via `SLOT_CONTENT` map in `playground-route.ts`, rendering 6 states: default, with helper text, with error, disabled, small, large.

## Deviations from Plan

**1. [Rule 2 - Deviation] Slot location adjusted per prompt instructions**
- Plan: `web/apps/admin/src/playground/slots/select.ts`
- Actual: `web/packages/ui/src/components/shell/slots/select.ts`
- Reason: W0.0-03 deviation (playground lives in packages/ui shell, not apps/admin)

**2. [Rule 1 - Bug] Used TC39 accessor syntax for all @property fields**
- Found during: implementation
- Issue: `@property({ type: String }) value = '';` caused "Unsupported decorator location: field" in vitest/esbuild
- Fix: Added `accessor` keyword to all decorated fields to use TC39 standard decorator protocol (consistent with existing project components like data-table.ts)

**3. [Rule 2 - Deviation] SLOT_CONTENT map approach for playground wiring**
- Plan: Replace `<div class="playground-slot">` with slot import directly
- Actual: Added `SLOT_CONTENT: Partial<Record<string, TemplateResult>>` map keyed by slot ID; render method checks map before falling back to pending state
- Reason: Allows incremental wiring of slots as each W0.0-1X plan completes without restructuring the SLOTS array definition

## Test Results

6 tests pass: renders options, value bind, or-change event, uk-form-danger class, disabled, placeholder.
Total suite: 170 tests (from 164 baseline).

## Self-Check: PASSED

- `/Users/luong/workspace/dev/open-solutions/.claude/worktrees/agent-aa8c66dfc860fd11a/web/packages/ui/src/components/primitives/or-select.ts` EXISTS
- `/Users/luong/workspace/dev/open-solutions/.claude/worktrees/agent-aa8c66dfc860fd11a/web/packages/ui/src/components/primitives/or-select.test.ts` EXISTS
- `/Users/luong/workspace/dev/open-solutions/.claude/worktrees/agent-aa8c66dfc860fd11a/web/packages/ui/src/components/shell/slots/select.ts` EXISTS
- Commit `0b41e6f` EXISTS
