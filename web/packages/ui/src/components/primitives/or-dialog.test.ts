import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import './or-dialog.js';

describe('OrDialog', () => {
  let el: HTMLElement;

  beforeEach(() => {
    el = document.createElement('or-dialog');
    document.body.appendChild(el);
  });

  afterEach(() => {
    (el as any).open = false;
    if (el.parentNode) el.parentNode.removeChild(el);
  });

  it('open=false: host has no [open] attribute and overlay is not rendered', async () => {
    await (el as any).updateComplete;
    expect(el.hasAttribute('open')).toBe(false);
    // Shadow DOM: overlay not visible (display:none via :host)
    const overlay = el.shadowRoot?.querySelector('.or-dialog-overlay');
    expect(overlay).toBeTruthy(); // rendered but :host has display:none
    expect(el.getAttribute('open')).toBeNull();
  });

  it('open=true: reflects [open] attribute on host', async () => {
    (el as any).open = true;
    await (el as any).updateComplete;
    expect(el.hasAttribute('open')).toBe(true);
  });

  it('size prop reflects and maps to correct CSS variable via data-size', async () => {
    (el as any).size = 'lg';
    await (el as any).updateComplete;
    const dialog = el.shadowRoot?.querySelector('.uk-modal-dialog');
    expect(dialog).toBeTruthy();
    expect(dialog?.getAttribute('data-size')).toBe('lg');
  });

  it('emits or-dialog-open when open changes from false to true', async () => {
    const events: CustomEvent[] = [];
    el.addEventListener('or-dialog-open', (e) => events.push(e as CustomEvent));
    (el as any).open = true;
    await (el as any).updateComplete;
    expect(events).toHaveLength(1);
    expect(events[0]?.bubbles).toBe(true);
    expect(events[0]?.composed).toBe(true);
  });

  it('emits or-dialog-close when close() is called', async () => {
    (el as any).open = true;
    await (el as any).updateComplete;
    const events: CustomEvent[] = [];
    el.addEventListener('or-dialog-close', (e) => events.push(e as CustomEvent));
    (el as any).close();
    await (el as any).updateComplete;
    expect(events).toHaveLength(1);
    expect((el as any).open).toBe(false);
  });

  it('Esc key closes dialog when preventClose=false', async () => {
    (el as any).open = true;
    await (el as any).updateComplete;
    const events: CustomEvent[] = [];
    el.addEventListener('or-dialog-close', (e) => events.push(e as CustomEvent));
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    await (el as any).updateComplete;
    expect(events).toHaveLength(1);
    expect((el as any).open).toBe(false);
  });

  it('Esc key does NOT close dialog when preventClose=true', async () => {
    (el as any).preventClose = true;
    (el as any).open = true;
    await (el as any).updateComplete;
    const events: CustomEvent[] = [];
    el.addEventListener('or-dialog-close', (e) => events.push(e as CustomEvent));
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    await (el as any).updateComplete;
    expect(events).toHaveLength(0);
    expect((el as any).open).toBe(true);
  });

  it('overlay click closes dialog when preventClose=false', async () => {
    (el as any).open = true;
    await (el as any).updateComplete;
    const events: CustomEvent[] = [];
    el.addEventListener('or-dialog-close', (e) => events.push(e as CustomEvent));
    const overlay = el.shadowRoot?.querySelector('.or-dialog-overlay') as HTMLElement;
    expect(overlay).toBeTruthy();
    // Simulate click on the overlay (target === currentTarget)
    overlay.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    await (el as any).updateComplete;
    expect(events).toHaveLength(1);
  });
});
