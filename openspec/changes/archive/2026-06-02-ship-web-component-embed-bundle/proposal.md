# Ship Web Component Embed Bundle

## Why

v0.1 is not complete until host products can mount the catalog configuration surface as a Web Component. Phase 6 shipped the shared UI library and standalone admin app; Phase 7 packages the shared shell for embedded use, verifies Shadow DOM isolation, and makes the bundle safe for host applications.

## What Changes

**Embed delivery surface**

- From: `web/apps/embed` is not yet the production embed package.
- To: `web/apps/embed` builds a tarball-ready ES module that registers `<open-routing-catalog>`.
- Reason: Host products need a framework-neutral way to embed catalog configuration.
- Impact: Non-breaking addition to the v0.1 delivery surface.

**Shared shell routing**

- From: the shared catalog shell is optimized for standalone admin history routing.
- To: the same shell supports admin history routing and embed hash routing, with entity screens loaded lazily.
- Reason: The embed bundle must stay small without forking the catalog UX.
- Impact: Admin should preserve behavior while gaining lazy loading.

**Host integration contract**

- From: org context, API base URL, theme, modules, and locale are admin-owned concerns.
- To: embed reads them from explicit HTML attributes and dispatches public CustomEvents for request context and auth expiry.
- Reason: Embedded surfaces cannot depend on host framework internals or browser globals.
- Impact: Host integrations receive a stable v0.1 API contract.

**Verification**

- From: admin UI tests cover the standalone app.
- To: Playwright host matrix covers React 18, Vue 3, and plain HTML mounting, plus bundle size checks.
- Reason: The core risk in Phase 7 is host integration behavior, not only component rendering.
- Impact: CI gains embed-specific required checks.

## Source Planning

Migrated from `.planning/phases/07-web-component-embed-bundle/` and `.planning/STATE.md`.
