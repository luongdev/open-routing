// Phase 6 Plan 10 Task 1: <or-queue-picker> unit tests.
// TDD RED phase: all tests fail before implementation.
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
    const hasSpinner = shadow.querySelector('sl-spinner') !== null;
    const hasLoadingText = text.includes('Loading');
    expect(hasSpinner || hasLoadingText).toBe(true);

    // Resolve to avoid hanging
    resolveFn({ data: { items: [], has_more: false }, error: null });
  });

  it('Test 2: renders sl-option elements for each queue returned by API', async () => {
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

    const shadow = el.shadowRoot!;
    const options = shadow.querySelectorAll('sl-option');
    // Should have at least 2 queue options (+ "(none)" option)
    expect(options.length).toBeGreaterThanOrEqual(2);
    // Queue code or name should appear in the shadow DOM
    const text = shadow.textContent ?? '';
    expect(text.includes('q_voice') || text.includes('Voice Queue')).toBe(true);
  });

  it('Test 3: search input dispatches GET with name param after 300ms debounce', async () => {
    vi.useFakeTimers();

    (el as any).orgId = '01901b2c-7f3a-7000-8000-000000000001';
    const mockGet = vi.fn().mockResolvedValue({
      data: { items: [], has_more: false, next_cursor: null },
      error: null,
    });
    (el as any).client = { GET: mockGet };

    await (el as any).updateComplete;

    // Record baseline call count (initial load)
    const callsBefore = mockGet.mock.calls.length;

    // Simulate user typing in search input
    const shadow = el.shadowRoot!;
    const searchInput = shadow.querySelector('sl-input') as any;
    if (searchInput) {
      searchInput.value = 'voice';
      searchInput.dispatchEvent(new CustomEvent('sl-input', { bubbles: true, composed: true }));
    }

    // Before 300ms — no new call
    vi.advanceTimersByTime(200);
    await (el as any).updateComplete;
    expect(mockGet.mock.calls.length).toBe(callsBefore);

    // After 300ms debounce — new call fires
    vi.advanceTimersByTime(150);
    await (el as any).updateComplete;
    expect(mockGet.mock.calls.length).toBeGreaterThan(callsBefore);

    vi.useRealTimers();
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
    await new Promise((r) => setTimeout(r, 50));
    await (el as any).updateComplete;

    const events: CustomEvent[] = [];
    el.addEventListener('or-queue-picker-change', (e) => events.push(e as CustomEvent));

    // Simulate selecting "(none)" — value=''
    const shadow = el.shadowRoot!;
    const select = shadow.querySelector('sl-select') as any;
    if (select) {
      select.value = '';
      select.dispatchEvent(new CustomEvent('sl-change', { bubbles: true, composed: true }));
    }

    await (el as any).updateComplete;

    expect(events.length).toBeGreaterThan(0);
    expect(events[0]?.detail?.queueId).toBeNull();
  });
});
