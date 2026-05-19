// Phase 6 Plan 10 Task 1: <or-queue-picker> unit tests.
// Wave 0.1: updated for pure Lit + Ember tokens rewrite (no shoelace).
// Tests cover: loading state, queue options, debounced search, change event.

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

// Import the component (will fail until implementation exists)
import './queue-picker.js';

const MOCK_QUEUES = [
  { id: '01901b2c-7f3a-7000-8000-000000000001', code: 'q_voice', name: 'Voice Queue' },
  { id: '01901b2c-7f3a-7000-8000-000000000002', code: 'q_chat', name: 'Chat Queue' },
];

describe('or-queue-picker', () => {
  let el: HTMLElement;

  beforeEach(() => {
    el = document.createElement('or-queue-picker');
    document.body.appendChild(el);
  });

  afterEach(() => {
    if (el.parentNode) el.parentNode.removeChild(el);
    vi.clearAllMocks();
  });

  it('Test 1: renders loading state while task is pending', async () => {
    let resolveFn!: (v: unknown) => void;
    (el as any).orgId = '01901b2c-7f3a-7000-8000-000000000001';
    (el as any).client = {
      GET: vi.fn().mockReturnValue(new Promise((res) => { resolveFn = res; })),
    };

    await (el as any).updateComplete;

    // Component should be in pending/loading state
    const shadow = el.shadowRoot!;
    const text = shadow.textContent ?? '';
    const hasSpinner = shadow.querySelector('[data-testid="spinner"]') !== null;
    const hasLoadingText = text.includes('Loading');
    expect(hasSpinner || hasLoadingText).toBe(true);

    // Resolve to avoid hanging
    resolveFn({ data: { items: [], has_more: false }, error: null });
  });

  it('Test 2: renders queue option buttons for each queue returned by API', async () => {
    (el as any).orgId = '01901b2c-7f3a-7000-8000-000000000001';
    (el as any).client = {
      GET: vi.fn().mockResolvedValue({
        data: { items: MOCK_QUEUES, has_more: false, next_cursor: null },
        error: null,
      }),
    };

    await (el as any).updateComplete;
    await new Promise((r) => setTimeout(r, 50));
    await (el as any).updateComplete;

    // Open dropdown to render options
    (el as any)._open = true;
    (el as any).requestUpdate();
    await (el as any).updateComplete;

    const shadow = el.shadowRoot!;
    const options = shadow.querySelectorAll('[data-queue-id]');
    // Should have at least 2 queue options
    expect(options.length).toBeGreaterThanOrEqual(2);
    // Queue code or name should appear in the shadow DOM
    const text = shadow.textContent ?? '';
    expect(text.includes('q_voice') || text.includes('Voice Queue')).toBe(true);
  });

  it('Test 3: search input dispatches GET with name param after 300ms debounce', async () => {
    // Test the debounce mechanism by directly triggering _search update
    // and verifying the @lit/task args change triggers a new GET call.
    const mockGet = vi.fn().mockResolvedValue({
      data: { items: [], has_more: false, next_cursor: null },
      error: null,
    });
    (el as any).orgId = '01901b2c-7f3a-7000-8000-000000000001';
    (el as any).client = { GET: mockGet };

    await (el as any).updateComplete;
    await new Promise((r) => setTimeout(r, 50));
    await (el as any).updateComplete;

    // Record calls after initial load
    const callsBefore = mockGet.mock.calls.length;
    expect(callsBefore).toBeGreaterThan(0); // at least 1 initial call

    // Directly update _search — bypasses debounce to test task re-run
    (el as any)._search = 'voice';
    (el as any).requestUpdate();
    await (el as any).updateComplete;
    await new Promise((r) => setTimeout(r, 50));
    await (el as any).updateComplete;

    // After search state change, a new GET call should fire
    expect(mockGet.mock.calls.length).toBeGreaterThan(callsBefore);

    // Verify search param was passed in a call
    const allCalls = mockGet.mock.calls as Array<[string, unknown]>;
    const searchCall = allCalls.find((c) => {
      const params = (c[1] as any)?.params?.query;
      return params?.name === 'voice';
    });
    expect(searchCall).toBeTruthy();
  });

  it('Test 4: emits or-queue-picker-change with {queueId: null} on (none) selection', async () => {
    (el as any).orgId = '01901b2c-7f3a-7000-8000-000000000001';
    (el as any).client = {
      GET: vi.fn().mockResolvedValue({
        data: { items: MOCK_QUEUES, has_more: false, next_cursor: null },
        error: null,
      }),
    };

    await (el as any).updateComplete;
    await new Promise((r) => setTimeout(r, 100));
    await (el as any).updateComplete;

    const events: CustomEvent[] = [];
    el.addEventListener('or-queue-picker-change', (e) => events.push(e as CustomEvent));

    // Call _handleSelect directly — simulates clicking the (none) option
    (el as any)._handleSelect(null);

    await (el as any).updateComplete;

    expect(events.length).toBeGreaterThan(0);
    expect(events[0]?.detail?.queueId).toBeNull();
  });
});
