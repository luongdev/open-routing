import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import './or-select.js';

describe('OrSelect', () => {
  let el: HTMLElement;

  beforeEach(() => {
    el = document.createElement('or-select');
    document.body.appendChild(el);
  });

  afterEach(() => {
    if (el.parentNode) el.parentNode.removeChild(el);
  });

  it('renders options from options prop', async () => {
    (el as any).options = [
      { value: 'a', label: 'Alpha' },
      { value: 'b', label: 'Beta' },
    ];
    await (el as any).updateComplete;
    const opts = el.querySelectorAll('option');
    expect(opts.length).toBe(2);
    expect(opts[0]!.value).toBe('a');
    expect(opts[1]!.value).toBe('b');
  });

  it('binds value to select element', async () => {
    (el as any).options = [{ value: 'x', label: 'X' }, { value: 'y', label: 'Y' }];
    (el as any).value = 'x';
    await (el as any).updateComplete;
    const select = el.querySelector('select') as HTMLSelectElement;
    expect(select.value).toBe('x');
  });

  it('dispatches or-change event on change', async () => {
    (el as any).options = [{ value: '1', label: 'One' }, { value: '2', label: 'Two' }];
    await (el as any).updateComplete;
    const events: CustomEvent[] = [];
    el.addEventListener('or-change', (e) => events.push(e as CustomEvent));
    const select = el.querySelector('select') as HTMLSelectElement;
    select.value = '2';
    select.dispatchEvent(new Event('change', { bubbles: true }));
    expect(events.length).toBe(1);
    expect(events[0]!.detail.value).toBe('2');
  });

  it('adds uk-form-danger class on errorText', async () => {
    (el as any).errorText = 'Required';
    await (el as any).updateComplete;
    const select = el.querySelector('select')!;
    expect(select.className).toContain('uk-form-danger');
  });

  it('disables the select when disabled prop set', async () => {
    (el as any).disabled = true;
    await (el as any).updateComplete;
    const select = el.querySelector('select') as HTMLSelectElement;
    expect(select.disabled).toBe(true);
  });

  it('renders placeholder option when placeholder set', async () => {
    (el as any).placeholder = 'Pick one';
    (el as any).options = [{ value: 'a', label: 'Alpha' }];
    await (el as any).updateComplete;
    const opts = el.querySelectorAll('option');
    expect(opts[0]!.textContent).toBe('Pick one');
    expect(opts[0]!.hidden).toBe(true);
  });
});
