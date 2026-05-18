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
    ['sidebar', '07-w0-22'],
  ])('slot %s shows pending plan %s', async (id, plan) => {
    await (el as any).updateComplete;
    const slot = el.shadowRoot!.querySelector(`[data-component="${id}"]`);
    expect(slot?.textContent).toContain(plan);
  });

  it('table slot is wired with or-table element (W0.0-19)', async () => {
    await (el as any).updateComplete;
    const section = el.shadowRoot!.querySelector('section[data-component="table"]');
    expect(section).toBeTruthy();
    expect(section!.querySelector('or-table')).toBeTruthy();
  });

  it('button slot is wired with or-button elements', async () => {
    await (el as any).updateComplete;
    const buttonSection = el.shadowRoot!.querySelector('section[data-component="button"]');
    expect(buttonSection).toBeTruthy();
    const buttons = buttonSection!.querySelectorAll('or-button');
    expect(buttons.length).toBeGreaterThan(0);
  });

  it('input slot is wired with or-input elements', async () => {
    await (el as any).updateComplete;
    const inputSection = el.shadowRoot!.querySelector('section[data-component="input"]');
    expect(inputSection).toBeTruthy();
    const inputs = inputSection!.querySelectorAll('or-input');
    expect(inputs.length).toBeGreaterThan(0);
  });

  it('select slot is wired with or-select elements', async () => {
    await (el as any).updateComplete;
    const section = el.shadowRoot!.querySelector('section[data-component="select"]');
    expect(section).toBeTruthy();
    expect(section!.querySelectorAll('or-select').length).toBeGreaterThan(0);
  });

  it('card slot is wired with or-card elements', async () => {
    await (el as any).updateComplete;
    const section = el.shadowRoot!.querySelector('section[data-component="card"]');
    expect(section).toBeTruthy();
    expect(section!.querySelectorAll('or-card').length).toBeGreaterThan(0);
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
