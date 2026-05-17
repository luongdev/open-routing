// Phase 6 Plan 10 Task 2: <or-channel-detail> unit tests.
// TDD RED phase: tests fail before implementation.
// Tests: code readonly, channel_type sl-select, or-queue-picker bound, 409 via error.current.

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

// Import the component (will fail until implementation exists)
import './channel-detail.js';

const MOCK_CHANNEL = {
  id: '01901b2c-7f3a-7000-8000-000000000010',
  org_id: '01901b2c-7f3a-7000-8000-000000000001',
  code: 'ch_voice',
  name: 'Voice Channel',
  external_id: null,
  channel_type: 'voice',
  default_queue_id: '01901b2c-7f3a-7abc-8d4e-5f6a7b8c9d0e',
  enabled: true,
  version: 3,
  created_at: '2026-05-01T00:00:00Z',
  updated_at: '2026-05-17T00:00:00Z',
};

describe('or-channel-detail', () => {
  let el: HTMLElement;

  beforeEach(() => {
    el = document.createElement('or-channel-detail');
    document.body.appendChild(el);
  });

  afterEach(() => {
    if (el.parentNode) el.parentNode.removeChild(el);
    vi.clearAllMocks();
  });

  it('Test 1: code field renders as or-code-input with readonly=true (D04_1-02)', async () => {
    (el as any).orgId = '01901b2c-7f3a-7000-8000-000000000001';
    (el as any).entityId = MOCK_CHANNEL.id;
    (el as any).client = {
      GET: vi.fn().mockResolvedValue({ data: MOCK_CHANNEL, error: null }),
    };

    await (el as any).updateComplete;
    await new Promise((r) => setTimeout(r, 50));
    await (el as any).updateComplete;

    const shadow = el.shadowRoot!;
    const codeInput = shadow.querySelector('or-code-input') as any;
    expect(codeInput).toBeTruthy();
    expect(codeInput?.readonly === true || codeInput?.getAttribute('readonly') !== null || codeInput?._readonly === true).toBe(true);
  });

  it('Test 2: channel_type renders as sl-select with 3 sl-option elements (voice/chat/email)', async () => {
    (el as any).orgId = '01901b2c-7f3a-7000-8000-000000000001';
    (el as any).entityId = MOCK_CHANNEL.id;
    (el as any).client = {
      GET: vi.fn().mockResolvedValue({ data: MOCK_CHANNEL, error: null }),
    };

    await (el as any).updateComplete;
    await new Promise((r) => setTimeout(r, 50));
    await (el as any).updateComplete;

    const shadow = el.shadowRoot!;
    const selects = shadow.querySelectorAll('sl-select');
    // At least one sl-select for channel_type
    expect(selects.length).toBeGreaterThan(0);

    // Should have 3 options: voice, chat, email
    const options = shadow.querySelectorAll('sl-option');
    const optionValues = Array.from(options).map((o) => o.getAttribute('value'));
    expect(optionValues.includes('voice')).toBe(true);
    expect(optionValues.includes('chat')).toBe(true);
    expect(optionValues.includes('email')).toBe(true);
  });

  it('Test 3: default_queue_id renders as or-queue-picker; clearing sets null in form data', async () => {
    (el as any).orgId = '01901b2c-7f3a-7000-8000-000000000001';
    (el as any).entityId = MOCK_CHANNEL.id;
    (el as any).client = {
      GET: vi.fn().mockResolvedValue({ data: MOCK_CHANNEL, error: null }),
    };

    await (el as any).updateComplete;
    await new Promise((r) => setTimeout(r, 50));
    await (el as any).updateComplete;

    const shadow = el.shadowRoot!;
    const queuePicker = shadow.querySelector('or-queue-picker');
    expect(queuePicker).toBeTruthy();

    // Simulate or-queue-picker-change with null (cleared)
    queuePicker?.dispatchEvent(new CustomEvent('or-queue-picker-change', {
      detail: { queueId: null },
      bubbles: true,
      composed: true,
    }));

    await (el as any).updateComplete;

    // formData.default_queue_id should be null
    expect((el as any)._formData?.default_queue_id).toBeNull();
  });

  it('Test 4: 409 PATCH response uses error.current without re-GET; renders or-conflict-banner', async () => {
    const currentVersion = { ...MOCK_CHANNEL, version: 4, name: 'Updated Elsewhere' };
    const mockGet = vi.fn().mockResolvedValue({ data: MOCK_CHANNEL, error: null });
    const mockPatch = vi.fn().mockResolvedValue({
      data: null,
      error: { error: 'version_conflict', current: currentVersion },
    });

    (el as any).orgId = '01901b2c-7f3a-7000-8000-000000000001';
    (el as any).entityId = MOCK_CHANNEL.id;
    (el as any).client = { GET: mockGet, PATCH: mockPatch };

    await (el as any).updateComplete;
    await new Promise((r) => setTimeout(r, 50));
    await (el as any).updateComplete;

    // Make form dirty and attempt save
    (el as any)._dirty = true;
    (el as any)._formData = { ...(el as any)._formData, name: 'My Local Change' };
    await (el as any)._handleSave();
    await (el as any).updateComplete;

    // Should NOT have called GET again (D6-03: no re-GET)
    expect(mockGet.mock.calls.length).toBe(1);

    // _conflictServer should be set from error.current
    expect((el as any)._conflictServer).toBeTruthy();
    expect((el as any)._conflictServer?.version).toBe(4);

    // or-conflict-banner should render
    const shadow = el.shadowRoot!;
    const banner = shadow.querySelector('or-conflict-banner');
    expect(banner).toBeTruthy();
  });
});
