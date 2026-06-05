// Tests for <or-route-tester> (v0.2 Wave 3): create a route request from a
// published binding, then resolve an offered reservation. Mirrors status-panel
// test style — drive the mock client, assert the POST shape + state wiring.
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

import './route-tester.js';

const BINDING = { channel: 'voice', entry_code: 'main', flow_code: 'flow_inbound', active: true };
const ROUTE = {
  id: '01935c00-0000-7000-8000-0000000000a1',
  channel: 'voice',
  entry_code: 'main',
  flow_code: 'flow_inbound',
  status: 'waiting',
  trace_id: '01935c00-0000-7000-8000-0000000000b1',
  created_at: '2026-06-04T00:00:00Z',
};
const RESERVATION = {
  id: '01935c00-0000-7000-8000-0000000000c1',
  route_request_id: ROUTE.id,
  agent_id: '01935c00-0000-7000-8000-0000000000d1',
  state: 'offered',
  attempt: 0,
  offered_at: '2026-06-04T00:00:00Z',
  expires_at: '2026-06-04T00:00:30Z',
};
const TRACE = { id: ROUTE.trace_id, outcome: null, steps: [{ index: 0, node_id: 'n1', node_kind: 'reservation', status: 'suspended', port: null }] };

function routeGet(path: string) {
  if (path.includes('/bindings')) return Promise.resolve({ data: { items: [BINDING] }, error: null });
  if (path.endsWith('/reservations')) return Promise.resolve({ data: { items: [RESERVATION] }, error: null });
  if (path.endsWith('/trace')) return Promise.resolve({ data: TRACE, error: null });
  return Promise.resolve({ data: ROUTE, error: null }); // GET route-requests/{id}
}

describe('OrRouteTester', () => {
  let el: HTMLElement;

  beforeEach(() => {
    el = document.createElement('or-route-tester');
    document.body.appendChild(el);
  });

  afterEach(() => {
    if (el.parentNode) el.parentNode.removeChild(el);
    vi.restoreAllMocks();
  });

  async function settle() {
    await (el as any).updateComplete;
    await new Promise((r) => setTimeout(r, 30));
    await (el as any).updateComplete;
  }

  it('loads active bindings and seeds channel/entry from the first one', async () => {
    (el as any).orgId = 'test-org';
    (el as any).client = { GET: vi.fn().mockImplementation(routeGet) };
    await settle();
    expect((el as any)._bindings).toHaveLength(1);
    expect((el as any)._channel).toBe('voice');
    expect((el as any)._entryCode).toBe('main');
  });

  it('POSTs a route request with parsed interaction_input and renders status + offered reservation', async () => {
    const post = vi.fn().mockResolvedValue({ data: ROUTE, error: null });
    (el as any).orgId = 'test-org';
    (el as any).client = { GET: vi.fn().mockImplementation(routeGet), POST: post };
    await settle();
    (el as any)._inputJson = '{"customer":{"tier":"gold"}}';
    await (el as any)._createRoute();
    await settle();

    expect(post).toHaveBeenCalled();
    const [path, opts] = post.mock.calls[0] as [string, any];
    expect(path).toBe('/v1/orgs/{org_id}/route-requests');
    expect(opts.body).toMatchObject({ channel: 'voice', entry_code: 'main', interaction_input: { customer: { tier: 'gold' } } });
    expect((el as any)._route?.status).toBe('waiting');
    expect((el as any)._reservations[0]?.state).toBe('offered');
    (el as any)._stopPolling?.();
  });

  it('loads the trace by route id even when the route DTO has no trace_id', async () => {
    // Regression: the API always leaves route.trace_id null (the trace references
    // the route). Gating the trace fetch on route.trace_id meant it never loaded.
    const noTraceId = { ...ROUTE, trace_id: undefined };
    const get = vi.fn().mockImplementation((p: string) =>
      p.endsWith('/trace') ? Promise.resolve({ data: TRACE, error: null })
        : p.endsWith('/reservations') ? Promise.resolve({ data: { items: [] }, error: null })
          : p.includes('/bindings') ? Promise.resolve({ data: { items: [BINDING] }, error: null })
            : Promise.resolve({ data: noTraceId, error: null }));
    (el as any).orgId = 'test-org';
    (el as any).client = { GET: get, POST: vi.fn().mockResolvedValue({ data: noTraceId, error: null }) };
    await settle();
    await (el as any)._createRoute();
    await settle();
    expect((el as any)._route?.trace_id).toBeUndefined();
    expect((el as any)._trace?.steps?.length).toBeGreaterThan(0);
    (el as any)._stopPolling?.();
  });

  it('rejects malformed interaction_input JSON before POSTing', async () => {
    const post = vi.fn();
    (el as any).orgId = 'test-org';
    (el as any).client = { GET: vi.fn().mockImplementation(routeGet), POST: post };
    await settle();
    (el as any)._inputJson = '{not json';
    await (el as any)._createRoute();
    expect(post).not.toHaveBeenCalled();
    expect((el as any)._createError).toContain('Invalid JSON');
  });

  it('accept POSTs to the reservation accept endpoint', async () => {
    const post = vi.fn().mockResolvedValue({ data: { ...RESERVATION, state: 'accepted' }, error: null });
    (el as any).orgId = 'test-org';
    (el as any).client = { GET: vi.fn().mockImplementation(routeGet), POST: post };
    await settle();
    (el as any)._route = ROUTE;
    await (el as any)._resolve(RESERVATION.id, 'accept');
    const [path, opts] = post.mock.calls[0] as [string, any];
    expect(path).toBe('/v1/orgs/{org_id}/reservations/{id}/accept');
    expect(opts.params.path.id).toBe(RESERVATION.id);
    (el as any)._stopPolling?.();
  });
});
