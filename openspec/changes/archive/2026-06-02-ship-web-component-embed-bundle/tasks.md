# Tasks: Ship Web Component Embed Bundle

## 1. Embed Package

- [x] Configure `web/apps/embed` as a Vite library build that emits `dist/embed.js`.
- [x] Add package metadata for tarball-ready distribution.
- [x] Register `<open-routing-catalog>` from the embed entrypoint.
- [x] Document install, attributes, events, theme shape, CORS, and limitations.

## 2. Shared Shell Refactor

- [x] Preserve the Phase 7 W1-01 Ember-style catalog shell work recorded in `.planning/STATE.md`.
- [x] Add `routingMode` support to the shared catalog shell.
- [x] Convert entity route imports to lazy dynamic imports.
- [x] Keep admin behavior stable under history routing.

## 3. Embed Element

- [x] Parse and validate `org-id`, `api-base-url`, `theme`, `modules`, and `locale`.
- [x] Render invalid org configuration inside Shadow DOM.
- [x] Mount the shared catalog shell with hash routing.
- [x] Dispatch `open-routing:request-context` on connect.
- [x] Dispatch `open-routing:auth-expired` for every 401 response.
- [x] Render auth-expired state as an inline banner.

## 4. API CORS

- [x] Add chi CORS middleware.
- [x] Configure allowed origins through `CORS_ALLOWED_ORIGINS`.
- [x] Allow `X-Org-Id`, `Content-Type`, `Idempotency-Key`, and `Accept-Language`.
- [x] Insert CORS before org context middleware.
- [x] Extend API tests for preflight and cross-origin requests.

## 5. Host Integration Tests

- [x] Add React 18 stub host.
- [x] Add Vue 3 stub host.
- [x] Add plain HTML stub host with aggressive CSS reset.
- [x] Verify Shadow DOM CSS isolation in both directions.
- [x] Verify `X-Org-Id` propagation.
- [x] Verify auth-expired event dispatch.
- [x] Verify module filtering.

## 6. CI and Size

- [x] Add embed bundle-size check with 70 KB gzipped eager budget.
- [x] Add embed Playwright matrix job.
- [x] Include embed in frontend lint and typecheck coverage.
- [x] Verify package dry-run output.
