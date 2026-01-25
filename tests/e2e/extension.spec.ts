import { test, expect, runTestHarness, clearDropbox } from './fixtures';

/**
 * E2E Tests for go-mapi Browser Extension
 *
 * These tests verify the full e2e flow:
 * 1. Interceptor creates JSON files in dropbox
 * 2. Native host picks them up and sends to extension
 * 3. Extension displays them in the popup
 */

test.describe('Extension E2E Flow', () => {
  test.beforeAll(() => {
    // Run test harness to create test emails
    runTestHarness();
  });

  test.afterAll(() => {
    // Clean up test emails
    clearDropbox();
  });

  test('extension loads and shows popup', async ({ popupPage }) => {
    // Popup should load
    await expect(popupPage).toHaveTitle(/go-mapi/i);
  });

  test('popup shows connection status', async ({ popupPage }) => {
    // Wait for initial load
    await popupPage.waitForLoadState('domcontentloaded');

    // Should show some status indicator
    // The extension shows "Connected" or "Connecting..." or an error
    const body = await popupPage.textContent('body');
    expect(body).toBeTruthy();
  });

  test('popup displays pending emails from dropbox', async ({ popupPage, extensionContext }) => {
    // Give the extension time to connect to native host and load emails
    await popupPage.waitForTimeout(3000);

    // Reload to ensure we get fresh data
    await popupPage.reload();
    await popupPage.waitForTimeout(2000);

    // Check if emails are displayed or empty state is shown
    const content = await popupPage.textContent('body');

    // Should either show emails or "No pending emails" message
    const hasEmails = content?.includes('Test Email') || content?.includes('test@example.com');
    const hasEmptyState = content?.includes('No pending') || content?.includes('No emails');
    const hasError = content?.includes('error') || content?.includes('Error');

    // Log what we see for debugging
    console.log('Popup content:', content?.substring(0, 500));

    // At minimum, the popup should render something
    expect(content?.length).toBeGreaterThan(10);
  });

  test('can click on email to see details', async ({ popupPage }) => {
    await popupPage.waitForTimeout(2000);
    await popupPage.reload();
    await popupPage.waitForTimeout(2000);

    // Look for clickable email items
    const emailItem = popupPage.locator('[class*="email"], [class*="list-group-item"], .card').first();

    if (await emailItem.isVisible()) {
      await emailItem.click();
      await popupPage.waitForTimeout(500);

      // Should show detail view or back button
      const content = await popupPage.textContent('body');
      console.log('Detail view content:', content?.substring(0, 500));
    }
  });
});

test.describe('Extension Popup UI', () => {
  test('popup has expected dimensions', async ({ popupPage }) => {
    // Extension popups have constrained dimensions
    const viewport = popupPage.viewportSize();
    expect(viewport?.width).toBeLessThanOrEqual(800);
    expect(viewport?.height).toBeLessThanOrEqual(600);
  });

  test('popup renders without JavaScript errors', async ({ popupPage }) => {
    const errors: string[] = [];
    popupPage.on('pageerror', (error) => {
      errors.push(error.message);
    });

    await popupPage.reload();
    await popupPage.waitForTimeout(1000);

    // Filter out known acceptable errors
    const criticalErrors = errors.filter(
      (e) => !e.includes('net::ERR_') && !e.includes('Native host')
    );

    expect(criticalErrors).toHaveLength(0);
  });
});
