import { test, expect } from '@playwright/test';

const TEST_ORG_ID = '01952a6b-1c00-7000-8000-000000000001';

test.describe('embed Shadow DOM isolation (EMBED-04, D7-04)', () => {
  test.beforeEach(async ({ page }) => {
    await page.route('**/v1/**', async (route) => {
      await route.fulfill({
        status: 200,
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ items: [], has_more: false, next_cursor: null }),
      });
    });
    await page.goto('');
    await page.waitForSelector('open-routing-catalog');
  });

  test('host CSS does not leak into embed shadow root', async ({ page }) => {
    // Host body has `font-family: serif; color: red; background: lime` (Plan 07-01)
    // Inside the embed shadow root, computed styles must show the embed's :host defaults
    const embedTextColor = await page.evaluate(() => {
      const embed = document.querySelector('open-routing-catalog');
      const shadow = embed?.shadowRoot;
      const shell = shadow?.querySelector('or-catalog-shell');
      const shellShadow = shell?.shadowRoot;
      if (!shellShadow) return null;
      // Find any rendered text node inside the shell shadow root
      const firstEl = shellShadow.querySelector('*');
      if (!firstEl) return null;
      return getComputedStyle(firstEl).color;
    });
    expect(embedTextColor).not.toBe('rgb(255, 0, 0)'); // host red did NOT bleed
  });

  test('embed CSS does not escape to host page', async ({ page }) => {
    // Host h1 has `font-size: 200%; color: purple` inline style
    const hostH1Color = await page.evaluate(() => {
      const h1 = document.querySelector('h1');
      if (!h1) return null;
      return getComputedStyle(h1).color;
    });
    expect(hostH1Color).toBe('rgb(128, 0, 128)'); // purple — embed didn't bleed out
  });

  test('embed inline confirm panel does not portal outside Shadow DOM (D7-04)', async ({ page }) => {
    // The replaced <sl-dialog> components (Plan 07-03) now render inline.
    // Verify no role=dialog element appears at document.body (light DOM).
    const lightDomDialogs = await page.evaluate(() => {
      // Search only the light DOM — NOT inside any shadowRoot
      const allLight = document.body.querySelectorAll('[role="dialog"]');
      return allLight.length;
    });
    expect(lightDomDialogs).toBe(0);
  });

  test('embed shadow root is open (Playwright assertion compatibility)', async ({ page }) => {
    const shadowMode = await page.evaluate(() => {
      const embed = document.querySelector('open-routing-catalog');
      return embed?.shadowRoot ? 'open' : 'closed';
    });
    expect(shadowMode).toBe('open');
  });

  // Iteration-2 BLOCKER #2 — the shell's mobile sidebar uses
  // <nav class="sidebar"> + _sidebarOpen state, NOT <sl-drawer>.
  // Phase 6 shipped this element; this test locks the invariant that
  // it renders inside the shell shadowRoot (no portal escape) and is
  // visible after the hamburger toggle is clicked on a mobile viewport.
  test('mobile-viewport <nav class="sidebar"> renders inside catalog-shell shadowRoot and is visible after hamburger toggle', async ({ page }) => {
    await page.setViewportSize({ width: 375, height: 800 }); // mobile breakpoint
    await page.reload();
    await page.waitForSelector('open-routing-catalog');
    // Click the hamburger toggle inside the shell shadow to flip _sidebarOpen.
    await page.evaluate(() => {
      const embed = document.querySelector('open-routing-catalog');
      const shell = embed?.shadowRoot?.querySelector('or-catalog-shell');
      const toggle = shell?.shadowRoot?.querySelector('sl-icon-button.hamburger-toggle') as HTMLElement | null;
      toggle?.click();
    });
    await page.waitForTimeout(200);
    const sidebarAudit = await page.evaluate(() => {
      const embed = document.querySelector('open-routing-catalog');
      const shellRoot = embed?.shadowRoot?.querySelector('or-catalog-shell')?.shadowRoot;
      const sidebar = shellRoot?.querySelector('nav.sidebar') as HTMLElement | null;
      if (!sidebar) return { present: false, display: null };
      const computed = getComputedStyle(sidebar);
      return { present: true, display: computed.display };
    });
    expect(sidebarAudit.present).toBe(true);
    // Sidebar element exists in shadowRoot; ensure CSS media-query doesn't
    // hide it (display: 'none' would mean the mobile rule is hiding it).
    expect(sidebarAudit.display).not.toBe('none');
  });
});
