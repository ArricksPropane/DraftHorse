import { test as base, chromium, type BrowserContext, type Page } from '@playwright/test';
import { execSync, spawn, type ChildProcess } from 'child_process';
import * as fs from 'fs';
import * as path from 'path';
import * as os from 'os';

// Paths
const ROOT_DIR = path.resolve(__dirname, '../..');
const EXTENSION_PATH = path.join(ROOT_DIR, 'src/extension/dist');
const NATIVE_HOST_PATH = path.join(ROOT_DIR, 'src/native-host/build/go-mapi-host.exe');
const TEST_HARNESS_PATH = path.join(ROOT_DIR, 'src/interceptor/build/bin/go-mapi-test-harness.exe');
const DROPBOX_PATH = path.join(os.tmpdir(), 'go-mapi');

// Find Edge or Chrome
function findBrowser(): string {
  const candidates = [
    'C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe',
    'C:\\Program Files\\Microsoft\\Edge\\Application\\msedge.exe',
    'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe',
    'C:\\Program Files (x86)\\Google\\Chrome\\Application\\chrome.exe',
  ];
  for (const p of candidates) {
    if (fs.existsSync(p)) return p;
  }
  throw new Error('No Chrome or Edge browser found');
}

// Custom fixtures
type ExtensionFixtures = {
  extensionContext: BrowserContext;
  extensionId: string;
  popupPage: Page;
};

export const test = base.extend<ExtensionFixtures>({
  // Browser context with extension loaded
  extensionContext: async ({}, use) => {
    const userDataDir = fs.mkdtempSync(path.join(os.tmpdir(), 'go-mapi-test-'));

    const context = await chromium.launchPersistentContext(userDataDir, {
      headless: false,
      executablePath: findBrowser(),
      args: [
        `--disable-extensions-except=${EXTENSION_PATH}`,
        `--load-extension=${EXTENSION_PATH}`,
        '--no-first-run',
        '--disable-default-apps',
      ],
    });

    await use(context);

    await context.close();
    // Clean up user data dir
    fs.rmSync(userDataDir, { recursive: true, force: true });
  },

  // Get extension ID from service worker
  extensionId: async ({ extensionContext }, use) => {
    // Wait for service worker to be available
    let serviceWorker = extensionContext.serviceWorkers()[0];
    if (!serviceWorker) {
      serviceWorker = await extensionContext.waitForEvent('serviceworker', { timeout: 10000 });
    }

    // Extract extension ID from service worker URL
    // URL format: chrome-extension://EXTENSION_ID/service-worker.js
    const extensionId = serviceWorker.url().split('/')[2];

    // Register native messaging host with this extension ID
    registerNativeHost(extensionId);

    await use(extensionId);
  },

  // Open popup page
  popupPage: async ({ extensionContext, extensionId }, use) => {
    const popupUrl = `chrome-extension://${extensionId}/popup.html`;
    const page = await extensionContext.newPage();
    await page.goto(popupUrl);
    await use(page);
    await page.close();
  },
});

// Register native messaging host for the given extension ID
function registerNativeHost(extensionId: string): void {
  const manifestDir = path.join(ROOT_DIR, 'src/native-host/build');
  const manifestPath = path.join(manifestDir, 'com.gomapi.host.json');

  // Create manifest
  const manifest = {
    name: 'com.gomapi.host',
    description: 'go-mapi Native Messaging Host (test)',
    path: NATIVE_HOST_PATH,
    type: 'stdio',
    allowed_origins: [`chrome-extension://${extensionId}/`],
  };

  fs.writeFileSync(manifestPath, JSON.stringify(manifest, null, 2));

  // Register in Windows registry (HKCU)
  const regCommands = [
    `reg add "HKCU\\Software\\Google\\Chrome\\NativeMessagingHosts\\com.gomapi.host" /ve /t REG_SZ /d "${manifestPath}" /f`,
    `reg add "HKCU\\Software\\Microsoft\\Edge\\NativeMessagingHosts\\com.gomapi.host" /ve /t REG_SZ /d "${manifestPath}" /f`,
  ];

  for (const cmd of regCommands) {
    try {
      execSync(cmd, { stdio: 'pipe' });
    } catch {
      // Ignore errors (e.g., if Chrome not installed)
    }
  }
}

// Run test harness to create test emails in dropbox
export function runTestHarness(): void {
  // Clear existing test emails
  if (fs.existsSync(DROPBOX_PATH)) {
    const files = fs.readdirSync(DROPBOX_PATH);
    for (const file of files) {
      if (file.endsWith('.json') && !file.includes('processed') && !file.includes('errors')) {
        fs.unlinkSync(path.join(DROPBOX_PATH, file));
      }
    }
  }

  // Run test harness
  execSync(TEST_HARNESS_PATH, {
    cwd: path.dirname(TEST_HARNESS_PATH),
    stdio: 'pipe',
  });
}

// Clean up dropbox
export function clearDropbox(): void {
  if (fs.existsSync(DROPBOX_PATH)) {
    const files = fs.readdirSync(DROPBOX_PATH);
    for (const file of files) {
      if (file.endsWith('.json')) {
        fs.unlinkSync(path.join(DROPBOX_PATH, file));
      }
    }
  }
}

export { expect } from '@playwright/test';
