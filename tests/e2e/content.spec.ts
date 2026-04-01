import { expect, test } from '@playwright/test';
import {
  ADMIN_CREDENTIALS,
  ADMIN_PASSWORD,
  ADMIN_USERNAME,
  apiLogin,
  createBoard,
  createPost,
  createVerifiedUser,
  getLatestCommentIDByPostUUIDAndContent,
  getPostIDByUUID,
  getUserIDByName,
  getUnreadCount,
  hideBoard,
  loginThroughUi,
  logoutThroughUi,
  seedNotification,
} from './helpers';

test('creates posts and drafts from the composer and renders them in feed, tag, and search views', async ({ page, request }) => {
  const adminToken = await apiLogin(request, ADMIN_USERNAME, ADMIN_PASSWORD);
  const boardName = `Playwright content board ${Date.now().toString(36)}`;
  const boardUUID = await createBoard(request, adminToken, boardName, `Board for ${boardName}`);
  const suffix = Date.now().toString(36);
  const publishedTitle = `Playwright publish ${suffix}`;
  const publishedBody = `Published body ${suffix}`;
  const publishedTags = 'playwright, ui';
  const draftTitle = `Playwright draft ${suffix}`;
  const draftBody = `Draft body ${suffix}`;

  await loginThroughUi(page, ADMIN_CREDENTIALS, '/me');

  await page.goto(`/boards/${boardUUID}/posts/new`);
  const publishForm = page.locator(`form[action="/boards/${boardUUID}/posts"]`).first();
  await publishForm.locator('input[name="title"]').fill(publishedTitle);
  await publishForm.locator('input[name="tags"]').fill(publishedTags);
  await publishForm.locator('textarea[name="content"]').fill(publishedBody);
  await publishForm.getByRole('button', { name: 'Publish' }).click();
  await page.waitForURL(/\/posts\/[0-9a-f-]{36}$/);
  const publishedUUID = new URL(page.url()).pathname.split('/').pop() as string;
  await expect(page.getByRole('heading', { name: publishedTitle })).toBeVisible();
  await expect(page.locator('pre.markdown')).toContainText(publishedBody);

  await page.goto(`/boards/${boardUUID}/posts/drafts/new`);
  const draftForm = page.locator(`form[action="/boards/${boardUUID}/posts/drafts"]`).first();
  await draftForm.locator('input[name="title"]').fill(draftTitle);
  await draftForm.locator('input[name="tags"]').fill('draft, playwright');
  await draftForm.locator('textarea[name="content"]').fill(draftBody);
  await Promise.all([
    page.waitForNavigation({ waitUntil: 'domcontentloaded' }),
    draftForm.getByRole('button', { name: 'Save draft' }).click(),
  ]);
  await page.waitForURL(/\/posts\/[0-9a-f-]{36}\/edit$/);
  const draftUUID = new URL(page.url()).pathname.split('/')[2];
  const editForm = page.locator(`form[action="/posts/${draftUUID}"]`).first();
  const updatedDraftTitle = `Playwright draft updated ${suffix}`;
  const updatedDraftBody = `Draft body updated ${suffix}`;
  await editForm.locator('input[name="title"]').fill(updatedDraftTitle);
  await editForm.locator('textarea[name="content"]').fill(updatedDraftBody);
  await editForm.getByRole('button', { name: 'Update draft' }).click();
  await expect(page.getByRole('heading', { name: 'Edit draft' })).toBeVisible();
  await editForm.getByRole('button', { name: 'Publish' }).click();
  await page.waitForURL(/\/posts\/[0-9a-f-]{36}$/);
  await expect(page.getByRole('heading', { name: updatedDraftTitle })).toBeVisible();

  await page.goto(`/boards/${boardUUID}`);
  await expect(page.getByText('Board feed.')).toBeVisible();
  await expect(page.getByText(publishedTitle)).toBeVisible();
  await expect(page.getByText(updatedDraftTitle)).toBeVisible();

  await page.goto('/tags/playwright');
  await expect(page.getByRole('heading', { name: '#playwright' })).toBeVisible();
  await expect(page.getByText(publishedTitle)).toBeVisible();
  await expect(page.getByText(updatedDraftTitle)).toBeVisible();

  await expect.poll(async () => {
    await page.goto(`/search?q=${encodeURIComponent('Playwright')}`);
    return await page.getByText(publishedTitle).count();
  }, { timeout: 15000 }).toBeGreaterThan(0);
  await expect(page.getByRole('heading', { name: 'Search' })).toBeVisible();
  await expect(page.getByText(publishedTitle)).toBeVisible();

  await page.goto(`/posts/${publishedUUID}`);
  await expect(page.getByRole('heading', { name: publishedTitle })).toBeVisible();
  await expect(page.getByText('At a glance')).toBeVisible();
  await expect(page.locator('#comments .section-title')).toHaveText('Comments');
  await expect(page.getByRole('link', { name: 'Back to feed' })).toBeVisible();
});

