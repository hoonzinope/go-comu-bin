import { expect, test } from '@playwright/test';
import {
  ADMIN_CREDENTIALS,
  ADMIN_PASSWORD,
  ADMIN_USERNAME,
  apiLogin,
  createBoard,
  createPost,
  createReport,
  createVerifiedUser,
  getBoardHiddenByUUID,
  getUserSuspension,
  getUserUUIDByName,
  hideBoard,
  loginThroughUi,
  logoutThroughUi,
  seedDeadOutboxMessage,
} from './helpers';

test('toggles board visibility and blocks regular users', async ({ page, request }) => {
  const adminToken = await apiLogin(request, ADMIN_USERNAME, ADMIN_PASSWORD);
  const boardToHide = await createBoard(request, adminToken, `Playwright hide ${Date.now().toString(36)}`, 'Visible board');
  const boardToShow = await createBoard(request, adminToken, `Playwright show ${Date.now().toString(36)}`, 'Hidden board');
  await hideBoard(request, adminToken, boardToShow, true);

  await loginThroughUi(page, ADMIN_CREDENTIALS, '/me');
  await page.goto('/admin/boards');
  await expect(page.getByRole('heading', { name: 'Boards' })).toBeVisible();

  const hideRow = page.locator('.table-row').filter({ hasText: boardToHide });
  const showRow = page.locator('.table-row').filter({ hasText: boardToShow });
  await expect(hideRow).toContainText('Visible');
  await expect(showRow).toContainText('Hidden');

  await Promise.all([
    page.waitForNavigation({ waitUntil: 'domcontentloaded' }),
    hideRow.getByRole('button', { name: 'Hide' }).click(),
  ]);
  await expect.poll(() => getBoardHiddenByUUID(boardToHide)).toBeTruthy();

  await Promise.all([
    page.waitForNavigation({ waitUntil: 'domcontentloaded' }),
    page.locator('.table-row').filter({ hasText: boardToShow }).getByRole('button', { name: 'Show' }).click(),
  ]);
  await expect.poll(() => getBoardHiddenByUUID(boardToShow)).toBeFalsy();

  await logoutThroughUi(page);
  const reader = await createVerifiedUser(page, request, 'reader-admin');
  await loginThroughUi(page, reader, '/me');
  await page.goto(`/boards/${boardToHide}`);
  await expect(page.getByRole('heading', { name: 'Not Found' })).toBeVisible();
  await expect(page.locator('.banner')).toContainText('board not found');
});

test('resolves reports from the moderation queue', async ({ page, request }) => {
  const adminToken = await apiLogin(request, ADMIN_USERNAME, ADMIN_PASSWORD);
  const boardUUID = await createBoard(request, adminToken, `Playwright reports ${Date.now().toString(36)}`, 'Reports board');
  const postUUID = await createPost(request, adminToken, boardUUID, `Playwright report target ${Date.now().toString(36)}`, 'Report body', ['playwright']);

  const reporter = await createVerifiedUser(page, request, 'reporter');
  const reporterToken = await apiLogin(request, reporter.username, reporter.password);
  const reportID = await createReport(request, reporterToken, 'post', postUUID, 'spam', 'Playwright spam');

  await loginThroughUi(page, ADMIN_CREDENTIALS, '/me');
  await page.goto('/admin/reports');
  await expect(page.getByRole('heading', { name: 'Reports' })).toBeVisible();
  const reportRow = page.locator('.table-row').filter({ hasText: `#${reportID}` });
  await expect(reportRow).toContainText('spam');
  await expect(reportRow).toContainText(postUUID);

  await reportRow.locator('input[name="status"]').fill('accepted');
  await reportRow.locator('input[name="resolution_note"]').fill('confirmed by Playwright');
  await Promise.all([
    page.waitForNavigation({ waitUntil: 'domcontentloaded' }),
    reportRow.getByRole('button', { name: 'Resolve' }).click(),
  ]);
  await expect(page.locator('.table-row').filter({ hasText: `#${reportID}` })).toContainText('resolved');
});

test('shows dead outbox messages and handles requeue/discard', async ({ page }) => {
  const deadRequeueID = `dead-requeue-${Date.now().toString(36)}`;
  const deadDiscardID = `dead-discard-${Date.now().toString(36)}`;
  await seedDeadOutboxMessage(deadRequeueID, 'playwright.dead.requeue', 'retry later');
  await seedDeadOutboxMessage(deadDiscardID, 'playwright.dead.discard', 'drop it');

  await loginThroughUi(page, ADMIN_CREDENTIALS, '/me');
  await page.goto('/admin/outbox');
  await expect(page.getByRole('heading', { name: 'Dead outbox' })).toBeVisible();

  const requeueRow = page.locator('.table-row').filter({ hasText: deadRequeueID });
  const discardRow = page.locator('.table-row').filter({ hasText: deadDiscardID });
  await expect(requeueRow).toContainText('Attempts x3');
  await expect(discardRow).toContainText('Attempts x3');

  await Promise.all([
    page.waitForNavigation({ waitUntil: 'domcontentloaded' }),
    requeueRow.getByRole('button', { name: 'Requeue' }).click(),
  ]);
  await expect(page.locator('.table-row').filter({ hasText: deadRequeueID })).toHaveCount(0);

  await Promise.all([
    page.waitForNavigation({ waitUntil: 'domcontentloaded' }),
    discardRow.getByRole('button', { name: 'Discard' }).click(),
  ]);
  await expect(page.locator('.table-row').filter({ hasText: deadDiscardID })).toHaveCount(0);
});

test('opens suspension page and suspends a user', async ({ page, request }) => {
  const adminToken = await apiLogin(request, ADMIN_USERNAME, ADMIN_PASSWORD);
  const target = await createVerifiedUser(page, request, 'suspend-target');
  const targetUUID = getUserUUIDByName(target.username);

  await loginThroughUi(page, ADMIN_CREDENTIALS, '/me');
  await page.goto(`/admin/users/${targetUUID}/suspension`);
  await expect(page.getByRole('heading', { name: 'User suspension' })).toBeVisible();

  const form = page.locator(`form[action="/admin/users/${targetUUID}/suspension"]`).first();
  await form.locator('input[name="reason"]').fill('Playwright abuse');
  await form.locator('input[name="duration"]').fill('7d');
  await Promise.all([
    page.waitForNavigation({ waitUntil: 'domcontentloaded' }),
    form.getByRole('button', { name: 'Suspend' }).click(),
  ]);

  const suspension = await getUserSuspension(request, adminToken, targetUUID);
  expect(suspension.status).toBe('suspended');
  expect(suspension.reason).toBe('Playwright abuse');
});

test('returns 403 for admin routes to regular users', async ({ page, request }) => {
  const user = await createVerifiedUser(page, request, 'forbidden');
  await loginThroughUi(page, user, '/me');
  await page.goto('/admin/reports');
  await expect(page.getByRole('heading', { name: 'Forbidden' })).toBeVisible();
  await expect(page.locator('.banner')).toContainText('admin access is required');
});
