import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { notify, notifyError } from './notify.js';

beforeEach(() => {
  document.querySelectorAll('.uk-notification').forEach((el) => el.remove());
});

afterEach(() => {
  document.querySelectorAll('.uk-notification').forEach((el) => el.remove());
});

describe('notify', () => {
  it('creates a notification container appended to document.body', () => {
    notify({ message: 'Hello', variant: 'info' });
    const container = document.querySelector('.uk-notification');
    expect(container).toBeTruthy();
    expect(document.body.contains(container)).toBe(true);
  });

  it('variant=destructive uses uk-notification-message-danger class', () => {
    notify({ message: 'Oops', variant: 'destructive' });
    const msg = document.querySelector('.uk-notification-message');
    expect(msg?.className).toContain('uk-notification-message-danger');
  });

  it('variant passthrough: success, warning, info map to their own classes', () => {
    for (const v of ['success', 'warning', 'info'] as const) {
      notify({ message: `msg-${v}`, variant: v });
      const msgs = document.querySelectorAll('.uk-notification-message');
      const last = msgs[msgs.length - 1]!;
      expect(last.className).toContain(`uk-notification-message-${v}`);
    }
  });

  it('position=top-right adds uk-notification-top-right to container', () => {
    notify({ message: 'Right', position: 'top-right' });
    const container = document.querySelector('.uk-notification');
    expect(container?.className).toContain('uk-notification-top-right');
  });

  it('default position is top-right', () => {
    notify({ message: 'Default pos' });
    const container = document.querySelector('.uk-notification');
    expect(container?.className).toContain('uk-notification-top-right');
  });

  it('message text appears in the DOM', () => {
    notify({ message: 'Save successful' });
    const msg = document.querySelector('.uk-notification-message');
    expect(msg?.textContent).toContain('Save successful');
  });

  it('close button removes the message', () => {
    notify({ message: 'Close me', duration: 0 });
    const btn = document.querySelector<HTMLButtonElement>('.uk-notification-close');
    expect(btn).toBeTruthy();
    btn!.click();
    expect(document.querySelector('.uk-notification-message')).toBeNull();
  });
});

describe('notifyError', () => {
  it('extracts Error.message', () => {
    notifyError(new Error('Something broke'));
    const msg = document.querySelector('.uk-notification-message');
    expect(msg?.textContent).toContain('Something broke');
    expect(msg?.className).toContain('uk-notification-message-danger');
  });

  it('uses string directly when passed a string', () => {
    notifyError('Raw string error');
    const msg = document.querySelector('.uk-notification-message');
    expect(msg?.textContent).toContain('Raw string error');
  });

  it('falls back to "Something went wrong" for unknown type', () => {
    notifyError({ code: 42 });
    const msg = document.querySelector('.uk-notification-message');
    expect(msg?.textContent).toContain('Something went wrong');
  });

  it('uses destructive variant', () => {
    notifyError(new Error('fail'));
    const msg = document.querySelector('.uk-notification-message');
    expect(msg?.className).toContain('uk-notification-message-danger');
  });
});
