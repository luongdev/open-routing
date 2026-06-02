import { test, expect } from '@playwright/test';

const TEST_ORG_ID = '01952a6b-1c00-7000-8000-000000000001';

test.describe('X-Org-Id propagation (EMBED-02)', () => {
  test('every API call carries X-Org-Id from org-id attribute', async ({ page }) => {
    const orgIds: string[] = [];

    await page.route('**/v1/**', async (route) => {
      orgIds.push(route.request().headers()['x-org-id'] ?? '');
      await route.fulfill({
        status: 200,
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ items: [], has_more: false, next_cursor: null }),
      });
    });

    await page.goto(`./#open-routing/orgs/${TEST_ORG_ID}/agents`);
    await page.waitForSelector('open-routing-catalog');
    await expect.poll(() => orgIds.length).toBeGreaterThan(0);

    expect(orgIds.every((orgId) => orgId === TEST_ORG_ID)).toBe(true);
  });
});
