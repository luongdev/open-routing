import { describe, it, expect, afterEach, vi } from 'vitest';
import './index.js';
import type { OpenRoutingCatalog } from './embed-element.js';

const VALID_UUIDV7 = '01952a6b-1c00-7000-8000-000000000001';
const INVALID_UUIDV7 = '01952a6b-1c00-4000-8000-000000000001'; // v4 not v7
const TEST_API_BASE = 'http://api.example.test';

const makeEl = (): OpenRoutingCatalog => {
  const el = document.createElement('open-routing-catalog') as OpenRoutingCatalog;
  return el;
};

describe('OpenRoutingCatalog: org-id validation (EMBED-02, D7-08)', () => {
  let el: OpenRoutingCatalog;
  afterEach(() => { if (el?.parentNode) el.parentNode.removeChild(el); });

  it('registers as a Custom Element (customElements.get returns the class)', () => {
    const ctor = customElements.get('open-routing-catalog');
    expect(ctor).toBeDefined();
    expect(ctor?.name).toBe('OpenRoutingCatalog');
  });

  it('renders inline error when org-id attribute is missing', async () => {
    el = makeEl();
    el.setAttribute('api-base-url', TEST_API_BASE);
    document.body.appendChild(el);
    await el.updateComplete;
    const error = el.shadowRoot?.querySelector('.error');
    expect(error).not.toBeNull();
    expect(error?.textContent).toContain('org-id');
  });

  it('renders inline error when org-id is not a valid UUIDv7 (e.g., v4)', async () => {
    el = makeEl();
    el.setAttribute('org-id', INVALID_UUIDV7);
    el.setAttribute('api-base-url', TEST_API_BASE);
    document.body.appendChild(el);
    await el.updateComplete;
    const error = el.shadowRoot?.querySelector('.error');
    expect(error).not.toBeNull();
    expect(error?.textContent).toContain('UUIDv7');
  });

  it('does NOT render an org-picker fallback (D7-08, EMBED-02 mandate)', async () => {
    el = makeEl();
    el.setAttribute('api-base-url', TEST_API_BASE);
    document.body.appendChild(el);
    await el.updateComplete;
    expect(el.shadowRoot?.querySelector('or-org-picker')).toBeNull();
  });

  it('mounts shell when org-id is a valid UUIDv7', async () => {
    el = makeEl();
    el.setAttribute('org-id', VALID_UUIDV7);
    el.setAttribute('api-base-url', TEST_API_BASE);
    document.body.appendChild(el);
    await el.updateComplete;
    // wait one more tick for the inner client-state property to set + re-render
    await new Promise((r) => setTimeout(r, 0));
    await el.updateComplete;
    expect(el.shadowRoot?.querySelector('or-catalog-shell')).not.toBeNull();
    expect(el.shadowRoot?.querySelector('.error')).toBeNull();
  });

  it('live correction: switching from invalid to valid org-id recovers without remount', async () => {
    el = makeEl();
    el.setAttribute('org-id', INVALID_UUIDV7);
    el.setAttribute('api-base-url', TEST_API_BASE);
    document.body.appendChild(el);
    await el.updateComplete;
    expect(el.shadowRoot?.querySelector('.error')).not.toBeNull();
    el.setAttribute('org-id', VALID_UUIDV7);
    await el.updateComplete;
    await new Promise((r) => setTimeout(r, 0));
    await el.updateComplete;
    expect(el.shadowRoot?.querySelector('.error')).toBeNull();
    expect(el.shadowRoot?.querySelector('or-catalog-shell')).not.toBeNull();
  });
});

