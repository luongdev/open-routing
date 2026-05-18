// Phase 6 Plan 06-06: Admin SPA smoke test (D6-30).
// Covers: org-picker → agent list → agent create wizard → verify agent in list.
//
// This test runs MANUALLY in Phase 6 (developer runs `pnpm test:e2e` locally
// with the Go API running). Phase 7 (EMBED-10) promotes it to CI.
//
// Prerequisites to run this test:
//   1. Go API running: `task dev` in services/api directory (port 8080)
//   2. Vite dev server: `pnpm --filter @open-routing/admin dev` (port 5173)
//   3. TEST_ORG_ID env var set to a valid org UUID that exists in the test DB,
//      or use the default dev fixture UUID below.
//
// D6-30: The test UUID is a dev fixture — not production data.
// T-06-06-02: hardcoded test org_id is accepted (dev fixture, no production data).

import { test, expect } from '@playwright/test';

// Dev fixture org UUID — set TEST_ORG_ID env var to override with a real org.
// Must be a valid UUIDv7 format: version byte = 7, variant byte = [89ab].
const TEST_ORG_ID =
  process.env.TEST_ORG_ID ?? '01952a6b-1c00-7000-8000-000000000001';

// Unique agent code for this test run (timestamp suffix to avoid duplicate_code conflicts).
const TEST_AGENT_CODE = `smoke${Date.now().toString(36)}`;
const TEST_AGENT_NAME = `Smoke Test Agent ${TEST_AGENT_CODE}`;
const TEST_AGENT_EMAIL = `${TEST_AGENT_CODE}@test.open-routing.io`;

