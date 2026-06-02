import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './e2e',
  testMatch: 'specs/*.spec.ts',
  timeout: 30_000,
  retries: 1,
  reporter: [['list'], ['html', { open: 'never' }]],
  use: {
    baseURL: 'http://127.0.0.1:4173',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure'
  },
  projects: [
    {
      name: 'react',
      use: {
        ...devices['Desktop Chrome'],
        baseURL: 'http://127.0.0.1:4173/react/'
      }
    },
    {
      name: 'vue',
      use: {
        ...devices['Desktop Chrome'],
        baseURL: 'http://127.0.0.1:4173/vue/'
      }
    },
    {
      name: 'html',
      use: {
        ...devices['Desktop Chrome'],
        baseURL: 'http://127.0.0.1:4173/html/'
      }
    }
  ],
  webServer: {
    command: 'pnpm --filter @open-routing/catalog-embed exec vite preview --host 127.0.0.1 --port 4173',
    url: 'http://127.0.0.1:4173/html/',
    reuseExistingServer: true,
    timeout: 60_000
  }
});