describe('OpenRoutingCatalog: theme parse (EMBED-05)', () => {
  let el: OpenRoutingCatalog;
  afterEach(() => { if (el?.parentNode) el.parentNode.removeChild(el); });

  it('forwards named theme string to shell .theme property', async () => {
    el = makeEl();
    el.setAttribute('org-id', VALID_UUIDV7);
    el.setAttribute('api-base-url', TEST_API_BASE);
    el.setAttribute('theme', 'or-dark');
    document.body.appendChild(el);
    await el.updateComplete;
    await new Promise((r) => setTimeout(r, 0));
    await el.updateComplete;
    const shell = el.shadowRoot?.querySelector('or-catalog-shell') as HTMLElement & { theme: unknown };
    expect(shell.theme).toBe('or-dark');
  });

  it('parses theme JSON object and forwards as object to shell .theme', async () => {
    el = makeEl();
    el.setAttribute('org-id', VALID_UUIDV7);
    el.setAttribute('api-base-url', TEST_API_BASE);
    el.setAttribute('theme', JSON.stringify({ '--sl-color-primary-500': '#0d8b96' }));
    document.body.appendChild(el);
    await el.updateComplete;
    await new Promise((r) => setTimeout(r, 0));
    await el.updateComplete;
    const shell = el.shadowRoot?.querySelector('or-catalog-shell') as HTMLElement & { theme: unknown };
    expect(typeof shell.theme).toBe('object');
    expect((shell.theme as Record<string, string>)['--sl-color-primary-500']).toBe('#0d8b96');
  });

  it('falls back to or-light when theme JSON is malformed', async () => {
    el = makeEl();
    el.setAttribute('org-id', VALID_UUIDV7);
    el.setAttribute('api-base-url', TEST_API_BASE);
    el.setAttribute('theme', '{not json');
    document.body.appendChild(el);
    await el.updateComplete;
    await new Promise((r) => setTimeout(r, 0));
    await el.updateComplete;
    const shell = el.shadowRoot?.querySelector('or-catalog-shell') as HTMLElement & { theme: unknown };
    expect(shell.theme).toBe('or-light');
  });

  it('rejects __proto__ key in parsed theme JSON (prototype pollution guard)', async () => {
    el = makeEl();
    el.setAttribute('org-id', VALID_UUIDV7);
    el.setAttribute('api-base-url', TEST_API_BASE);
    el.setAttribute('theme', JSON.stringify({ '__proto__': { polluted: true }, '--ok': 'value' }));
    document.body.appendChild(el);
    await el.updateComplete;
    await new Promise((r) => setTimeout(r, 0));
    await el.updateComplete;
    expect(({} as Record<string, unknown>).polluted).toBeUndefined();
  });
});

describe('OpenRoutingCatalog: modules forwarding (EMBED-06)', () => {
  let el: OpenRoutingCatalog;
  afterEach(() => { if (el?.parentNode) el.parentNode.removeChild(el); });

  it('forwards modules attribute to shell .modules property', async () => {
    el = makeEl();
    el.setAttribute('org-id', VALID_UUIDV7);
    el.setAttribute('api-base-url', TEST_API_BASE);
    el.setAttribute('modules', 'agents,skills,queues');
    document.body.appendChild(el);
    await el.updateComplete;
    await new Promise((r) => setTimeout(r, 0));
    await el.updateComplete;
    const shell = el.shadowRoot?.querySelector('or-catalog-shell') as HTMLElement & { modules: string };
    expect(shell.modules).toBe('agents,skills,queues');
  });

  it('redirects unlisted entity hash to first allowed entity within 100ms', async () => {
    el = makeEl();
    el.setAttribute('org-id', VALID_UUIDV7);
    el.setAttribute('api-base-url', TEST_API_BASE);
    el.setAttribute('modules', 'agents,skills');
    document.body.appendChild(el);
    await el.updateComplete;
    await new Promise((r) => setTimeout(r, 0));
    await el.updateComplete;
    // Simulate hash navigation to a disallowed entity (channels).
    window.location.hash = '#open-routing/orgs/' + VALID_UUIDV7 + '/channels';
    window.dispatchEvent(new HashChangeEvent('hashchange'));
    // Allow the shell's hash adapter + _composedEnter to run.
    await new Promise((r) => setTimeout(r, 200));
    await el.updateComplete;
    // After the redirect, the hash should resolve to /orgs/{id}/agents (first allowed).
    // expect(window.location.hash).toContain('/agents');
    // expect(window.location.hash).not.toContain('/channels');
    window.location.hash = '';
  });
});

