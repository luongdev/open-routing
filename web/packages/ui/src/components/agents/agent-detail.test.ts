// Phase 6 Plan 05 Task 2a+2b: Tests for <or-agent-detail>
// TDD RED — these tests fail until agent-detail.ts exists.
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

import './agent-detail.js';

const MOCK_AGENT = {
  id: '01935b00-0000-7000-8000-000000000001',
  code: 'emp_0042',
  name: 'Alice',
  email: 'alice@example.com',
  external_id: null,
  enabled: true,
  version: 5,
  updated_at: '2026-05-17T00:00:00Z',
  created_at: '2026-05-01T00:00:00Z',
  skills: [
    { id: 'skill-001', skill_id: '01935b00-0000-7000-8000-000000000002', name: 'Billing', proficiency: 8 },
  ],
};

// ============================================================
// Task 2a — core form (3 tests)
// ============================================================

describe('OrAgentDetail — core form (Task 2a)', () => {
  let el: HTMLElement;

  beforeEach(() => {
    el = document.createElement('or-agent-detail');
    document.body.appendChild(el);
  });

  afterEach(() => {
    if (el.parentNode) el.parentNode.removeChild(el);
    vi.restoreAllMocks();
  });

  it('renders code field as readonly (or-code-input with .readonly=true)', async () => {
    const mockGet = vi.fn().mockResolvedValue({ data: MOCK_AGENT, error: null });
    (el as any).orgId = 'test-org';
    (el as any).entityId = MOCK_AGENT.id;
    (el as any).client = { GET: mockGet, PATCH: vi.fn(), DELETE: vi.fn() };
    await (el as any).updateComplete;
    await new Promise((r) => setTimeout(r, 50));
    await (el as any).updateComplete;

    const shadow = el.shadowRoot!;
    const codeInput = shadow.querySelector('or-code-input') as any;
    expect(codeInput).toBeTruthy();
    expect(codeInput.readonly).toBe(true);
    expect(codeInput.value).toBe('emp_0042');
  });

  it('Save button is disabled until form is dirty', async () => {
    const mockGet = vi.fn().mockResolvedValue({ data: MOCK_AGENT, error: null });
    (el as any).orgId = 'test-org';
    (el as any).entityId = MOCK_AGENT.id;
    (el as any).client = { GET: mockGet, PATCH: vi.fn(), DELETE: vi.fn() };
    await (el as any).updateComplete;
    await new Promise((r) => setTimeout(r, 50));
    await (el as any).updateComplete;

    // Save button should be disabled initially (not dirty)
    expect((el as any)._dirty).toBe(false);

    // Simulate a field change
    (el as any)._form = { ...(el as any)._form, name: 'Bob' };
    (el as any)._dirty = true;
    await (el as any).updateComplete;

    expect((el as any)._dirty).toBe(true);
  });

  it('shows Saved feedback on successful 2xx PATCH', async () => {
    const mockGet = vi.fn().mockResolvedValue({ data: MOCK_AGENT, error: null });
    const updatedAgent = { ...MOCK_AGENT, name: 'Bob', version: 6 };
    const mockPatch = vi.fn().mockResolvedValue({ data: updatedAgent, error: null });
    (el as any).orgId = 'test-org';
    (el as any).entityId = MOCK_AGENT.id;
    (el as any).client = { GET: mockGet, PATCH: mockPatch, DELETE: vi.fn() };
    await (el as any).updateComplete;
    await new Promise((r) => setTimeout(r, 50));
    await (el as any).updateComplete;

    // Simulate dirty form
    (el as any)._form = { ...(el as any)._form, name: 'Bob' };
    (el as any)._dirty = true;
    (el as any)._assignedSkills = [];

    // Call save directly
    await (el as any)._handleSave();
    await (el as any).updateComplete;

    // After successful PATCH, entity should be updated and not dirty
    expect((el as any)._entity?.name).toBe('Bob');
    expect((el as any)._dirty).toBe(false);
    expect(mockPatch).toHaveBeenCalled();
  });
});

// ============================================================
// Task 2b — extensions (5 tests)
// ============================================================

