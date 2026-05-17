// Phase 6 Plan 08 Task 2: Tests for <or-queue-form>
// TDD RED — these tests fail until queue-form.ts exists.
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

import './queue-form.js';

describe('OrQueueForm', () => {
  let el: HTMLElement;

  beforeEach(() => {
    el = document.createElement('or-queue-form');
    (el as any).orgId = 'test-org';
    (el as any).client = {
      POST: vi.fn().mockResolvedValue({
        data: { id: '01935c00-0000-7000-8000-000000000099', code: 'queue_new' },
        error: null,
      }),
    };
    document.body.appendChild(el);
  });

  afterEach(() => {
    if (el.parentNode) el.parentNode.removeChild(el);
    vi.restoreAllMocks();
  });

  it('single-step wizard (stepper hidden via hideNav or single step); header "Create queue"', async () => {
    await (el as any).updateComplete;

    const shadow = el.shadowRoot!;
    // or-form-wizard should be rendered with 1 step
    const wizard = shadow.querySelector('or-form-wizard') as any;
    expect(wizard).toBeTruthy();
    const steps = wizard.steps as Array<{ key: string; label: string }>;
    expect(steps).toHaveLength(1);
    expect(steps[0]?.label).toBe('Basics');

    // Header should say "Create queue"
    expect(shadow.textContent).toContain('Create queue');
  });

  it('channel_types multi-select is required; submit blocked if empty', async () => {
    await (el as any).updateComplete;

    // Set channel_types empty
    (el as any)._formData = {
      ...(el as any)._formData,
      channel_types: [],
    };

    const mockPost = vi.fn().mockResolvedValue({
      data: { id: '01935c00-0000-7000-8000-000000000099', code: 'queue_new' },
      error: null,
    });
    (el as any).client = { POST: mockPost };

    await (el as any)._handleSubmit();
    await (el as any).updateComplete;

    // POST should NOT have been called — validation should block it
    expect(mockPost).not.toHaveBeenCalled();
    // Errors should include channel_types
    const errors = (el as any)._errors as Record<string, string>;
    expect(errors['channel_types']).toBeTruthy();
  });

  it('priority and acw_sec fields accept only integers (type="number" step="1")', async () => {
    await (el as any).updateComplete;

    const shadow = el.shadowRoot!;
    // Find number inputs in shadow DOM
    const inputs = shadow.querySelectorAll('sl-input[type="number"]');
    // Should have priority and acw_sec number inputs
    expect(inputs.length).toBeGreaterThanOrEqual(2);

    // Both should have step="1" attribute
    const stepsValues = Array.from(inputs).map((i) => i.getAttribute('step'));
    expect(stepsValues.every((s) => s === '1')).toBe(true);
  });

  it('successful POST navigates to /orgs/{orgId}/queues/{newId}', async () => {
    await (el as any).updateComplete;

    const newId = '01935c00-0000-7000-8000-000000000099';
    const mockPost = vi.fn().mockResolvedValue({
      data: { id: newId, code: 'queue_new' },
      error: null,
    });
    (el as any).client = { POST: mockPost };

    // Populate required fields
    (el as any)._formData = {
      code: 'queue_test',
      name: 'Test Queue',
      external_id: '',
      channel_types: ['voice'],
      priority: 5,
      acw_sec: 60,
      enabled: true,
    };

    const events: CustomEvent[] = [];
    el.addEventListener('open-routing:navigate', (e) => events.push(e as CustomEvent));

    await (el as any)._handleSubmit();
    await (el as any).updateComplete;

    expect(mockPost).toHaveBeenCalled();
    expect(events).toHaveLength(1);
    expect(events[0]?.detail?.path).toContain(newId);
    expect(events[0]?.detail?.path).toContain('/queues/');
  });
});
