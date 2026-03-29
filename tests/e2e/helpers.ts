import { expect, type APIRequestContext, type Page } from '@playwright/test';
import { execFileSync } from 'node:child_process';
import { existsSync, readFileSync } from 'node:fs';
import { randomUUID } from 'node:crypto';
import { setTimeout as delay } from 'node:timers/promises';

export const ADMIN_USERNAME = 'playwright-admin';
export const ADMIN_PASSWORD = 'playwright-admin-password';
export const E2E_DB_PATH = 'data/playwright-e2e.db';
export const MAIL_CAPTURE_PATH = 'data/playwright-mail-capture.jsonl';
export const ADMIN_CREDENTIALS: Credentials = {
  username: ADMIN_USERNAME,
  email: 'playwright-admin@example.invalid',
  password: ADMIN_PASSWORD,
};

export interface Credentials {
  username: string;
  email: string;
  password: string;
}

function quoteSql(value: string): string {
  return `'${value.replaceAll("'", "''")}'`;
}

export function querySqlite<T extends Record<string, unknown>>(sql: string): T[] {
  const output = execFileSync('sqlite3', ['-json', E2E_DB_PATH, sql], {
    encoding: 'utf8',
  }).trim();
  if (!output) {
    return [];
  }
  return JSON.parse(output) as T[];
}

export function execSqlite(sql: string): void {
  execFileSync('sqlite3', [E2E_DB_PATH, sql], {
    encoding: 'utf8',
  });
}

export function uniqueCredentials(prefix: string): Credentials {
  const suffix = randomUUID().replaceAll('-', '').slice(0, 10);
  return {
    username: `${prefix}-${suffix}`,
    email: `${prefix}-${suffix}@example.com`,
    password: `pw-${suffix}`,
  };
}

export async function apiLogin(request: APIRequestContext, username: string, password: string): Promise<string> {
  const response = await request.post('/api/v1/auth/login', {
    data: { username, password },
  });
  expect(response.ok(), await response.text()).toBeTruthy();
  const authorization = response.headers()['authorization'];
  expect(authorization).toMatch(/^Bearer\s+\S+/);
  return authorization.replace(/^Bearer\s+/, '');
}

export async function createBoard(request: APIRequestContext, token: string, name: string, description: string): Promise<string> {
  const response = await request.post('/api/v1/boards', {
    headers: { Authorization: `Bearer ${token}` },
    data: { name, description },
  });
  expect(response.status(), await response.text()).toBe(201);
  const body = (await response.json()) as { uuid: string };
  expect(body.uuid).toMatch(/[0-9a-f-]{36}/i);
  return body.uuid;
}

export async function hideBoard(request: APIRequestContext, token: string, boardUUID: string, hidden: boolean): Promise<void> {
  const response = await request.put(`/api/v1/admin/boards/${boardUUID}/visibility`, {
    headers: { Authorization: `Bearer ${token}` },
    data: { hidden },
  });
  expect(response.status(), await response.text()).toBe(204);
}

export async function createPost(
  request: APIRequestContext,
  token: string,
  boardUUID: string,
  title: string,
  content: string,
  tags: string[] = ['playwright', 'ui'],
): Promise<string> {
  const response = await request.post(`/api/v1/boards/${boardUUID}/posts`, {
    headers: { Authorization: `Bearer ${token}` },
    data: { title, content, tags },
  });
  expect(response.status(), await response.text()).toBe(201);
  const body = (await response.json()) as { uuid: string };
  expect(body.uuid).toMatch(/[0-9a-f-]{36}/i);
  return body.uuid;
}

export async function createDraft(
  request: APIRequestContext,
  token: string,
  boardUUID: string,
  title: string,
  content: string,
  tags: string[] = ['draft'],
): Promise<string> {
  const response = await request.post(`/api/v1/boards/${boardUUID}/posts/drafts`, {
    headers: { Authorization: `Bearer ${token}` },
    data: { title, content, tags },
  });
  expect(response.status(), await response.text()).toBe(201);
  const body = (await response.json()) as { uuid: string };
  expect(body.uuid).toMatch(/[0-9a-f-]{36}/i);
  return body.uuid;
}

