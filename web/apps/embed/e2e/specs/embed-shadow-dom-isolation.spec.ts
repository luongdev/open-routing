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
    await page.goto(`./#open-routing/orgs/${TEST_ORG_ID}/agents`);
    await page.waitForSelector('open-routing-catalog');
  });

  test('host CSS does not leak into embed shadow root', async ({ page }) => {
    const embedTextColor = await page.evaluate(() => {
      const embed = document.querySelector('open-routing-catalog');
      const shadow = embed?.shadowRoot;
      const shell = shadow?.querySelector('or-catalog-shell');
      const shellShadow = shell?.shadowRoot;
      if (!shellShadow) return null;
      const firstEl = shellShadow.querySelector('.app-grid');
      if (!firstEl) return null;
      return getComputedStyle(firstEl).color;
    });
    expect(embedTextColor).not.toBe('rgb(255, 0, 0)');
  });

  test('embed CSS does not escape to host page', async ({ page }) => {
    const hostH1Color = await page.evaluate(() => {
      const h1 = document.querySelector('h1');
      if (!h1) return null;
      return getComputedStyle(h1).color;
    });
    expect(hostH1Color).toBe('rgb(128, 0, 128)');
  });

  test('embed inline confirm panel does not portal outside Shadow DOM (D7-04)', async ({ page }) => {
    const lightDomDialogs = await page.evaluate(() => {
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

  test('mobile-viewport sidebar renders inside catalog-shell shadowRoot and is visible after hamburger toggle', async ({ page }) => {
    await page.setViewportSize({ width: 375, height: 800 });
    await page.reload();
    await page.waitForSelector('open-routing-catalog');
    await page.evaluate(() => {
      const embed = document.querySelector('open-routing-catalog');
      const shell = embed?.shadowRoot?.querySelector('or-catalog-shell');
      const toggle = shell?.shadowRoot?.querySelector('button.hamburger') as HTMLElement | null;
      toggle?.click();
    });
    await page.waitForTimeout(200);
    const sidebarAudit = await page.evaluate(() => {
      const embed = document.querySelector('open-routing-catalog');
      const shellRoot = embed?.shadowRoot?.querySelector('or-catalog-shell')?.shadowRoot;
      const sidebar = shellRoot?.querySelector('aside.sidebar') as HTMLElement | null;
      if (!sidebar) return { present: false, display: null };
      const computed = getComputedStyle(sidebar);
      return { present: true, display: computed.display };
    });
    expect(sidebarAudit.present).toBe(true);
    expect(sidebarAudit.display).not.toBe('none');
  });
});
