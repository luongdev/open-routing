// Phase 6 Plan 10 Task 1: <or-channel-list> unit tests.
// TDD RED phase: all tests fail before implementation.
// Tests cover: empty state, default_queue_id truncation+tooltip, null em-dash, row click.

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

// Import the component (will fail until implementation exists)
import './channel-list.js';

const MOCK_CHANNEL = {
  id: '01901b2c-7f3a-7000-8000-000000000010',
  org_id: '01901b2c-7f3a-7000-8000-000000000001',
  code: 'ch_voice',
  name: 'Voice Channel',
  channel_type: 'voice',
  default_queue_id: null,
  enabled: true,
  version: 1,
  created_at: new Date().toISOString(),
  updated_at: new Date().toISOString(),
};

const FULL_QUEUE_UUID = '01901b2c-7f3a-7abc-8d4e-5f6a7b8c9d0e';

describe('or-channel-list', () => {
  let el: HTMLElement;

  beforeEach(() => {
    el = document.createElement('or-channel-list');
    document.body.appendChild(el);
  });

  afterEach(() => {
    if (el.parentNode) el.parentNode.removeChild(el);
    vi.clearAllMocks();
  });

  it('Test 1: renders "No channels yet" empty state when items=[]', async () => {
    (el as any).orgId = '01901b2c-7f3a-7000-8000-000000000001';
    (el as any).client = {
      GET: vi.fn().mockResolvedValue({
        data: { items: [], has_more: false, next_cursor: null },
        error: null,
      }),
    };

    await (el as any).updateComplete;
    await new Promise((r) => setTimeout(r, 50));
    await (el as any).updateComplete;

    const text = el.shadowRoot?.textContent ?? '';
    expect(text).toContain('No channels');
  });

  it('Test 2: default_queue_id column shows first 8 chars + "..." + sl-tooltip with full value', async () => {
    const channel = { ...MOCK_CHANNEL, default_queue_id: FULL_QUEUE_UUID };
    (el as any).orgId = '01901b2c-7f3a-7000-8000-000000000001';
    (el as any).client = {
      GET: vi.fn().mockResolvedValue({
        data: { items: [channel], has_more: false, next_cursor: null },
        error: null,
      }),
    };

    await (el as any).updateComplete;
    await new Promise((r) => setTimeout(r, 50));
    await (el as any).updateComplete;

    // Check column render function produces truncated ID + tooltip
    const columns = (el as any)._columns as Array<{ key: string; render?: (row: Record<string, unknown>) => unknown }>;
    const queueCol = columns.find((c) => c.key === 'default_queue_id');
    expect(queueCol?.render).toBeTruthy();

    const rendered = JSON.stringify(queueCol!.render!({ default_queue_id: FULL_QUEUE_UUID }));
    // Should contain first 8 chars of UUID
    expect(rendered).toContain(FULL_QUEUE_UUID.slice(0, 8));
    // Should contain full UUID (in tooltip content)
    expect(rendered).toContain(FULL_QUEUE_UUID);
  });

  it('Test 3: default_queue_id null/empty shows "—" (em-dash) in that column', async () => {
    const channel = { ...MOCK_CHANNEL, default_queue_id: null };
    (el as any).orgId = '01901b2c-7f3a-7000-8000-000000000001';
    (el as any).client = {
      GET: vi.fn().mockResolvedValue({
        data: { items: [channel], has_more: false, next_cursor: null },
        error: null,
      }),
    };

    await (el as any).updateComplete;
    await new Promise((r) => setTimeout(r, 50));
    await (el as any).updateComplete;

    const columns = (el as any)._columns as Array<{ key: string; render?: (row: Record<string, unknown>) => unknown }>;
    const queueCol = columns.find((c) => c.key === 'default_queue_id');
    expect(queueCol?.render).toBeTruthy();

    const rendered = JSON.stringify(queueCol!.render!({ default_queue_id: null }));
    // em-dash character or HTML entity
    expect(rendered.includes('—') || rendered.includes('\\u2014') || rendered.includes('&mdash')).toBe(true);
  });

  it('Test 4: row click dispatches open-routing:navigate to /orgs/{orgId}/channels/{id}', async () => {
    const channel = { ...MOCK_CHANNEL };
    (el as any).orgId = '01901b2c-7f3a-7000-8000-000000000001';
    (el as any).client = {
      GET: vi.fn().mockResolvedValue({
        data: { items: [channel], has_more: false, next_cursor: null },
        error: null,
      }),
    };

    await (el as any).updateComplete;
    await new Promise((r) => setTimeout(r, 50));
    await (el as any).updateComplete;

    const events: CustomEvent[] = [];
    el.addEventListener('open-routing:navigate', (e) => events.push(e as CustomEvent));

    // Dispatch or-row-click from data-table
    const shadow = el.shadowRoot!;
    const table = shadow.querySelector('or-data-table');
    table?.dispatchEvent(new CustomEvent('or-row-click', {
      detail: { row: channel },
      bubbles: true,
      composed: true,
    }));

    await (el as any).updateComplete;

    expect(events.length).toBeGreaterThan(0);
    expect(events[0]?.detail?.path).toContain('/channels/01901b2c-7f3a-7000-8000-000000000010');
  });
});