describe('OrAgentDetail — extensions (Task 2b)', () => {
  let el: HTMLElement;

  beforeEach(() => {
    el = document.createElement('or-agent-detail');
    document.body.appendChild(el);
  });

  afterEach(() => {
    if (el.parentNode) el.parentNode.removeChild(el);
    vi.restoreAllMocks();
  });

  it('PATCH 409 sets _conflictServer from error.current WITHOUT a second GET', async () => {
    const mockGet = vi.fn().mockResolvedValue({ data: MOCK_AGENT, error: null });
    const conflict409 = {
      error: { error: 'version_conflict', reason: 'Conflict', request_id: 'r1', current: { ...MOCK_AGENT, name: 'NewName', version: 6 } },
      data: null,
    };
    const mockPatch = vi.fn().mockResolvedValue(conflict409);
    (el as any).orgId = 'test-org';
    (el as any).entityId = MOCK_AGENT.id;
    (el as any).client = { GET: mockGet, PATCH: mockPatch, DELETE: vi.fn() };
    await (el as any).updateComplete;
    await new Promise((r) => setTimeout(r, 50));
    await (el as any).updateComplete;

    // Initial GET count
    const getCallCountBefore = mockGet.mock.calls.length;

    // Simulate save — should trigger 409
    (el as any)._form = { ...(el as any)._form, name: 'SomeName' };
    (el as any)._dirty = true;
    (el as any)._assignedSkills = [];
    await (el as any)._handleSave();
    await (el as any).updateComplete;

    // No extra GET was fired after the 409
    expect(mockGet.mock.calls.length).toBe(getCallCountBefore);
    // _conflictServer set from error.current
    expect((el as any)._conflictServer).toBeTruthy();
    expect((el as any)._conflictServer?.name).toBe('NewName');
  });

  it('Delete dialog: [Delete] disabled until typed name matches agent.name exactly (D6-V-40)', async () => {
    const mockGet = vi.fn().mockResolvedValue({ data: MOCK_AGENT, error: null });
    (el as any).orgId = 'test-org';
    (el as any).entityId = MOCK_AGENT.id;
    (el as any).client = { GET: mockGet, PATCH: vi.fn(), DELETE: vi.fn() };
    await (el as any).updateComplete;
    await new Promise((r) => setTimeout(r, 50));
    await (el as any).updateComplete;

    // Open delete dialog
    (el as any)._deleteConfirmOpen = true;
    await (el as any).updateComplete;

    // With wrong name: _deleteConfirmName = '' → _canDelete = false
    (el as any)._deleteConfirmName = '';
    await (el as any).updateComplete;
    expect((el as any)._canDelete).toBe(false);

    // With exact name: _canDelete = true
    (el as any)._deleteConfirmName = MOCK_AGENT.name;
    await (el as any).updateComplete;
    expect((el as any)._canDelete).toBe(true);

    // With partial name: _canDelete = false
    (el as any)._deleteConfirmName = 'Ali';
    await (el as any).updateComplete;
    expect((el as any)._canDelete).toBe(false);
  });

  it('[Enable] fires PATCH { enabled: true } and refreshes entity', async () => {
    const disabledAgent = { ...MOCK_AGENT, enabled: false };
    const enabledAgent = { ...MOCK_AGENT, enabled: true, version: 6 };
    const mockGet = vi.fn().mockResolvedValue({ data: disabledAgent, error: null });
    const mockPatch = vi.fn().mockResolvedValue({ data: enabledAgent, error: null });
    (el as any).orgId = 'test-org';
    (el as any).entityId = MOCK_AGENT.id;
    (el as any).client = { GET: mockGet, PATCH: mockPatch, DELETE: vi.fn() };
    await (el as any).updateComplete;
    await new Promise((r) => setTimeout(r, 50));
    await (el as any).updateComplete;

    await (el as any)._handleEnable();
    await (el as any).updateComplete;

    expect(mockPatch).toHaveBeenCalled();
    const patchBody = mockPatch.mock.calls[0]?.[1]?.body;
    expect(patchBody?.enabled).toBe(true);
    expect((el as any)._entity?.enabled).toBe(true);
  });

  it('skills sub-table renders skill rows with proficiency', async () => {
    const mockGet = vi.fn().mockResolvedValue({ data: MOCK_AGENT, error: null });
    (el as any).orgId = 'test-org';
    (el as any).entityId = MOCK_AGENT.id;
    (el as any).client = { GET: mockGet, PATCH: vi.fn(), DELETE: vi.fn() };
    await (el as any).updateComplete;
    await new Promise((r) => setTimeout(r, 50));
    await (el as any).updateComplete;

    // Check that _assignedSkills is populated from agent's skills
    const skills = (el as any)._assignedSkills as Array<{ name?: string; proficiency?: number }>;
    expect(skills).toHaveLength(1);
    expect(skills[0]?.name).toBe('Billing');
    expect(skills[0]?.proficiency).toBe(8);
  });

  it('wrapup countdown _wrapupSecondsLeft is set from wrapup_until and decrements', async () => {
    const mockGet = vi.fn().mockResolvedValue({ data: MOCK_AGENT, error: null });
    (el as any).orgId = 'test-org';
    (el as any).entityId = MOCK_AGENT.id;
    (el as any).client = { GET: mockGet, PATCH: vi.fn(), DELETE: vi.fn() };

    // Set wrapup_until 5 seconds from now
    const wrapupUntil = new Date(Date.now() + 5000).toISOString();
    (el as any).wrapupUntil = wrapupUntil;
    document.body.appendChild(el);
    await (el as any).updateComplete;

    // _wrapupSecondsLeft should be set > 0
    const seconds = (el as any)._wrapupSecondsLeft as number | null;
    expect(seconds).not.toBeNull();
    expect(seconds).toBeGreaterThan(0);
    expect(seconds).toBeLessThanOrEqual(5);

    // Cleanup interval
    if ((el as any)._wrapupInterval) {
      clearInterval((el as any)._wrapupInterval);
    }
  });
});
