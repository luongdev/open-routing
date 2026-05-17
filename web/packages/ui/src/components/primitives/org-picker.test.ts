import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

// Register the element
import './org-picker.js';

// Valid UUIDv7 for testing
const VALID_UUID = '01901b2c-7f3a-7abc-8d4e-a1b2c3d4e5f6';
const INVALID_UUID = 'not-a-uuid';

describe('OrOrgPicker', () => {
  let el: HTMLElement;

  beforeEach(() => {
    // Clear localStorage so tests are isolated from each other
    localStorage.clear();
    el = document.createElement('or-org-picker');
    document.body.appendChild(el);
  });

  afterEach(() => {
    if (el.parentNode) {
      el.parentNode.removeChild(el);
    }
    vi.restoreAllMocks();
    localStorage.clear();
  });

  it('dispatches open-routing:org-selected event with orgId on valid UUID submit', async () => {
    await (el as any).updateComplete;

    // Set value via internal state and wait for re-render
    (el as any)._value = VALID_UUID;
    await (el as any).updateComplete;

    const events: CustomEvent[] = [];
    el.addEventListener('open-routing:org-selected', (e) => events.push(e as CustomEvent));

    // Click the continue button
    const shadow = el.shadowRoot!;
    const btn = shadow.querySelector('button.continue-btn') as HTMLButtonElement;
    expect(btn).toBeTruthy();
    btn.click();
    await (el as any).updateComplete;

    expect(events).toHaveLength(1);
    expect(events[0]?.detail?.orgId).toBe(VALID_UUID);
    expect(events[0]?.bubbles).toBe(true);
    expect(events[0]?.composed).toBe(true);
  });

  it('shows error text and does NOT dispatch event on invalid UUID submit', async () => {
    await (el as any).updateComplete;

    // Set invalid value and wait for re-render
    (el as any)._value = INVALID_UUID;
    await (el as any).updateComplete;

    const events: CustomEvent[] = [];
    el.addEventListener('open-routing:org-selected', (e) => events.push(e as CustomEvent));

    // Click the continue button
    const shadow = el.shadowRoot!;
    const btn = shadow.querySelector('button.continue-btn') as HTMLButtonElement;
    expect(btn).toBeTruthy();
    btn.click();
    await (el as any).updateComplete;

    // No event dispatched
    expect(events).toHaveLength(0);

    // Error message visible in shadow DOM
    const errorEl = shadow.querySelector('.validation-error');
    expect(errorEl?.textContent).toContain('UUIDv7');
  });

  it('pre-fills value from lastUsedOrgId property', async () => {
    (el as any).lastUsedOrgId = VALID_UUID;
    await (el as any).updateComplete;

    // The internal _value should match lastUsedOrgId when set via property before connect
    // (firstUpdated reads from localStorage then falls back to lastUsedOrgId)
    // Since localStorage is empty in tests, _value should come from lastUsedOrgId
    expect((el as any)._value).toBe(VALID_UUID);
  });

  it('saves orgId to localStorage under or-last-org-id on valid submit', async () => {
    await (el as any).updateComplete;

    // Set value and wait for render to settle before querying shadow DOM
    (el as any)._value = VALID_UUID;
    await (el as any).updateComplete;

    // Spy directly on localStorage instance (happy-dom may not extend Storage.prototype)
    const setItemSpy = vi.spyOn(localStorage, 'setItem');

    const shadow = el.shadowRoot!;
    const btn = shadow.querySelector('button.continue-btn') as HTMLButtonElement;
    expect(btn).toBeTruthy(); // guard against null button
    btn.click();
    await (el as any).updateComplete;

    expect(setItemSpy).toHaveBeenCalledWith('or-last-org-id', VALID_UUID);
  });
});
