# @open-routing/catalog-embed

`<open-routing-catalog>` — a Web Component that embeds the full Open
Routing catalog admin UI inside any host application (React, Vue, plain
HTML — anything that runs in a modern browser).

> **v0.1 pre-release.** Not yet published to npm; install via tarball
> per the [Install](#install) section.

## Install

### Tarball (v0.1 path)

```bash
# In the host project
pnpm add file:./path/to/open-routing-catalog-embed-0.1.0-pre.1.tgz
```

### CDN (esm.sh; available after v1 npm publish)

> **v0.1 status — tarball install only.** D7-15 + CONTEXT.md
> `<deferred>` defer npm publish to v1; until the package is on the
> registry, the esm.sh URL below 404s. v1 will publish to npm and
> esm.sh will mirror automatically.

```html
<!-- Available after v1 publish; not reachable on v0.1 -->
<script type="module"
        src="https://esm.sh/@open-routing/catalog-embed@<version>"></script>
```

Importing the module side-effect registers `<open-routing-catalog>` as
a Custom Element. WARNING #11 — for v0.1, use the [Tarball](#tarball-v01-path)
path above.

## Quick start

```html
<open-routing-catalog
  org-id="01952a6b-1c00-7000-8000-000000000001"
  api-base-url="https://api.example.com"
  theme="or-light"
  modules="agents,skills,queues">
</open-routing-catalog>

<script type="module" src="@open-routing/catalog-embed"></script>
```

## Attributes

| Attribute       | Required | Type                | Default      | Notes |
|-----------------|----------|---------------------|--------------|-------|
| `org-id`        | yes      | string (UUIDv7)     | —            | Tenant identifier; flows to every API call as the `X-Org-Id` header. Invalid format renders an inline error state. |
| `api-base-url`  | yes      | string (URL)        | —            | Absolute or relative URL pointing at the Open Routing API. Relative URLs are resolved against the host's origin (configure host reverse-proxy accordingly). |
| `theme`         | no       | string (named or JSON) | `or-light` | One of `or-light`, `or-dark`, `or-brand` — OR a JSON-encoded object of CSS custom properties (see [Theming](#theming)). Malformed JSON falls back to `or-light` silently. |
| `modules`       | no       | string (CSV)        | (all)        | Comma-separated entity keys: `agents,skills,queues,channels,adapters,break-reasons`. Unlisted entities are removed from navigation and routes. |
| `locale`        | no       | `en` \| `vi`        | `en`         | Switches lit-localize translation bundles. |

## Events

All events use `composed: true` and `bubbles: true`, so they cross the
Shadow DOM boundary and can be listened to from `document` or any
ancestor.

| Event | When fired | `detail` shape |
|-------|-----------|----------------|
| `open-routing:request-context` | Once on mount after attributes validate. Announce-only — for diagnostics or v1 auth integration. | `{ orgId, apiBaseUrl, theme, modules }` |
| `open-routing:auth-expired`    | On every HTTP 401 response. Embed continues to fetch — host owns recovery (refresh token, sign in). | `{ statusCode: 401, requestId, path }` |

### Listening from a host

```javascript
document.addEventListener('open-routing:auth-expired', (event) => {
  console.log('Session expired:', event.detail.requestId);
  // Trigger your auth refresh flow here.
});
```

## Theming

Pass a JSON object of CSS custom properties via the `theme` attribute:

```html
<open-routing-catalog
  org-id="..."
  api-base-url="..."
  theme='{"--sl-color-primary-500":"#0d8b96","--or-color-app-bg":"#f9fafb"}'>
</open-routing-catalog>
```

The 10 most-impactful tokens:

| Token | Affects |
|-------|---------|
| `--sl-color-primary-500` | Primary brand color (buttons, links, focus) |
| `--or-color-app-bg`      | Page background inside the embed |
| `--or-color-text-strong` | Heading text |
| `--or-color-card-bg`     | Card surfaces (lists, detail panels) |
| `--or-color-card-border` | Card borders |
| `--sl-border-radius-medium` | Corner radius |
| `--sl-font-family`       | Font stack |
| `--or-color-conflict-bg` | 409 conflict banner background |
| `--or-color-focus-ring`  | Focus outline color |
| `--or-color-divider`     | Table/sidebar dividers |

The complete token list is exported from `@open-routing/ui/themes`.

## API requirements

### CORS

The Open Routing API must allow the host origin via the
`CORS_ALLOWED_ORIGINS` environment variable. For example, in a Go API
binary with the Phase 7 CORS middleware:

```bash
export CORS_ALLOWED_ORIGINS="https://your-host-app.example.com"
```

Multiple origins are comma-separated. Use `*` in dev only.

### Headers

Every API request from the embed carries:

- `X-Org-Id: <org-id attribute value>`
- `Content-Type: application/json` (for POST/PATCH)
- `Accept-Language: <locale>`
- `Idempotency-Key: <ulid>` (on bulk-import POST only)

## Limitations (v0.1)

- **Single embed per page.** `window.location.hash` carries the
  embed's nav state; two instances on the same page will fight for it
  (last write wins). v0.2 may add a `hash-prefix` attribute for
  multi-embed; track interest in the project.
- **No SSR.** Custom Elements are client-rendered only. Hosts wanting
  SSR should render a placeholder and let the element upgrade on
  `connectedCallback`.
- **Hash routing only.** History-mode routing inside the embed would
  require coordination with the host's URL space (a `base-path`
  attribute is on the v1+ roadmap).
- **Stub auth.** v0.1 reads `X-Org-Id` from a trusted host. Real auth
  (tokens, RBAC) arrives in v1 alongside the `open-routing:auth-expired`
  recovery flow.

## Browser support

Modern browsers from the last ~2 years (Chrome 89+, Safari 14+,
Firefox 89+). The bundle uses native ES modules, Custom Elements v1,
Shadow DOM v1, and URLPattern (polyfilled via lit-labs/router
dependency on older Chromium).

## Bundle size

The eager baseline (`dist/embed.js`) is ≤ 70 KB gzipped. Per-entity
modules lazy-load on first navigation. Brotli-served reality (on
Cloudflare Pro+ or similar CDN) is ~52 KB.

## License

MIT — see [LICENSE](./LICENSE).

## Changelog

See [CHANGELOG.md](./CHANGELOG.md).
