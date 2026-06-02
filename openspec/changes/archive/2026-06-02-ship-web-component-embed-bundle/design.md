# Design: Web Component Embed Bundle

## Approach

The embed package wraps the existing `<or-catalog-shell>` from `web/packages/ui` instead of creating a second catalog UI. The shell remains the source of truth for entity navigation and CRUD workflows. Phase 7 adds the thin Web Component boundary, hash routing for embedded contexts, lazy route loading, host-facing attributes, public events, CORS support, and host integration tests.

## Key Decisions

- Use a hybrid wrapper: mount the shared shell and lazy-load entity routes with dynamic `import()`.
- Keep admin and embed on the same shell; do not fork entity screens.
- Use history routing by default for admin and hash routing for embed.
- Keep embed alerts inline because portal-based dialogs or toasts can escape Shadow DOM.
- Require `org-id` and `api-base-url` attributes for embedded operation.
- Use composed, bubbling CustomEvents for host-observable integration events.
- Add CORS middleware before org context middleware so preflight requests do not require `X-Org-Id`.
- Treat npm/CDN publishing as out of scope for v0.1; ship a package that is ready to pack.

## Public Attributes

- `org-id`: required org UUID
- `api-base-url`: required absolute or host-relative API base URL
- `theme`: optional named theme or JSON object of CSS custom properties
- `modules`: optional comma-separated entity keys
- `locale`: optional locale key

## Public Events

- `open-routing:request-context`: fired once on connect with org, API, theme, and module details
- `open-routing:auth-expired`: fired for every HTTP 401 with status, request ID, and path details

## Validation Strategy

- Unit and component tests for attribute parsing, invalid org state, module filtering, and event dispatch.
- Admin regression tests to confirm shell lazy loading does not break existing standalone behavior.
- Playwright host matrix for React 18, Vue 3, and plain HTML.
- Bundle-size job that fails when the eager gzipped embed bundle exceeds 70 KB.