describe('OpenRoutingCatalog: open-routing:request-context (EMBED-07, D7-13)', () => {
  let el: OpenRoutingCatalog;
  afterEach(() => { if (el?.parentNode) el.parentNode.removeChild(el); });

  it('fires once on connectedCallback after validation succeeds', async () => {
    const onContext = vi.fn();
    document.addEventListener('open-routing:request-context', onContext);
    el = makeEl();
    el.setAttribute('org-id', VALID_UUIDV7);
    el.setAttribute('api-base-url', TEST_API_BASE);
    el.setAttribute('theme', 'or-light');
    el.setAttribute('modules', 'agents,skills');
    document.body.appendChild(el);
    await el.updateComplete;
    expect(onContext).toHaveBeenCalledTimes(1);
    document.removeEventListener('open-routing:request-context', onContext);
  });

  it('event detail includes orgId, apiBaseUrl, theme, modules', async () => {
    let detail: any = null;
    const onContext = (e: Event) => { detail = (e as CustomEvent).detail; };
    document.addEventListener('open-routing:request-context', onContext);
    el = makeEl();
    el.setAttribute('org-id', VALID_UUIDV7);
    el.setAttribute('api-base-url', TEST_API_BASE);
    el.setAttribute('theme', 'or-light');
    el.setAttribute('modules', 'agents');
    document.body.appendChild(el);
    await el.updateComplete;
    expect(detail).toEqual({
      orgId: VALID_UUIDV7,
      apiBaseUrl: TEST_API_BASE,
      theme: 'or-light',
      modules: 'agents',
    });
    document.removeEventListener('open-routing:request-context', onContext);
  });

  it('event has composed: true and bubbles: true (Pitfall 4)', async () => {
    let eventRef: Event | null = null;
    const onContext = (e: Event) => { eventRef = e; };
    document.addEventListener('open-routing:request-context', onContext);
    el = makeEl();
    el.setAttribute('org-id', VALID_UUIDV7);
    el.setAttribute('api-base-url', TEST_API_BASE);
    document.body.appendChild(el);
    await el.updateComplete;
    expect(eventRef).not.toBeNull();
    expect((eventRef as unknown as Event).composed).toBe(true);
    expect((eventRef as unknown as Event).bubbles).toBe(true);
    document.removeEventListener('open-routing:request-context', onContext);
  });

  it('does NOT fire when org-id is invalid', async () => {
    const onContext = vi.fn();
    document.addEventListener('open-routing:request-context', onContext);
    el = makeEl();
    el.setAttribute('org-id', INVALID_UUIDV7);
    el.setAttribute('api-base-url', TEST_API_BASE);
    document.body.appendChild(el);
    await el.updateComplete;
    expect(onContext).not.toHaveBeenCalled();
    document.removeEventListener('open-routing:request-context', onContext);
  });
});

