import { test, expect } from '@playwright/test';

const TEST_ORG_ID = '01952a6b-1c00-7000-8000-000000000001';

type AuthExpiredEventAudit = {
  bubbles: boolean;
  composed: boolean;
  detail: {
    statusCode?: number;
    requestId?: string;
    path?: string;
  };
};

test.describe('auth-expired event (EMBED-07, D7-14)', () => {
  test('host receives composed auth-expired event with request details on 401', async ({ page }) => {
    await page.addInitScript(() => {
      const hostWindow = window as unknown as { __authExpiredEvents: AuthExpiredEventAudit[] };
      hostWindow.__authExpiredEvents = [];
      document.addEventListener('open-routing:auth-expired', (event) => {
        const customEvent = event as CustomEvent;
        hostWindow.__authExpiredEvents.push({
          bubbles: customEvent.bubbles,
          composed: customEvent.composed,
          detail: customEvent.detail,
        });
      });
    });

    await page.route('**/v1/**', async (route) => {
      await route.fulfill({
        status: 401,
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ error: 'auth_expired', request_id: 'rid-e2e-401' }),
      });
    });

    await page.goto(`./#open-routing/orgs/${TEST_ORG_ID}/agents`);
    await page.waitForSelector('open-routing-catalog');
    await expect.poll(async () => {
      return page.evaluate(() => {
        return ((window as unknown as { __authExpiredEvents?: AuthExpiredEventAudit[] }).__authExpiredEvents ?? []).length;
      });
    }).toBeGreaterThan(0);

    const [event] = await page.evaluate(() => {
      return (window as unknown as { __authExpiredEvents: AuthExpiredEventAudit[] }).__authExpiredEvents;
    });
    expect(event?.bubbles).toBe(true);
    expect(event?.composed).toBe(true);
    expect(event?.detail.statusCode).toBe(401);
    expect(event?.detail.requestId).toBe('rid-e2e-401');
    expect(event?.detail.path).toMatch(/\/v1\//);
  });
});
