# open-routing

## Quickstart

1. `cp .env.example .env`
2. `docker compose up -d postgres redis`
3. `task migrate-up`
4. `task dev`

## Structure

- `services/api/` — Go API (chi + pgx + sqlc + slog + OTel)
- `web/` — pnpm workspace stub for admin + embed + ui (filled in Phases 6/7)
- `openapi/` — OpenAPI 3.1 spec (filled in Phase 2)
- `migrations/` — golang-migrate SQL (`NNNNNN_name.up.sql` / `.down.sql`)
