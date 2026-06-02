import { test, expect } from '@playwright/test';

const TEST_ORG_ID = '01952a6b-1c00-7000-8000-000000000001';

test.describe('embed modules filter (EMBED-06)', () => {
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

  test('sidebar nav contains only listed modules (agents,skills)', async ({ page }) => {
    const visibleEntries = await page.evaluate(() => {
      const embed = document.querySelector('open-routing-catalog');
      const shellShadow = embed?.shadowRoot?.querySelector('or-catalog-shell')?.shadowRoot;
      if (!shellShadow) return null;
      const buttons = shellShadow.querySelectorAll('nav button[data-entity]');
      const keys: string[] = [];
      buttons.forEach((el) => {
        const key = el.getAttribute('data-entity');
        if (key) keys.push(key);
      });
      return keys;
    });
    expect(visibleEntries).not.toBeNull();
    expect(visibleEntries).toContain('agents');
    expect(visibleEntries).toContain('skills');
    expect(visibleEntries).not.toContain('channels');
    expect(visibleEntries).not.toContain('adapters');
    expect(visibleEntries).not.toContain('break-reasons');
  });
});
