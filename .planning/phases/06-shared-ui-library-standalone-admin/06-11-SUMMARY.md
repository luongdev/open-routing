---
phase: 06-shared-ui-library-standalone-admin
plan: 06-11
status: complete
date: 2026-05-18
---

# Plan 06-11 Summary — Adapters Entity

## What shipped

Three Lit 3 components + tests for the Adapters catalog entity (CAT-06), following the Agents exemplar pattern established by 06-05 with the adapter-specific JSONB `config` field treatment:

- **`<or-adapter-list>`** — Data table with columns: code, name, adapter_type, enabled, updated_at, [⋮ row menu]. NO config column per UI-SPEC §4 adapter-list (config is too wide; only shown in detail).
- **`<or-adapter-detail>`** — Detail/edit page; JSONB `config` rendered as monospace `<sl-textarea>` with `JSON.parse` validation on blur and "Invalid JSON" error copy; 409 conflict-banner via `error.current` (D6-03 — NO re-GET); typed-name delete confirm (D6-V-40).
- **`<or-adapter-form>`** — Single-step form (D6-17 — adapters are simple entities); `config` field optional → null when blank; uses `CreateAdapterRequest` ajv validator (schema is `{type: 'object'}` per RESEARCH Open Questions Q4 RESOLVED).

## Files created

- `web/packages/ui/src/components/adapters/adapter-list.ts`
- `web/packages/ui/src/components/adapters/adapter-list.test.ts` (6 tests)
- `web/packages/ui/src/components/adapters/adapter-detail.ts`
- `web/packages/ui/src/components/adapters/adapter-detail.test.ts` (5 tests — code readonly, monospace config, empty→null, invalid JSON blocked, 409 via error.current)
- `web/packages/ui/src/components/adapters/adapter-form.ts`
- `web/packages/ui/src/components/adapters/adapter-form.test.ts` (4 tests)
- `web/packages/ui/src/components/adapters/index.ts` — barrel exporting OrAdapterList, OrAdapterDetail, OrAdapterForm

## Tests

- 15 new adapter unit tests pass (3 test files)
- Total suite: 72 tests pass (57 pre-existing + 15 adapter)
- `pnpm --filter @open-routing/ui test -- --testPathPattern=adapter` → 13 files / 72 tests pass

## Notes

- Original spawn (agent-aef846fe1077bd71d) wrote the files but its worktree was based on origin/main (not the Wave 3 base 3fe616f), so it couldn't run tests or commit. Auto-mode classifier correctly blocked `git reset --hard` recovery. Files were recovered into the main phase worktree by the orchestrator and committed there.
- adapter-detail.ts also handles the must_haves.truths revision from iteration-1: "409 version_conflict consumes error.current from the response body and renders <or-conflict-banner> inline — NO re-GET (D6-03 overrides REQUIREMENTS.md ADMIN-05 'reload?' wording)".

## Carry-forward to verification

- Manual smoke: open `/orgs/{id}/adapters/{id}` with a known adapter; verify config renders as pretty-printed JSON in the detail's display zone.
- Manual smoke: edit config to invalid JSON in form; verify "Invalid JSON" error appears and Save is disabled.
- Cross-org isolation Playwright smoke (Wave 5): verify adapters list in org A does not show adapters from org B.
