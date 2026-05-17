// Phase 6 Plan 07 Task 2: Tests for <or-skill-detail>
// TDD RED — these tests fail until skill-detail.ts exists.
// Key test: 409 uses error.current (NOT response.json()) per D6-03 / Pitfall 9.
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

import './skill-detail.js';

const MOCK_SKILL = {
  id: '01935b00-0000-7000-8000-000000000010',
  code: 'skill_billing',
  name: 'Billing Support',
  external_id: null,
  description: 'Handle billing inquiries.',
  skill_type: 'support',
  enabled: true,
  version: 5,
  updated_at: '2026-05-17T00:00:00Z',
  created_at: '2026-05-01T00:00:00Z',
};

describe('OrSkillDetail', () => {
  let el: HTMLElement;

  beforeEach(() => {
    el = document.createElement('or-skill-detail');
    // Do NOT append to DOM here — tests set props before connecting
  });

  afterEach(() => {
    if (el.parentNode) el.parentNode.removeChild(el);
    vi.restoreAllMocks();
  });

  async function mountWithSkill(element: HTMLElement, skill = MOCK_SKILL) {
    const mockGet = vi.fn().mockResolvedValue({ data: skill, error: null });
    (element as any).orgId = 'test-org';
    (element as any).entityId = skill.id;
    (element as any).client = { GET: mockGet, PATCH: vi.fn(), DELETE: vi.fn() };
    document.body.appendChild(element);
    await (element as any).updateComplete;
    await new Promise((r) => setTimeout(r, 50));
    await (element as any).updateComplete;
    return mockGet;
  }

  it('renders code field as or-code-input with readonly=true always (D04_1-02)', async () => {
    await mountWithSkill(el);
    const shadow = el.shadowRoot!;
    const codeInput = shadow.querySelector('or-code-input') as any;
    expect(codeInput).toBeTruthy();
    expect(codeInput.readonly).toBe(true);
    expect(codeInput.value).toBe('skill_billing');
  });

  it('Save button disabled until form is dirty; enabled after user edits any field', async () => {
    await mountWithSkill(el);

    // Initially not dirty
    expect((el as any)._dirty).toBe(false);

    // Simulate a field change
    (el as any)._form = { ...(el as any)._form, name: 'Updated Name' };
    (el as any)._dirty = true;
    await (el as any).updateComplete;

    expect((el as any)._dirty).toBe(true);
  });

  it('409 response uses error.current (NOT response.json()) and renders _conflictServer without re-GET', async () => {
    const mockGet = vi.fn().mockResolvedValue({ data: MOCK_SKILL, error: null });
    const conflict409 = {
      error: {
        error: 'version_conflict',
        reason: 'Conflict',
        request_id: 'r1',
        current: { ...MOCK_SKILL, name: 'Server Updated Name', version: 6 },
      },
      data: null,
    };
    const mockPatch = vi.fn().mockResolvedValue(conflict409);
    (el as any).orgId = 'test-org';
    (el as any).entityId = MOCK_SKILL.id;
    (el as any).client = { GET: mockGet, PATCH: mockPatch, DELETE: vi.fn() };
    document.body.appendChild(el);
    await (el as any).updateComplete;
    await new Promise((r) => setTimeout(r, 50));
    await (el as any).updateComplete;

    const getCallCountBefore = mockGet.mock.calls.length;

    // Trigger save with dirty form
    (el as any)._form = { ...(el as any)._form, name: 'My Edit' };
    (el as any)._dirty = true;
    await (el as any)._handleSave();
    await (el as any).updateComplete;

    // No extra GET fired after the 409 (D6-03: NO re-GET)
    expect(mockGet.mock.calls.length).toBe(getCallCountBefore);
    // _conflictServer set from error.current (Pitfall 9 pattern)
    expect((el as any)._conflictServer).toBeTruthy();
    expect((el as any)._conflictServer?.name).toBe('Server Updated Name');
  });

  it('Cancel with dirty form opens sl-dialog confirm', async () => {
    await mountWithSkill(el);

    // Simulate dirty form
    (el as any)._dirty = true;
    (el as any)._form = { ...(el as any)._form, name: 'Changed' };
    await (el as any).updateComplete;

    // Trigger cancel — should open dialog, not navigate immediately
    (el as any)._handleCancel();
    await (el as any).updateComplete;

    expect((el as any)._discardDialogOpen).toBe(true);
  });

  it('successful PATCH shows _showSavedToast and updates form state from response body', async () => {
    const updatedSkill = { ...MOCK_SKILL, name: 'Updated Name', version: 6 };
    const mockPatch = vi.fn().mockResolvedValue({ data: updatedSkill, error: null });
    (el as any).orgId = 'test-org';
    (el as any).entityId = MOCK_SKILL.id;
    (el as any).client = {
      GET: vi.fn().mockResolvedValue({ data: MOCK_SKILL, error: null }),
      PATCH: mockPatch,
      DELETE: vi.fn(),
    };
    document.body.appendChild(el);
    await (el as any).updateComplete;
    await new Promise((r) => setTimeout(r, 50));
    await (el as any).updateComplete;

    // Simulate dirty form
    (el as any)._form = { ...(el as any)._form, name: 'Updated Name' };
    (el as any)._dirty = true;

    await (el as any)._handleSave();
    await (el as any).updateComplete;

    expect((el as any)._entity?.name).toBe('Updated Name');
    expect((el as any)._dirty).toBe(false);
    expect((el as any)._showSavedToast).toBe(true);
    expect(mockPatch).toHaveBeenCalled();
  });
});
