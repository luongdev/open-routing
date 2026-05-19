// Playground = public design-system demo (rewritten W0.1 / d2d879d).
// Old slot/sandbox structure replaced with hero + tokens + components + preview.

import { describe, it, expect, beforeEach } from 'vitest';
import './playground-route.js';

describe('OrPlaygroundRoute', () => {
  let el: HTMLElement;

  beforeEach(() => {
    el = document.createElement('or-playground-route');
    document.body.appendChild(el);
  });

  it('renders hero with branded title', async () => {
    await (el as any).updateComplete;
    const sr = el.shadowRoot!;
    const h1 = sr.querySelector('.hero h1');
    expect(h1?.textContent).toContain('routing catalogs');
  });

  it('renders top-bar nav with three section anchors', async () => {
    await (el as any).updateComplete;
    const sr = el.shadowRoot!;
    const navLinks = sr.querySelectorAll('header.demo-top nav a');
    expect(navLinks.length).toBe(3);
    expect(navLinks[0]!.getAttribute('href')).toBe('#tokens');
    expect(navLinks[1]!.getAttribute('href')).toBe('#components');
    expect(navLinks[2]!.getAttribute('href')).toBe('#preview');
  });

  it('renders theme switch pills (Light + Dark)', async () => {
    await (el as any).updateComplete;
    const sr = el.shadowRoot!;
    const pills = sr.querySelectorAll('.theme-pill');
    expect(pills.length).toBe(2);
    expect(pills[0]!.textContent).toContain('Light');
    expect(pills[1]!.textContent).toContain('Dark');
  });

  it('renders 9 design-token swatches', async () => {
    await (el as any).updateComplete;
    const sr = el.shadowRoot!;
    const swatches = sr.querySelectorAll('.swatch');
    expect(swatches.length).toBe(9);
  });

  it('renders 4 component preview cards', async () => {
    await (el as any).updateComplete;
    const sr = el.shadowRoot!;
    const cards = sr.querySelectorAll('.preview-card');
    expect(cards.length).toBe(4);
    const headings = Array.from(cards).map((c) => c.querySelector('h3')?.textContent);
    expect(headings).toContain('Buttons');
    expect(headings).toContain('Inputs');
    expect(headings).toContain('Status badges');
    expect(headings).toContain('Avatar + identity');
  });

  it('renders admin live-preview frame with sidebar items + table', async () => {
    await (el as any).updateComplete;
    const sr = el.shadowRoot!;
    const preview = sr.querySelector('.admin-preview');
    expect(preview).toBeTruthy();
    expect(preview!.querySelectorAll('.admin-side-item').length).toBeGreaterThanOrEqual(6);
    expect(preview!.querySelector('.admin-table')).toBeTruthy();
  });

  it('dispatches open-routing:theme-change with ember-dark when Dark pill clicked', async () => {
    await (el as any).updateComplete;
    let fired: { theme?: string } | null = null;
    el.addEventListener('open-routing:theme-change', (e) => {
      fired = (e as CustomEvent).detail;
    });
    const pills = el.shadowRoot!.querySelectorAll('.theme-pill');
    (pills[1] as HTMLButtonElement).click();
    expect((fired as unknown as { theme?: string })?.theme).toBe('ember-dark');
  });

  it('dispatches open-routing:theme-change with ember-light when Light pill clicked', async () => {
    await (el as any).updateComplete;
    let fired: { theme?: string } | null = null;
    el.addEventListener('open-routing:theme-change', (e) => {
      fired = (e as CustomEvent).detail;
    });
    const pills = el.shadowRoot!.querySelectorAll('.theme-pill');
    (pills[0] as HTMLButtonElement).click();
    expect((fired as unknown as { theme?: string })?.theme).toBe('ember-light');
  });
});
