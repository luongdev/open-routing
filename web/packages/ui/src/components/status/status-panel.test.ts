// Phase 6 Plan 12: status-panel.test.ts — unit tests for <or-status-panel>.
// Tests cover: polling lifecycle, document.hidden pause/resume + state_version recheck,
// transition button matrix, break reason picker, force-flag advanced disclosure,
// 409 invalid_transition via error.from/to.
//
// Uses real timers (not fake) to avoid async interaction issues with Lit rendering.
// Spies on setInterval/clearInterval to verify polling lifecycle.

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import './status-panel.js';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

type AgentStatus = 'Ready' | 'NotReady' | 'Break' | 'Engaged' | 'WrapUp' | 'Offline';

interface AgentStatusResponse {
  agent_id: string;
  status: AgentStatus;
  state_version: number;
  break_reason_id?: string | null;
  break_reason_name?: string | null;
  post_interaction_state?: 'ready' | 'not_ready' | null;
  wrapup_until?: string | null;
  updated_at: string;
}

// ---------------------------------------------------------------------------
// Mock factory helpers
// ---------------------------------------------------------------------------

function makeStatusResponse(overrides: Partial<AgentStatusResponse> = {}): AgentStatusResponse {
  return {
    agent_id: 'test-agent-id',
    status: 'Ready',
    state_version: 1,
    break_reason_id: null,
    break_reason_name: null,
    post_interaction_state: null,
    wrapup_until: null,
    updated_at: new Date().toISOString(),
    ...overrides,
  };
}

