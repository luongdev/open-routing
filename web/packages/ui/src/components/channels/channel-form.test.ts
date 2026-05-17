// Phase 6 Plan 10 Task 2: <or-channel-form> unit tests.
// TDD RED phase: tests fail before implementation.
// Tests: 3-step wizard stepper, Step 1 validation, Step 2 or-queue-picker, Review + POST.

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

// Import the component (will fail until implementation exists)
import './channel-form.js';

const ORG_ID = '01901b2c-7f3a-7000-8000-000000000001';
const NEW_CHANNEL_ID = '01901b2c-7f3a-7000-8000-000000000020';

describe('or-channel-form', () => {
  let el: HTMLElement;

  beforeEach(() => {
    el = document.createElement('or-channel-form');
    document.body.appendChild(el);
  });

  afterEach(() => {
    if (el.parentNode) el.parentNode.removeChild(el);
    vi.clearAllMocks();
  });

  it('Test 1: 3-step wizard shows "Basics", "Default queue", "Review" steps', async () => {
    (el as any).orgId = ORG_ID;
    (el as any).client = {
      GET: vi.fn().mockResolvedValue({ data: { items: [], has_more: false }, error: null }),
      POST: vi.fn().mockResolvedValue({ data: { id: NEW_CHANNEL_ID }, error: null }),
    };

    await (el as any).updateComplete;

    const shadow = el.shadowRoot!;
    // or-form-wizard should be present with 3 steps
    const wizard = shadow.querySelector('or-form-wizard') as any;
    expect(wizard).toBeTruthy();

    const steps = (wizard?.steps ?? (el as any)._wizardSteps ?? []) as Array<{ label: string }>;
    const labels = steps.map((s) => s.label);
    expect(labels.length).toBe(3);
    expect(labels[0]).toContain('Basics');
    expect(labels[1]).toContain('Default queue');
    expect(labels[2]).toContain('Review');
  });

  it('Test 2: Step 1 Next validates code + name + channel_type before advancing', async () => {
    (el as any).orgId = ORG_ID;
    (el as any).client = {
      GET: vi.fn().mockResolvedValue({ data: { items: [], has_more: false }, error: null }),
      POST: vi.fn(),
    };

    await (el as any).updateComplete;

    // Initial step should be 0 (Basics)
    expect((el as any)._currentStep).toBe(0);

    // Call _handleNext without setting required fields — should stay on step 0
    await (el as any)._handleNext();
    await (el as any).updateComplete;

    expect((el as any)._currentStep).toBe(0);

    // Errors should be set for missing required fields
    const errors = (el as any)._errors as Record<string, string>;
    const hasErrors = Object.values(errors).some((v) => v && v.length > 0);
    expect(hasErrors).toBe(true);
  });

  it('Test 3: Step 2 shows or-queue-picker (optional — null allowed)', async () => {
    (el as any).orgId = ORG_ID;
    (el as any).client = {
      GET: vi.fn().mockResolvedValue({ data: { items: [], has_more: false }, error: null }),
      POST: vi.fn(),
    };

    await (el as any).updateComplete;

    // Set step to 1 (Default queue step) — bypass validation for this test
    (el as any)._currentStep = 1;
    await (el as any).updateComplete;

    const shadow = el.shadowRoot!;
    // Should have or-queue-picker in step 2
    const queuePicker = shadow.querySelector('or-queue-picker');
    expect(queuePicker).toBeTruthy();

    // Advance to step 2 (review) without selecting a queue — should be allowed
    await (el as any)._handleNext();
    expect((el as any)._currentStep).toBe(2);
  });

  it('Test 4: Review step shows entered values; Create button POSTs CreateChannelRequest', async () => {
    const mockPost = vi.fn().mockResolvedValue({
      data: { id: NEW_CHANNEL_ID, code: 'ch_test' },
      error: null,
    });
    (el as any).orgId = ORG_ID;
    (el as any).client = {
      GET: vi.fn().mockResolvedValue({ data: { items: [], has_more: false }, error: null }),
      POST: mockPost,
    };

    await (el as any).updateComplete;

    // Set form data and advance to review step
    (el as any)._formData = {
      code: 'ch_test',
      name: 'Test Channel',
      external_id: '',
      channel_type: 'voice',
      default_queue_id: null,
      enabled: true,
    };
    (el as any)._currentStep = 2;
    await (el as any).updateComplete;

    // Call _handleSubmit
    await (el as any)._handleSubmit();
    await (el as any).updateComplete;

    // POST should have been called
    expect(mockPost).toHaveBeenCalledTimes(1);
    const callBody = (mockPost.mock.calls[0] as any[])[1]?.body;
    expect(callBody?.code).toBe('ch_test');
    expect(callBody?.name).toBe('Test Channel');
    expect(callBody?.channel_type).toBe('voice');
  });
});
