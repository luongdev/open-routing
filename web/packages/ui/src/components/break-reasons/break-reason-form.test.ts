// Phase 6 Plan 09 Task 2: Tests for <or-break-reason-form>
// W0.1-18: Updated to match Ember layout (native checkbox toggles, uk-input, zero sl-*).
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

  it('single-step form; header "Create break reason"', async () => {
    await (el as any).updateComplete;
    const shadow = el.shadowRoot!;
    const shadowHtml = shadow.innerHTML;

    expect(shadowHtml).toContain('Create break reason');

    const formData = (el as any)._formData;
    expect(formData).toBeTruthy();
  });

  it('routable toggle defaults to checked=true (routable=true by default per spec)', async () => {
    await (el as any).updateComplete;

    const formData = (el as any)._formData as { routable: boolean };
    expect(formData.routable).toBe(true);

    const shadow = el.shadowRoot!;
    // Ember uses native checkboxes — toggle-block contains helper text about routing
    const shadowHtml = shadow.innerHTML;
    expect(shadowHtml).toContain('still receive routed interactions');
  });

  it('display_order required; form blocked if display_order empty', async () => {
    await (el as any).updateComplete;

    (el as any)._formData = {
      code: 'break_lunch',
      name: 'Lunch',
      external_id: '',
      routable: true,
      display_order: '',
      enabled: true,
    };

    await (el as any)._handleSubmit();
    await (el as any).updateComplete;

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
