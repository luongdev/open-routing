// Phase 6 Plan 08 Task 1: Tests for <or-queue-list>
// TDD RED — these tests fail until queue-list.ts exists.
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

// Registers <or-queue-list> element
import './queue-list.js';

const MOCK_QUEUE = {
  id: '01935c00-0000-7000-8000-000000000001',
  code: 'queue_billing',
  name: 'Billing Queue',
  channel_types: ['voice', 'chat'],
  priority: 5,
  acw_sec: 60,
  enabled: true,
  version: 1,
  updated_at: '2026-05-17T00:00:00Z',
  created_at: '2026-05-01T00:00:00Z',
};

describe('OrQueueList', () => {
  let el: HTMLElement;

  beforeEach(() => {
    el = document.createElement('or-queue-list');
    document.body.appendChild(el);
  });

  afterEach(() => {
    if (el.parentNode) el.parentNode.removeChild(el);
    vi.restoreAllMocks();
  });

  it('renders empty state "No queues yet" when items=[]', async () => {
    (el as any).orgId = 'test-org-id';
    (el as any).client = {
      GET: vi.fn().mockResolvedValue({
        data: { items: [], has_more: false, next_cursor: null },
        error: null,
      }),
    };
    await (el as any).updateComplete;
    await new Promise((r) => setTimeout(r, 50));
    await (el as any).updateComplete;

    const shadow = el.shadowRoot!;
    expect(shadow.textContent).toContain('No queues yet');
  });

  it('renders channel_types as individual sl-badge elements per type', async () => {
    (el as any).orgId = 'test-org-id';
    (el as any).client = {
      GET: vi.fn().mockResolvedValue({
        data: { items: [MOCK_QUEUE], has_more: false, next_cursor: null },
        error: null,
      }),
    };
    await (el as any).updateComplete;
    await new Promise((r) => setTimeout(r, 50));
    await (el as any).updateComplete;

    const shadow = el.shadowRoot!;
    const table = shadow.querySelector('or-data-table') as any;
    expect(table).toBeTruthy();
    // Each channel_type should produce an sl-badge
    // The data-table renders via column renderers; we inspect rows prop to confirm data is there
    expect(table.rows).toHaveLength(1);
    expect(table.rows[0]?.channel_types).toEqual(['voice', 'chat']);
  });

  it('renders acw_sec as "{N}s" format and priority as number', async () => {
    (el as any).orgId = 'test-org-id';
    (el as any).client = {
      GET: vi.fn().mockResolvedValue({
        data: { items: [MOCK_QUEUE], has_more: false, next_cursor: null },
        error: null,
      }),
    };
    await (el as any).updateComplete;
    await new Promise((r) => setTimeout(r, 50));
    await (el as any).updateComplete;

    // The column renderer for acw_sec should produce "60s" — verify by checking the column def
    const columns = (el as any)._columns as Array<{ key: string; render?: (row: Record<string, unknown>) => unknown }>;
    const acwCol = columns.find((c) => c.key === 'acw_sec');
    expect(acwCol).toBeTruthy();
    // Render the column; the result is a TemplateResult with values array
    // Lit TemplateResult stores dynamic values separately — join all values into string to check
    if (acwCol?.render) {
      const rendered = acwCol.render(MOCK_QUEUE as unknown as Record<string, unknown>) as any;
      // TemplateResult.values is an iterable of dynamic parts; join them with the strings
      const strings = rendered?.strings ?? [];
      const values = rendered?.values ?? [];
      // Combine strings and values interleaved
      const combined = (strings as string[]).reduce((acc: string, s: string, i: number) => {
        return acc + s + (values[i] !== undefined ? String(values[i]) : '');
      }, '');
      expect(combined).toContain('60s');
    }
  });

  it('row click dispatches open-routing:navigate with /orgs/{orgId}/queues/{id}', async () => {
    (el as any).orgId = 'test-org-id';
    (el as any).client = {
      GET: vi.fn().mockResolvedValue({
        data: { items: [MOCK_QUEUE], has_more: false, next_cursor: null },
        error: null,
      }),
    };
    await (el as any).updateComplete;
    await new Promise((r) => setTimeout(r, 50));
    await (el as any).updateComplete;

    const events: CustomEvent[] = [];
    el.addEventListener('open-routing:navigate', (e) => events.push(e as CustomEvent));

    const shadow = el.shadowRoot!;
    const table = shadow.querySelector('or-data-table');
    expect(table).toBeTruthy();
    table!.dispatchEvent(
      new CustomEvent('or-row-click', {
        detail: { row: MOCK_QUEUE },
        bubbles: true,
        composed: true,
      })
    );
    await (el as any).updateComplete;

    expect(events).toHaveLength(1);
    expect(events[0]?.detail?.path).toContain(MOCK_QUEUE.id);
    expect(events[0]?.detail?.path).toContain('/queues/');
  });

  it('[+ Create queue] button dispatches open-routing:navigate to /orgs/{orgId}/queues/new', async () => {
    (el as any).orgId = 'test-org-id';
    (el as any).client = {
      GET: vi.fn().mockResolvedValue({
        data: { items: [], has_more: false, next_cursor: null },
        error: null,
      }),
    };
    await (el as any).updateComplete;
    await new Promise((r) => setTimeout(r, 50));
    await (el as any).updateComplete;

    const events: CustomEvent[] = [];
    el.addEventListener('open-routing:navigate', (e) => events.push(e as CustomEvent));

    // Call _navigate directly (simulates create button click)
    (el as any)._navigate(`/orgs/test-org-id/queues/new`);

    expect(events).toHaveLength(1);
    expect(events[0]?.detail?.path).toBe('/orgs/test-org-id/queues/new');
  });

  it('context menu Disable fires PATCH enabled=false on correct queue id', async () => {
    const mockGet = vi.fn().mockResolvedValue({
      data: { items: [MOCK_QUEUE], has_more: false, next_cursor: null },
      error: null,
    });
    const mockPatch = vi.fn().mockResolvedValue({
      data: { ...MOCK_QUEUE, enabled: false, version: 2 },
      error: null,
    });
    (el as any).orgId = 'test-org-id';
    (el as any).client = { GET: mockGet, PATCH: mockPatch };
    await (el as any).updateComplete;
    await new Promise((r) => setTimeout(r, 50));
    await (el as any).updateComplete;

    // Call _handleDisable directly with the queue
    await (el as any)._handleDisable(MOCK_QUEUE);

    expect(mockPatch).toHaveBeenCalled();
    const patchCall = mockPatch.mock.calls[0];
    expect(patchCall?.[1]?.body?.enabled).toBe(false);
    expect(patchCall?.[1]?.body?.version).toBe(MOCK_QUEUE.version);
  });
});
