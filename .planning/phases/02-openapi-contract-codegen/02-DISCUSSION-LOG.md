# Phase 2: OpenAPI Contract & Codegen - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-05-15
**Phase:** 2-OpenAPI Contract & Codegen
**Areas discussed:** Spec scope, Error envelope, TS client distribution, oapi-codegen flavor + /openapi.yaml serving

---

## Spec scope this phase

| Option | Description | Selected |
|--------|-------------|----------|
| Big-bang full v0.1 spec | Phase 2 writes every endpoint (catalog 6×4 + agent state + bulk import) with full schemas. Phases 3-5 generate stubs against a frozen contract. | ✓ |
| Conventions + 1-2 smoke paths | Phase 2 ships error envelope, pagination, security, ID rules + e.g. /healthz and a placeholder catalog path. Phases 3-5 grow the spec as they implement. | |
| Skeleton + reserved paths | Phase 2 declares every URL + verb with placeholder request/response schemas. Phases 3-5 fill in the shape details as they land. | |

**User's choice:** Big-bang full v0.1 spec.

**Follow-up — UI before API?** User raised the strategic question "Should we design the UI before thinking about the API?" Three options surfaced:

| Option | Description | Selected |
|--------|-------------|----------|
| (A) Big-bang spec + UI sketch pass | Keep Phase 2 = full spec. Before writing spec, do paper-sketches of 2-3 most complex screens to validate data shapes. Phase order intact, de-risks spec design. | ✓ |
| (B) Slice-per-phase | Phase 2 = conventions + 1-2 smoke endpoints. Each Phase 3/4/5 authors its own spec slice when implementing. | |
| (C) Reverse order: UI before API | Move Phase 6 (UI library + admin) ahead of Phase 2. Spec derived from real UI. Big roadmap edit, loses parallelism. | |

**User's choice:** (A) — Big-bang + UI sketch pass.

**Follow-up — sketch scope?**

| Option | Description | Selected |
|--------|-------------|----------|
| All 4 screens, ASCII markdown | Agent detail, Bulk import result, Agent status panel, Catalog list — all in 02-UI-SKETCHES.md. ~1-2h work, validates every complex shape. | ✓ |
| Only 2 most complex screens | Agent detail + Bulk import result. Catalog list + status panel get spec'd straight from REQUIREMENTS. | |
| Sketch all ~6-7 screens | Add Agent list, Queue/Channel/Adapter edit, Break-reason management. Higher cost but covers entire admin UX. | |

**User's choice:** All 4 screens, ASCII markdown.

**Notes:** User's strategic challenge re-framed the area: instead of just choosing spec scope, they questioned the macro UI-vs-API ordering. The resolution preserves the roadmap (API-first) but inserts a UI validation step that ensures the spec isn't written in a vacuum. Sketches are throwaway markdown — explicitly NOT UI implementation; Phase 6 still owns real UI work.

---

## Error envelope

| Option | Description | Selected |
|--------|-------------|----------|
| (A) Stick with {error, reason} | Keep Phase 1 shape (WriteError at scaffold). Special cases (CAT-08, STATE-03, IMP-05) add extension fields. Code already there, no rewrite. | ✓ |
| (B) RFC 7807 Problem Details | application/problem+json with type/title/status/detail. Special cases become 7807 extensions. Industry standard but verbose, rewrites WriteError. | |
| (C) Hybrid: 7807 for generic + custom for special | Generic 4xx/5xx use 7807; CAT-08/STATE-03/IMP-05 use bespoke shapes. Cleanest shape-by-shape, more client complexity. | |

**User's choice:** (A) — Stick with {error, reason}.

**Follow-up — error codes type + request_id echo?**

| Option | Description | Selected |
|--------|-------------|----------|
| Closed enum + request_id in body | Lock ~10 well-known codes. Echo request_id into body for DX. Spec edits required for new codes. | ✓ |
| Open string + request_id in body | error field is free-form string; spec description lists well-known codes. Zero churn but no type safety. | |
| Closed enum, no request_id (header only) | Enum locks codes, request_id stays in X-Request-Id header. Minimal body. | |
| Closed enum + 'other' escape + request_id | Enum for well-known + 'other' value for exotic cases. Pragmatic middle. | |

**User's choice:** Closed enum + request_id in body.

**Notes:** Closed enum is initial v0.1 set of 10 codes (invalid_body, invalid_id, not_found, internal, version_conflict, cross_org, invalid_org_id, invalid_transition, import_failed, rate_limited). `rate_limited` reserved even though v0.1 doesn't implement rate limiting — keeps the TS client stable when v0.2 adds it. Special-case shapes (CAT-08 with `current`, STATE-03 with `from`/`to`, IMP-05 multi-status) extend the base envelope rather than replacing it.

---

## TS client distribution

