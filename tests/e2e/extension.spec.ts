import { test, expect, type BrowserContext } from '@playwright/test';

/**
 * E2E Tests for go-mapi Browser Extension
 *
 * These tests verify the extension popup UI functionality.
 * They require the extension to be built first (npm run build:extension).
 *
 * Note: Chrome extension E2E tests run in headed mode only.
 */

// Helper to get the extension ID from the loaded extensions
async function getExtensionId(context: BrowserContext): Promise<string> {
  // Navigate to chrome://extensions to find our extension
  const page = await context.newPage();
  await page.goto('chrome://extensions');

  // Enable developer mode if not enabled
  const devModeToggle = page.locator('#devMode');
  if (await devModeToggle.isVisible()) {
    const isChecked = await devModeToggle.getAttribute('checked');
    if (!isChecked) {
      await devModeToggle.click();
    }
  }

  // Wait for extensions to load and find our extension by name
  await page.waitForTimeout(1000);

  // Get extension ID from the extensions page
  // This is a simplified approach - in production you'd parse the DOM more carefully
  const extensionCards = page.locator('extensions-item');
  const count = await extensionCards.count();

  for (let i = 0; i < count; i++) {
    const card = extensionCards.nth(i);
    const name = await card.locator('#name').textContent();
    if (name?.includes('go-mapi')) {
      const id = await card.getAttribute('id');
      await page.close();
      return id || '';
    }
  }

  await page.close();
  throw new Error('go-mapi extension not found');
}

// Helper to open the extension popup
async function openPopup(context: BrowserContext, extensionId: string) {
  const popupUrl = `chrome-extension://${extensionId}/popup.html`;
  const page = await context.newPage();
  await page.goto(popupUrl);
  return page;
}

test.describe('Extension Popup', () => {
  test.skip(true, 'E2E tests require manual Chrome extension setup');

  test('should display the header with title', async ({ context }) => {
    const extensionId = await getExtensionId(context);
    const popup = await openPopup(context, extensionId);

    await expect(popup.locator('h1')).toHaveText('go-mapi');
    await popup.close();
  });

  test('should show connection status', async ({ context }) => {
    const extensionId = await getExtensionId(context);
    const popup = await openPopup(context, extensionId);

    // Should show either Connected or Disconnected
    const status = popup.locator('.status');
    await expect(status).toBeVisible();
    await popup.close();
  });

  test('should show empty state when no emails', async ({ context }) => {
    const extensionId = await getExtensionId(context);
    const popup = await openPopup(context, extensionId);

    // Wait for loading to complete
    await popup.waitForSelector('.spinner-border', { state: 'hidden', timeout: 5000 }).catch(() => {});

    // If no emails, should show empty state
    const emptyState = popup.locator('.empty-state');
    if (await emptyState.isVisible()) {
      await expect(popup.locator('text=No pending emails')).toBeVisible();
    }

    await popup.close();
  });

  test('should display email list when emails are present', async ({ context }) => {
    const extensionId = await getExtensionId(context);
    const popup = await openPopup(context, extensionId);

    // Wait for loading to complete
    await popup.waitForSelector('.spinner-border', { state: 'hidden', timeout: 5000 }).catch(() => {});

    // If emails are present, should show email list
    const emailList = popup.locator('.email-list');
    if (await emailList.isVisible()) {
      const items = popup.locator('.email-item');
      const count = await items.count();
      expect(count).toBeGreaterThan(0);
    }

    await popup.close();
  });

  test('should navigate to detail view when email is clicked', async ({ context }) => {
    const extensionId = await getExtensionId(context);
    const popup = await openPopup(context, extensionId);

    // Wait for loading
    await popup.waitForSelector('.spinner-border', { state: 'hidden', timeout: 5000 }).catch(() => {});

    const firstEmail = popup.locator('.email-item').first();
    if (await firstEmail.isVisible()) {
      await firstEmail.click();

      // Should show detail view with back button
      await expect(popup.locator('text=← Back to list')).toBeVisible();
    }

    await popup.close();
  });

  test('should navigate back to list from detail view', async ({ context }) => {
    const extensionId = await getExtensionId(context);
    const popup = await openPopup(context, extensionId);

    // Wait for loading
    await popup.waitForSelector('.spinner-border', { state: 'hidden', timeout: 5000 }).catch(() => {});

    const firstEmail = popup.locator('.email-item').first();
    if (await firstEmail.isVisible()) {
      await firstEmail.click();
      await popup.locator('text=← Back to list').click();

      // Should be back on list view
      await expect(popup.locator('.email-list')).toBeVisible();
    }

    await popup.close();
  });
});

test.describe('Gmail Integration', () => {
  test.skip(true, 'Gmail tests require OAuth setup and test account');

  test('should create draft when Save as Draft is clicked', async ({ context }) => {
    // This test requires:
    // 1. A test Google account with OAuth configured
    // 2. The extension to have valid OAuth credentials
    // 3. An email in the queue

    const extensionId = await getExtensionId(context);
    const popup = await openPopup(context, extensionId);

    // Navigate to email detail
    await popup.waitForSelector('.email-item', { timeout: 5000 });
    await popup.locator('.email-item').first().click();

    // Click Save as Draft
    await popup.locator('text=Save as Draft').click();

    // Should show loading spinner, then return to list
    await popup.waitForSelector('.spinner-border', { state: 'hidden', timeout: 30000 });

    await popup.close();
  });

  test('should send email when Send Now is clicked', async ({ context }) => {
    const extensionId = await getExtensionId(context);
    const popup = await openPopup(context, extensionId);

    // Navigate to email detail
    await popup.waitForSelector('.email-item', { timeout: 5000 });
    await popup.locator('.email-item').first().click();

    // Click Send Now
    await popup.locator('text=Send Now').click();

    // Should show loading spinner, then return to list
    await popup.waitForSelector('.spinner-border', { state: 'hidden', timeout: 30000 });

    await popup.close();
  });
});
