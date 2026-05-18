import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './e2e',
  testMatch: 'specs/*.spec.ts',
  timeout: 30_000,
  retries: 1,
  reporter: [['list'], ['html', { open: 'never' }]],
  use: {
    baseURL: 'http://localhost:4173',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure'
  },
  projects: [
    {
      name: 'react',
      use: {
        ...devices['Desktop Chrome'],
        baseURL: 'http://localhost:4173/hosts/react/'
      }
    },
    {
      name: 'vue',
      use: {
        ...devices['Desktop Chrome'],
        baseURL: 'http://localhost:4173/hosts/vue/'
      }
    },
    {
      name: 'html',
      use: {
        ...devices['Desktop Chrome'],
        baseURL: 'http://localhost:4173/hosts/html/'
      }
    }
  ],
  webServer: {
    command: 'pnpm --filter @open-routing/catalog-embed preview',
    url: 'http://localhost:4173',
    reuseExistingServer: true,
    timeout: 60_000
  }
});