test.describe('Admin SPA smoke test', () => {
  test('org-picker → enter UUID → agents list loads', async ({ page }) => {
    // Navigate to root — expect org-picker
    await page.goto('/');

    // The org-picker should be visible
    const orgPickerHeading = page.locator('or-org-picker').first();
    await expect(orgPickerHeading).toBeAttached({ timeout: 10_000 });

    // Enter org_id into the UUIDv7 input (org-picker component renders an input in shadow DOM)
    const orgPicker = page.locator('or-catalog-shell or-org-picker');
    await expect(orgPicker).toBeAttached({ timeout: 5_000 });

    // Access shadow DOM of or-org-picker to find the input
    const orgInput = page.locator('or-org-picker').locator(':scope >> input[type="text"], :scope >> sl-input').first();
    // Fallback: try direct shadow root access via evaluate
    await page.evaluate((orgId: string) => {
      // Find the or-org-picker element in or-catalog-shell shadow root
      const shell = document.querySelector('or-catalog-shell');
      const shellShadow = shell?.shadowRoot;
      const picker = shellShadow?.querySelector('or-org-picker');
      const pickerShadow = picker?.shadowRoot;
      // or-org-picker renders a native input
      const nativeInput = pickerShadow?.querySelector('input');
      if (nativeInput) {
        nativeInput.value = orgId;
        nativeInput.dispatchEvent(new Event('input', { bubbles: true, composed: true }));
        nativeInput.dispatchEvent(new Event('change', { bubbles: true, composed: true }));
      }
    }, TEST_ORG_ID);

    // Click the Continue button inside or-org-picker shadow root
    await page.evaluate(() => {
      const shell = document.querySelector('or-catalog-shell');
      const picker = shell?.shadowRoot?.querySelector('or-org-picker');
      const btn = picker?.shadowRoot?.querySelector('.continue-btn') as HTMLElement | null;
      if (btn) btn.click();
      else console.error('Playwright: .continue-btn not found!');
      
      setTimeout(() => {
        const err = picker?.shadowRoot?.querySelector('.validation-error');
        if (err) console.error('Playwright Error found:', err.textContent);
      }, 500);
    });

    // After submitting, shell should navigate to /orgs/:org_id/agents
    await page.waitForURL(`**/orgs/${TEST_ORG_ID}/agents`, { timeout: 10_000 });

    // Agents list component should be rendered
    await expect(page.locator('or-agent-list')).toBeAttached({ timeout: 10_000 });
  });

  test('direct navigation to /orgs/:org_id/agents bypasses org-picker', async ({ page }) => {
    // Navigate directly to org/agents URL (bookmarked link scenario)
    await page.goto(`/orgs/${TEST_ORG_ID}/agents`);

    // Shell should render agent list (not redirect to org-picker for valid UUIDv7)
    await expect(page.locator('or-agent-list')).toBeAttached({ timeout: 10_000 });
  });

  test('malformed org_id in URL redirects to org-picker', async ({ page }) => {
    // UUIDv4 (version 4, not 7) should be rejected by the UUIDv7 route guard
    await page.goto('/orgs/not-a-valid-uuid/agents');

    // Should redirect to root (org-picker)
    await page.waitForURL('**/', { timeout: 5_000 });
    await expect(page.locator('or-org-picker')).toBeAttached({ timeout: 5_000 });
  });

  test('agent create wizard flow (requires live API)', async ({ page }) => {
    // Navigate directly to agents list
    await page.goto(`/orgs/${TEST_ORG_ID}/agents`);
    await expect(page.locator('or-agent-list')).toBeAttached({ timeout: 10_000 });

    // Click [+ Create Agent] button inside or-agent-list shadow DOM
    await page.evaluate(() => {
      const shell = document.querySelector('or-catalog-shell');
      const shellShadow = shell?.shadowRoot;
      const main = shellShadow?.querySelector('main.content');
      const agentList = main?.querySelector('or-agent-list');
      const agentListShadow = agentList?.shadowRoot;
      // Find the create agent button — or-agent-list renders <sl-button> with "Create agent" label
      const buttons = agentListShadow?.querySelectorAll('sl-button, button');
      const createBtn = Array.from(buttons ?? []).find(
        (b) => b.textContent?.toLowerCase().includes('create agent')
      ) as HTMLElement | null;
      createBtn?.click();
    });

    // Should navigate to /orgs/:org_id/agents/new
    await page.waitForURL(`**/orgs/${TEST_ORG_ID}/agents/new`, { timeout: 5_000 });
    await expect(page.locator('or-agent-form')).toBeAttached({ timeout: 5_000 });

    // Step 1: Fill agent basics (code, name, email)
    await page.evaluate(
      ({ code, name, email }: { code: string; name: string; email: string }) => {
        const shell = document.querySelector('or-catalog-shell');
        const shellShadow = shell?.shadowRoot;
        const main = shellShadow?.querySelector('main.content');
        const agentForm = main?.querySelector('or-agent-form');
        const formShadow = agentForm?.shadowRoot;

        // Fill code field via or-code-input shadow DOM
        const codeInput = formShadow?.querySelector('or-code-input');
        if (codeInput) {
          const codeInputShadow = codeInput.shadowRoot;
          const input = codeInputShadow?.querySelector('sl-input, input') as HTMLElement & { value?: string } | null;
          if (input) {
            input.value = code;
            input.dispatchEvent(new Event('sl-input', { bubbles: true, composed: true }));
          }
        }

        // Fill name field
        const nameInput = formShadow?.querySelector('sl-input[name="name"], sl-input[label="Name"]') as HTMLElement & { value?: string } | null;
        if (nameInput) {
          nameInput.value = name;
          nameInput.dispatchEvent(new Event('sl-input', { bubbles: true, composed: true }));
        }

        // Fill email field
        const emailInput = formShadow?.querySelector('sl-input[name="email"], sl-input[label="Email"]') as HTMLElement & { value?: string } | null;
        if (emailInput) {
          emailInput.value = email;
          emailInput.dispatchEvent(new Event('sl-input', { bubbles: true, composed: true }));
        }
      },
      { code: TEST_AGENT_CODE, name: TEST_AGENT_NAME, email: TEST_AGENT_EMAIL }
    );

    // Click Next to proceed to Step 2 (Skills)
    await page.evaluate(() => {
      const shell = document.querySelector('or-catalog-shell');
      const shellShadow = shell?.shadowRoot;
      const main = shellShadow?.querySelector('main.content');
      const agentForm = main?.querySelector('or-agent-form');
      const formShadow = agentForm?.shadowRoot;
      const nextBtn = Array.from(formShadow?.querySelectorAll('sl-button, button') ?? []).find(
        (b) => b.textContent?.toLowerCase().includes('next')
      ) as HTMLElement | null;
      nextBtn?.click();
    });

    // Step 2: Skills (skip — no skills needed for smoke test)
    // Click Next to proceed to Step 3 (Review)
    await page.evaluate(() => {
      const shell = document.querySelector('or-catalog-shell');
      const shellShadow = shell?.shadowRoot;
      const main = shellShadow?.querySelector('main.content');
      const agentForm = main?.querySelector('or-agent-form');
      const formShadow = agentForm?.shadowRoot;
      const nextBtn = Array.from(formShadow?.querySelectorAll('sl-button, button') ?? []).find(
        (b) => b.textContent?.toLowerCase().includes('next')
      ) as HTMLElement | null;
      nextBtn?.click();
    });

    // Step 3: Review — click Create Agent
    await page.evaluate(() => {
      const shell = document.querySelector('or-catalog-shell');
      const shellShadow = shell?.shadowRoot;
      const main = shellShadow?.querySelector('main.content');
      const agentForm = main?.querySelector('or-agent-form');
      const formShadow = agentForm?.shadowRoot;
      const createBtn = Array.from(formShadow?.querySelectorAll('sl-button, button') ?? []).find(
        (b) =>
          b.textContent?.toLowerCase().includes('create agent') ||
          b.textContent?.toLowerCase().includes('submit')
      ) as HTMLElement | null;
      createBtn?.click();
    });

    // After successful create: shell navigates to /orgs/:org_id/agents/:newId
    await page.waitForURL(`**/orgs/${TEST_ORG_ID}/agents/**`, { timeout: 10_000 });

    // Verify we're on the agent detail page (or-agent-detail mounted)
    await expect(page.locator('or-agent-detail')).toBeAttached({ timeout: 5_000 });
  });

  test('Switch org navigates back to org-picker', async ({ page }) => {
    await page.goto(`/orgs/${TEST_ORG_ID}/agents`);
    await expect(page.locator('or-agent-list')).toBeAttached({ timeout: 10_000 });

    // Click "Switch org" in the top bar
    await page.evaluate(() => {
      const shell = document.querySelector('or-catalog-shell');
      const shellShadow = shell?.shadowRoot;
      const switchBtn = Array.from(shellShadow?.querySelectorAll('sl-button') ?? []).find(
        (b) => b.textContent?.toLowerCase().includes('switch org')
      ) as HTMLElement | null;
      switchBtn?.click();
    });

    // Should navigate to root (org-picker)
    await page.waitForURL('**/', { timeout: 5_000 });
    await expect(page.locator('or-org-picker')).toBeAttached({ timeout: 5_000 });
  });
});