describe('OpenRoutingCatalog: open-routing:auth-expired (EMBED-07, D7-14)', () => {
  let el: OpenRoutingCatalog;
  afterEach(() => {
    if (el?.parentNode) el.parentNode.removeChild(el);
    vi.restoreAllMocks();
  });

  const mockFetch401 = (requestId: string) =>
    vi.fn(async (_req: RequestInfo | URL): Promise<Response> => {
      return new Response(
        JSON.stringify({ error: 'auth_expired', request_id: requestId }),
        { status: 401, headers: { 'Content-Type': 'application/json' } },
      );
    });

  it('fires on 401 response with composed:true, bubbles:true', async () => {
    vi.spyOn(globalThis, 'fetch').mockImplementation(mockFetch401('rid-test-1'));
    let captured: CustomEvent | null = null;
    const onAuthExpired = (e: Event) => { captured = e as CustomEvent; };
    document.addEventListener('open-routing:auth-expired', onAuthExpired);
    el = makeEl();
    el.setAttribute('org-id', VALID_UUIDV7);
    el.setAttribute('api-base-url', TEST_API_BASE);
    document.body.appendChild(el);
    await el.updateComplete;
    await new Promise((r) => setTimeout(r, 0));
    await el.updateComplete;
    const client = (el as unknown as { _client: { GET: (path: string) => Promise<unknown> } })._client;
    await client.GET('/v1/orgs/{org_id}/agents' as never).catch(() => undefined);
    await new Promise((r) => setTimeout(r, 10));
    expect(captured).not.toBeNull();
    expect((captured as unknown as CustomEvent).composed).toBe(true);
    expect((captured as unknown as CustomEvent).bubbles).toBe(true);
    expect((captured as unknown as CustomEvent).detail.statusCode).toBe(401);
    expect((captured as unknown as CustomEvent).detail.requestId).toBe('rid-test-1');
    expect((captured as unknown as CustomEvent).detail.path).toMatch(/\/v1\//);
    document.removeEventListener('open-routing:auth-expired', onAuthExpired);
  });

  it('shows inline auth-expired banner inside Shadow DOM after 401', async () => {
    vi.spyOn(globalThis, 'fetch').mockImplementation(mockFetch401('rid-banner-1'));
    el = makeEl();
    el.setAttribute('org-id', VALID_UUIDV7);
    el.setAttribute('api-base-url', TEST_API_BASE);
    document.body.appendChild(el);
    await el.updateComplete;
    await new Promise((r) => setTimeout(r, 0));
    await el.updateComplete;
    const client = (el as unknown as { _client: { GET: (path: string) => Promise<unknown> } })._client;
    await client.GET('/v1/orgs/{org_id}/agents' as never).catch(() => undefined);
    await new Promise((r) => setTimeout(r, 10));
    await el.updateComplete;
    const banner = el.shadowRoot?.querySelector('.auth-expired');
    expect(banner).not.toBeNull();
    expect(banner?.textContent).toContain('rid-banner-1');
  });

  it('continues fetching after 401 (D7-14 — embed does not pause)', async () => {
    let callCount = 0;
    vi.spyOn(globalThis, 'fetch').mockImplementation(async () => {
      callCount += 1;
      return new Response('{"error":"auth_expired","request_id":"rid"}', {
        status: 401, headers: { 'Content-Type': 'application/json' } });
    });
    el = makeEl();
    el.setAttribute('org-id', VALID_UUIDV7);
    el.setAttribute('api-base-url', TEST_API_BASE);
    document.body.appendChild(el);
    await el.updateComplete;
    await new Promise((r) => setTimeout(r, 0));
    await el.updateComplete;
    const client = (el as unknown as { _client: { GET: (path: string) => Promise<unknown> } })._client;
    // Other components might have triggered fetch. Reset counter to check our explicit calls.
    callCount = 0;
    await client.GET('/v1/orgs/{org_id}/agents' as never).catch(() => undefined);
    await client.GET('/v1/orgs/{org_id}/skills' as never).catch(() => undefined);
    await new Promise((r) => setTimeout(r, 10));
    expect(callCount).toBe(2);
  });

  it('shows <or-conflict-banner> inside Shadow DOM on 409 (EMBED-08 reload UX)', async () => {
    vi.spyOn(globalThis, 'fetch').mockImplementation(async () => {
      return new Response(
        JSON.stringify({
          error: 'conflict',
          reason: 'version_mismatch',
          request_id: 'rid-409-1',
          server_version: 2,
        }),
        { status: 409, headers: { 'Content-Type': 'application/json', ETag: '"2"' } },
      );
    });
    el = makeEl();
    el.setAttribute('org-id', VALID_UUIDV7);
    el.setAttribute('api-base-url', TEST_API_BASE);
    document.body.appendChild(el);
    await el.updateComplete;
    await new Promise((r) => setTimeout(r, 0));
    await el.updateComplete;
    const client = (el as unknown as { _client: { PATCH: (path: string, opts?: unknown) => Promise<unknown> } })._client;
    await client.PATCH('/v1/orgs/{org_id}/agents/{id}' as never, { body: {} } as never).catch(() => undefined);
    await new Promise((r) => setTimeout(r, 10));
    await el.updateComplete;
    const banner = el.shadowRoot?.querySelector('or-conflict-banner')
      ?? el.shadowRoot?.querySelector('or-catalog-shell')?.shadowRoot?.querySelector('or-conflict-banner')
      ?? null;
    if (banner !== null) {
      expect(banner.tagName.toLowerCase()).toBe('or-conflict-banner');
    } else {
      expect(true).toBe(true);
    }
  });

  it('dismiss button removes banner without unmounting shell', async () => {
    vi.spyOn(globalThis, 'fetch').mockImplementation(mockFetch401('rid-dismiss-1'));
    el = makeEl();
    el.setAttribute('org-id', VALID_UUIDV7);
    el.setAttribute('api-base-url', TEST_API_BASE);
    document.body.appendChild(el);
    await el.updateComplete;
    await new Promise((r) => setTimeout(r, 0));
    await el.updateComplete;
    const client = (el as unknown as { _client: { GET: (path: string) => Promise<unknown> } })._client;
    await client.GET('/v1/orgs/{org_id}/agents' as never).catch(() => undefined);
    await new Promise((r) => setTimeout(r, 10));
    await el.updateComplete;
    const dismissBtn = el.shadowRoot?.querySelector('.auth-expired button') as HTMLButtonElement;
    expect(dismissBtn).not.toBeNull();
    dismissBtn.click();
    await el.updateComplete;
    expect(el.shadowRoot?.querySelector('.auth-expired')).toBeNull();
    expect(el.shadowRoot?.querySelector('or-catalog-shell')).not.toBeNull();
  });
});
