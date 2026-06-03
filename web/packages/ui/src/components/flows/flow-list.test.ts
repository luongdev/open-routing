// Tests for <or-flow-list> (v0.2 Layer 2). Mirrors queue-list.test.ts: the
// component takes orgId + a (mock) client and renders via or-data-table.
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

import './flow-list.js';

const MOCK_FLOW = {
  id: '01935c00-0000-7000-8000-0000000000f1',
  code: 'flow_inbound',
  name: 'Inbound Voice',
  graph: { nodes: [], edges: [] },
  enabled: true,
  version: 1,
  updated_at: '2026-05-17T00:00:00Z',
  created_at: '2026-05-01T00:00:00Z',
};

function renderToString(tr: any): string {
  const s = (tr?.strings ?? []) as string[];
  const v = (tr?.values ?? []) as unknown[];
  return s.reduce((acc, str, i) => acc + str + (v[i] !== undefined ? String(v[i]) : ''), '');
}

describe('OrFlowList', () => {
  let el: HTMLElement;

  beforeEach(() => {
    el = document.createElement('or-flow-list');
    document.body.appendChild(el);
  });

  afterEach(() => {
    if (el.parentNode) el.parentNode.removeChild(el);
    vi.restoreAllMocks();
  });

  async function settle() {
    await (el as any).updateComplete;
    await new Promise((r) => setTimeout(r, 50));
    await (el as any).updateComplete;
  }

  it('renders empty state "No flows yet" when items=[]', async () => {
    (el as any).orgId = 'test-org';
    (el as any).client = {
      GET: vi.fn().mockResolvedValue({ data: { items: [], has_more: false, next_cursor: null }, error: null }),
    };
    await settle();
    const shadow = el.shadowRoot!;
    expect(shadow.textContent).toContain('No flows yet');
    expect(shadow.querySelector('.empty-state button')?.textContent?.trim()).toBe('+ Create flow');
    expect(shadow.querySelector('.page-header button.uk-button-primary')?.textContent?.trim()).toBe('+ Create flow');
  });

  it('renders rows and the Status (Draft/Archived) + pending Published columns', async () => {
    (el as any).orgId = 'test-org';
    (el as any).client = {
      GET: vi.fn().mockResolvedValue({ data: { items: [MOCK_FLOW], has_more: false, next_cursor: null }, error: null }),
    };
    await settle();
    const table = el.shadowRoot!.querySelector('or-data-table') as any;
    expect(table).toBeTruthy();
    expect(table.rows).toHaveLength(1);
    expect(table.rows[0]?.code).toBe('flow_inbound');

    const cols = (el as any)._columns as Array<{ key: string; render: (r: any) => unknown }>;
    const status = cols.find((c) => c.key === 'enabled')!;
    expect(renderToString(status.render(MOCK_FLOW))).toContain('Draft');
    expect(renderToString(status.render({ ...MOCK_FLOW, enabled: false }))).toContain('Archived');
    const published = cols.find((c) => c.key === 'published')!;
    expect(renderToString(published.render(MOCK_FLOW))).toContain('—');
  });

  it('row click dispatches open-routing:navigate to /orgs/{org}/flows/{id}', async () => {
    (el as any).orgId = 'test-org';
    (el as any).client = {
      GET: vi.fn().mockResolvedValue({ data: { items: [MOCK_FLOW], has_more: false, next_cursor: null }, error: null }),
    };
    await settle();
    const events: CustomEvent[] = [];
    el.addEventListener('open-routing:navigate', (e) => events.push(e as CustomEvent));
    el.shadowRoot!
      .querySelector('or-data-table')!
      .dispatchEvent(new CustomEvent('or-row-click', { detail: { row: MOCK_FLOW }, bubbles: true, composed: true }));
    await (el as any).updateComplete;
    expect(events).toHaveLength(1);
    expect(events[0]?.detail?.path).toBe(`/orgs/test-org/flows/${MOCK_FLOW.id}`);
  });

  it('disable action PATCHes enabled=false with the current version', async () => {
    const mockPatch = vi.fn().mockResolvedValue({ data: { ...MOCK_FLOW, enabled: false, version: 2 }, error: null });
    (el as any).orgId = 'test-org';
    (el as any).client = {
      GET: vi.fn().mockResolvedValue({ data: { items: [MOCK_FLOW], has_more: false, next_cursor: null }, error: null }),
      PATCH: mockPatch,
    };
    await settle();
    await (el as any)._handleRowAction(
      new CustomEvent('or-row-action', { detail: { row: MOCK_FLOW, action: 'disable' } })
    );
    expect(mockPatch).toHaveBeenCalled();
    expect(mockPatch.mock.calls[0]?.[1]?.body?.enabled).toBe(false);
    expect(mockPatch.mock.calls[0]?.[1]?.body?.version).toBe(1);
  });
});
