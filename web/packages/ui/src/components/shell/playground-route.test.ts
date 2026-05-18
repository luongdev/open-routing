import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import './playground-route.js';

describe('OrPlaygroundRoute', () => {
  let el: HTMLElement;

  beforeEach(() => {
    el = document.createElement('or-playground-route');
    document.body.appendChild(el);
  });

  afterEach(() => {
    if (el.parentNode) {
      el.parentNode.removeChild(el);
    }
  });

  it('renders 13 slots', async () => {
    await (el as any).updateComplete;
    const slots = el.shadowRoot!.querySelectorAll('section.slot');
    expect(slots.length).toBe(13);
  });

  it('renders theme toggle buttons in header', async () => {
    await (el as any).updateComplete;
    const header = el.shadowRoot!.querySelector('.header');
    expect(header).toBeTruthy();
    const buttons = header!.querySelectorAll('.theme-btn');
    expect(buttons.length).toBe(2);
    expect(buttons[0]!.textContent).toContain('Ember Light');
    expect(buttons[1]!.textContent).toContain('Ember Dark');
  });

  it.each([
    ['button', '07-w0-10'],
    ['table', '07-w0-19'],
    ['sidebar', '07-w0-22'],
  ])('slot %s references plan %s', async (id, plan) => {
    await (el as any).updateComplete;
    const slot = el.shadowRoot!.querySelector(`[data-component="${id}"]`);
    expect(slot?.textContent).toContain(plan);
  });

  it('dispatches open-routing:theme-change event on Ember Dark click', async () => {
    await (el as any).updateComplete;
    const events: CustomEvent[] = [];
    el.addEventListener('open-routing:theme-change', (e) => events.push(e as CustomEvent));
    const darkBtn = el.shadowRoot!.querySelectorAll('.theme-btn')[1] as HTMLButtonElement;
    darkBtn.click();
    expect(events.length).toBe(1);
    expect(events[0]!.detail.theme).toBe('ember-dark');
  });
});