export async function createComment(
  request: APIRequestContext,
  token: string,
  postUUID: string,
  content: string,
  parentUUID?: string,
): Promise<string> {
  const response = await request.post(`/api/v1/posts/${postUUID}/comments`, {
    headers: { Authorization: `Bearer ${token}` },
    data: parentUUID ? { content, parent_uuid: parentUUID } : { content },
  });
  expect(response.status(), await response.text()).toBe(201);
  const body = (await response.json()) as { uuid: string };
  expect(body.uuid).toMatch(/[0-9a-f-]{36}/i);
  return body.uuid;
}

export async function createReport(
  request: APIRequestContext,
  token: string,
  targetType: 'post' | 'comment',
  targetUUID: string,
  reasonCode: string,
  reasonDetail: string,
): Promise<number> {
  const response = await request.post('/api/v1/reports', {
    headers: { Authorization: `Bearer ${token}` },
    data: { target_type: targetType, target_uuid: targetUUID, reason_code: reasonCode, reason_detail: reasonDetail },
  });
  expect(response.status(), await response.text()).toBe(201);
  const body = (await response.json()) as { id: number };
  expect(body.id).toBeGreaterThan(0);
  return body.id;
}

export async function getUnreadCount(request: APIRequestContext, token: string): Promise<number> {
  const response = await request.get('/api/v1/users/me/notifications/unread-count', {
    headers: { Authorization: `Bearer ${token}` },
  });
  expect(response.ok(), await response.text()).toBeTruthy();
  const body = (await response.json()) as { count: number };
  return body.count;
}

export async function getUserSuspension(request: APIRequestContext, token: string, userUUID: string): Promise<{ user_uuid: string; status: string; reason?: string }> {
  const response = await request.get(`/api/v1/users/${userUUID}/suspension`, {
    headers: { Authorization: `Bearer ${token}` },
  });
  expect(response.ok(), await response.text()).toBeTruthy();
  return (await response.json()) as { user_uuid: string; status: string; reason?: string };
}

export function getUserUUIDByName(username: string): string {
  const rows = querySqlite<{ uuid: string }>(`
SELECT uuid
FROM users
WHERE name = ${quoteSql(username)}
LIMIT 1;
  `);
  const uuid = rows[0]?.uuid;
  if (!uuid) {
    throw new Error(`user not found: ${username}`);
  }
  return uuid;
}

export function getUserIDByName(username: string): number {
  const rows = querySqlite<{ id: number }>(`
SELECT id
FROM users
WHERE name = ${quoteSql(username)}
LIMIT 1;
  `);
  const id = rows[0]?.id;
  if (typeof id !== 'number') {
    throw new Error(`user not found: ${username}`);
  }
  return id;
}

export function getPostIDByUUID(postUUID: string): number {
  const rows = querySqlite<{ id: number }>(`
SELECT id
FROM posts
WHERE uuid = ${quoteSql(postUUID)}
LIMIT 1;
  `);
  const id = rows[0]?.id;
  if (typeof id !== 'number') {
    throw new Error(`post not found: ${postUUID}`);
  }
  return id;
}

export function getBoardHiddenByUUID(boardUUID: string): boolean {
  const rows = querySqlite<{ hidden: number }>(`
SELECT hidden
FROM boards
WHERE uuid = ${quoteSql(boardUUID)}
LIMIT 1;
  `);
  const hidden = rows[0]?.hidden;
  if (typeof hidden !== 'number') {
    throw new Error(`board not found: ${boardUUID}`);
  }
  return hidden !== 0;
}

