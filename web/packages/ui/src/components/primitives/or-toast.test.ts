import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import './or-toast.js';

describe('OrToast', () => {
  let el: HTMLElement;

  beforeEach(() => {
    el = document.createElement('or-toast');
    document.body.appendChild(el);
  });

  afterEach(() => {
    if (el.parentNode) el.parentNode.removeChild(el);
  });

  it('renders uk-notification-message wrapper', async () => {
    await (el as any).updateComplete;
    const wrapper = el.querySelector('.uk-notification-message');
    expect(wrapper).toBeTruthy();
    expect(wrapper?.getAttribute('role')).toBe('alert');
  });

  it('default variant renders info class', async () => {
    await (el as any).updateComplete;
    const wrapper = el.querySelector('.uk-notification-message');
    expect(wrapper?.className).toContain('uk-notification-message-info');
  });

  it('variant=success adds uk-notification-message-success', async () => {
    (el as any).variant = 'success';
    await (el as any).updateComplete;
    const wrapper = el.querySelector('.uk-notification-message');
    expect(wrapper?.className).toContain('uk-notification-message-success');
  });

  it('variant=destructive adds uk-notification-message-danger', async () => {
    (el as any).variant = 'destructive';
    await (el as any).updateComplete;
    const wrapper = el.querySelector('.uk-notification-message');
    expect(wrapper?.className).toContain('uk-notification-message-danger');
  });

  it('close button removes the element', async () => {
    await (el as any).updateComplete;
    const btn = el.querySelector<HTMLButtonElement>('.uk-notification-close');
    expect(btn).toBeTruthy();
    btn!.click();
    expect(document.body.contains(el)).toBe(false);
  });
});
