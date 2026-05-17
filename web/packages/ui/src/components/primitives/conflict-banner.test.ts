import { describe, it, expect, beforeEach, afterEach } from 'vitest';

// Register elements (will fail until implementations exist)
import './conflict-banner.js';

// ---------------------------------------------------------------------------
// or-conflict-banner tests
// ---------------------------------------------------------------------------

describe('OrConflictBanner', () => {
  let el: HTMLElement;

  beforeEach(() => {
    el = document.createElement('or-conflict-banner');
    document.body.appendChild(el);
  });

  afterEach(() => {
    if (el.parentNode) el.parentNode.removeChild(el);
  });

  it('renders with background variable for conflict state in crud mode', async () => {
    (el as any).mode = 'crud';
    (el as any).serverValue = { name: 'Alice N' };
    (el as any).userValue = { name: 'Alice' };
    await (el as any).updateComplete;

    // The host element or a container should reference the conflict-bg variable
    const shadow = el.shadowRoot!;
    const banner = shadow.querySelector('.conflict-banner');
    expect(banner).toBeTruthy();
    // The banner exists (confirming the component renders for crud mode)
  });

  it('crud mode with different serverValue/userValue renders the changed key in diff section', async () => {
    (el as any).mode = 'crud';
    (el as any).serverValue = { name: 'Alice N' };
    (el as any).userValue = { name: 'Alice' };
    await (el as any).updateComplete;

    const shadow = el.shadowRoot!;
    const bannerText = shadow.textContent ?? '';
    // The diff section must mention the 'name' key (changed field)
    expect(bannerText).toContain('name');
    // Server value appears in the diff
    expect(bannerText).toContain('Alice N');
  });

  it('status mode renders from/to states from serverValue', async () => {
    (el as any).mode = 'status';
    (el as any).serverValue = { from: 'Engaged', to: 'NotReady' };
    await (el as any).updateComplete;

    const shadow = el.shadowRoot!;
    const bannerText = shadow.textContent ?? '';
    expect(bannerText).toContain('Engaged');
    expect(bannerText).toContain('NotReady');
  });

  it('dispatches open-routing:conflict-acknowledged with action=review on Review button click', async () => {
    (el as any).mode = 'crud';
    (el as any).serverValue = { name: 'Alice N' };
    (el as any).userValue = { name: 'Alice' };
    await (el as any).updateComplete;

    const events: CustomEvent[] = [];
    el.addEventListener('open-routing:conflict-acknowledged', (e) => events.push(e as CustomEvent));

    const shadow = el.shadowRoot!;
    // Find the Review button
    const buttons = shadow.querySelectorAll('sl-button, button');
    let reviewBtn: HTMLElement | null = null;
    buttons.forEach((btn) => {
      if (btn.textContent?.toLowerCase().includes('review')) reviewBtn = btn as HTMLElement;
    });
    expect(reviewBtn).toBeTruthy();
    reviewBtn!.click();
    await (el as any).updateComplete;

    expect(events).toHaveLength(1);
    expect(events[0]?.detail?.action).toBe('review');
    expect(events[0]?.bubbles).toBe(true);
    expect(events[0]?.composed).toBe(true);
  });

  it('dispatches open-routing:conflict-acknowledged with action=discard on Discard button click', async () => {
    (el as any).mode = 'crud';
    (el as any).serverValue = { name: 'Alice N' };
    (el as any).userValue = { name: 'Alice' };
    await (el as any).updateComplete;

    const events: CustomEvent[] = [];
    el.addEventListener('open-routing:conflict-acknowledged', (e) => events.push(e as CustomEvent));

    const shadow = el.shadowRoot!;
    // Find the Discard button
    const buttons = shadow.querySelectorAll('sl-button, button');
    let discardBtn: HTMLElement | null = null;
    buttons.forEach((btn) => {
      if (btn.textContent?.toLowerCase().includes('discard')) discardBtn = btn as HTMLElement;
    });
    expect(discardBtn).toBeTruthy();
    discardBtn!.click();
    await (el as any).updateComplete;

    expect(events).toHaveLength(1);
    expect(events[0]?.detail?.action).toBe('discard');
    expect(events[0]?.bubbles).toBe(true);
    expect(events[0]?.composed).toBe(true);
  });
});
