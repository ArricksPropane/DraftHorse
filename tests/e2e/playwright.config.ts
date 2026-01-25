import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: '.',
  testMatch: '**/*.spec.ts',
  timeout: 60000,
  retries: 0,
  workers: 1, // Extensions require single worker

  use: {
    headless: false, // Chrome extensions require headed mode
    viewport: { width: 400, height: 600 },
  },

  reporter: [['list'], ['html', { outputFolder: 'playwright-report' }]],
});
