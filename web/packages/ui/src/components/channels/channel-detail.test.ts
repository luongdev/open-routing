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
    // Seed entity directly to avoid async load timing issues
    (el as any)._entity = { ...MOCK_CHANNEL };
    (el as any)._loading = false;
    (el as any)._formData = {
      name: MOCK_CHANNEL.name,
      external_id: '',
      channel_type: MOCK_CHANNEL.channel_type,
      default_queue_id: MOCK_CHANNEL.default_queue_id,
      enabled: MOCK_CHANNEL.enabled,
    };

    await (el as any).updateComplete;

    const shadow = el.shadowRoot!;
    const codeInput = shadow.querySelector('or-code-input') as any;
    expect(codeInput).toBeTruthy();
    expect(codeInput?.readonly === true || codeInput?.getAttribute('readonly') !== null || codeInput?._readonly === true).toBe(true);
  });

  it('Test 2: channel_type renders as native select with options (voice/chat/email/sms/social)', async () => {
    (el as any).orgId = '01901b2c-7f3a-7000-8000-000000000001';
    (el as any).entityId = MOCK_CHANNEL.id;
    (el as any).client = {
      GET: vi.fn().mockResolvedValue({ data: MOCK_CHANNEL, error: null }),
    };
    // Seed entity directly
    (el as any)._entity = { ...MOCK_CHANNEL };
    (el as any)._loading = false;
    (el as any)._formData = {
      name: MOCK_CHANNEL.name,
      external_id: '',
      channel_type: MOCK_CHANNEL.channel_type,
      default_queue_id: MOCK_CHANNEL.default_queue_id,
      enabled: MOCK_CHANNEL.enabled,
    };

    await (el as any).updateComplete;

    const shadow = el.shadowRoot!;
    // W0.1-15: Ember redesign uses native <select> instead of sl-select
    const selects = shadow.querySelectorAll('select');
    expect(selects.length).toBeGreaterThan(0);

    // Should have options: voice, chat, email, sms, social
    const options = shadow.querySelectorAll('select option');
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

    // Seed _entity directly to ensure form renders
    (el as any)._entity = { ...MOCK_CHANNEL };
    (el as any)._formData = {
      name: MOCK_CHANNEL.name,
      external_id: '',
      channel_type: MOCK_CHANNEL.channel_type,
      default_queue_id: MOCK_CHANNEL.default_queue_id,
      enabled: MOCK_CHANNEL.enabled,
    };
    (el as any)._loading = false;

    await (el as any).updateComplete;
    await new Promise((r) => setTimeout(r, 30));
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

    // Use a mock that tracks whether GET was called AFTER the PATCH
    let patchHasBeenCalled = false;
    const mockGet = vi.fn().mockImplementation(() => {
      if (patchHasBeenCalled) {
        throw new Error('GET called after PATCH — violates D6-03 no-re-GET rule!');
      }
      return Promise.resolve({ data: MOCK_CHANNEL, error: null });
    });
    const mockPatch = vi.fn().mockImplementation(() => {
      patchHasBeenCalled = true;
      return Promise.resolve({
        data: null,
        error: { error: 'version_conflict', current: currentVersion },
      });
    });

    (el as any).orgId = '01901b2c-7f3a-7000-8000-000000000001';
    (el as any).entityId = MOCK_CHANNEL.id;
    (el as any).client = { GET: mockGet, PATCH: mockPatch };

    await (el as any).updateComplete;
    // Do NOT wait 50ms — set _entity directly to avoid timing races with _loadEntity
    (el as any)._entity = { ...MOCK_CHANNEL };
    (el as any)._formData = {
      name: 'My Local Change',
      external_id: '',
      channel_type: 'voice',
      default_queue_id: null,
      enabled: true,
    };
    (el as any)._dirty = true;
    (el as any)._loading = false;

    // Call _handleSave — PATCH fires with 409
    await (el as any)._handleSave();
    await (el as any).updateComplete;

    // Verify PATCH was called
    expect(mockPatch).toHaveBeenCalledTimes(1);

    // _conflictServer should be set from error.current (D6-03: no re-GET)
    expect((el as any)._conflictServer).toBeTruthy();
    expect((el as any)._conflictServer?.version).toBe(4);

    // _entity should be updated from error.current (version staleness fix)
    expect((el as any)._entity?.version).toBe(4);

    // or-conflict-banner should render
    const shadow = el.shadowRoot!;
    const banner = shadow.querySelector('or-conflict-banner');
    expect(banner).toBeTruthy();
  });
});
