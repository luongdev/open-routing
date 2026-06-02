import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import './or-input.js';

describe('OrInput', () => {
  let el: HTMLElement;

  beforeEach(() => {
    el = document.createElement('or-input');
    document.body.appendChild(el);
  });

  afterEach(() => {
    if (el.parentNode) el.parentNode.removeChild(el);
  });

  it('renders native input with uk-input class', async () => {
    await (el as any).updateComplete;
    const input = el.querySelector('input');
    expect(input).toBeTruthy();
    expect(input?.classList.contains('uk-input')).toBe(true);
  });

  it('reflects value property to input', async () => {
    (el as any).value = 'hello';
    await (el as any).updateComplete;
    expect((el.querySelector('input') as HTMLInputElement).value).toBe('hello');
  });

  it('renders label when label prop is set', async () => {
    (el as any).label = 'Email';
    await (el as any).updateComplete;
    const label = el.querySelector('label.uk-form-label');
    expect(label?.textContent).toBe('Email');
  });

  it('renders helper text when helperText is set', async () => {
    (el as any).helperText = 'Enter your email';
    await (el as any).updateComplete;
    const help = el.querySelector('.uk-form-help');
    expect(help?.textContent).toBe('Enter your email');
  });

  it('renders error text with uk-form-danger class on input when errorText is set', async () => {
    (el as any).errorText = 'Invalid value';
    await (el as any).updateComplete;
    const input = el.querySelector('input');
    expect(input?.classList.contains('uk-form-danger')).toBe(true);
    const errEl = el.querySelector('.uk-form-help');
    expect(errEl?.textContent).toBe('Invalid value');
  });

  it('disabled propagates to native input', async () => {
    (el as any).disabled = true;
    await (el as any).updateComplete;
    expect((el.querySelector('input') as HTMLInputElement).disabled).toBe(true);
  });

  it('readonly propagates to native input', async () => {
    (el as any).readonly = true;
    await (el as any).updateComplete;
    expect((el.querySelector('input') as HTMLInputElement).readOnly).toBe(true);
  });

  it('dispatches or-input event with detail.value on input', async () => {
    await (el as any).updateComplete;
    const events: CustomEvent[] = [];
    el.addEventListener('or-input', (e) => events.push(e as CustomEvent));

    const input = el.querySelector('input') as HTMLInputElement;
    input.value = 'typed';
    input.dispatchEvent(new Event('input'));

    expect(events.length).toBe(1);
    expect(events[0]!.detail.value).toBe('typed');
  });
});
