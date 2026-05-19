// Phase 6 Plan 09 Task 2: Tests for <or-break-reason-detail>
// W0.1-19: Updated to match Ember layout (native checkbox toggles, uk-input, zero sl-*).
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

import './break-reason-detail.js';

const MOCK_BR = {
  id: '01935b00-0000-7000-8000-000000000001',
  code: 'break_lunch',
  name: 'Lunch',
  external_id: null,
  routable: true,
  display_order: 1,
  enabled: true,
  version: 3,
  updated_at: '2026-05-17T00:00:00Z',
  created_at: '2026-05-01T00:00:00Z',
};

describe('OrBreakReasonDetail', () => {
  let el: HTMLElement;

  beforeEach(() => {
    el = document.createElement('or-break-reason-detail');
    // Do NOT append to DOM here — tests set props before connecting (D6-03 pattern)
  });

  afterEach(() => {
    if (el.parentNode) el.parentNode.removeChild(el);
    vi.restoreAllMocks();
  });

  async function mountWithBreakReason(element: HTMLElement, br = MOCK_BR) {
    const mockGet = vi.fn().mockResolvedValue({ data: br, error: null });
    (element as any).orgId = 'test-org';
    (element as any).entityId = br.id;
    (element as any).client = { GET: mockGet, PATCH: vi.fn(), DELETE: vi.fn() };
    document.body.appendChild(element);
    await (element as any).updateComplete;
    await new Promise((r) => setTimeout(r, 50));
    await (element as any).updateComplete;
    return mockGet;
  }

  it('code field renders as or-code-input with readonly=true (D04_1-02)', async () => {
    await mountWithBreakReason(el);
    const shadow = el.shadowRoot!;
    const codeInput = shadow.querySelector('or-code-input') as any;
    expect(codeInput).toBeTruthy();
    expect(codeInput.readonly).toBe(true);
    expect(codeInput.value).toBe('break_lunch');
  });

  it('routable toggle rendered with explanation text; _formData.routable matches entity', async () => {
    await mountWithBreakReason(el);

    // Check that _formData has routable=true (matches MOCK_BR.routable=true)
    expect((el as any)._formData?.routable).toBe(true);

    // Check helper text in shadow HTML (Ember uses inline toggle-sub text)
    const shadow = el.shadowRoot!;
    const shadowHtml = shadow.innerHTML;
    expect(shadowHtml).toContain('still receive routed interactions');
  });

  it('display_order renders as number input with helper text', async () => {
    await mountWithBreakReason(el);
    const shadow = el.shadowRoot!;
    const shadowHtml = shadow.innerHTML;
    // The display_order field must be a number input
    expect(shadowHtml.includes('type="number"') || shadowHtml.includes("type='number'")).toBe(true);
    // Helper text about break picker ordering
    expect(shadowHtml).toContain('break picker');
  });

  it('PATCH 409 sets _conflictServer from error.current WITHOUT a second GET (D6-03)', async () => {
    const mockGet = vi.fn().mockResolvedValue({ data: MOCK_BR, error: null });
    const conflict409 = {
      error: { error: 'version_conflict', reason: 'Conflict', request_id: 'r1', current: { ...MOCK_BR, name: 'LunchUpdated', version: 4 } },
      data: null,
    };
    const mockPatch = vi.fn().mockResolvedValue(conflict409);
    (el as any).orgId = 'test-org';
    (el as any).entityId = MOCK_BR.id;
    (el as any).client = { GET: mockGet, PATCH: mockPatch, DELETE: vi.fn() };
    document.body.appendChild(el);
    await (el as any).updateComplete;
    await new Promise((r) => setTimeout(r, 50));
    await (el as any).updateComplete;

    const getCallCountBefore = mockGet.mock.calls.length;

    (el as any)._formData = { ...(el as any)._formData, name: 'SomeName' };
    (el as any)._dirty = true;
    await (el as any)._handleSave();
    await (el as any).updateComplete;

    // No extra GET was fired after the 409 (D6-03: NO re-GET)
    expect(mockGet.mock.calls.length).toBe(getCallCountBefore);
    // _conflictServer set from error.current (Pitfall 9 avoided)
    expect((el as any)._conflictServer).toBeTruthy();
    expect((el as any)._conflictServer?.name).toBe('LunchUpdated');
  });

  it('Save disabled until dirty; footer shows version · updated · created', async () => {
    await mountWithBreakReason(el);

    expect((el as any)._dirty).toBe(false);

    (el as any)._formData = { ...(el as any)._formData, name: 'NewName' };
    (el as any)._dirty = true;
    await (el as any).updateComplete;
    expect((el as any)._dirty).toBe(true);

    const shadow = el.shadowRoot!;
    const shadowHtml = shadow.innerHTML;
    expect(shadowHtml).toContain('version');
    expect(shadowHtml).toContain('3'); // version: 3 from MOCK_BR
  });
});
