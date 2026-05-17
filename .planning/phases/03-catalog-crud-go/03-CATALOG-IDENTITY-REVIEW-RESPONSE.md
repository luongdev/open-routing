---
phase: 03-catalog-crud-go
reviewed: 2026-05-17
topic: cross-AI peer review of catalog identity contract review
status: consensus_reached
severity: HIGH
review_type: cross_ai_synthesis
reviewers: [claude-opus-4.7, codex-gpt-5.5-high, gemini-3.1-pro-preview]
---

# Cross-AI Peer Review — Catalog Identity Contract

Triangulation of `03-CATALOG-IDENTITY-REVIEW.md` per HARD RULE in `~/.claude/CLAUDE.md` § Cross-AI Peer Review. Three independent reviews (Codex high-reasoning, Gemini, Claude Opus) on the BLOCK PHASE 5 recommendation. Reviewer artifacts: `/tmp/codex-identity-review.md`, `/tmp/gemini-identity-review.md`, `/tmp/claude-opus-identity-review.md`.

## Consensus

**Verdict: `BLOCK PHASE 5 ONLY`** — unanimous (4/4 including original review).

All three independent reviewers and the original author agree:
- The design issue is **real** (severity HIGH).
- PR #4 (Phase 4) merging was correct — Phase 4 is independent of catalog identity.
- The fix should be a bounded Phase 4.5 inserted between Phase 4 (shipped) and Phase 5 (this planning attempt).
- Phase 5 planning must pause until the contract is corrected.

## Where Reviewers Diverge

| Question | Original | Codex | Gemini | Claude Opus |
|----------|----------|-------|--------|-------------|
| Severity | HIGH | HIGH | HIGH | MED→HIGH (after Codex argument) |
| Rename `external_id` → `code`? | YES | YES | YES | NO (initially) → **revised: YES** |
| Add `external_source` in v0.1? | YES (preferred) / OPTIONAL fallback | YES (recommends now) | DEFER to v0.2 (acceptable) | DEFER to v0.2 |
| Apply `code` to all 6 entities? | YES | YES | YES | YES (after Codex argument) |

**Tie-breaker on the `external_id` rename:** Codex's argument carried the day. The cost of keeping the `external_id` name "tiết kiệm ngắn hạn nhưng khóa semantic sai vào OpenAPI, tests, docs, CSV templates, admin UI" (cheap short-term but locks wrong semantics into the whole downstream surface). Renaming once is cheaper than living with wrong vocabulary in every customer-facing artifact.

**Tie-breaker on `external_source` timing:** 2/4 vote DEFER (Gemini, Claude). Acceptable v0.1 simplification: `external_id TEXT NULL` with partial unique `WHERE external_id IS NOT NULL`. Add `external_source` in v0.2 when sync arrives.

## Final Recommended Contract (consensus shape)

### Universal user-facing identity

Apply to all 6 primary catalog entities (`agents`, `skills`, `queues`, `channels`, `adapters`, `break_reasons`):

```sql
code TEXT NOT NULL
UNIQUE (org_id, code)
```

### External mapping (v0.1 simplified, v0.2-extensible)

```sql
external_id TEXT NULL
UNIQUE (org_id, external_id) WHERE external_id IS NOT NULL
```

Defer `external_source` to v0.2. Document the constraint: v0.1 supports at most one external source per entity per org. v0.2 will add `external_source TEXT` and switch to triple-unique `(org_id, external_source, external_id)`.

### API semantics

- `Create*Request`: requires `code` and `name`; may accept `external_id`.
- `Update*Request`: `code` is immutable post-create in v0.1 (defer renames).
- Generic import upserts by `(org_id, code)`.
- All cross-entity FK refs in import payloads use the target's `code` (e.g., `skill_code`, `queue_code`, `break_reason_code`).
- `external_id` is informational in v0.1 (not used as upsert key).

## What Changes — Concrete Scope of Phase 4.5

### Migration (single edit to `services/api/migrations/000002_catalog_v0_1.up.sql`)

Per D-61 the migration is editable until v0.1 ships. Six tables get:
- Add `code TEXT NOT NULL`
- Add `UNIQUE (org_id, code)`
- Replace existing identity-key constraint:
  - `agents/skills/queues/channels`: drop `UNIQUE (org_id, external_id) NOT NULL`; add `external_id TEXT NULL` + partial unique
  - `adapters`: add `code` + add `external_id TEXT NULL` + partial unique (currently has neither)
  - `break_reasons`: add `code` + drop `UNIQUE (org_id, name)` + add `external_id TEXT NULL` + partial unique

### OpenAPI (`openapi/openapi.yaml`)

- Update 6 entity schemas: required `code`, optional `external_id` (was: required `external_id`, no `code`)
- Update 6 `Create*Request` schemas: require `code`, optional `external_id`
- Update 6 `Update*Request` schemas: `code` not patchable (or document immutability); `external_id` patchable
- Update 6 `*ListItem` schemas: include `code`, `external_id`
- Tighten the `external_id` description: "Optional caller-assigned identifier from an external system (HR, CRM, etc.). Distinct from `code` which is the Open Routing canonical key. v0.1 supports at most one external source per org; multi-source disambiguation deferred to v0.2."
- Update 409/422 error response descriptions to reference `code` collisions

### sqlc + Handlers + Tests

