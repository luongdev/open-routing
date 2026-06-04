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

  it('GETs /v1/orgs/{org_id}/flows with include_disabled and limit in the query', async () => {
    const mockGet = vi.fn().mockResolvedValue({ data: { items: [], has_more: false, next_cursor: null }, error: null });
    (el as any).orgId = 'test-org';
    (el as any).client = { GET: mockGet };
    await settle();
    expect(mockGet).toHaveBeenCalled();
    const [path, opts] = mockGet.mock.calls[0] as [string, any];
    expect(path).toBe('/v1/orgs/{org_id}/flows');
    expect(opts?.params?.path?.org_id).toBe('test-org');
    expect(opts?.params?.query).toHaveProperty('include_disabled');
    expect(opts?.params?.query).toHaveProperty('limit');
    expect(opts?.params?.query?.limit).toBe(25);
  });

  it('delete action confirms then DELETEs with the right path params', async () => {
    const mockDelete = vi.fn().mockResolvedValue({ data: null, error: null });
    (el as any).orgId = 'test-org';
    (el as any).client = {
      GET: vi.fn().mockResolvedValue({ data: { items: [MOCK_FLOW], has_more: false, next_cursor: null }, error: null }),
      DELETE: mockDelete,
    };
    await settle();
    el.shadowRoot!
      .querySelector('or-data-table')!
      .dispatchEvent(
        new CustomEvent('or-row-action', { detail: { row: MOCK_FLOW, action: 'delete' }, bubbles: true, composed: true })
      );
    // The styled confirm dialog mounts on document.body — confirm via its
    // destructive button (replaces window.confirm).
    await settle();
    const dialog = document.querySelector('or-dialog');
    expect(dialog).toBeTruthy();
    const confirmBtn = dialog!.querySelector('button[data-action="confirm"]') as HTMLElement;
    expect(confirmBtn).toBeTruthy();
    confirmBtn.click();
    await settle();
    expect(mockDelete).toHaveBeenCalled();
    const [path, opts] = mockDelete.mock.calls[0] as [string, any];
    expect(path).toBe('/v1/orgs/{org_id}/flows/{id}');
    expect(opts?.params?.path?.org_id).toBe('test-org');
    expect(opts?.params?.path?.id).toBe(MOCK_FLOW.id);
  });

  it('surfaces a version_conflict from a disable action and does not treat it as success', async () => {
    const mockGet = vi.fn().mockResolvedValue({ data: { items: [MOCK_FLOW], has_more: false, next_cursor: null }, error: null });
    const mockPatch = vi.fn().mockResolvedValue({ data: null, error: { reason: 'version_conflict' } });
    (el as any).orgId = 'test-org';
    (el as any).client = { GET: mockGet, PATCH: mockPatch };
    await settle();
    const getCallsBefore = mockGet.mock.calls.length;
    el.shadowRoot!
      .querySelector('or-data-table')!
      .dispatchEvent(
        new CustomEvent('or-row-action', { detail: { row: MOCK_FLOW, action: 'disable' }, bubbles: true, composed: true })
      );
    await settle();
    expect(mockPatch).toHaveBeenCalled();
    const alert = el.shadowRoot!.querySelector('.uk-alert-danger');
    expect(alert).toBeTruthy();
    expect(alert?.textContent).toContain('version_conflict');
    expect((el as any)._actionError).toBe('version_conflict');
    // A 409 re-pulls the current version (one extra GET) but never silently
    // succeeds — the conflict banner above proves it was not swallowed.
    expect(mockGet.mock.calls.length).toBe(getCallsBefore + 1);
  });

  it('renders the error alert with Retry when the list GET returns an error', async () => {
    (el as any).orgId = 'test-org';
    (el as any).client = {
      GET: vi.fn().mockResolvedValue({ data: null, error: { reason: 'boom' } }),
    };
    await settle();
    const alert = el.shadowRoot!.querySelector('.uk-alert-danger');
    expect(alert).toBeTruthy();
    expect(alert?.textContent).toContain('Failed to load flows');
    expect(alert?.textContent).toContain('boom');
    const retry = Array.from(alert!.querySelectorAll('button')).find((b) => b.textContent?.includes('Retry'));
    expect(retry).toBeTruthy();
  });
});