export function getLatestCommentIDByPostUUIDAndContent(postUUID: string, content: string): number {
  const rows = querySqlite<{ id: number }>(`
SELECT c.id
FROM comments c
JOIN posts p ON p.id = c.post_id
WHERE p.uuid = ${quoteSql(postUUID)}
  AND c.content = ${quoteSql(content)}
ORDER BY c.id DESC
LIMIT 1;
  `);
  const id = rows[0]?.id;
  if (typeof id !== 'number') {
    throw new Error(`comment not found for post ${postUUID}`);
  }
  return id;
}

export async function seedNotification(
  recipientUserID: number,
  actorUserID: number,
  type: 'post_commented' | 'comment_replied' | 'mentioned',
  postID: number,
  commentID: number,
  actorNameSnapshot: string,
  postTitleSnapshot: string,
  commentPreviewSnapshot: string,
): Promise<void> {
  const nowNs = (BigInt(Date.now()) * 1_000_000n).toString();
  execSqlite(`
INSERT INTO notifications (
  uuid, recipient_user_id, actor_user_id, type, post_id, comment_id,
  actor_name_snapshot, post_title_snapshot, comment_preview_snapshot,
  read_at, created_at, dedup_key
)
VALUES (
  ${quoteSql(randomUUID())},
  ${recipientUserID},
  ${actorUserID},
  ${quoteSql(type)},
  ${postID},
  ${commentID},
  ${quoteSql(actorNameSnapshot)},
  ${quoteSql(postTitleSnapshot)},
  ${quoteSql(commentPreviewSnapshot)},
  NULL,
  ${nowNs},
  NULL
);
  `);
}

export async function seedDeadOutboxMessage(id: string, eventName: string, lastError: string): Promise<void> {
  const nowNs = (BigInt(Date.now()) * 1_000_000n).toString();
  execSqlite(`
INSERT INTO outbox_messages (
  id, event_name, payload, occurred_at, attempt_count, next_attempt_at, status, last_error
)
VALUES (
  ${quoteSql(id)},
  ${quoteSql(eventName)},
  json_object('id', ${quoteSql(id)}, 'event_name', ${quoteSql(eventName)}, 'kind', 'playwright'),
  ${nowNs},
  3,
  ${nowNs},
  'dead',
  ${quoteSql(lastError)}
);
  `);
}

export async function waitForOutboxRawToken(eventName: string, email: string, timeoutMs = 8000): Promise<string> {
  const deadline = Date.now() + timeoutMs;
  let lastError: unknown;
  while (Date.now() < deadline) {
    try {
      const rawToken = latestCapturedToken(eventName, email);
      if (typeof rawToken === 'string' && rawToken.trim() !== '') {
        return rawToken;
      }
    } catch (error) {
      lastError = error;
    }
    await delay(150);
  }
  throw new Error(`timed out waiting for ${eventName} token for ${email}${lastError ? `: ${String(lastError)}` : ''}`);
}

function latestCapturedToken(eventName: string, email: string): string | null {
  if (!existsSync(MAIL_CAPTURE_PATH)) {
    return null;
  }
  const content = readFileSync(MAIL_CAPTURE_PATH, 'utf8').trim();
  if (!content) {
    return null;
  }
  const lines = content.split('\n').filter((line) => line.trim() !== '');
  for (let index = lines.length - 1; index >= 0; index--) {
    const record = JSON.parse(lines[index]) as { event_name?: string; email?: string; raw_token?: string };
    if (record.event_name === eventName && record.email === email && typeof record.raw_token === 'string') {
      return record.raw_token;
    }
  }
  return null;
}

export async function signupThroughUi(page: Page, credentials: Credentials, redirect = '/') : Promise<void> {
  await page.goto(`/signup${redirect === '/' ? '' : `?redirect=${encodeURIComponent(redirect)}`}`);
  const form = page.locator('form[action^="/signup"]').first();
  await form.locator('input[name="username"]').fill(credentials.username);
  await form.locator('input[name="email"]').fill(credentials.email);
  await form.locator('input[name="password"]').fill(credentials.password);
  await Promise.all([
    page.waitForURL((url) => new URL(url.toString()).pathname === '/login', { timeout: 10000 }),
    form.getByRole('button', { name: 'Create account' }).click(),
  ]);
  await expect(page.getByRole('heading', { name: 'Login' })).toBeVisible();
  await expect(page.getByText('Account created. Please log in.')).toBeVisible();
}

