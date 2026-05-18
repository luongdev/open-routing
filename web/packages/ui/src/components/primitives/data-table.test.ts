import { describe, it, expect, beforeEach, afterEach } from 'vitest';

// Register elements (will fail until implementations exist)
import './data-table.js';
import './cursor-paginator.js';
import './code-input.js';

// ---------------------------------------------------------------------------
// or-data-table tests
// ---------------------------------------------------------------------------

describe('OrDataTable', () => {
  let el: HTMLElement;

  beforeEach(() => {
    el = document.createElement('or-data-table');
    document.body.appendChild(el);
  });

  afterEach(() => {
    if (el.parentNode) el.parentNode.removeChild(el);
  });

  it('renders <tbody> with one <tr> when rows has one item', async () => {
    (el as any).columns = [
      { key: 'code', label: 'Code' },
      { key: 'name', label: 'Name' },
    ];
    (el as any).rows = [{ code: 'a', name: 'b' }];
    await (el as any).updateComplete;

    const shadow = el.shadowRoot!;
    const tbody = shadow.querySelector('tbody');
    expect(tbody).toBeTruthy();
    const rows = tbody!.querySelectorAll('tr');
    expect(rows.length).toBe(1);
  });

  it('renders [data-testid="loading"] when loading=true', async () => {
    (el as any).loading = true;
    await (el as any).updateComplete;

    const shadow = el.shadowRoot!;
    const loadingEl = shadow.querySelector('[data-testid="loading"]');
    expect(loadingEl).toBeTruthy();
  });

  it('dispatches or-row-click event with detail.row on row click', async () => {
    const row = { code: 'test-agent', name: 'Test Agent' };
    (el as any).columns = [{ key: 'code', label: 'Code' }];
    (el as any).rows = [row];
    await (el as any).updateComplete;

    const events: CustomEvent[] = [];
    el.addEventListener('or-row-click', (e) => events.push(e as CustomEvent));

    const shadow = el.shadowRoot!;
    const tr = shadow.querySelector('tbody tr') as HTMLTableRowElement;
    expect(tr).toBeTruthy();
    tr.click();
    await (el as any).updateComplete;

    expect(events).toHaveLength(1);
    expect(events[0]?.detail?.row).toEqual(row);
    expect(events[0]?.bubbles).toBe(true);
    expect(events[0]?.composed).toBe(true);
  });
});

// ---------------------------------------------------------------------------
// or-cursor-paginator tests
// ---------------------------------------------------------------------------

describe('OrCursorPaginator', () => {
  let el: HTMLElement;

  beforeEach(() => {
    el = document.createElement('or-cursor-paginator');
    document.body.appendChild(el);
  });

  afterEach(() => {
    if (el.parentNode) el.parentNode.removeChild(el);
  });

  it('Next button is disabled when hasMore=false', async () => {
    (el as any).hasMore = false;
    await (el as any).updateComplete;

    const shadow = el.shadowRoot!;
    // Find the Next button by text content or data attribute
    const buttons = shadow.querySelectorAll('sl-button, button');
    let nextBtn: HTMLElement | null = null;
    buttons.forEach((btn) => {
      if (btn.textContent?.trim().includes('Next')) nextBtn = btn as HTMLElement;
    });
    expect(nextBtn).toBeTruthy();
    expect((nextBtn as unknown as { disabled?: boolean })?.disabled || nextBtn!.hasAttribute('disabled')).toBe(true);
  });

  it('Previous button click emits or-page-changed with direction=prev', async () => {
    // Pre-load a cursor stack so Previous is enabled
    (el as any).cursorStack = ['cursor-abc'];
    (el as any).hasMore = false;
    await (el as any).updateComplete;

    const events: CustomEvent[] = [];
    el.addEventListener('or-page-changed', (e) => events.push(e as CustomEvent));

    const shadow = el.shadowRoot!;
    const buttons = shadow.querySelectorAll('sl-button, button');
    let prevBtn: HTMLElement | null = null;
    buttons.forEach((btn) => {
      if (btn.textContent?.trim().includes('Previous') || btn.textContent?.trim().includes('Prev')) {
        prevBtn = btn as HTMLElement;
      }
    });
    expect(prevBtn).toBeTruthy();
    prevBtn!.click();
    await (el as any).updateComplete;

    expect(events).toHaveLength(1);
    expect(events[0]?.detail?.direction).toBe('prev');
  });
});

// ---------------------------------------------------------------------------
// or-code-input tests
// ---------------------------------------------------------------------------

describe('OrCodeInput', () => {
  let el: HTMLElement;

  beforeEach(() => {
    el = document.createElement('or-code-input');
    document.body.appendChild(el);
  });

  afterEach(() => {
    if (el.parentNode) el.parentNode.removeChild(el);
  });

  it('passes validation for a value that matches the code regex (valid_code)', async () => {
    (el as any).value = 'valid_code';
    await (el as any).updateComplete;

    const result = (el as any).validate();
    expect(result).toBe(true);
  });

  it('fails validation for a value starting with a digit (123invalid)', async () => {
    (el as any).value = '123invalid';
    await (el as any).updateComplete;

    const result = (el as any).validate();
    expect(result).toBe(false);
  });
});
