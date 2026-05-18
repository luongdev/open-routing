// Hash routing adapter for the embed Custom Element. Structurally
// compatible with CatalogShellRouterAdapter (the interface Plan
// 07-04 exports from @open-routing/ui via the shell barrel) —
// Iteration-2 BLOCKER #1: this file does NOT import the interface;
// TypeScript verifies the shape at Plan 07-06's assignment site
// (`shell.routerAdapter = new HashRouterAdapter()`). Zero compile-time
// dep on @open-routing/ui keeps wave-1 placement parallel-safe with
// Plan 07-04 (which exports the interface in the same wave).
//
// The shell hands its private Routes instance via start(routes) in
// firstUpdated; the adapter never reaches into shell internals.
// HASH_PREFIX guards against Pitfall 9: hosts may use their own
// hash routing for in-page anchors or React-Router-hash; we only
// navigate the embed when the hash starts with 'open-routing/'
// (D7-06 + RESEARCH §Pitfall 9). 'open-routing/' is also the v0.2
// multi-embed prefix groundwork (D7-07 documents single-embed-per-page
// as a v0.1 limitation).

import type { Routes } from '@lit-labs/router';

const HASH_PREFIX = 'open-routing/';

export class HashRouterAdapter {
  private _routes: Routes | null = null;
  private _onHashChange = (): void => { this._navigate(); };

  start(routes: Routes): void {
    this._routes = routes;
    window.addEventListener('hashchange', this._onHashChange);
    this._navigate();
  }

  stop(): void {
    window.removeEventListener('hashchange', this._onHashChange);
    this._routes = null;
  }

  private _navigate(): void {
    if (!this._routes) return;
    const raw = window.location.hash.slice(1);
    if (!raw) {
      void this._routes.goto('/');
      return;
    }
    if (!raw.startsWith(HASH_PREFIX)) {
      return;
    }
    const stripped = raw.slice(HASH_PREFIX.length) || '/';
    const path = stripped.startsWith('/') ? stripped : '/' + stripped;
    void this._routes.goto(path);
  }

  static buildHash(path: string): string {
    const clean = path.startsWith('/') ? path.slice(1) : path;
    return '#' + HASH_PREFIX + clean;
  }
}
