---
phase: 07-web-component-embed-bundle
plan: 07-03
status: complete
started: 2026-05-18
completed: 2026-05-18
key-files:
  created:
    - .planning/phases/07-web-component-embed-bundle/07-03-SUMMARY.md
  modified:
    - web/packages/ui/src/components/adapters/adapter-detail.ts
    - web/packages/ui/src/components/agents/agent-detail.ts
    - web/packages/ui/src/components/break-reasons/break-reason-detail.ts
    - web/packages/ui/src/components/channels/channel-detail.ts
    - web/packages/ui/src/components/queues/queue-detail.ts
    - web/packages/ui/src/components/skills/skill-detail.ts
    - web/packages/ui/src/components/status/status-panel.ts
    - web/packages/ui/src/components/shell/catalog-shell.ts
---

# 07-03: Replace sl-dialog in UI components

**Goal:** Replace `<sl-dialog>` with an inline confirm panel in 7 UI components to avoid focus-trap issues in nested Shadow DOM and portal-escape risks. Remove unused `<sl-drawer>` from `catalog-shell.ts`.

## Execution
- Replaced `<sl-dialog>` with an inline `role="dialog"` panel in all 7 components.
- Added `aria-modal="true"`, `aria-live="polite"`, and manual focus/escape handling.
- Removed unused `import '@shoelace-style/shoelace/dist/components/drawer/drawer.js';` from `catalog-shell.ts`.
- Verified all unit tests still pass successfully.

## Deviations
None.