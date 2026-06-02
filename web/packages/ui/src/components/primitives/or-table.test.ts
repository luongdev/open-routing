import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import './or-table.js';

describe('OrTable', () => {
  let el: HTMLElement;

  beforeEach(() => {
    el = document.createElement('or-table');
    document.body.appendChild(el);
  });

  afterEach(() => {
    if (el.parentNode) el.parentNode.removeChild(el);
  });

  it('renders <table class="uk-table"> by default', async () => {
    await (el as any).updateComplete;
    const table = el.querySelector('table');
    expect(table).toBeTruthy();
    expect(table!.className).toBe('uk-table');
  });

  it('adds uk-table-striped when striped=true', async () => {
    (el as any).striped = true;
    await (el as any).updateComplete;
    expect(el.querySelector('table')!.className).toContain('uk-table-striped');
  });

  it('adds uk-table-hover when hover=true', async () => {
    (el as any).hover = true;
    await (el as any).updateComplete;
    expect(el.querySelector('table')!.className).toContain('uk-table-hover');
  });

  it('adds uk-table-divider and uk-table-small when both props are true', async () => {
    (el as any).divider = true;
    (el as any).small = true;
    await (el as any).updateComplete;
    const cls = el.querySelector('table')!.className;
    expect(cls).toContain('uk-table-divider');
    expect(cls).toContain('uk-table-small');
  });

  it('wraps in uk-overflow-auto div when responsive=true', async () => {
    (el as any).responsive = true;
    await (el as any).updateComplete;
    const wrapper = el.querySelector('.uk-overflow-auto');
    expect(wrapper).toBeTruthy();
    expect(wrapper!.querySelector('table')).toBeTruthy();
  });

  it('renders <thead> and <tbody> inside the table structure', async () => {
    await (el as any).updateComplete;
    const table = el.querySelector('table');
    expect(table).toBeTruthy();
    expect(table!.querySelector('thead')).toBeTruthy();
    expect(table!.querySelector('tbody')).toBeTruthy();
  });
});
