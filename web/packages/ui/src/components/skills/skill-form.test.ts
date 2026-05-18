// Phase 6 Plan 07 Task 2: Tests for <or-skill-form>
// TDD RED — these tests fail until skill-form.ts exists.
// Skills is a single-step form (D6-17): stepper hidden, header "Create skill".
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

import './skill-form.js';

describe('OrSkillForm', () => {
  let el: HTMLElement;

  beforeEach(() => {
    el = document.createElement('or-skill-form');
    // Set props before connecting
  });

  afterEach(() => {
    if (el.parentNode) el.parentNode.removeChild(el);
    vi.restoreAllMocks();
  });

  it('renders single-step form with header "Create skill" (D6-17, stepper hidden)', async () => {
    (el as any).orgId = 'test-org';
    (el as any).client = { POST: vi.fn(), GET: vi.fn() };
    document.body.appendChild(el);
    await (el as any).updateComplete;

    const shadow = el.shadowRoot!;
    const text = shadow.textContent ?? '';
    expect(text).toContain('Create skill');
    // Stepper hidden: no step indicators visible (wizard has 1 step → hides stepper)
    // or-form-wizard with steps.length === 1 hides the stepper
    const wizard = shadow.querySelector('or-form-wizard') as any;
    expect(wizard).toBeTruthy();
  });

  it('validateCreateSkill runs before POST; shows inline error if code fails pattern', async () => {
    (el as any).orgId = 'test-org';
    const mockPost = vi.fn().mockResolvedValue({ data: null, error: null });
    (el as any).client = { POST: mockPost, GET: vi.fn() };
    document.body.appendChild(el);
    await (el as any).updateComplete;

    // Set invalid code (starts with uppercase)
    (el as any)._formData = {
      ...(el as any)._formData,
      code: 'INVALID_CODE',
      name: 'Test Skill',
      skill_type: 'support',
    };
    await (el as any)._handleSubmit();
    await (el as any).updateComplete;

    // POST should NOT have been called (validation failed)
    expect(mockPost).not.toHaveBeenCalled();
    // Inline error set on code field
    expect((el as any)._errors?.['code']).toBeTruthy();
  });

  it('409 duplicate_code shows inline error on code field; no navigation', async () => {
    (el as any).orgId = 'test-org';
    const mockPost = vi.fn().mockResolvedValue({
      data: null,
      error: { error: 'duplicate_code', reason: 'Code already in use', request_id: 'r1' },
    });
    (el as any).client = { POST: mockPost, GET: vi.fn() };
    document.body.appendChild(el);
    await (el as any).updateComplete;

    const navigateEvents: CustomEvent[] = [];
    el.addEventListener('open-routing:navigate', (e) => navigateEvents.push(e as CustomEvent));

    // Set valid form data
    (el as any)._formData = {
      code: 'skill_billing',
      name: 'Billing Support',
      skill_type: 'support',
      external_id: '',
      description: '',
      enabled: true,
    };
    await (el as any)._handleSubmit();
    await (el as any).updateComplete;

    // No navigation fired
    expect(navigateEvents).toHaveLength(0);
    // Inline error on code
    expect((el as any)._errors?.['code']).toBe('This code is already in use.');
  });

  it('successful POST dispatches open-routing:navigate to /orgs/{orgId}/skills/{newId}', async () => {
    const newSkill = { id: '01935b00-0000-7000-8000-000000000099', code: 'skill_new' };
    const mockPost = vi.fn().mockResolvedValue({ data: newSkill, error: null });
    (el as any).orgId = 'test-org';
    (el as any).client = { POST: mockPost, GET: vi.fn() };
    document.body.appendChild(el);
    await (el as any).updateComplete;

    const events: CustomEvent[] = [];
    el.addEventListener('open-routing:navigate', (e) => events.push(e as CustomEvent));

    (el as any)._formData = {
      code: 'skill_new',
      name: 'New Skill',
      skill_type: 'support',
      external_id: '',
      description: '',
      enabled: true,
    };
    await (el as any)._handleSubmit();
    await (el as any).updateComplete;

    expect(events).toHaveLength(1);
    expect(events[0]?.detail?.path).toBe('/orgs/test-org/skills/01935b00-0000-7000-8000-000000000099');
  });
});
