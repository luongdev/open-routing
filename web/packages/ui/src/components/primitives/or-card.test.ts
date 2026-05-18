import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import './or-card.js';

describe('OrCard', () => {
  let el: HTMLElement;

  beforeEach(() => {
    el = document.createElement('or-card');
    document.body.appendChild(el);
  });

  afterEach(() => {
    if (el.parentNode) el.parentNode.removeChild(el);
  });

  it('renders default markup with uk-card uk-card-default', async () => {
    await (el as any).updateComplete;
    const div = el.querySelector('.uk-card')!;
    expect(div).toBeTruthy();
    expect(div.className).toContain('uk-card-default');
  });

  it('applies variant class for primary variant', async () => {
    (el as any).variant = 'primary';
    await (el as any).updateComplete;
    const div = el.querySelector('.uk-card')!;
    expect(div.className).toContain('uk-card-primary');
    expect(div.className).not.toContain('uk-card-default');
  });

  it('applies uk-card-small for sm padding', async () => {
    (el as any).padding = 'sm';
    await (el as any).updateComplete;
    const div = el.querySelector('.uk-card')!;
    expect(div.className).toContain('uk-card-small');
  });

  it('applies uk-card-hover when hoverable is true', async () => {
    (el as any).hoverable = true;
    await (el as any).updateComplete;
    const div = el.querySelector('.uk-card')!;
    expect(div.className).toContain('uk-card-hover');
  });

  it('renders header, body, and footer slot containers', async () => {
    await (el as any).updateComplete;
    expect(el.querySelector('.uk-card-header')).toBeTruthy();
    expect(el.querySelector('.uk-card-body')).toBeTruthy();
    expect(el.querySelector('.uk-card-footer')).toBeTruthy();
  });
});
