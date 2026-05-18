import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import './or-dropdown.js';
import type { DropdownItem } from './or-dropdown.js';

describe('OrDropdown', () => {
  let el: HTMLElement;

  beforeEach(() => {
    el = document.createElement('or-dropdown');
    document.body.appendChild(el);
  });

  afterEach(() => {
    if (el.parentNode) el.parentNode.removeChild(el);
  });

  const setItems = async (items: DropdownItem[]) => {
    (el as any).items = items;
    await (el as any).updateComplete;
  };

  it('renders menu items in a uk-nav uk-dropdown-nav list', async () => {
    await setItems([
      { id: 'edit', label: 'Edit' },
      { id: 'delete', label: 'Delete' },
    ]);
    const ul = el.querySelector('ul.uk-nav.uk-dropdown-nav');
    expect(ul).toBeTruthy();
    const items = ul!.querySelectorAll('li a[role="menuitem"]');
    expect(items.length).toBe(2);
    expect(items[0]!.textContent?.trim()).toBe('Edit');
    expect(items[1]!.textContent?.trim()).toBe('Delete');
  });

  it('renders a divider as li.uk-nav-divider', async () => {
    await setItems([
      { id: 'profile', label: 'Profile' },
      { divider: true },
      { id: 'logout', label: 'Log out' },
    ]);
    const divider = el.querySelector('.uk-nav-divider');
    expect(divider).toBeTruthy();
    expect(divider?.getAttribute('role')).toBe('separator');
    const menuItems = el.querySelectorAll('a[role="menuitem"]');
    expect(menuItems.length).toBe(2);
  });

  it('click on item dispatches or-dropdown-select with detail.id', async () => {
    await setItems([{ id: 'settings', label: 'Settings' }]);
    const events: CustomEvent[] = [];
    el.addEventListener('or-dropdown-select', (e) => events.push(e as CustomEvent));

    (el as any)._open = true;
    await (el as any).updateComplete;

    const link = el.querySelector<HTMLElement>('a[role="menuitem"]');
    link?.click();

    expect(events.length).toBe(1);
    expect(events[0]!.detail.id).toBe('settings');
  });

  it('disabled item does not dispatch or-dropdown-select', async () => {
    await setItems([{ id: 'restricted', label: 'Restricted', disabled: true }]);
    const events: CustomEvent[] = [];
    el.addEventListener('or-dropdown-select', (e) => events.push(e as CustomEvent));

    (el as any)._open = true;
    await (el as any).updateComplete;

    const link = el.querySelector<HTMLElement>('a[role="menuitem"]');
    link?.click();

    expect(events.length).toBe(0);
  });

  it('disabled item has uk-disabled class on li', async () => {
    await setItems([{ id: 'gone', label: 'Gone', disabled: true }]);
    const li = el.querySelector('li.uk-nav-item');
    expect(li?.className).toContain('uk-disabled');
  });

  it('destructive item has danger color style', async () => {
    await setItems([{ id: 'remove', label: 'Remove', destructive: true }]);
    (el as any)._open = true;
    await (el as any).updateComplete;
    const link = el.querySelector<HTMLElement>('a[role="menuitem"]');
    expect(link?.getAttribute('style')).toContain('var(--uk-danger-f)');
  });
});
