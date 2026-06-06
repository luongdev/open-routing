// Tests for <or-ops-view> (v0.3 W6): the live operations dashboard polls
// /routing/stats + recent route-requests and renders the stat cards + feed.
import { describe, it, expect, vi, afterEach } from 'vitest';

import './ops-view.js';

const STATS = { waiting_match: 3, offering: 1, waiting_offer: 2, oldest_waiting_seconds: 95, held_slots: 4 };
const ROUTES = {
  items: [
    { id: '01935c00-0000-7000-8000-0000000000a1', channel: 'voice', entry_code: 'main', status: 'waiting_match', created_at: '2026-06-04T00:00:00Z' },
    { id: '01935c00-0000-7000-8000-0000000000a2', channel: 'voice', entry_code: 'main', status: 'completed', created_at: '2026-06-04T00:01:00Z' },
  ],
};

function opsGet(path: string) {
  if (path.endsWith('/routing/stats')) return Promise.resolve({ data: STATS, error: null });
  if (path.endsWith('/route-requests')) return Promise.resolve({ data: ROUTES, error: null });
  return Promise.resolve({ data: null, error: null });
}

describe('OrOpsView', () => {
  let el: HTMLElement;

  afterEach(() => {
    if (el?.parentNode) el.parentNode.removeChild(el); // disconnectedCallback clears the poll timer
    vi.restoreAllMocks();
  });

  async function settle() {
    await (el as any).updateComplete;
    await new Promise((r) => setTimeout(r, 30));
    await (el as any).updateComplete;
  }

  it('polls stats + routes and renders the cards and feed', async () => {
    el = document.createElement('or-ops-view');
    document.body.appendChild(el);
    (el as any).orgId = 'test-org';
    (el as any).client = { GET: vi.fn().mockImplementation(opsGet) };
    await settle();

    expect((el as any)._stats).toMatchObject({ waiting_match: 3, held_slots: 4, oldest_waiting_seconds: 95 });
    expect((el as any)._routes).toHaveLength(2);

    const text = el.shadowRoot?.textContent ?? '';
    expect(text).toContain('Queue depth');
    expect(text).toContain('Held slots');
    expect(text).toContain('1m 35s'); // oldest wait humanised
    expect(text).toContain('waiting_match'); // route status pill
  });

  it('surfaces an error when stats are unavailable (matcher off)', async () => {
    el = document.createElement('or-ops-view');
    document.body.appendChild(el);
    (el as any).orgId = 'test-org';
    (el as any).client = { GET: vi.fn().mockResolvedValue({ data: null, error: null }) };
    await settle();
    expect((el as any)._error).toContain('MATCHER_ENABLED');
  });
});