| Option | Description | Selected |
|--------|-------------|----------|
| openapi-typescript + openapi-fetch + factory in packages/ui | packages/ui exports createApiClient({baseURL, getOrgId}). Admin + embed configure with their own getOrgId. Type safety + DRY + Web Component compatible. | ✓ |
| Only openapi-typescript types, hand-write fetch | packages/ui re-exports raw paths types. Each consumer writes its own fetch. Minimal runtime overhead but lots of boilerplate. | |
| openapi-typescript + bespoke client in packages/ui | Hand-write thin namespaced client (apiClient.agents.list, etc.). Pro: control. Con: maintains code that duplicates openapi-fetch. | |

**User's choice:** openapi-typescript + openapi-fetch + factory in packages/ui.

**Follow-up — packages/ui API surface beyond factory + types?**

| Option | Description | Selected |
|--------|-------------|----------|
| Minimal: factory + types + isApiError | Just the essentials. Phase 6/7 add helpers if needed. Smallest bundle, fewest deps. | ✓ (with #2 + #3) |
| + Error parsing helpers | parseApiError + ErrorCodes constants. Good DX, +~30 lines, no new dep. | ✓ |
| + Async/state helper (Lit-aware) | Wrap @lit/task for loading/error/data pattern. Best DX for admin SPA, locks Lit ecosystem decision into Phase 2. | ✓ |

**User's choice:** All three (Cả 1,2,3 luôn đi).

**Notes:** packages/ui exports the factory + types + isApiError + parseApiError + ErrorCodes constants + Lit-aware async helper (createApiTask using @lit/task). Phase 7 embed can opt out of the Lit helper by importing only createApiClient + types — keeps embed bundle minimal. @lit/task is already implied by PROJECT.md's Lit lock, so no new ecosystem commitment.

---

## oapi-codegen flavor + /openapi.yaml serving

### Server flavor

| Option | Description | Selected |
|--------|-------------|----------|
| strict-server | Typed request/response objects. Codegen handles decode/validate/marshal. Compile-time shape enforcement. Less boilerplate for 30+ CRUD endpoints. | ✓ |
| chi-server (default) | Generated ServerInterface with (w, r) + typed binding. Familiar like Phase 1, max control, but per-endpoint boilerplate repeats. | |

**User's choice:** strict-server.

### Spec serving

| Option | Description | Selected |
|--------|-------------|----------|
| /openapi.yaml + /docs interactive viewer | Serve spec from API runtime + Swagger UI/RapiDoc/Scalar at /docs (CDN-loaded HTML, no Go dep). Bypass org middleware. Highest DX. | ✓ |
| Only /openapi.yaml runtime, no /docs | Serve raw spec from API. Devs use IDE plugins or other tools to view. | |
| Static file only, no runtime serving | Spec only exists in repo, no new routes on API binary. Minimal. | |

**User's choice:** /openapi.yaml + /docs interactive viewer.

**Notes:** Spec is served from embedded bytes (oapi-codegen `--generate spec` produces `spec.gen.go`), so the served spec is always exactly what the binary was built against — no stale `openapi.yaml` on disk drift. /docs HTML loads Scalar (or RapiDoc — planner picks) from CDN; no static-asset pipeline needed. Both routes bypass org middleware (extending the D-21 bypass list).

---

## Claude's Discretion

- Paper-sketch ASCII character set convention (consistency across 4 screens matters more than specific style).
- Scalar API Reference vs RapiDoc at /docs (both single-script HTML+CDN, ~equivalent UX; default recommendation: Scalar).
- oapi-codegen.yaml flags beyond `--generate strict-server,types,spec` (planner tunes `output-options.skip-prune` and `compatibility.always-prefix-enum-values` based on first-pass output).
- Whether `parseApiError` retries on network errors (default: strict parse, network is caller's concern).
- CDN URL version pinning for the /docs viewer (pin to a specific hash, not @latest).
- Generated TS file path naming convention (`generated.ts` vs `__generated__/index.ts`).

## Deferred Ideas

- RFC 7807 Problem Details migration (if v1 external consumers expect standard envelopes).
- Real auth on /docs (v1 RBAC gates behind admin role).
- Per-tag splitting of generated Go packages (revisit if v0.2 exceeds ~80 endpoints).
- Spec splitting with openapi-format / swagger-cli bundle (revisit if openapi.yaml > ~3000 lines).
- Mock server from spec (Prism, MSW) — UI sketches replace this for Phase 2; spin up Prism if Phase 6 needs to develop ahead of Phase 3.
- Spec contract tests (Schemathesis, Dredd) — defer to dedicated testing phase.
- Rate-limiting headers (`X-RateLimit-*`) and 429 handling — v0.2.
- Webhook / async response specs — out of v0.1.
- GraphQL or gRPC alternative surfaces — REST locked for v0.1.
- API versioning beyond URL /v1/ — revisit when v0.2 introduces breaking changes.
