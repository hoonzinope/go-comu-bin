import { expect, test, type Page } from '@playwright/test';
import {
  ADMIN_CREDENTIALS,
  ADMIN_USERNAME,
  apiLogin,
  createBoard,
  createPost,
  createReport,
  createVerifiedUser,
  hideBoard,
  loginThroughUi,
} from './helpers';

const visualSnapshotCss = `
  *, *::before, *::after {
    animation: none !important;
    animation-delay: 0s !important;
    animation-duration: 0s !important;
    caret-color: transparent !important;
    transition: none !important;
  }

  .page-heading p,
  .meta-row span:not(.chip),
  .meta-list small,
  .table-sub,
  .overlay small,
  .overlay .meta-row,
  .error-panel p {
    display: none !important;
  }
`;

async function prepareVisualPage(page: Page, width: number, height: number): Promise<void> {
  await page.setViewportSize({ width, height });
  await page.addStyleTag({ content: visualSnapshotCss });
}

test('captures the public shell and responsive menu', async ({ page }) => {
  await page.goto('/');
  await expect(page).toHaveTitle(/Commu Bin/);
  await prepareVisualPage(page, 1440, 1200);
  await expect(page).toHaveScreenshot('public-shell-desktop.png', { fullPage: true });

  await page.setViewportSize({ width: 390, height: 844 });
  await page.reload();
  await page.addStyleTag({ content: visualSnapshotCss });
  await page.getByRole('button', { name: 'Menu' }).click();
  await expect(page.locator('.overlay')).toBeVisible();
  await expect(page).toHaveScreenshot('public-shell-mobile-menu.png', { fullPage: true });
});

test('captures the login and signup surfaces', async ({ page }) => {
  await page.goto('/login?redirect=%2Fme');
  await prepareVisualPage(page, 1440, 1200);
  await expect(page.getByRole('heading', { name: 'Login' })).toBeVisible();
  await expect(page).toHaveScreenshot('login-page.png', { fullPage: true });
});

test('captures the feed, composer, and post detail surfaces', async ({ page, request }) => {
  const adminToken = await apiLogin(request, ADMIN_USERNAME, ADMIN_CREDENTIALS.password);
  const boardUUID = await createBoard(request, adminToken, 'Visual feed board', 'Board used for Playwright snapshots');
  const publishedUUID = await createPost(
    request,
    adminToken,
    boardUUID,
    'Visual feed title',
    'Visual feed body',
    ['visual', 'snapshot'],
  );

  await page.goto(`/boards/${boardUUID}`);
  await prepareVisualPage(page, 1440, 1200);
  await expect(page.getByRole('heading', { name: boardUUID })).toBeVisible();
  await expect(page.getByText('Visual feed title')).toBeVisible();
  await page.addStyleTag({ content: '.page-heading h1 { display: none !important; }' });
  await page.addStyleTag({ content: '.feed-item .chip { display: none !important; }' });
  await expect(page.locator('main > section.page')).toHaveScreenshot('feed-page.png');

  await loginThroughUi(page, ADMIN_CREDENTIALS, '/me');
  await page.goto(`/boards/${boardUUID}/posts/new`);
  const publishForm = page.locator(`form[action="/boards/${boardUUID}/posts"]`).first();
  await publishForm.locator('input[name="title"]').fill('Visual compose title');
  await publishForm.locator('input[name="tags"]').fill('visual, compose');
  await publishForm.locator('textarea[name="content"]').fill('Visual compose body');
  await prepareVisualPage(page, 1440, 1400);
  await expect(page.getByRole('heading', { name: 'New post' })).toBeVisible();
  await expect(page.locator('main > section.page')).toHaveScreenshot('compose-page.png');

  await page.goto(`/posts/${publishedUUID}`);
  await prepareVisualPage(page, 1440, 1600);
  await expect(page.getByRole('heading', { name: 'Visual feed title' })).toBeVisible();
  await page.addStyleTag({
    content: `
      .post-sidebar .meta-item:nth-child(2),
      .post-sidebar .meta-item:nth-child(3) {
        display: none !important;
      }
    `,
  });
  await expect(page.locator('main > section.page')).toHaveScreenshot('post-detail-page.png');
});

test('captures the admin dashboards and moderation queue', async ({ page, request }) => {
  const adminToken = await apiLogin(request, ADMIN_USERNAME, ADMIN_CREDENTIALS.password);
  const visibleBoardUUID = await createBoard(request, adminToken, 'Visual admin visible', 'Visible board');
  const hiddenBoardUUID = await createBoard(request, adminToken, 'Visual admin hidden', 'Hidden board');
  await hideBoard(request, adminToken, hiddenBoardUUID, true);
  const reportTargetUUID = await createPost(
    request,
    adminToken,
    visibleBoardUUID,
    'Visual moderation target',
    'Visual moderation body',
    ['visual', 'report'],
  );

  const reporter = await createVerifiedUser(page, request, 'visual-reporter');
  const reporterToken = await apiLogin(request, reporter.username, reporter.password);
  const reportID = await createReport(request, reporterToken, 'post', reportTargetUUID, 'spam', 'Visual moderation report');

  await loginThroughUi(page, ADMIN_CREDENTIALS, '/me');
  await page.goto('/admin/boards');
  await prepareVisualPage(page, 1440, 1200);
  await expect(page.getByRole('heading', { name: 'Boards' })).toBeVisible();
  await expect(page.locator('main > section.page')).toHaveScreenshot('admin-boards-page.png');

  await page.goto('/admin/reports');
  await prepareVisualPage(page, 1440, 1200);
  await expect(page.getByRole('heading', { name: 'Reports' })).toBeVisible();
  await expect(page.locator('main > section.page')).toHaveScreenshot('admin-reports-page.png');
  await expect(reportID).toBeGreaterThan(0);
  await expect(visibleBoardUUID).toMatch(/[0-9a-f-]{36}/i);
  await expect(hiddenBoardUUID).toMatch(/[0-9a-f-]{36}/i);
  await expect(reportTargetUUID).toMatch(/[0-9a-f-]{36}/i);
});

test('captures the verified auth flow', async ({ page, request }) => {
  const credentials = await createVerifiedUser(page, request, 'visual-auth');
  await loginThroughUi(page, credentials, '/me');
  await prepareVisualPage(page, 1440, 1200);
  await expect(page.getByRole('heading', { name: 'My page' })).toBeVisible();
  await expect(page.locator('main > section.page')).toHaveScreenshot('my-page.png');
});
