# Changelog

All notable changes to `@open-routing/catalog-embed` are documented here.
The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0-pre.1] — 2026-05-18

First pre-release of the embed bundle. Not yet published to npm —
install via tarball (`pnpm pack`).

### Added
- `<open-routing-catalog>` Custom Element registers on import.
- Public attributes: `org-id` (UUIDv7), `api-base-url`, `theme`,
  `modules`, `locale`.
- Public events: `open-routing:request-context` (announce-only on
  mount) and `open-routing:auth-expired` (every HTTP 401). Both
  `composed: true` and `bubbles: true`.
- Shadow DOM isolation (mode: open) — host styles do not bleed in;
  embed styles do not escape.
- Hash routing inside the embed (`#open-routing/...`).
- Lazy entity bundles — 6 catalog entities + status panel + bulk
  import load on first navigation.
- Bundle size budget enforced via gzip file-size gate (≤ 70 KB gzipped
  eager baseline; lazy chunks reported separately).
- Playwright integration tests across React 18, Vue 3, plain HTML
  hosts (3 × 4 specs).

### Known limitations (v0.1)
- Single embed per page (hash space conflict — see README).
- No npm/CDN publish yet; install via tarball.
- Stub auth (`X-Org-Id` header from trusted host); real auth in v1.
- No SSR; Custom Elements are client-rendered only.

[0.1.0-pre.1]: https://github.com/luongdev/open-routing/releases/tag/v0.1.0-pre.1