- Regenerate sqlc models/queries (`task gen`)
- Update 6 CRUD handlers: read/write `code`; treat `external_id` as optional nullable
- Update Phase 3 SUMMARY/PATTERNS to reflect new identity model
- Update 6 CRUD test suites (fixtures, isolation tests, 409 collision tests)
- Update two-org isolation harness fixtures to use `code`

### Phase 5 CONTEXT.md updates (post Phase 4.5)

- D5-13 (Idempotency-Key) — no change, independent of identity model
- D5-15: rewrite — `Import*Request` schemas reference FKs by `code` instead of `external_id`
- D5-16: `ImportAgentRequest.skills[].skill_code` (was `skill_external_id`)
- D5-17: CSV agent `skills` column syntax becomes `CODE:prof|CODE:prof` (already named `CODE` — semantic match!)
- D5-25: 6 new `Import*Request` schemas mirror `Create*Request` with FKs by `*_code` (was `*_external_id`)
- IMP-03 in REQUIREMENTS.md: upsert by `(org_id, code)` instead of `(org_id, external_id)`

## Migration/API Impact Summary

| Cost | Estimate |
|------|----------|
| Migration file edits | 1 day |
| OpenAPI schema edits + regen | 1 day |
| sqlc + 6 CRUD handlers | 1-2 days |
| Test suite updates | 2 days |
| Phase 3 SUMMARY/PATTERNS doc updates | 0.5 day |
| Cross-AI peer review of Phase 4.5 diff | 0.5 day |
| **Total Phase 4.5** | **~1 week of focused work** |
| Phase 5 CONTEXT.md update | 0.5 day (after Phase 4.5 merges) |

**vs. cost of NOT fixing now** (Codex's argument):
- Wrong vocabulary baked into 6 CSV templates that customers will save as their own data dictionary
- Wrong vocabulary in admin UI form labels, error messages, docs, support tickets
- v0.2 sync engine will need a rename migration on user-facing field
- Phase 5 import logic will be designed against the wrong key (D5-15..D5-20 currently FK-by-external_id)

## What I Recommend the User Do Next

### Path A (consensus path — recommended)

1. **Pause Phase 5 planning.** The current branch `gsd/phase-05-bulk-import-go` with cherry-picked context docs stays — it'll be useful when Phase 5 resumes after Phase 4.5.
2. **Insert Phase 4.5** in ROADMAP.md via `/gsd-phase --insert 4.5 "catalog-identity-normalization"`.
3. **Run** `/gsd-discuss-phase 4.5` to scope the identity correction (decisions needed: rename details, NULL semantics, immutability rules, error code shape for `code` collisions).
4. **Plan + execute** Phase 4.5 (~1 week scope).
5. **Resume Phase 5** via `/gsd-plan-phase 5` after Phase 4.5 ships. Update 05-CONTEXT.md D5-15..D5-25 to reference `code` instead of `external_id`.

### Path B (overrule the consensus — proceed Phase 5 as-is)

Only sane if the user has strong product information not visible to the reviewers — e.g., "we already shipped CSV templates to customers using `external_id`" or "the rename cost is greater than I'm modelling." Otherwise this is paying interest on tech debt that compounds with every downstream artifact.

### Path C (split the difference — Claude's original Option B)

Add `external_id` to `adapters` and `break_reasons` without the rename. Codex explicitly rejected this for the semantic-lock reason above. Not recommended — but cheapest if velocity is critical and the user accepts the vocabulary debt.

## Open Questions for the User

These must be answered during `/gsd-discuss-phase 4.5`:

1. **Code immutability** — Is `code` immutable after create in v0.1? (Default recommendation: YES, defer rename support to v0.2 with a documented `RenameAgent` operation if needed.)
2. **Code format constraint** — Free-form string, or constrained (e.g., `^[A-Z][A-Z0-9_]{0,63}$`)?
3. **Existing seed data** — Are there any non-test rows in the catalog tables already that need backfilling? (Probably not — Phase 3 is recently shipped.)
4. **`external_id` rollback** — If a row was created with `external_id` in pre-Phase-4.5 code, what happens? Backfill `code = external_id` on migration?
5. **OpenAPI version bump** — Is this a breaking change for any external consumer? (Probably not yet — admin app and embed not built.)
6. **Phase 5 CONTEXT.md retroactive edit** — Update CONTEXT.md in place to reference `code`, or write a delta companion file? (Recommend: edit in place, commit as `docs(05): update CONTEXT for code identity post Phase 4.5`.)

## Acceptance Criteria for This Synthesis

- ✓ All three external reviewers' reviews captured in `/tmp/{codex,gemini,claude-opus}-identity-review.md`
- ✓ Verdict consensus identified (4/4 BLOCK PHASE 5 ONLY)
- ✓ Divergence between reviewers explicitly tabled
- ✓ Tie-breakers documented with reasoning
- ✓ Concrete Phase 4.5 scope laid out
- ✓ Cost estimate provided
- ✓ Open questions surfaced for discuss-phase
- ✓ User has three paths (A/B/C) with clear tradeoff articulation

---

*This synthesis is not the final word — it routes the decision to the user. The HARD RULE in CLAUDE.md says "if either reviewer says BLOCK, do not proceed without addressing." All three reviewers say BLOCK PHASE 5 ONLY; the addressing step is the user's go/no-go on Path A vs B vs C.*
