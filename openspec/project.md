# Open Routing

## Purpose

Open Routing is an embeddable, multi-channel routing platform for products that need ACD-like routing without owning media, CRM, ticketing, or channel systems. It provides flow-oriented routing configuration, catalog-driven setup, org-facing audit foundations, and adapter boundaries for voice, chat, email, and later channel providers.

## Archived Milestone

v0.1 Catalog Foundation is complete and archived. It includes the catalog data model, agent status model, REST management API, bulk import, standalone admin UI, and the Phase 7 Web Component embed bundle.

Current OpenSpec state:

- Completed capabilities live under `openspec/specs/`.
- Phase 7 is complete and archived as `openspec/changes/archive/2026-06-02-ship-web-component-embed-bundle/`.
- Milestone 1/v0.1 is archived.
- Milestone v0.2 is proposed as `openspec/changes/define-v0-2-core-routing-runtime-foundation/`.
- Original GSD planning remains in `.planning/` as migration source and historical detail.

## Locked Stack

- Backend: Go, chi, pgx, sqlc, golang-migrate, slog, OpenTelemetry
- Database: PostgreSQL 17 and Redis
- API contract: OpenAPI 3.0, oapi-codegen, openapi-typescript
- Frontend: Vite, Lit, Shoelace, TypeScript
- Embed: Custom Elements with Shadow DOM
- Repo: polyglot monorepo with Go module and pnpm workspaces

## Product Boundaries

Open Routing owns routing decisions, catalog configuration, state projections, flow authoring, publish governance, simulation, and traceability. It does not own media holding, channel session execution, CRM, ticketing, WFM, or agent desktop surfaces.

## Current Focus

Milestone 1/v0.1 is closed. The active planning focus is the proposed v0.2 Core Routing Runtime Foundation change. Implementation should wait until the gray areas in that change are confirmed.
