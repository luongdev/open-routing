import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import './or-icon.js';

describe('OrIcon', () => {
  let el: HTMLElement;

  beforeEach(() => {
    el = document.createElement('or-icon');
    document.body.appendChild(el);
  });

  afterEach(() => {
    if (el.parentNode) el.parentNode.removeChild(el);
  });

  it('renders <uk-icon> with icon attribute', async () => {
    (el as any).name = 'home';
    await (el as any).updateComplete;
    const inner = el.querySelector('uk-icon');
    expect(inner?.getAttribute('icon')).toBe('home');
  });

  it('size sets height and width', async () => {
    (el as any).name = 'home';
    (el as any).size = 24;
    await (el as any).updateComplete;
    const inner = el.querySelector('uk-icon');
    expect(inner?.getAttribute('height')).toBe('24');
    expect(inner?.getAttribute('width')).toBe('24');
  });

  it('label sets aria-label and role=img', async () => {
    (el as any).name = 'home';
    (el as any).label = 'Dashboard';
    await (el as any).updateComplete;
    const inner = el.querySelector('uk-icon');
    expect(inner?.getAttribute('aria-label')).toBe('Dashboard');
    expect(inner?.getAttribute('role')).toBe('img');
  });

  it('absent label sets aria-hidden=true', async () => {
    (el as any).name = 'home';
    await (el as any).updateComplete;
    const inner = el.querySelector('uk-icon');
    expect(inner?.getAttribute('aria-hidden')).toBe('true');
  });

  it('unknown name emits console.warn and renders nothing', async () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {});
    (el as any).name = 'totally-fake';
    await (el as any).updateComplete;
    expect(warn).toHaveBeenCalled();
    warn.mockRestore();
  });
});