export async function confirmEmailThroughUi(page: Page, token: string): Promise<void> {
  await page.goto(`/verify-email?token=${encodeURIComponent(token)}`);
  await expect(page.getByRole('heading', { name: 'Verify email' })).toBeVisible();
  const form = page.locator('form[action="/verify-email"]').first();
  await expect(form.locator('input[name="token"]')).toHaveValue(token);
  await Promise.all([
    page.waitForURL((url) => new URL(url.toString()).pathname === '/login', { timeout: 10000 }),
    form.getByRole('button', { name: 'Confirm' }).click(),
  ]);
  await expect(page.getByText('Email verified. Please log in.')).toBeVisible();
}

export async function requestPasswordReset(request: APIRequestContext, email: string): Promise<void> {
  const response = await request.post('/api/v1/auth/password-reset/request', {
    data: { email },
  });
  expect(response.ok(), await response.text()).toBeTruthy();
}

export async function confirmPasswordResetThroughUi(page: Page, token: string, newPassword: string): Promise<void> {
  await page.goto(`/reset-password?token=${encodeURIComponent(token)}`);
  await expect(page.getByRole('heading', { name: 'Reset password' })).toBeVisible();
  const form = page.locator('form[action="/reset-password"]').first();
  await expect(form.locator('input[name="token"]')).toHaveValue(token);
  await form.locator('input[name="new_password"]').fill(newPassword);
  await Promise.all([
    page.waitForURL((url) => new URL(url.toString()).pathname === '/login', { timeout: 10000 }),
    form.getByRole('button', { name: 'Reset' }).click(),
  ]);
  await expect(page.getByText('Password reset complete.')).toBeVisible();
}

export async function loginThroughUi(page: Page, credentials: Credentials, redirect = '/me'): Promise<void> {
  await page.goto(`/login?redirect=${encodeURIComponent(redirect)}`);
  const form = page.locator('form[action^="/login"]').first();
  await form.locator('input[name="username"]').fill(credentials.username);
  await form.locator('input[name="password"]').fill(credentials.password);
  await Promise.all([
    page.waitForURL((url) => new URL(url.toString()).pathname === redirect, { timeout: 10000 }),
    form.getByRole('button', { name: 'Login' }).click(),
  ]);
}

export async function logoutThroughUi(page: Page): Promise<void> {
  const forms = page.locator('form[action="/logout"]');
  await Promise.all([
    page.waitForURL((url) => new URL(url.toString()).pathname === '/login', { timeout: 10000 }),
    forms.first().getByRole('button', { name: 'Logout' }).click(),
  ]);
  await expect(page.getByRole('heading', { name: 'Login' })).toBeVisible();
}

export async function deleteAccountThroughUi(page: Page, password: string): Promise<void> {
  const form = page.locator('form[action="/me/delete"]').first();
  await form.locator('input[name="password"]').fill(password);
  await Promise.all([
    page.waitForURL((url) => new URL(url.toString()).pathname === '/', { timeout: 10000 }),
    form.getByRole('button', { name: 'Delete' }).click(),
  ]);
  await expect(page.getByRole('heading', { name: 'Feed' })).toBeVisible();
}

export async function createVerifiedUser(page: Page, request: APIRequestContext, prefix: string): Promise<Credentials> {
  const credentials = uniqueCredentials(prefix);
  await signupThroughUi(page, credentials);
  const token = await waitForOutboxRawToken('email.verification.signup.requested', credentials.email);
  await confirmEmailThroughUi(page, token);
  return credentials;
}