test('adds a comment and surfaces a notification', async ({ page, request }) => {
  const adminToken = await apiLogin(request, ADMIN_USERNAME, ADMIN_PASSWORD);
  const boardUUID = await createBoard(request, adminToken, `Playwright notifications ${Date.now().toString(36)}`, 'Notification board');
  const postTitle = `Playwright notification post ${Date.now().toString(36)}`;
  const postUUID = await createPost(request, adminToken, boardUUID, postTitle, 'Notification body', ['playwright', 'notify']);

  const commenter = await createVerifiedUser(page, request, 'commenter');
  await loginThroughUi(page, commenter, '/me');
  await page.goto(`/posts/${postUUID}`);
  const commentForm = page.locator('form.comment-form');
  await commentForm.locator('textarea[name="content"]').fill('Playwright comment');
  await Promise.all([
    page.waitForNavigation({ waitUntil: 'domcontentloaded' }),
    commentForm.getByRole('button', { name: 'Comment' }).click(),
  ]);
  const commentID = getLatestCommentIDByPostUUIDAndContent(postUUID, 'Playwright comment');
  expect(commentID).toBeGreaterThan(0);

  await seedNotification(
    getUserIDByName(ADMIN_USERNAME),
    getUserIDByName(commenter.username),
    'post_commented',
    getPostIDByUUID(postUUID),
    getLatestCommentIDByPostUUIDAndContent(postUUID, 'Playwright comment'),
    commenter.username,
    postTitle,
    'Playwright comment',
  );

  const unreadBefore = await getUnreadCount(request, adminToken);
  expect(unreadBefore).toBeGreaterThanOrEqual(0);
  await seedNotification(
    getUserIDByName(ADMIN_USERNAME),
    getUserIDByName(commenter.username),
    'post_commented',
    getPostIDByUUID(postUUID),
    getLatestCommentIDByPostUUIDAndContent(postUUID, 'Playwright comment'),
    commenter.username,
    postTitle,
    'Playwright comment',
  );

  const unreadAfterSeed = await getUnreadCount(request, adminToken);
  expect(unreadAfterSeed).toBeGreaterThan(unreadBefore);

  await logoutThroughUi(page);
  await loginThroughUi(page, ADMIN_CREDENTIALS, '/me');
  const unreadCount = await getUnreadCount(request, adminToken);
  expect(unreadCount).toBe(unreadAfterSeed);
  await page.goto('/notifications');
  await expect(page.getByRole('heading', { name: 'Notifications' })).toBeVisible();
  await expect(page.locator('main').getByText(postTitle).first()).toBeVisible();

  const readAllForm = page.locator('form[action="/notifications/read-all"]').first();
  await readAllForm.getByRole('button', { name: 'Mark all read' }).click();
  await expect(page).toHaveURL(/\/notifications$/);
  await expect(page.getByText('Unread: 0')).toBeVisible();
});

test('highlights the current reaction on post detail after voting', async ({ page, request }) => {
  const adminToken = await apiLogin(request, ADMIN_USERNAME, ADMIN_PASSWORD);
  const boardUUID = await createBoard(request, adminToken, `Playwright reaction board ${Date.now().toString(36)}`, 'Reaction board');
  const postUUID = await createPost(request, adminToken, boardUUID, `Playwright reaction post ${Date.now().toString(36)}`, 'Reaction body', ['playwright']);

  const voter = await createVerifiedUser(page, request, 'reaction-voter');
  await loginThroughUi(page, voter, '/me');
  await page.goto(`/posts/${postUUID}`);

  const upvoteButton = page.getByRole('button', { name: '▲ Upvote' });
  await Promise.all([
    page.waitForNavigation({ waitUntil: 'domcontentloaded' }),
    upvoteButton.click(),
  ]);

  await expect(page.getByRole('button', { name: '▲ Upvote' })).toHaveClass(/button-primary/);
  await expect(page.getByRole('button', { name: '▼ Downvote' })).not.toHaveClass(/button-primary/);

  await page.reload();
  await expect(page.getByRole('button', { name: '▲ Upvote' })).toHaveClass(/button-primary/);
});

test('blocks hidden boards for regular users', async ({ page, request }) => {
  const adminToken = await apiLogin(request, ADMIN_USERNAME, ADMIN_PASSWORD);
  const hiddenBoardUUID = await createBoard(request, adminToken, `Playwright hidden ${Date.now().toString(36)}`, 'Hidden board');
  await hideBoard(request, adminToken, hiddenBoardUUID, true);

  const reader = await createVerifiedUser(page, request, 'reader');
  await loginThroughUi(page, reader, '/me');
  const response = await page.goto(`/boards/${hiddenBoardUUID}`);
  expect(response?.status()).toBe(404);
  await expect(page.getByRole('heading', { name: 'Not Found' })).toBeVisible();
  await expect(page.locator('.banner')).toContainText('board not found');
});
