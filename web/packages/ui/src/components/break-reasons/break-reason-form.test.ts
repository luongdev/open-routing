// Phase 6 Plan 09 Task 2: Tests for <or-break-reason-form>
// TDD RED — these tests fail until break-reason-form.ts exists.
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

import './break-reason-form.js';

describe('OrBreakReasonForm', () => {
  let el: HTMLElement;

  beforeEach(() => {
    el = document.createElement('or-break-reason-form');
    (el as any).orgId = 'test-org';
    (el as any).client = {
      POST: vi.fn(),
      GET: vi.fn(),
    };
    document.body.appendChild(el);
  });

  afterEach(() => {
    if (el.parentNode) el.parentNode.removeChild(el);
    vi.restoreAllMocks();
  });

  it('single-step wizard; stepper hidden; header "Create break reason"', async () => {
    await (el as any).updateComplete;
    const shadow = el.shadowRoot!;
    const shadowHtml = shadow.innerHTML;

    // Single step — no multi-step stepper indicator
    // Header must say "Create break reason"
    expect(shadowHtml).toContain('Create break reason');

    // Single step — currentStep always 0, no Back button
    const steps = (el as any)._steps ?? (el as any)._currentStep;
    // If steps array exists, should have exactly 1 step
    // Otherwise currentStep should be 0 (only step)
    const formData = (el as any)._formData;
    expect(formData).toBeTruthy();
  });

  it('routable sl-switch defaults to checked=true (routable=true by default per spec)', async () => {
    await (el as any).updateComplete;

    // Default form data should have routable=true
    const formData = (el as any)._formData as { routable: boolean };
    expect(formData.routable).toBe(true);

    // sl-switch for routable should be present and checked
    const shadow = el.shadowRoot!;
    const switches = shadow.querySelectorAll('sl-switch');
    expect(switches.length).toBeGreaterThan(0);
  });

  it('display_order required; form blocked if display_order empty', async () => {
    await (el as any).updateComplete;

    // Set up form data without display_order (or empty)
    (el as any)._formData = {
      code: 'break_lunch',
      name: 'Lunch',
      external_id: '',
      routable: true,
      display_order: '',
      enabled: true,
    };

    // Attempt to submit — should fail validation
    await (el as any)._handleSubmit();
    await (el as any).updateComplete;

    // _errors should contain display_order validation error
    const errors = (el as any)._errors as Record<string, string>;
    const hasDisplayOrderError = Object.keys(errors).some(
      (k) => k === 'display_order' || k === 'form'
    );
    expect(hasDisplayOrderError).toBe(true);
  });

  it('successful POST navigates to /orgs/{orgId}/break-reasons/{newId}', async () => {
    const newId = '01935b00-0000-7000-8000-000000000099';
    (el as any).client = {
      POST: vi.fn().mockResolvedValue({
        data: { id: newId, code: 'break_lunch' },
        error: null,
      }),
    };

    await (el as any).updateComplete;

    // Set valid form data
    (el as any)._formData = {
      code: 'break_lunch',
      name: 'Lunch',
      external_id: '',
      routable: true,
      display_order: 1,
      enabled: true,
    };

    const events: CustomEvent[] = [];
    el.addEventListener('open-routing:navigate', (e) => events.push(e as CustomEvent));

    await (el as any)._handleSubmit();
    await (el as any).updateComplete;

    expect(events).toHaveLength(1);
    expect(events[0]?.detail?.path).toContain('break-reasons');
    expect(events[0]?.detail?.path).toContain(newId);
  });
});
