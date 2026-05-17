// Playwright configuration for @open-routing/admin smoke tests.
// Phase 6 Plan 06-06: manual-only smoke test (D6-30).
// Phase 7 (EMBED-10) will promote to CI.
//
// webServer: reuses an existing Vite dev server if already running (reuseExistingServer: true).
// This avoids a double startup when the developer has `pnpm dev` already open in another terminal.

import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './e2e',
  testMatch: '**/*.spec.ts',
  timeout: 30_000,
  retries: 0,
  reporter: [['list'], ['html', { open: 'never' }]],
  use: {
    baseURL: 'http://localhost:5173',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
  // Reuse a running Vite dev server; or start one for the test run.
  // D6-30: smoke test is manual — developer runs `pnpm dev` in terminal 1 + `pnpm test:e2e` in terminal 2.
  // reuseExistingServer: true prevents double startup.
  webServer: {
    command: 'pnpm --filter @open-routing/admin dev',
    url: 'http://localhost:5173',
    reuseExistingServer: true,
    timeout: 30_000,
  },
});
