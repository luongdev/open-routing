// Phase 6 Plan 05 Task 1: Tests for <or-agent-list>
// TDD RED — these tests fail until agent-list.ts exists.
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

// Registers <or-agent-list> element
import './agent-list.js';

const MOCK_AGENT = {
  id: '01935b00-0000-7000-8000-000000000001',
  code: 'emp_0042',
  name: 'Alice',
  email: 'a@b.com',
  enabled: true,
  version: 1,
  updated_at: '2026-05-17T00:00:00Z',
  created_at: '2026-05-01T00:00:00Z',
};

describe('OrAgentList', () => {
  let el: HTMLElement;

  beforeEach(() => {
    el = document.createElement('or-agent-list');
    document.body.appendChild(el);
  });

  afterEach(() => {
    if (el.parentNode) el.parentNode.removeChild(el);
  });

  it('renders or-data-table with rows when client.GET resolves successfully', async () => {
    (el as any).orgId = 'test-org-id';
    (el as any).client = {
      GET: vi.fn().mockResolvedValue({
        data: { items: [MOCK_AGENT], has_more: false, next_cursor: null },
        error: null,
      }),
    };
    await (el as any).updateComplete;

    // Wait for the async Task to settle
    await new Promise((r) => setTimeout(r, 50));
    await (el as any).updateComplete;

    const shadow = el.shadowRoot!;
    // The component renders or-data-table; check it received the rows
    const dataTable = shadow.querySelector('or-data-table') as any;
    expect(dataTable).toBeTruthy();
    // Verify the rows property is set with the agent data
    expect(dataTable.rows).toHaveLength(1);
    expect(dataTable.rows[0]?.code).toBe('emp_0042');
  });

  it('shows loading state when task is pending (never-resolving promise)', async () => {
    (el as any).orgId = 'test-org-id';
    (el as any).client = {
      GET: vi.fn().mockReturnValue(new Promise(() => {})),
    };
    await (el as any).updateComplete;

    const shadow = el.shadowRoot!;
    // The data-table shows loading skeleton
    const loadingEl = shadow.querySelector('[data-testid="loading"], or-data-table, .loading-state');
    expect(loadingEl).toBeTruthy();
  });

  it('dispatches open-routing:navigate when row is clicked', async () => {
    (el as any).orgId = 'test-org-id';
    (el as any).client = {
      GET: vi.fn().mockResolvedValue({
        data: { items: [MOCK_AGENT], has_more: false, next_cursor: null },
        error: null,
      }),
    };
    await (el as any).updateComplete;
    await new Promise((r) => setTimeout(r, 50));
    await (el as any).updateComplete;

    const events: CustomEvent[] = [];
    el.addEventListener('open-routing:navigate', (e) => events.push(e as CustomEvent));

    // Simulate row click via the or-row-click event from or-data-table
    const shadow = el.shadowRoot!;
    const dataTable = shadow.querySelector('or-data-table');
    expect(dataTable).toBeTruthy();
    dataTable!.dispatchEvent(
      new CustomEvent('or-row-click', {
        detail: { row: MOCK_AGENT },
        bubbles: true,
        composed: true,
      })
    );
    await (el as any).updateComplete;

    expect(events).toHaveLength(1);
    expect(events[0]?.detail?.path).toContain(MOCK_AGENT.id);
  });

  it('calls client.GET with name query param after search input changes', async () => {
    const mockGet = vi.fn().mockResolvedValue({
      data: { items: [], has_more: false, next_cursor: null },
      error: null,
    });
    (el as any).orgId = 'test-org-id';
    (el as any).client = { GET: mockGet };
    await (el as any).updateComplete;
    await new Promise((r) => setTimeout(r, 50));

    // Reset call count after initial load
    mockGet.mockClear();

    // Directly set the search state (bypassing debounce for test speed)
    (el as any)._search = 'alice';
    (el as any).requestUpdate();
    await (el as any).updateComplete;
    await new Promise((r) => setTimeout(r, 50));
    await (el as any).updateComplete;

    expect(mockGet).toHaveBeenCalled();
    const lastCall = mockGet.mock.calls[mockGet.mock.calls.length - 1];
    const query = lastCall?.[1]?.params?.query;
    expect(query?.name).toBe('alice');
  });
});
