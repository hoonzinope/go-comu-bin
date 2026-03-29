import { expect, test } from '@playwright/test';
import {
  confirmPasswordResetThroughUi,
  createVerifiedUser,
  deleteAccountThroughUi,
  loginThroughUi,
  logoutThroughUi,
  requestPasswordReset,
  waitForOutboxRawToken,
} from './helpers';

test('signs up, verifies email, logs in, and logs out', async ({ page, request }) => {
  const credentials = await createVerifiedUser(page, request, 'auth');

  await loginThroughUi(page, credentials, '/me');
  await expect(page.getByRole('heading', { name: 'My page' })).toBeVisible();
  await expect(page.getByText(credentials.username)).toBeVisible();
  await expect(page.getByText(credentials.email)).toBeVisible();

  await logoutThroughUi(page);
  await page.goto('/me');
  await expect(page).toHaveURL(/\/login\?redirect=%2Fme$/);
});

test('redirects protected routes to login when unauthenticated', async ({ page }) => {
  await page.goto('/me');
  await expect(page).toHaveURL(/\/login\?redirect=%2Fme$/);
  await expect(page.getByRole('heading', { name: 'Login' })).toBeVisible();
});

test('resets a password from the confirm page', async ({ page, request }) => {
  const credentials = await createVerifiedUser(page, request, 'reset');
  const newPassword = `${credentials.password}-new`;

  await requestPasswordReset(request, credentials.email);
  const token = await waitForOutboxRawToken('password.reset.requested', credentials.email);
  await confirmPasswordResetThroughUi(page, token, newPassword);

  const updated = { ...credentials, password: newPassword };
  await loginThroughUi(page, updated, '/me');
  await expect(page.getByRole('heading', { name: 'My page' })).toBeVisible();
  await expect(page.getByText(credentials.username)).toBeVisible();
});

test('deletes the current account from My page and clears access', async ({ page, request }) => {
  const credentials = await createVerifiedUser(page, request, 'delete');

  await loginThroughUi(page, credentials, '/me');
  await expect(page.getByRole('heading', { name: 'My page' })).toBeVisible();

  await deleteAccountThroughUi(page, credentials.password);
  await page.goto('/me');
  await expect(page).toHaveURL(/\/login\?redirect=%2Fme$/);
});
