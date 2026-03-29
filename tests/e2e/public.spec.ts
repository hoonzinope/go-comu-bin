import { expect, test } from '@playwright/test';

test('renders the public shell and responsive menu', async ({ page }) => {
  await page.goto('/');
  await expect(page).toHaveTitle(/Commu Bin/);
  await expect(page.getByRole('heading', { name: 'Feed' })).toBeVisible();
  await expect(page.locator('meta[name="csrf-token"]')).toHaveAttribute('content', /.+/);

  await page.setViewportSize({ width: 390, height: 844 });
  await page.reload();
  await expect(page.locator('.bottom-nav')).toBeVisible();
  await expect(page.locator('.sidebar')).not.toBeVisible();

  await page.getByRole('button', { name: 'Menu' }).click();
  await expect(page.getByText('Navigation')).toBeVisible();
  await expect(page.locator('.overlay').getByRole('link', { name: 'Login' })).toBeVisible();
  await expect(page.locator('.overlay').getByRole('link', { name: 'Notifications', exact: true })).toBeVisible();
});
