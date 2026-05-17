// Phase 6 Plan 07 Task 1: Tests for <or-skill-list>
// TDD RED — these tests fail until skill-list.ts exists.
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

// Registers <or-skill-list> element
import './skill-list.js';

const MOCK_SKILL = {
  id: '01935b00-0000-7000-8000-000000000010',
  code: 'skill_billing',
  name: 'Billing Support',
  skill_type: 'support',
  enabled: true,
  version: 1,
  updated_at: '2026-05-17T00:00:00Z',
  created_at: '2026-05-01T00:00:00Z',
};

const MOCK_SKILL_2 = {
  id: '01935b00-0000-7000-8000-000000000011',
  code: 'skill_refund',
  name: 'Refund Processing',
  skill_type: 'finance',
  enabled: false,
  version: 2,
  updated_at: '2026-05-16T00:00:00Z',
  created_at: '2026-04-20T00:00:00Z',
};

const MOCK_SKILL_3 = {
  id: '01935b00-0000-7000-8000-000000000012',
  code: 'skill_tech',
  name: 'Technical Support',
  skill_type: 'technical',
  enabled: true,
  version: 1,
  updated_at: '2026-05-15T00:00:00Z',
  created_at: '2026-04-10T00:00:00Z',
};

describe('OrSkillList', () => {
  let el: HTMLElement;

  beforeEach(() => {
    el = document.createElement('or-skill-list');
    document.body.appendChild(el);
  });

  afterEach(() => {
    if (el.parentNode) el.parentNode.removeChild(el);
    vi.restoreAllMocks();
  });

  it('renders "No skills yet" empty state when task resolves with items=[]', async () => {
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
    expect(text).toContain('No skills yet');
  });

  it('renders 3 rows when task resolves with 3 skills', async () => {
    (el as any).orgId = 'test-org-id';
    (el as any).client = {
      GET: vi.fn().mockResolvedValue({
        data: {
          items: [MOCK_SKILL, MOCK_SKILL_2, MOCK_SKILL_3],
          has_more: false,
          next_cursor: null,
        },
        error: null,
      }),
    };
    await (el as any).updateComplete;
    await new Promise((r) => setTimeout(r, 50));
    await (el as any).updateComplete;

    const shadow = el.shadowRoot!;
    const dataTable = shadow.querySelector('or-data-table') as any;
    expect(dataTable).toBeTruthy();
    expect(dataTable.rows).toHaveLength(3);
    expect(dataTable.rows[0]?.code).toBe('skill_billing');
  });

  it('search input change resets cursor to null after 300ms debounce', async () => {
    const mockGet = vi.fn().mockResolvedValue({
      data: { items: [], has_more: false, next_cursor: null },
      error: null,
    });
    (el as any).orgId = 'test-org-id';
    (el as any).client = { GET: mockGet };
    await (el as any).updateComplete;
    await new Promise((r) => setTimeout(r, 50));

    // Set a non-null cursor to verify it gets reset
    (el as any)._cursor = 'some-cursor';
    (el as any)._cursorStack = ['prev-cursor'];

    // Directly set the search state (simulating debounce resolution)
    (el as any)._search = 'billing';
    (el as any)._cursor = null;
    (el as any)._cursorStack = [];
    (el as any).requestUpdate();
    await (el as any).updateComplete;
    await new Promise((r) => setTimeout(r, 50));

    expect((el as any)._cursor).toBeNull();
    expect((el as any)._cursorStack).toHaveLength(0);
  });

  it('dispatches open-routing:navigate with /orgs/{orgId}/skills/{id} on row click', async () => {
    (el as any).orgId = 'test-org-id';
    (el as any).client = {
      GET: vi.fn().mockResolvedValue({
        data: { items: [MOCK_SKILL], has_more: false, next_cursor: null },
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
        detail: { row: MOCK_SKILL },
        bubbles: true,
        composed: true,
      })
    );
    await (el as any).updateComplete;

    expect(events).toHaveLength(1);
    expect(events[0]?.detail?.path).toContain('test-org-id');
    expect(events[0]?.detail?.path).toContain(MOCK_SKILL.id);
  });

  it('renders "No skills found matching..." when search active and items=[]', async () => {
    (el as any).orgId = 'test-org-id';
    (el as any).client = {
      GET: vi.fn().mockResolvedValue({
        data: { items: [], has_more: false, next_cursor: null },
        error: null,
      }),
    };
    // Set search before mounting to simulate active search
    (el as any)._search = 'xyz';
    await (el as any).updateComplete;
    await new Promise((r) => setTimeout(r, 50));
    await (el as any).updateComplete;

    const shadow = el.shadowRoot!;
    const text = shadow.textContent ?? '';
    expect(text).toContain('No skills found matching');
  });

  it('calls client.GET with name query param after search changes', async () => {
    const mockGet = vi.fn().mockResolvedValue({
      data: { items: [], has_more: false, next_cursor: null },
      error: null,
    });
    (el as any).orgId = 'test-org-id';
    (el as any).client = { GET: mockGet };
    await (el as any).updateComplete;
    await new Promise((r) => setTimeout(r, 50));

    mockGet.mockClear();

    (el as any)._search = 'billing';
    (el as any).requestUpdate();
    await (el as any).updateComplete;
    await new Promise((r) => setTimeout(r, 50));
    await (el as any).updateComplete;

    expect(mockGet).toHaveBeenCalled();
    const lastCall = mockGet.mock.calls[mockGet.mock.calls.length - 1];
    const query = lastCall?.[1]?.params?.query;
    expect(query?.name).toBe('billing');
  });
});
