// Phase 6 Plan 11 Task 2: Tests for <or-adapter-detail>
// TDD RED — these tests fail until adapter-detail.ts exists.
// Key behaviors: code readonly, JSONB config as monospace textarea,
// empty → null coercion, invalid JSON blocked, 409 via error.current (D6-03).
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

import './adapter-detail.js';

const MOCK_ADAPTER = {
  id: '01935c00-0000-7000-8000-000000000001',
  code: 'adapter_freeswitch_dc1',
  name: 'FreeSWITCH Bridge - DC1',
  external_id: 'MDM-ADAPTER-FS-DC1',
  adapter_type: 'freeswitch',
  config: { host: '10.0.0.1', port: 5060 },
  enabled: true,
  version: 5,
  updated_at: '2026-05-17T00:00:00Z',
  created_at: '2026-05-01T00:00:00Z',
};

describe('OrAdapterDetail', () => {
  let el: HTMLElement;

  beforeEach(() => {
    el = document.createElement('or-adapter-detail');
    // Do NOT append to DOM here — tests set props before connecting
  });

  afterEach(() => {
    if (el.parentNode) el.parentNode.removeChild(el);
    vi.restoreAllMocks();
  });

  async function mountWithAdapter(element: HTMLElement, adapter = MOCK_ADAPTER) {
    const mockGet = vi.fn().mockResolvedValue({ data: adapter, error: null });
    (element as any).orgId = 'test-org';
    (element as any).entityId = adapter.id;
    (element as any).client = { GET: mockGet, PATCH: vi.fn(), DELETE: vi.fn() };
    document.body.appendChild(element);
    await (element as any).updateComplete;
    await new Promise((r) => setTimeout(r, 50));
    await (element as any).updateComplete;
    return mockGet;
  }

  // Test 1: code field renders as or-code-input with readonly=true (D04_1-02)
  it('renders code field as readonly or-code-input (D04_1-02)', async () => {
    await mountWithAdapter(el);
    const shadow = el.shadowRoot!;
    const codeInput = shadow.querySelector('or-code-input') as any;
    expect(codeInput).toBeTruthy();
    expect(codeInput.readonly).toBe(true);
    expect(codeInput.value).toBe('adapter_freeswitch_dc1');
  });

  // Test 2: config renders as sl-textarea with monospace font; displays JSON.stringify(config, null, 2)
  it('renders config as sl-textarea with monospace font and pretty-printed JSON', async () => {
    await mountWithAdapter(el);
    const shadow = el.shadowRoot!;

    // Find sl-textarea for config
    const textareas = shadow.querySelectorAll('sl-textarea');
    let configTextarea: Element | null = null;
    for (const ta of textareas) {
      // Check by label or by value content (JSON)
      const label = ta.getAttribute('label') ?? '';
      if (label.toLowerCase().includes('config')) {
        configTextarea = ta;
        break;
      }
    }
    expect(configTextarea).toBeTruthy();

    // Should have monospace style
    const style = (configTextarea as HTMLElement).getAttribute('style') ?? '';
    expect(style).toContain('monospace');

    // Internal state should have pretty-printed JSON
    const configText = (el as any)._formData?.configText as string;
    expect(configText).toBe(JSON.stringify(MOCK_ADAPTER.config, null, 2));
  });

  // Test 3: config textarea value "" on save is sent as null
  it('sends config: null when config textarea is empty on save', async () => {
    const mockPatch = vi.fn().mockResolvedValue({
      data: { ...MOCK_ADAPTER, config: null, version: 6 },
      error: null,
    });
    (el as any).orgId = 'test-org';
    (el as any).entityId = MOCK_ADAPTER.id;
    (el as any).client = {
      GET: vi.fn().mockResolvedValue({ data: MOCK_ADAPTER, error: null }),
      PATCH: mockPatch,
      DELETE: vi.fn(),
    };
    document.body.appendChild(el);
    await (el as any).updateComplete;
    await new Promise((r) => setTimeout(r, 50));
    await (el as any).updateComplete;

    // Clear the config text
    (el as any)._formData = { ...(el as any)._formData, configText: '' };
    (el as any)._dirty = true;

    await (el as any)._handleSave();
    await (el as any).updateComplete;

    expect(mockPatch).toHaveBeenCalled();
    const patchBody = mockPatch.mock.calls[0]?.[1]?.body;
    expect(patchBody?.config).toBeNull();
  });

  // Test 4: invalid JSON in config textarea shows inline error and blocks save
  it('shows "Config must be valid JSON" error and blocks save on invalid JSON', async () => {
    await mountWithAdapter(el);

    // Set invalid JSON in configText
    (el as any)._formData = { ...(el as any)._formData, configText: '{invalid json}' };
    (el as any)._configError = 'Config must be valid JSON';
    (el as any)._dirty = true;
    await (el as any).updateComplete;

    // Error message should be in the DOM
    const shadow = el.shadowRoot!;
    const text = shadow.textContent ?? '';
    expect(text).toContain('Config must be valid JSON');

    // Save should not proceed when _configError is set
    const mockPatch = vi.fn();
    (el as any).client = { ...(el as any).client, PATCH: mockPatch };
    await (el as any)._handleSave();
    await (el as any).updateComplete;

    // PATCH should NOT be called because config is invalid
    expect(mockPatch).not.toHaveBeenCalled();
  });

  // Test 5: 409 PATCH uses error.current; does NOT fire a second GET (D6-03)
  it('PATCH 409 sets _conflictServer from error.current WITHOUT a second GET', async () => {
    const mockGet = vi.fn().mockResolvedValue({ data: MOCK_ADAPTER, error: null });
    const conflict409 = {
      error: {
        error: 'version_conflict',
        reason: 'Conflict',
        request_id: 'r1',
        current: {
          ...MOCK_ADAPTER,
          name: 'Updated Remotely',
          config: { host: '10.0.0.2', port: 5061 },
          version: 6,
        },
      },
      data: null,
    };
    const mockPatch = vi.fn().mockResolvedValue(conflict409);
    (el as any).orgId = 'test-org';
    (el as any).entityId = MOCK_ADAPTER.id;
    (el as any).client = { GET: mockGet, PATCH: mockPatch, DELETE: vi.fn() };
    document.body.appendChild(el);
    await (el as any).updateComplete;
    await new Promise((r) => setTimeout(r, 50));
    await (el as any).updateComplete;

    // Initial GET count
    const getCallCountBefore = mockGet.mock.calls.length;

    // Simulate save — should trigger 409
    (el as any)._formData = { ...(el as any)._formData, name: 'SomeName' };
    (el as any)._dirty = true;

    await (el as any)._handleSave();
    await (el as any).updateComplete;

    // No extra GET was fired after the 409
    expect(mockGet.mock.calls.length).toBe(getCallCountBefore);
    // _conflictServer set from error.current (D6-03)
    expect((el as any)._conflictServer).toBeTruthy();
    expect((el as any)._conflictServer?.name).toBe('Updated Remotely');
  });
});
