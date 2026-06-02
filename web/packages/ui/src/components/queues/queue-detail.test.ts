// Phase 6 Plan 08 Task 2: Tests for <or-queue-detail>
// TDD RED — these tests fail until queue-detail.ts exists.
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

import './queue-detail.js';

const MOCK_QUEUE = {
  id: '01935c00-0000-7000-8000-000000000001',
  code: 'queue_billing',
  name: 'Billing Queue',
  external_id: null,
  channel_types: ['voice', 'chat'],
  priority: 5,
  acw_sec: 60,
  enabled: true,
  version: 3,
  updated_at: '2026-05-17T00:00:00Z',
  created_at: '2026-05-01T00:00:00Z',
};

describe('OrQueueDetail', () => {
  let el: HTMLElement;

  beforeEach(() => {
    el = document.createElement('or-queue-detail');
    // Set properties BEFORE appending to DOM (Deviation 7 from Plan 06-05)
  });

  afterEach(() => {
    if (el.parentNode) el.parentNode.removeChild(el);
    vi.restoreAllMocks();
  });

  async function mountWithQueue(element: HTMLElement, queue = MOCK_QUEUE) {
    const mockGet = vi.fn().mockResolvedValue({ data: queue, error: null });
    (element as any).orgId = 'test-org';
    (element as any).entityId = queue.id;
    (element as any).client = { GET: mockGet, PATCH: vi.fn(), DELETE: vi.fn() };
    document.body.appendChild(element);
    await (element as any).updateComplete;
    await new Promise((r) => setTimeout(r, 50));
    await (element as any).updateComplete;
    return mockGet;
  }

  it('code field renders as or-code-input with readonly=true (D04_1-02)', async () => {
    await mountWithQueue(el);
    const shadow = el.shadowRoot!;
    const codeInput = shadow.querySelector('or-code-input') as any;
    expect(codeInput).toBeTruthy();
    expect(codeInput.readonly).toBe(true);
    expect(codeInput.value).toBe('queue_billing');
  });

  it('channel_types renders as pill checkboxes for voice/chat/email', async () => {
    await mountWithQueue(el);
    const shadow = el.shadowRoot!;
    const labels = Array.from(shadow.querySelectorAll('label, .pill-check, [data-channel-type]'))
      .map((el) => el.textContent?.toLowerCase() ?? '');
    const allText = labels.join(' ');
    expect(allText).toContain('voice');
    expect(allText).toContain('chat');
    expect(allText).toContain('email');
  });

  it('validateUpdateQueue rejects if channel_types array is empty (minItems=1)', async () => {
    await mountWithQueue(el);

    // Set channel_types to empty in form state
    (el as any)._formData = { ...(el as any)._formData, channel_types: [] };
    (el as any)._dirty = true;

    // Try to save — should not call PATCH
    const mockPatch = vi.fn().mockResolvedValue({ data: MOCK_QUEUE, error: null });
    (el as any).client = {
      GET: vi.fn().mockResolvedValue({ data: MOCK_QUEUE, error: null }),
      PATCH: mockPatch,
      DELETE: vi.fn(),
    };

    await (el as any)._handleSave();
    await (el as any).updateComplete;

    // PATCH should NOT have been called — validation should have blocked it
    expect(mockPatch).not.toHaveBeenCalled();
    // Field errors should include channel_types
    const fieldErrors = (el as any)._fieldErrors as Record<string, string>;
    expect(fieldErrors['channel_types']).toBeTruthy();
  });

  it('409 PATCH response: uses error.current (not response.json()); renders or-conflict-banner', async () => {
    await mountWithQueue(el);

    const conflictData = { ...MOCK_QUEUE, version: 4, name: 'Renamed by someone else' };
    const mockPatch = vi.fn().mockResolvedValue({
      data: null,
      error: {
        error: 'version_conflict',
        reason: 'Version conflict',
        current: conflictData,
      },
    });
    (el as any).client = {
      GET: vi.fn().mockResolvedValue({ data: MOCK_QUEUE, error: null }),
      PATCH: mockPatch,
      DELETE: vi.fn(),
    };

    // Make form dirty and save
    (el as any)._dirty = true;
    (el as any)._entity = MOCK_QUEUE;
    (el as any)._formData = {
      name: 'My change',
      external_id: '',
      channel_types: ['voice'],
      priority: 5,
      acw_sec: 60,
      enabled: true,
    };

    await (el as any)._handleSave();
    await (el as any).updateComplete;

    // _conflictServer should be set from error.current — NOT from a second GET
    expect((el as any)._conflictServer).toEqual(conflictData);
    // No second GET should have been made (only the initial load)
    const mockGet = (el as any).client.GET as ReturnType<typeof vi.fn>;
    expect(mockGet).not.toHaveBeenCalled(); // GET was from the mountWithQueue client

    // or-conflict-banner should be rendered
    const banner = el.shadowRoot!.querySelector('or-conflict-banner');
    expect(banner).toBeTruthy();
  });
});
