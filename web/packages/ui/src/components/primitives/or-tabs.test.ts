import { describe, it, expect, beforeEach, afterEach } from 'vitest';

import './or-tabs.js';
import type { Tab } from './or-tabs.js';

const HEALTH_TABS: Tab[] = [
  { id: 'summary',  label: 'Summary' },
  { id: 'history',  label: 'History' },
  { id: 'labs',     label: 'Labs' },
];

describe('OrTabs', () => {
  let el: HTMLElement;

  beforeEach(() => {
    el = document.createElement('or-tabs');
    document.body.appendChild(el);
  });

  afterEach(() => {
    if (el.parentNode) el.parentNode.removeChild(el);
  });

  it('renders a li per tab with correct labels', async () => {
    (el as any).tabs = HEALTH_TABS;
    (el as any).activeTab = 'summary';
    await (el as any).updateComplete;

    const items = el.querySelectorAll('ul.uk-tab li');
    expect(items.length).toBe(3);
    expect(items[0]?.textContent?.trim()).toBe('Summary');
    expect(items[1]?.textContent?.trim()).toBe('History');
    expect(items[2]?.textContent?.trim()).toBe('Labs');
  });

  it('applies uk-active class to the active tab only', async () => {
    (el as any).tabs = HEALTH_TABS;
    (el as any).activeTab = 'history';
    await (el as any).updateComplete;

    const items = el.querySelectorAll('ul.uk-tab li');
    expect(items[0]?.classList.contains('uk-active')).toBe(false);
    expect(items[1]?.classList.contains('uk-active')).toBe(true);
    expect(items[2]?.classList.contains('uk-active')).toBe(false);
  });

  it('emits or-tab-change with { id } on tab click', async () => {
    (el as any).tabs = HEALTH_TABS;
    (el as any).activeTab = 'summary';
    await (el as any).updateComplete;

    const events: CustomEvent[] = [];
    el.addEventListener('or-tab-change', (e) => events.push(e as CustomEvent));

    const anchors = el.querySelectorAll('ul.uk-tab li a');
    (anchors[2] as HTMLElement)?.click();
    await (el as any).updateComplete;

    expect(events).toHaveLength(1);
    expect(events[0]?.detail?.id).toBe('labs');
    expect(events[0]?.bubbles).toBe(true);
    expect(events[0]?.composed).toBe(true);
  });

  it('does not emit or-tab-change when disabled tab is clicked', async () => {
    const tabs: Tab[] = [
      { id: 'a', label: 'A' },
      { id: 'b', label: 'B', disabled: true },
    ];
    (el as any).tabs = tabs;
    (el as any).activeTab = 'a';
    await (el as any).updateComplete;

    const events: CustomEvent[] = [];
    el.addEventListener('or-tab-change', (e) => events.push(e as CustomEvent));

    const anchors = el.querySelectorAll('ul.uk-tab li a');
    (anchors[1] as HTMLElement)?.click();
    await (el as any).updateComplete;

    expect(events).toHaveLength(0);
  });

  it('hides panel for inactive tabs and shows panel for active tab', async () => {
    (el as any).tabs = HEALTH_TABS;
    (el as any).activeTab = 'labs';
    await (el as any).updateComplete;

    const panels = el.querySelectorAll('[role="tabpanel"]');
    expect((panels[0] as HTMLElement)?.hidden).toBe(true);
    expect((panels[1] as HTMLElement)?.hidden).toBe(true);
    expect((panels[2] as HTMLElement)?.hidden).toBe(false);
  });
});
