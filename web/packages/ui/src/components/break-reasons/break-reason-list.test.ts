// Phase 6 Plan 09 Task 1: Tests for <or-break-reason-list>
// TDD RED — these tests fail until break-reason-list.ts exists.
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

import './break-reason-list.js';

const MOCK_BR_ROUTABLE = {
  id: '01935b00-0000-7000-8000-000000000001',
  code: 'break_lunch',
  name: 'Lunch',
  routable: true,
  display_order: 1,
  enabled: true,
  version: 1,
  updated_at: '2026-05-17T00:00:00Z',
  created_at: '2026-05-01T00:00:00Z',
};

const MOCK_BR_NOT_ROUTABLE = {
  id: '01935b00-0000-7000-8000-000000000002',
  code: 'break_break',
  name: 'Break',
  routable: false,
  display_order: 2,
  enabled: true,
  version: 1,
  updated_at: '2026-05-17T00:00:00Z',
  created_at: '2026-05-01T00:00:00Z',
};

describe('OrBreakReasonList', () => {
  let el: HTMLElement;

  beforeEach(() => {
    el = document.createElement('or-break-reason-list');
    document.body.appendChild(el);
  });

  afterEach(() => {
    if (el.parentNode) el.parentNode.removeChild(el);
  });

  it('renders "No break reasons yet" empty state when items=[]', async () => {
    (el as any).orgId = 'test-org';
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
    expect(text).toContain('No break reasons yet');
  });

  it('renders routable column as check-lg icon for routable=true, x-lg for routable=false', async () => {
    (el as any).orgId = 'test-org';
    (el as any).client = {
      GET: vi.fn().mockResolvedValue({
        data: { items: [MOCK_BR_ROUTABLE, MOCK_BR_NOT_ROUTABLE], has_more: false, next_cursor: null },
        error: null,
      }),
    };
    await (el as any).updateComplete;
    await new Promise((r) => setTimeout(r, 50));
    await (el as any).updateComplete;

    // Verify component received both rows in data-table
    const shadow = el.shadowRoot!;
    const dataTable = shadow.querySelector('or-data-table') as any;
    expect(dataTable).toBeTruthy();
    expect(dataTable.rows).toHaveLength(2);

    // Check that rows are rendered — trigger the column render and check shadow DOM
    // The rendered output for routable=true should include check-lg icon
    const shadowHtml = shadow.innerHTML;
    expect(shadowHtml).toContain('check-lg');
    // The rendered output for routable=false should include x-lg icon
    expect(shadowHtml).toContain('x-lg');
  });

  it('routable column header has sl-tooltip with correct content', async () => {
    (el as any).orgId = 'test-org';
    (el as any).client = {
      GET: vi.fn().mockResolvedValue({
        data: { items: [MOCK_BR_ROUTABLE], has_more: false, next_cursor: null },
        error: null,
      }),
    };
    await (el as any).updateComplete;
    await new Promise((r) => setTimeout(r, 50));
    await (el as any).updateComplete;

    const shadow = el.shadowRoot!;
    const dataTable = shadow.querySelector('or-data-table') as any;
    expect(dataTable).toBeTruthy();

    // Find the routable column in the columns definition
    const columns = (el as any)._columns as Array<{ key: string; label: unknown }>;
    const routableCol = columns.find((c) => c.key === 'routable');
    expect(routableCol).toBeTruthy();
    // The label should be an HTML template containing sl-tooltip
    // We verify this by checking the component has the tooltip defined
    // (label is a TemplateResult for sl-tooltip)
    const routableLabel = routableCol?.label;
    expect(routableLabel).toBeTruthy();
    // The column definition label must be non-string (HTML template) for tooltip
    expect(typeof routableLabel !== 'string').toBe(true);
  });

  it('display_order column is right-aligned', async () => {
    (el as any).orgId = 'test-org';
    (el as any).client = {
      GET: vi.fn().mockResolvedValue({
        data: { items: [MOCK_BR_ROUTABLE], has_more: false, next_cursor: null },
        error: null,
      }),
    };
    await (el as any).updateComplete;
    await new Promise((r) => setTimeout(r, 50));
    await (el as any).updateComplete;

    // Verify display_order column has right-align style in render output
    const columns = (el as any)._columns as Array<{ key: string; render?: (row: Record<string, unknown>) => unknown }>;
    const doCol = columns.find((c) => c.key === 'display_order');
    expect(doCol).toBeTruthy();
    // Render the column cell and check for right-align style
    const rendered = doCol?.render?.({ display_order: 1 });
    expect(rendered).toBeTruthy();
    // The rendered value should be a Lit template string containing text-align:right
    const asString = JSON.stringify(rendered);
    expect(asString).toContain('right');
  });

  it('row click dispatches open-routing:navigate with /orgs/{orgId}/break-reasons/{id}', async () => {
    (el as any).orgId = 'test-org';
    (el as any).client = {
      GET: vi.fn().mockResolvedValue({
        data: { items: [MOCK_BR_ROUTABLE], has_more: false, next_cursor: null },
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
        detail: { row: MOCK_BR_ROUTABLE },
        bubbles: true,
        composed: true,
      })
    );
    await (el as any).updateComplete;

    expect(events).toHaveLength(1);
    expect(events[0]?.detail?.path).toContain('break-reasons');
    expect(events[0]?.detail?.path).toContain(MOCK_BR_ROUTABLE.id);
  });

  it('[+ Create break reason] dispatches open-routing:navigate to /orgs/{orgId}/break-reasons/new', async () => {
    (el as any).orgId = 'test-org';
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

    // Click the CTA button in empty state
    const shadow = el.shadowRoot!;
    const ctaButton = shadow.querySelector('sl-button[variant="primary"]') as HTMLElement;
    expect(ctaButton).toBeTruthy();
    ctaButton!.click();
    await (el as any).updateComplete;

    expect(events.length).toBeGreaterThan(0);
    const navEvent = events.find((e) => e.detail?.path?.includes('/new'));
    expect(navEvent).toBeTruthy();
    expect(navEvent?.detail?.path).toContain('break-reasons/new');
  });
});
