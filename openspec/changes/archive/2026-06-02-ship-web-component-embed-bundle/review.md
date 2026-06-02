# Phase 7 Peer Review Record

## Review Attempt

- Requested tool: Gemini, per repository cross-AI review rule.
- Result: `gemini` was not installed in the environment.
- Fallback: Claude Code CLI 2.1.160.
- Full-diff review attempts: two large prompts were killed after hanging without output.
- Completed review: summary-only Claude review completed on 2026-06-02.

## Findings And Disposition

- H1: Empty CORS config risked permissive `Access-Control-Allow-Origin: *`.
  - Disposition: Fixed. `NewCORS([]string{})` now fails closed through a sentinel origin, and `TestCORS_EmptyAllowedOriginsDenyByDefault` locks it.
- H2: Lazy chunks are unbounded.
  - Disposition: Accepted by scope. Phase 7 gates only eager `dist/embed.js <= 70 KiB gzip`; lazy chunks are logged.
- M1: Package declarations could reference internals not present in tarball consumers.
  - Disposition: Fixed. Embed locale loading is self-contained, public `.d.ts` files are tarball-safe, and `pack:smoke` runs scratch install plus `tsc --noEmit`.
- M2: E2E could run stale dist if CI did not build first.
  - Disposition: Covered. `web-embed-e2e` builds before Playwright.
- M3: OpenSpec CLI validation unavailable.
  - Disposition: Noted. `npx openspec validate --all` failed because npm could not determine an executable; no local OpenSpec CLI exists in this repo.
- M4: Dual history/hash routing can drift.
  - Disposition: Covered by embed host e2e and shared web/admin build/typecheck gates.
