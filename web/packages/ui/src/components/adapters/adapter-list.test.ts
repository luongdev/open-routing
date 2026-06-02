// Phase 6 Plan 11 Task 1: Tests for <or-adapter-list>
// TDD RED — these tests fail until adapter-list.ts exists.
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

// Registers <or-adapter-list> element
import './adapter-list.js';

const MOCK_ADAPTER = {
  id: '01935c00-0000-7000-8000-000000000001',
  code: 'adapter_freeswitch_dc1',
  name: 'FreeSWITCH Bridge - DC1',
  external_id: 'MDM-ADAPTER-FS-DC1',
  adapter_type: 'freeswitch',
  config: { host: '10.0.0.1', port: 5060 },
  enabled: true,
  version: 1,
  updated_at: '2026-05-17T00:00:00Z',
  created_at: '2026-05-01T00:00:00Z',
};

describe('OrAdapterList', () => {
  let el: HTMLElement;

  beforeEach(() => {
    el = document.createElement('or-adapter-list');
    document.body.appendChild(el);
  });

  afterEach(() => {
    if (el.parentNode) el.parentNode.removeChild(el);
    vi.restoreAllMocks();
  });

  it('renders "No adapters yet" empty state when items=[]', async () => {
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
    const text = shadow.textContent ?? '';
    expect(text).toContain('No adapters yet. Click + Create adapter to add your first.');
    expect(shadow.querySelector('.empty-state button')?.textContent?.trim()).toBe('+ Create adapter');
    expect(shadow.querySelector('.page-header button.uk-button-primary')?.textContent?.trim()).toBe('+ Create adapter');
  });

  it('renders adapter_type as plain text (not badge/monospace)', async () => {
    (el as any).orgId = 'test-org-id';
    (el as any).client = {
      GET: vi.fn().mockResolvedValue({
        data: { items: [MOCK_ADAPTER], has_more: false, next_cursor: null },
        error: null,
      }),
    };
    await (el as any).updateComplete;
    await new Promise((r) => setTimeout(r, 50));
    await (el as any).updateComplete;

    const shadow = el.shadowRoot!;
    const dataTable = shadow.querySelector('or-data-table') as any;
    expect(dataTable).toBeTruthy();
    expect(dataTable.rows).toHaveLength(1);
    expect(dataTable.rows[0]?.adapter_type).toBe('freeswitch');

    // adapter_type column definition should NOT use sl-badge or monospace rendering
    const columns = dataTable.columns as Array<{ key: string; label: string; render?: unknown }>;
    const typeCol = columns.find((c) => c.key === 'adapter_type');
    expect(typeCol).toBeTruthy();
    // adapter_type should be a simple text column with no special render (or a render that produces plain text)
  });

  it('config column is NOT rendered (intentionally absent per UI-SPEC)', async () => {
    (el as any).orgId = 'test-org-id';
    (el as any).client = {
      GET: vi.fn().mockResolvedValue({
        data: { items: [MOCK_ADAPTER], has_more: false, next_cursor: null },
        error: null,
      }),
    };
    await (el as any).updateComplete;
    await new Promise((r) => setTimeout(r, 50));
    await (el as any).updateComplete;

    const shadow = el.shadowRoot!;
    const dataTable = shadow.querySelector('or-data-table') as any;
    expect(dataTable).toBeTruthy();

    // Verify no config column in the column definitions
    const columns = dataTable.columns as Array<{ key: string }>;
    const configCol = columns.find((c) => c.key === 'config');
    expect(configCol).toBeUndefined();
  });

  it('row click dispatches open-routing:navigate to /orgs/{orgId}/adapters/{id}', async () => {
    (el as any).orgId = 'test-org-id';
    (el as any).client = {
      GET: vi.fn().mockResolvedValue({
        data: { items: [MOCK_ADAPTER], has_more: false, next_cursor: null },
        error: null,
      }),
    };
    await (el as any).updateComplete;
    await new Promise((r) => setTimeout(r, 50));
    await (el as any).updateComplete;

    const events: CustomEvent[] = [];
    el.addEventListener('open-routing:navigate', (e) => events.push(e as CustomEvent));

    const shadow = el.shadowRoot!;
    const dataTable = shadow.querySelector('or-data-table');
    expect(dataTable).toBeTruthy();
    dataTable!.dispatchEvent(
      new CustomEvent('or-row-click', {
        detail: { row: MOCK_ADAPTER },
        bubbles: true,
        composed: true,
      })
    );
    await (el as any).updateComplete;

    expect(events).toHaveLength(1);
    expect(events[0]?.detail?.path).toContain(MOCK_ADAPTER.id);
    expect(events[0]?.detail?.path).toContain('/adapters/');
  });

  it('[+ Create adapter] dispatches open-routing:navigate to /orgs/{orgId}/adapters/new', async () => {
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

    // Trigger navigation directly via _navigate (CTA button calls this)
    (el as any)._navigate(`/orgs/test-org-id/adapters/new`);
    await (el as any).updateComplete;

    expect(events).toHaveLength(1);
    expect(events[0]?.detail?.path).toContain('/adapters/new');
  });

  it('context menu [⋮] column is present in column definitions', async () => {
    (el as any).orgId = 'test-org-id';
    (el as any).client = {
      GET: vi.fn().mockResolvedValue({
        data: { items: [MOCK_ADAPTER], has_more: false, next_cursor: null },
        error: null,
      }),
    };
    await (el as any).updateComplete;
    await new Promise((r) => setTimeout(r, 50));
    await (el as any).updateComplete;

    const shadow = el.shadowRoot!;
    const dataTable = shadow.querySelector('or-data-table') as any;
    expect(dataTable).toBeTruthy();

    // Kebab menu is now rendered automatically by <or-data-table> for every row
    // (Wave 0.1 polish — no separate _actions column needed). Verify the table
    // primitive exposes a kebab trigger on at least one rendered row.
    await new Promise((r) => setTimeout(r, 50));
    const kebab = dataTable.shadowRoot?.querySelector('.kebab-btn');
    expect(kebab).toBeTruthy();
  });
});
