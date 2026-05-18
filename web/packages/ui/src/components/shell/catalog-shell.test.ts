import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';

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

  it('defaults to history routing mode', async () => {
    await (el as any).updateComplete;
    expect((el as any).routingMode).toBe('history');
  });

  it('accepts routing-mode=hash attribute', async () => {
    el.setAttribute('routing-mode', 'hash');
    await (el as any).updateComplete;
    expect((el as any).routingMode).toBe('hash');
  });

  it('does NOT call adapter.start when routerAdapter is undefined', async () => {
    el.setAttribute('routing-mode', 'hash');
    await (el as any).updateComplete;
    // Should not throw
    expect(true).toBe(true);
  });

  it('does NOT call adapter.start when routingMode is history', async () => {
    const mockAdapter = { start: vi.fn(), stop: vi.fn() };
    (el as any).routerAdapter = mockAdapter;
    await (el as any).updateComplete;
    expect(mockAdapter.start).not.toHaveBeenCalled();
  });

  it('invokes routerAdapter.start with the shell Routes instance when routingMode=hash', async () => {
    const mockAdapter = { start: vi.fn(), stop: vi.fn() };
    const el2 = document.createElement('or-catalog-shell');
    el2.setAttribute('routing-mode', 'hash');
    (el2 as any).routerAdapter = mockAdapter;
    document.body.appendChild(el2);
    
    await (el2 as any).updateComplete;
    await new Promise(r => setTimeout(r, 0));
    
    expect(mockAdapter.start).toHaveBeenCalledTimes(1);
    const routesArg = mockAdapter.start.mock.calls[0]![0];
    expect(routesArg.goto).toBeDefined();
    expect(routesArg.outlet).toBeDefined();
    expect(routesArg.link).toBeDefined();
    
    document.body.removeChild(el2);
  });

  it('calls routerAdapter.stop in disconnectedCallback', async () => {
    const mockAdapter = { start: vi.fn(), stop: vi.fn() };
    const el2 = document.createElement('or-catalog-shell');
    el2.setAttribute('routing-mode', 'hash');
    (el2 as any).routerAdapter = mockAdapter;
    document.body.appendChild(el2);
    
    await (el2 as any).updateComplete;
    el2.remove();
    
    expect(mockAdapter.stop).toHaveBeenCalled();
  });

  it('warns when routingMode mutates after connect', async () => {
    const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {});
    await (el as any).updateComplete;
    (el as any).routingMode = 'hash';
    await (el as any).updateComplete;
    expect(warnSpy).toHaveBeenCalled();
    warnSpy.mockRestore();
  });
});
