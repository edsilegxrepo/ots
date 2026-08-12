import { expect, test } from '@playwright/test';

test.describe('OTS UI E2E Browser Testing', () => {
  test('creates a secret, retrieves URL, and redeems secret in browser', async ({ page }) => {
    // 1. Navigate to live OTS web app
    await page.goto('/');

    // 2. Verify page load & core elements
    await expect(page).toHaveTitle(/OTS/i);
    const secretTextarea = page.locator('textarea').first();
    await expect(secretTextarea).toBeVisible();

    // 3. Fill secret content
    const testSecretContent = 'UI Browser E2E Confidential Secret Payload ' + Date.now();
    await secretTextarea.fill(testSecretContent);

    // 4. Click Submit/Create Secret button
    const createBtn = page.locator('button[type="submit"], button.btn-success, button.btn-primary').first();
    await expect(createBtn).toBeVisible();
    await createBtn.click();

    // 5. Wait for generated secret URL box
    const secretUrlBox = page.locator('input[readonly], textarea[readonly]').first();
    await expect(secretUrlBox).toBeVisible({ timeout: 10000 });
    const generatedUrl = await secretUrlBox.inputValue();
    expect(generatedUrl).toContain('/#');

    // 6. Navigate directly to generated secret URL in browser
    await page.goto(generatedUrl);

    // 7. Click Reveal/Decrypt Secret button
    const revealBtn = page.locator('button').filter({ hasText: /Show the secret|reveal|Mostra|Zobrazit|Secret/i }).first();
    await expect(revealBtn).toBeVisible({ timeout: 10000 });
    await revealBtn.click();

    // 8. Verify decrypted secret content displayed on screen
    const secretDisplay = page.locator('textarea.font-monospace, pre.font-monospace, textarea, pre').first();
    await expect(secretDisplay).toHaveValue(testSecretContent, { timeout: 10000 }).catch(async () => {
      await expect(secretDisplay).toContainText(testSecretContent, { timeout: 10000 });
    });
  });
});