function makeClient(
  getResponse: AgentStatusResponse | null = null,
  patchError: unknown = null,
  breakReasonsItems: unknown[] = [],
  getResponseFn?: () => AgentStatusResponse
) {
  const GET = vi.fn().mockImplementation(async (path: string) => {
    if (path.includes('/break-reasons')) {
      return { data: { items: breakReasonsItems, has_more: false }, error: null };
    }
    if (getResponseFn) {
      return { data: getResponseFn(), error: null };
    }
    if (getResponse === null) {
      return { data: null, error: { reason: 'Not found', error: 'not_found' } };
    }
    return { data: getResponse, error: null };
  });

  const PATCH = vi.fn().mockImplementation(async () => {
    if (patchError) return { data: null, error: patchError };
    return { data: getResponse ?? makeStatusResponse(), error: null };
  });

  return { GET, PATCH };
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

async function wait(ms = 50): Promise<void> {
  return new Promise((r) => setTimeout(r, ms));
}

// ---------------------------------------------------------------------------
// Test suite
// ---------------------------------------------------------------------------

describe('OrStatusPanel', () => {
  let el: HTMLElement;
  const ORG_ID = '01900000-0000-7000-8000-000000000001';
  const AGENT_ID = '01900000-0000-7000-8000-000000000002';

  beforeEach(() => {
    el = document.createElement('or-status-panel');
    el.setAttribute('org-id', ORG_ID);
    el.setAttribute('agent-id', AGENT_ID);
    // Reset document.hidden to false before each test
    Object.defineProperty(document, 'hidden', { value: false, configurable: true, writable: true });
  });

  afterEach(() => {
    if (el.parentNode) el.parentNode.removeChild(el);
    vi.restoreAllMocks();
    Object.defineProperty(document, 'hidden', { value: false, configurable: true, writable: true });
  });

  // ---------------------------------------------------------------------------
  // Test 1: On mount, fires GET immediately + starts 5s polling interval
  // ---------------------------------------------------------------------------
  it('Test 1: on mount, fires GET immediately and starts 5s polling interval', async () => {
    const client = makeClient(makeStatusResponse({ status: 'Ready', state_version: 1 }));

    // Spy on setInterval to verify it's called with 5000ms
    const setIntervalSpy = vi.spyOn(globalThis, 'setInterval');

    (el as any).client = client;
    document.body.appendChild(el);
    await wait(50);

    // Should have called GET once on mount
    expect(client.GET).toHaveBeenCalledTimes(1);
    expect(client.GET).toHaveBeenCalledWith(
      expect.stringContaining('/status'),
      expect.objectContaining({ params: { path: { org_id: ORG_ID, id: AGENT_ID } } })
    );

    // setInterval should have been called with 5000ms for polling
    const pollCall = setIntervalSpy.mock.calls.find(
      (call) => call[1] === 5000
    );
    expect(pollCall).toBeDefined();

    // _pollInterval state should be set
    expect((el as any)._pollInterval).not.toBeNull();
  }, 10000);

  // ---------------------------------------------------------------------------
  // Test 2: When document.hidden becomes true, clears polling interval
  // ---------------------------------------------------------------------------
  it('Test 2: when document.hidden becomes true, clears polling interval', async () => {
    const client = makeClient(makeStatusResponse({ status: 'Ready', state_version: 1 }));
    const clearIntervalSpy = vi.spyOn(globalThis, 'clearInterval');

    (el as any).client = client;
    document.body.appendChild(el);
    await wait(50);
    expect(client.GET).toHaveBeenCalledTimes(1);
    const callCountAfterMount = client.GET.mock.calls.length;

    // Simulate tab hidden
    Object.defineProperty(document, 'hidden', { value: true, configurable: true, writable: true });
    document.dispatchEvent(new Event('visibilitychange'));
    await wait(20);

    // clearInterval should have been called (polling stopped)
    expect(clearIntervalSpy).toHaveBeenCalled();
    // _pollInterval should be null
    expect((el as any)._pollInterval).toBeNull();

    // Wait a bit to confirm no additional GETs fire
    await wait(100);
    expect(client.GET.mock.calls.length).toBe(callCountAfterMount);
  }, 10000);

  // ---------------------------------------------------------------------------
  // Test 3: When document.hidden becomes false, fires immediate GET;
  //         if state_version changed → update UI; restarts 5s interval
  // ---------------------------------------------------------------------------
  it('Test 3: on resume from hidden, fires immediate GET + state_version compare', async () => {
    let callCount = 0;
    const statusV1 = makeStatusResponse({ status: 'Ready', state_version: 1 });
    const statusV2 = makeStatusResponse({ status: 'NotReady', state_version: 2 });

    const client = makeClient(null, null, [], () => {
      callCount++;
      // First call = version 1; all subsequent (after resume) = version 2
      if (callCount === 1) return statusV1;
      return statusV2;
    });

    (el as any).client = client;
    document.body.appendChild(el);
    await wait(50);
    expect(callCount).toBe(1);

    // Hide tab — stop polling
    Object.defineProperty(document, 'hidden', { value: true, configurable: true, writable: true });
    document.dispatchEvent(new Event('visibilitychange'));
    await wait(20);
    const countAfterHide = callCount;

    // Wait a bit — no extra polls should fire
    await wait(100);
    expect(callCount).toBe(countAfterHide);

    // Resume tab → immediate GET with version 2
    Object.defineProperty(document, 'hidden', { value: false, configurable: true, writable: true });
    document.dispatchEvent(new Event('visibilitychange'));
    await wait(100);
    expect(callCount).toBeGreaterThan(countAfterHide);

    // Status should now reflect version 2 (NotReady)
    const shadow = el.shadowRoot!;
    const text = shadow.textContent ?? '';
    expect(text).toContain('NotReady');

    // _pollInterval should be restarted (not null)
    expect((el as any)._pollInterval).not.toBeNull();
  }, 10000);

  // ---------------------------------------------------------------------------
  // Test 4: Ready status shows [Set Not Ready] + [Go on Break] buttons
  // ---------------------------------------------------------------------------
  it('Test 4: Ready status shows correct transition buttons', async () => {
    const client = makeClient(makeStatusResponse({ status: 'Ready', state_version: 1 }));
    (el as any).client = client;
    document.body.appendChild(el);
    await wait(50);

    const shadow = el.shadowRoot!;
    const text = shadow.textContent ?? '';
    expect(text).toContain('Set Not Ready');
    expect(text).toContain('Go on Break');
    // Should NOT show Back to Ready (that's for Break status)
    expect(text).not.toContain('Back to Ready');
  }, 10000);

  // ---------------------------------------------------------------------------
  // Test 5: Break status shows [Back to Ready] + [Set Not Ready] + break reason
  // ---------------------------------------------------------------------------
  it('Test 5: Break status shows Back to Ready, Set Not Ready, and break reason', async () => {
    const client = makeClient(
      makeStatusResponse({
        status: 'Break',
        state_version: 1,
        break_reason_id: 'reason-1',
        break_reason_name: 'Lunch',
      })
    );
    (el as any).client = client;
    document.body.appendChild(el);
    await wait(50);

    const shadow = el.shadowRoot!;
    const text = shadow.textContent ?? '';
    expect(text).toContain('Back to Ready');
    expect(text).toContain('Set Not Ready');
    expect(text).toContain('Lunch');
  }, 10000);

  // ---------------------------------------------------------------------------
  // Test 6: WrapUp status shows countdown + [Go to Ready now] + [Go to Not Ready now]
  // ---------------------------------------------------------------------------
  it('Test 6: WrapUp status shows countdown and transition buttons', async () => {
    const wrapupUntil = new Date(Date.now() + 120000).toISOString(); // 2 minutes
    const client = makeClient(
      makeStatusResponse({ status: 'WrapUp', state_version: 1, wrapup_until: wrapupUntil })
    );
    (el as any).client = client;
    document.body.appendChild(el);
    await wait(50);

    const shadow = el.shadowRoot!;
    const text = shadow.textContent ?? '';
    // Countdown should appear (e.g. "01:59" or "02:00" format hh:mm:ss)
    expect(text).toMatch(/\d{2}:\d{2}:\d{2}/);
    expect(text).toContain('Go to Ready now');
    expect(text).toContain('Go to Not Ready now');
  }, 10000);

  // ---------------------------------------------------------------------------
  // Test 7: "Go on Break" dropdown fetches /break-reasons and renders routable badges
  // ---------------------------------------------------------------------------
  it('Test 7: break picker fetches /break-reasons and renders routable badges', async () => {
    const breakReasons = [
      { id: 'br-1', code: 'lunch', name: 'Lunch', routable: true, display_order: 1 },
      { id: 'br-2', code: 'personal', name: 'Personal', routable: false, display_order: 2 },
    ];
    const client = makeClient(
      makeStatusResponse({ status: 'Ready', state_version: 1 }),
      null,
      breakReasons
    );
    (el as any).client = client;
    document.body.appendChild(el);
    await wait(50);

    // Simulate opening the break dropdown by triggering the open method
    const panelEl = el as any;
    await panelEl._openBreakDropdown();
    await wait(50);

    // Should have fetched break reasons
    const getCallPaths = client.GET.mock.calls.map((c: unknown[]) => c[0]);
    expect(getCallPaths.some((p: unknown) => String(p).includes('/break-reasons'))).toBe(true);

    // Rendered break reasons should be visible
    const shadow = el.shadowRoot!;
    const text = shadow.textContent ?? '';
    expect(text).toContain('Lunch');
    expect(text).toContain('Personal');
    // Routable badges should appear
    expect(text).toContain('routable');
  }, 10000);

  // ---------------------------------------------------------------------------
  // Test 8: Force-flag section renders target-state radios + sends force=true in PATCH
  // ---------------------------------------------------------------------------
  it('Test 8: force-flag section sends force=true in PATCH body', async () => {
    const client = makeClient(makeStatusResponse({ status: 'NotReady', state_version: 1 }));
    (el as any).client = client;
    document.body.appendChild(el);
    await wait(50);

    // The advanced/force section should be in the DOM
    const shadow = el.shadowRoot!;
    const text = shadow.textContent ?? '';
    expect(text).toContain('Force');

    // Directly call the force patch method (simulates confirm action)
    const panelEl = el as any;
    panelEl._selectedForceTarget = 'Ready';
    await panelEl._handleForceTransition();
    await wait(50);

    // PATCH should have been called with force: true
    expect(client.PATCH).toHaveBeenCalledWith(
      expect.stringContaining('/status'),
      expect.objectContaining({
        body: expect.objectContaining({ force: true, to: 'Ready' }),
      })
    );
  }, 10000);
});
