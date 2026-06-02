import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import './or-switch.js';

describe('OrSwitch', () => {
  let el: HTMLElement;

  beforeEach(() => {
    el = document.createElement('or-switch');
    document.body.appendChild(el);
  });

  afterEach(() => {
    if (el.parentNode) el.parentNode.removeChild(el);
  });

  it('renders a checkbox input with class uk-switch', async () => {
    await (el as any).updateComplete;
    const input = el.querySelector('input[type="checkbox"]');
    expect(input).toBeTruthy();
    expect(input?.classList.contains('uk-switch')).toBe(true);
  });

  it('checked prop sets input.checked', async () => {
    (el as any).checked = true;
    await (el as any).updateComplete;
    const input = el.querySelector('input') as HTMLInputElement;
    expect(input.checked).toBe(true);
  });

  it('disabled prop disables the input', async () => {
    (el as any).disabled = true;
    await (el as any).updateComplete;
    const input = el.querySelector('input') as HTMLInputElement;
    expect(input.disabled).toBe(true);
  });

  it('label prop renders label text', async () => {
    (el as any).label = 'Notifications enabled';
    await (el as any).updateComplete;
    expect(el.textContent).toContain('Notifications enabled');
  });

  it('change event dispatches or-change with checked detail', async () => {
    await (el as any).updateComplete;
    const events: CustomEvent[] = [];
    el.addEventListener('or-change', (e) => events.push(e as CustomEvent));
    const input = el.querySelector('input') as HTMLInputElement;
    input.checked = true;
    input.dispatchEvent(new Event('change', { bubbles: true }));
    expect(events).toHaveLength(1);
    expect(events[0]!.detail.checked).toBe(true);
  });
});
