// Phase 6 Plan 11 Task 2: Tests for <or-adapter-form> — single-step wizard
// TDD RED — these tests fail until adapter-form.ts exists.
// Key behaviors: single-step, config optional → null, invalid JSON blocks, POST navigation.
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

import './adapter-form.js';

describe('OrAdapterForm', () => {
  let el: HTMLElement;

  beforeEach(() => {
    el = document.createElement('or-adapter-form');
    (el as any).orgId = 'test-org';
    (el as any).client = {
      POST: vi.fn().mockResolvedValue({
        data: { id: '01935c00-0000-7000-8000-000000000099', code: 'adapter_new' },
        error: null,
      }),
    };
    document.body.appendChild(el);
  });

  afterEach(() => {
    if (el.parentNode) el.parentNode.removeChild(el);
    vi.restoreAllMocks();
  });

  // Test 1: single-step wizard; stepper hidden; header "Create adapter"
  it('renders single-step wizard with header "Create adapter" and stepper hidden', async () => {
    await (el as any).updateComplete;

    const shadow = el.shadowRoot!;
    const text = shadow.textContent ?? '';
    expect(text).toContain('Create adapter');

    // or-form-wizard with 1 step (stepper hidden when steps <= 1)
    const wizard = shadow.querySelector('or-form-wizard') as any;
    expect(wizard).toBeTruthy();
    const steps = wizard.steps as Array<{ key: string; label: string }>;
    expect(steps).toHaveLength(1);
    expect(steps[0]?.label).toBe('Basics');
  });

  // Test 2: config textarea optional; empty submission sends config: null in body
  it('sends config: null when config textarea is empty', async () => {
    const mockPost = vi.fn().mockResolvedValue({
      data: { id: '01935c00-0000-7000-8000-000000000099', code: 'adapter_new' },
      error: null,
    });
    (el as any).client = { POST: mockPost };

    await (el as any).updateComplete;

    // Set valid required fields, but leave config empty
    (el as any)._formData = {
      code: 'adapter_test',
      name: 'Test Adapter',
      external_id: '',
      adapter_type: 'freeswitch',
      configText: '',
      enabled: true,
    };

    // Call submit directly (single-step wizard goes straight to submit)
    await (el as any)._handleSubmit();
    await (el as any).updateComplete;

    expect(mockPost).toHaveBeenCalled();
    const postBody = mockPost.mock.calls[0]?.[1]?.body;
    expect(postBody?.config).toBeNull();
  });

  // Test 3: invalid JSON in config textarea blocks form completion with inline error
  it('blocks submit with inline error when config textarea has invalid JSON', async () => {
    const mockPost = vi.fn();
    (el as any).client = { POST: mockPost };

    await (el as any).updateComplete;

    // Set invalid JSON in configText
    (el as any)._formData = {
      code: 'adapter_test',
      name: 'Test Adapter',
      external_id: '',
      adapter_type: 'freeswitch',
      configText: '{not valid json',
      enabled: true,
    };
    (el as any)._configError = 'Config must be valid JSON';

    await (el as any)._handleSubmit();
    await (el as any).updateComplete;

    // POST should NOT be called
    expect(mockPost).not.toHaveBeenCalled();

    // Error should be displayed
    const shadow = el.shadowRoot!;
    const text = shadow.textContent ?? '';
    expect(text).toContain('Config must be valid JSON');
  });

  // Test 4: successful POST navigates to /orgs/{orgId}/adapters/{newId}
  it('navigates to adapter detail page after successful POST', async () => {
    const newId = '01935c00-0000-7000-8000-000000000099';
    const mockPost = vi.fn().mockResolvedValue({
      data: { id: newId, code: 'adapter_new' },
      error: null,
    });
    (el as any).client = { POST: mockPost };

    await (el as any).updateComplete;

    const events: CustomEvent[] = [];
    el.addEventListener('open-routing:navigate', (e) => events.push(e as CustomEvent));

    // Set valid form data
    (el as any)._formData = {
      code: 'adapter_new',
      name: 'New Adapter',
      external_id: '',
      adapter_type: 'sip',
      configText: '{"host": "10.0.0.1"}',
      enabled: true,
    };

    await (el as any)._handleSubmit();
    await (el as any).updateComplete;

    expect(mockPost).toHaveBeenCalled();
    expect(events).toHaveLength(1);
    expect(events[0]?.detail?.path).toContain(`/adapters/${newId}`);
  });
});
