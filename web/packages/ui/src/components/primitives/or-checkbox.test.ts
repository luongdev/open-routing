import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import './or-checkbox.js';

describe('OrCheckbox', () => {
  let el: HTMLElement;

  beforeEach(() => {
    el = document.createElement('or-checkbox');
    document.body.appendChild(el);
  });

  afterEach(() => {
    if (el.parentNode) el.parentNode.removeChild(el);
  });

  it('renders a checkbox input with class uk-checkbox', async () => {
    await (el as any).updateComplete;
    const input = el.querySelector('input[type="checkbox"]');
    expect(input).toBeTruthy();
    expect(input?.classList.contains('uk-checkbox')).toBe(true);
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
    (el as any).label = 'Accept terms';
    await (el as any).updateComplete;
    expect(el.textContent).toContain('Accept terms');
  });

  it('indeterminate prop sets input.indeterminate', async () => {
    (el as any).indeterminate = true;
    await (el as any).updateComplete;
    const input = el.querySelector('input') as HTMLInputElement;
    expect(input.indeterminate).toBe(true);
  });
});
