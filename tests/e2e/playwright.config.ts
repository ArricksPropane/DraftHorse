import { defineConfig } from '@playwright/test';
import path from 'path';

// Path to the built extension
const extensionPath = path.resolve(__dirname, '../../src/extension/dist');

export default defineConfig({
  testDir: '.',
  testMatch: '**/*.spec.ts',
  timeout: 60000,
  retries: 0,
  workers: 1, // Extensions require single worker

  use: {
    headless: false, // Chrome extensions require headed mode
    viewport: { width: 400, height: 600 }, // Popup size
  },

  projects: [
    {
      name: 'chrome-extension',
      use: {
        channel: 'chrome',
        launchOptions: {
          args: [
            `--disable-extensions-except=${extensionPath}`,
            `--load-extension=${extensionPath}`,
            '--no-first-run',
            '--disable-default-apps',
          ],
        },
      },
    },
  ],

  // Only run E2E tests explicitly
  reporter: [['list'], ['html', { outputFolder: 'playwright-report' }]],
});
