// Risk: role-gate-redirect — a signed-in user whose role may not use a page
// (here: architect on the admin-only /catalog) must be sent to /dashboard and
// never see that page's content. UI side of test-plan risks #3 / #5.
import { test, expect } from '@playwright/test';

test('architect opening the admin-only catalog is redirected to the dashboard', async ({ page }) => {
  await page.goto('/catalog');

  await page.waitForURL('**/dashboard');
  await expect(page.getByRole('heading', { name: 'Dashboard', level: 1 })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Catalog', level: 1 })).toBeHidden();
  // The admin-only page is not offered in the navigation either.
  await expect(page.getByRole('link', { name: 'Catalog' })).toHaveCount(0);

  // No cleanup: the test only navigates and creates no data.
});
