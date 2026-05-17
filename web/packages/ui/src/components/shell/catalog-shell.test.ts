import { describe, it, expect, beforeEach, afterEach } from 'vitest';

// Register the element
import './catalog-shell.js';

describe('OrCatalogShell', () => {
  let el: HTMLElement;

  beforeEach(() => {
    el = document.createElement('or-catalog-shell');
    document.body.appendChild(el);
  });

  afterEach(() => {
    if (el.parentNode) {
      el.parentNode.removeChild(el);
    }
  });

  it('sets --sl-color-primary-500 to #4faab2 when theme is or-dark', async () => {
    (el as any).theme = 'or-dark';
    await (el as any).updateComplete;
    expect((el as HTMLElement).style.getPropertyValue('--sl-color-primary-500')).toBe('#4faab2');
  });

  it('sets --sl-color-primary-500 to #0d8b96 when theme is or-brand', async () => {
    (el as any).theme = 'or-brand';
    await (el as any).updateComplete;
    expect((el as HTMLElement).style.getPropertyValue('--sl-color-primary-500')).toBe('#0d8b96');
  });

  it('renders sidebar with aria-label Navigation containing all 8 nav entries', async () => {
    (el as any).orgId = 'test-org';
    await (el as any).updateComplete;
    const shadow = el.shadowRoot;
    const nav = shadow?.querySelector('[aria-label="Navigation"]');
    expect(nav).toBeTruthy();
    const navText = nav?.textContent ?? '';
    expect(navText).toContain('Agents');
    expect(navText).toContain('Skills');
    expect(navText).toContain('Queues');
    expect(navText).toContain('Channels');
    expect(navText).toContain('Adapters');
    expect(navText).toContain('Break Reasons');
    expect(navText).toContain('Bulk Import');
    expect(navText).toContain('Agent Status');
  });

  it('hides filtered nav entries when modules property is set', async () => {
    (el as any).orgId = 'test-org';
    (el as any).modules = 'agents,skills';
    await (el as any).updateComplete;
    const shadow = el.shadowRoot;
    const nav = shadow?.querySelector('[aria-label="Navigation"]');
    const navText = nav?.textContent ?? '';
    // agents and skills should still be visible
    expect(navText).toContain('Agents');
    expect(navText).toContain('Skills');
    // queues, channels, adapters, break-reasons should be hidden
    expect(navText).not.toContain('Queues');
    expect(navText).not.toContain('Channels');
    expect(navText).not.toContain('Adapters');
    expect(navText).not.toContain('Break Reasons');
  });
});
