// Phase 6 Plan 05 Task 3: Tests for <or-agent-form> — 3-step wizard
// TDD RED — these tests fail until agent-form.ts exists.
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

import './agent-form.js';

const MOCK_SKILL = {
  id: '01935b00-0000-7000-8000-000000000010',
  code: 'billing',
  name: 'Billing Support',
};

describe('OrAgentForm', () => {
  let el: HTMLElement;

  beforeEach(() => {
    el = document.createElement('or-agent-form');
    // Set props before mounting
    (el as any).orgId = 'test-org';
    (el as any).client = {
      GET: vi.fn().mockResolvedValue({ data: { items: [], has_more: false }, error: null }),
      POST: vi.fn().mockResolvedValue({
        data: { id: '01935b00-0000-7000-8000-000000000099', code: 'emp_new' },
        error: null,
      }),
    };
    document.body.appendChild(el);
  });

  afterEach(() => {
    if (el.parentNode) el.parentNode.removeChild(el);
    vi.restoreAllMocks();
  });

  it('renders 3-step wizard with Basics, Skills, Review steps in or-form-wizard', async () => {
    await (el as any).updateComplete;

    const shadow = el.shadowRoot!;
    // or-form-wizard should be rendered with 3 steps
    const wizard = shadow.querySelector('or-form-wizard') as any;
    expect(wizard).toBeTruthy();
    const steps = wizard.steps as Array<{ key: string; label: string }>;
    expect(steps).toHaveLength(3);
    expect(steps[0]?.label).toBe('Basics');
    expect(steps[1]?.label).toBe('Skills');
    expect(steps[2]?.label).toBe('Review');
  });

  it('Step 1: empty code field prevents advancing — code validation error shown', async () => {
    await (el as any).updateComplete;

    // Try to advance to step 2 with empty code
    (el as any)._formData = { code: '', name: '', email: '', external_id: '', enabled: true };
    await (el as any)._handleNext();
    await (el as any).updateComplete;

    // Should still be on step 1 (index 0)
    expect((el as any)._currentStep).toBe(0);
    // Code error should be set
    expect((el as any)._errors.code).toBeTruthy();
  });

  it('on wizard-completed dispatches POST to client.POST with skills array', async () => {
    const mockPost = vi.fn().mockResolvedValue({
      data: { id: '01935b00-0000-7000-8000-000000000099', code: 'emp_new' },
      error: null,
    });
    (el as any).client = {
      GET: vi.fn().mockResolvedValue({ data: { items: [], has_more: false }, error: null }),
      POST: mockPost,
    };

    await (el as any).updateComplete;

    // Set valid form data
    (el as any)._formData = {
      code: 'emp_001',
      name: 'Test Agent',
      email: 'test@example.com',
      external_id: '',
      enabled: true,
    };
    (el as any)._assignedSkills = [{ skill_id: MOCK_SKILL.id, proficiency: 7 }];
    (el as any)._currentStep = 2; // on review step

    // Call submit
    await (el as any)._handleSubmit();
    await (el as any).updateComplete;

    expect(mockPost).toHaveBeenCalled();
    const postBody = mockPost.mock.calls[0]?.[1]?.body;
    expect(postBody?.code).toBe('emp_001');
    expect(postBody?.name).toBe('Test Agent');
    expect(Array.isArray(postBody?.skills)).toBe(true);
    expect(postBody?.skills).toHaveLength(1);
    expect(postBody?.skills[0].skill_id).toBe(MOCK_SKILL.id);
    expect(postBody?.skills[0].proficiency).toBe(7);
  });

  it('409 duplicate_code on POST returns to Step 1 with inline code error', async () => {
    const mockPost = vi.fn().mockResolvedValue({
      error: { error: 'duplicate_code', reason: 'Code already in use', request_id: 'r1' },
      data: null,
    });
    (el as any).client = {
      GET: vi.fn().mockResolvedValue({ data: { items: [], has_more: false }, error: null }),
      POST: mockPost,
    };

    await (el as any).updateComplete;

    // Set up valid form for step 3
    (el as any)._formData = {
      code: 'dup_code',
      name: 'Agent',
      email: 'a@b.com',
      external_id: '',
      enabled: true,
    };
    (el as any)._assignedSkills = [];
    (el as any)._currentStep = 2;

    await (el as any)._handleSubmit();
    await (el as any).updateComplete;

    // Should return to step 1
    expect((el as any)._currentStep).toBe(0);
    // Code error set
    expect((el as any)._errors.code).toBeTruthy();
    expect((el as any)._errors.code).toContain('already in use');
  });

  it('successful POST dispatches open-routing:navigate to /orgs/{orgId}/agents/{newId}', async () => {
    const newId = '01935b00-0000-7000-8000-000000000099';
    const mockPost = vi.fn().mockResolvedValue({
      data: { id: newId, code: 'emp_new' },
      error: null,
    });
    (el as any).client = {
      GET: vi.fn().mockResolvedValue({ data: { items: [], has_more: false }, error: null }),
      POST: mockPost,
    };

    await (el as any).updateComplete;

    const events: CustomEvent[] = [];
    el.addEventListener('open-routing:navigate', (e) => events.push(e as CustomEvent));

    (el as any)._formData = {
      code: 'emp_new',
      name: 'New Agent',
      email: 'new@example.com',
      external_id: '',
      enabled: true,
    };
    (el as any)._assignedSkills = [];
    (el as any)._currentStep = 2;

    await (el as any)._handleSubmit();
    await (el as any).updateComplete;

    expect(events).toHaveLength(1);
    expect(events[0]?.detail?.path).toBe(`/orgs/test-org/agents/${newId}`);
  });
});
