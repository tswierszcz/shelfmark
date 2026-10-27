import { test as setup, expect } from '@playwright/test';

const authFile = 'playwright/.auth/user.json';

setup('sign in once and save the session', async ({ page }) => {
  const username = process.env.E2E_USERNAME;
  const password = process.env.E2E_PASSWORD;
  if (!username || !password) {
    throw new Error('Set E2E_USERNAME and E2E_PASSWORD (see context/foundation/test-stack.md, ## E2E)');
  }

  await page.goto('/login');
  // The SPA drops input typed before it hydrates. Retry fill + submit until the
  // submit leaves /login. Clear before filling: re-filling the same value fires
  // no change event in a controlled input.
  await expect(async () => {
    await page.getByRole('textbox', { name: 'Email' }).fill('');
    await page.getByRole('textbox', { name: 'Email' }).fill(username);
    await page.getByRole('textbox', { name: 'Password' }).fill('');
    await page.getByRole('textbox', { name: 'Password' }).fill(password);
    await page.getByRole('button', { name: 'Log in' }).click();
    await page.waitForURL((url) => !url.pathname.startsWith('/login'), { timeout: 5_000 });
  }).toPass();
  await expect(page.getByRole('button', { name: 'Log out' })).toBeVisible();

  await page.context().storageState({ path: authFile });
});
