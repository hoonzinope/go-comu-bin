import { expect, test } from '@playwright/test';

test('renders the public shell and responsive menu', async ({ page }) => {
  await page.goto('/');
  await expect(page).toHaveTitle(/Commu Bin/);
  await expect(page.getByRole('heading', { name: 'Boards, latest, and top threads in one place.' })).toBeVisible();
  await expect(page.locator('meta[name="csrf-token"]')).toHaveAttribute('content', /.+/);
  await expect(page.locator('.sidebar').getByText('Discussion directory')).toBeVisible();
  await expect(page.locator('.sidebar').getByRole('link', { name: 'All threads', exact: true })).toBeVisible();
  await expect(page.locator('.method-list').getByText('01 Search')).toBeVisible();
  await expect(page.locator('.method-list').getByText('02 Compare')).toBeVisible();

  await page.setViewportSize({ width: 390, height: 844 });
  await page.reload();
  await expect(page.locator('.bottom-nav')).toBeVisible();
  await expect(page.locator('.sidebar')).not.toBeVisible();

  await page.getByRole('button', { name: 'Menu' }).click();
  await expect(page.locator('.overlay').getByText('Boards')).toBeVisible();
  await expect(page.locator('.overlay').getByRole('link', { name: 'Feed', exact: true })).toBeVisible();
});
