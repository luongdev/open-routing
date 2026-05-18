import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import type { Routes } from '@lit-labs/router';
import { HashRouterAdapter } from './hash-router-adapter.js';

const makeRoutes = () => {
  return {
    goto: vi.fn<Routes['goto']>().mockResolvedValue(undefined),
    outlet: vi.fn(),
    link: vi.fn(),
  } as unknown as Routes;
};

describe('HashRouterAdapter', () => {
  let routes: Routes;
  let adapter: HashRouterAdapter;
  const originalHash = window.location.hash;

  beforeEach(() => {
    routes = makeRoutes();
    adapter = new HashRouterAdapter();
    window.location.hash = '';
  });

  afterEach(() => {
    adapter.stop();
    window.location.hash = originalHash;
    vi.restoreAllMocks();
  });

  it('start(routes) syncs to the current hash and calls goto with the stripped path', () => {
    window.location.hash = '#open-routing/orgs/abc/agents';
    adapter.start(routes);
    expect(routes.goto).toHaveBeenCalledWith('/orgs/abc/agents');
  });

  it('hashchange event triggers goto with the stripped path', () => {
    adapter.start(routes);
    (routes.goto as ReturnType<typeof vi.fn>).mockClear();
    window.location.hash = '#open-routing/orgs/xyz/skills';
    window.dispatchEvent(new HashChangeEvent('hashchange'));
    expect(routes.goto).toHaveBeenCalledWith('/orgs/xyz/skills');
  });

  it('ignores hash without open-routing/ prefix (Pitfall 9)', () => {
    adapter.start(routes);
    (routes.goto as ReturnType<typeof vi.fn>).mockClear();
    window.location.hash = '#some-other-state';
    window.dispatchEvent(new HashChangeEvent('hashchange'));
    expect(routes.goto).not.toHaveBeenCalled();
  });

  it('ignores hash that looks like anchor link (#main-section)', () => {
    adapter.start(routes);
    (routes.goto as ReturnType<typeof vi.fn>).mockClear();
    window.location.hash = '#main-section';
    window.dispatchEvent(new HashChangeEvent('hashchange'));
    expect(routes.goto).not.toHaveBeenCalled();
  });

  it('treats empty hash as root navigation ("/")', () => {
    window.location.hash = '';
    adapter.start(routes);
    expect(routes.goto).toHaveBeenCalledWith('/');
  });

  it('treats prefix-only hash (#open-routing/) as root ("/")', () => {
    window.location.hash = '#open-routing/';
    adapter.start(routes);
    expect(routes.goto).toHaveBeenCalledWith('/');
  });

  it('stop() unregisters the hashchange listener', () => {
    adapter.start(routes);
    adapter.stop();
    (routes.goto as ReturnType<typeof vi.fn>).mockClear();
    window.location.hash = '#open-routing/orgs/abc/agents';
    window.dispatchEvent(new HashChangeEvent('hashchange'));
    expect(routes.goto).not.toHaveBeenCalled();
  });

  it('stop() clears the Routes reference (subsequent _navigate has no target)', () => {
    adapter.start(routes);
    adapter.stop();
    (routes.goto as ReturnType<typeof vi.fn>).mockClear();
    // Force-dispatch hashchange — even if listener weren't fully removed, Routes ref is null.
    window.location.hash = '#open-routing/orgs/abc/agents';
    window.dispatchEvent(new HashChangeEvent('hashchange'));
    expect(routes.goto).not.toHaveBeenCalled();
  });

  it('start can be called after stop with a new Routes instance', () => {
    adapter.start(routes);
    adapter.stop();
    const routes2 = makeRoutes();
    (routes.goto as ReturnType<typeof vi.fn>).mockClear();
    window.location.hash = '#open-routing/orgs/abc/agents';
    adapter.start(routes2);
    expect(routes2.goto).toHaveBeenCalledWith('/orgs/abc/agents');
    expect(routes.goto).not.toHaveBeenCalled();
  });

  it('buildHash() produces an embed-prefixed hash from a path', () => {
    expect(HashRouterAdapter.buildHash('/orgs/abc/agents'))
      .toBe('#open-routing/orgs/abc/agents');
  });

  it('buildHash() handles paths without leading slash', () => {
    expect(HashRouterAdapter.buildHash('orgs/abc/agents'))
      .toBe('#open-routing/orgs/abc/agents');
  });
});
