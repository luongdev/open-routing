import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import './or-button.js';

describe('OrButton', () => {
  let el: HTMLElement;

  beforeEach(() => {
    el = document.createElement('or-button');
    document.body.appendChild(el);
  });

  afterEach(() => {
    if (el.parentNode) el.parentNode.removeChild(el);
  });

  it('default variant + md size → uk-button uk-button-default', async () => {
    await (el as any).updateComplete;
    expect(el.querySelector('button')?.className).toBe('uk-button uk-button-default');
  });

  it('variant=primary, size=lg → uk-button uk-button-primary uk-button-large', async () => {
    (el as any).variant = 'primary';
    (el as any).size = 'lg';
    await (el as any).updateComplete;
    expect(el.querySelector('button')?.className).toBe('uk-button uk-button-primary uk-button-large');
  });

  it('variant=destructive maps to uk-button-danger', async () => {
    (el as any).variant = 'destructive';
    await (el as any).updateComplete;
    expect(el.querySelector('button')?.className).toContain('uk-button-danger');
  });

  it('size=sm adds uk-button-small', async () => {
    (el as any).variant = 'secondary';
    (el as any).size = 'sm';
    await (el as any).updateComplete;
    expect(el.querySelector('button')?.className).toBe('uk-button uk-button-secondary uk-button-small');
  });

  it('disabled propagates to button', async () => {
    (el as any).disabled = true;
    await (el as any).updateComplete;
    expect((el.querySelector('button') as HTMLButtonElement).disabled).toBe(true);
  });

  it('loading shows spinner + disables button', async () => {
    (el as any).loading = true;
    await (el as any).updateComplete;
    expect(el.querySelector('.uk-spinner')).toBeTruthy();
    expect((el.querySelector('button') as HTMLButtonElement).disabled).toBe(true);
  });

  it('slot content renders', async () => {
    el.textContent = 'Hello';
    await (el as any).updateComplete;
    expect(el.textContent).toContain('Hello');
  });

  it('type=submit on button', async () => {
    (el as any).type = 'submit';
    await (el as any).updateComplete;
    expect((el.querySelector('button') as HTMLButtonElement).type).toBe('submit');
  });
});
