// Tests for <or-agent-console> (v0.4 W5): polls the agent's live reservations,
// toggles presence, and accepts an offer over REST. Mirrors route-tester's style.
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

import './agent-console.js';

const AGENT = '01935c00-0000-7000-8000-0000000000a1';
const OFFER = {
  id: '01935c00-0000-7000-8000-0000000000c1',
  route_request_id: '01935c00-0000-7000-8000-0000000000b1',
  agent_id: AGENT,
  state: 'offered',
  attempt: 1,
  offered_at: '2026-06-05T00:00:00Z',
  expires_at: '2026-06-05T00:00:30Z',
};

function get(path: string) {
  if (path.endsWith('/status')) return Promise.resolve({ data: { status: 'Ready' }, error: null });
  if (path.endsWith('/reservations')) return Promise.resolve({ data: { items: [OFFER] }, error: null });
  return Promise.resolve({ data: null, error: null });
}

describe('OrAgentConsole', () => {
  let el: HTMLElement;

  beforeEach(() => {
    el = document.createElement('or-agent-console');
    document.body.appendChild(el);
  });
  afterEach(() => {
    if (el.parentNode) el.parentNode.removeChild(el);
    (el as any)._stopPolling?.();
    vi.restoreAllMocks();
  });

  async function settle() {
    await (el as any).updateComplete;
    await new Promise((r) => setTimeout(r, 20));
    await (el as any).updateComplete;
  }

  it('polls status + reservations once agent-id and client are set', async () => {
    (el as any).orgId = 'org-1';
    (el as any).client = { GET: vi.fn().mockImplementation(get), PATCH: vi.fn(), POST: vi.fn() };
    (el as any).agentId = AGENT;
    await settle();
    expect((el as any)._status).toBe('Ready');
    expect((el as any)._reservations).toHaveLength(1);
    expect((el as any)._reservations[0].state).toBe('offered');
  });

  it('Go Ready PATCHes the status transition endpoint', async () => {
    const patch = vi.fn().mockResolvedValue({ data: {}, error: null });
    (el as any).orgId = 'org-1';
    (el as any).client = { GET: vi.fn().mockImplementation(get), PATCH: patch, POST: vi.fn() };
    (el as any).agentId = AGENT;
    await settle();
    await (el as any)._setPresence('Ready');
    const [path, opts] = patch.mock.calls[0] as [string, any];
    expect(path).toBe('/v1/orgs/{org_id}/agents/{id}/status');
    expect(opts.body).toEqual({ to: 'Ready' });
    expect(opts.params.path.id).toBe(AGENT);
  });

  it('Accept POSTs to the reservation accept endpoint', async () => {
    const post = vi.fn().mockResolvedValue({ data: {}, error: null });
    (el as any).orgId = 'org-1';
    (el as any).client = { GET: vi.fn().mockImplementation(get), PATCH: vi.fn(), POST: post };
    (el as any).agentId = AGENT;
    await settle();
    await (el as any)._resolve(OFFER.id, 'accept');
    const [path, opts] = post.mock.calls[0] as [string, any];
    expect(path).toBe('/v1/orgs/{org_id}/reservations/{id}/accept');
    expect(opts.params.path.id).toBe(OFFER.id);
  });
});
