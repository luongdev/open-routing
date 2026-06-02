import { describe, it, expect, beforeEach, afterEach } from 'vitest';

import './or-badge.js';

describe('OrBadge', () => {
  let el: HTMLElement;

  beforeEach(() => {
    el = document.createElement('or-badge');
    document.body.appendChild(el);
  });

  afterEach(() => {
    if (el.parentNode) el.parentNode.removeChild(el);
  });

  it('renders default variant with uk-label class only', async () => {
    await (el as any).updateComplete;
    const span = el.querySelector('span');
    expect(span).toBeTruthy();
    expect(span!.classList.contains('uk-label')).toBe(true);
    expect(span!.className).toBe('uk-label');
  });

  it('renders success variant with uk-label-success class', async () => {
    (el as any).variant = 'success';
    await (el as any).updateComplete;
    const span = el.querySelector('span');
    expect(span!.classList.contains('uk-label-success')).toBe(true);
  });

  it('renders warning variant with uk-label-warning class', async () => {
    (el as any).variant = 'warning';
    await (el as any).updateComplete;
    const span = el.querySelector('span');
    expect(span!.classList.contains('uk-label-warning')).toBe(true);
  });

  it('renders destructive variant with uk-label-destructive class', async () => {
    (el as any).variant = 'destructive';
    await (el as any).updateComplete;
    const span = el.querySelector('span');
    expect(span!.classList.contains('uk-label-destructive')).toBe(true);
  });

  it('renders info variant with uk-label-info class', async () => {
    (el as any).variant = 'info';
    await (el as any).updateComplete;
    const span = el.querySelector('span');
    expect(span!.classList.contains('uk-label-info')).toBe(true);
  });

  it('renders primary variant with uk-label-primary class', async () => {
    (el as any).variant = 'primary';
    await (el as any).updateComplete;
    const span = el.querySelector('span');
    expect(span!.classList.contains('uk-label-primary')).toBe(true);
  });

  it('adds uk-label-pill class when pill=true', async () => {
    (el as any).pill = true;
    await (el as any).updateComplete;
    const span = el.querySelector('span');
    expect(span!.classList.contains('uk-label-pill')).toBe(true);
  });

  it('adds uk-label-dot class and suppresses slot when dot=true', async () => {
    (el as any).dot = true;
    await (el as any).updateComplete;
    const span = el.querySelector('span');
    expect(span!.classList.contains('uk-label-dot')).toBe(true);
    expect(span!.querySelector('slot')).toBeNull();
  });
});
