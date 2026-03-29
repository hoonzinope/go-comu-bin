import { defineConfig, devices } from '@playwright/test';

/**
 * Read environment variables from file.
 * https://github.com/motdotla/dotenv
 */
// import dotenv from 'dotenv';
// import path from 'path';
// dotenv.config({ path: path.resolve(__dirname, '.env') });

/**
 * See https://playwright.dev/docs/test-configuration.
 */
export default defineConfig({
  testDir: './tests',
  /* Run UI smoke tests sequentially so shared seeded state stays stable */
  fullyParallel: false,
  /* Fail the build on CI if you accidentally left test.only in the source code. */
  forbidOnly: !!process.env.CI,
  /* Retry on CI only */
  retries: process.env.CI ? 2 : 0,
  /* Opt out of parallel tests on CI. */
  workers: 1,
  /* Reporter to use. See https://playwright.dev/docs/test-reporters */
  reporter: 'html',
  /* Shared settings for all the projects below. See https://playwright.dev/docs/api/class-testoptions. */
  use: {
    /* Base URL to use in actions like `await page.goto('')`. */
    baseURL: 'http://127.0.0.1:18578',

    /* Collect trace when retrying the failed test. See https://playwright.dev/docs/trace-viewer */
    trace: 'on-first-retry',
  },

  /* Configure projects for major browsers */
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],

  /* Run the Go server before starting the tests */
  webServer: {
    command:
      'sh -lc "rm -f ./data/playwright-e2e.db ./data/playwright-mail-capture.jsonl ./logs/playwright-e2e.jsonl && rm -rf ./data/uploads-playwright && mkdir -p ./data ./logs && DELIVERY_HTTP_PORT=18578 DATABASE_PATH=./data/playwright-e2e.db LOGGING_FILEPATH=./logs/playwright-e2e.jsonl STORAGE_LOCAL_ROOT_DIR=./data/uploads-playwright DELIVERY_HTTP_AUTH_SECRET=playwright-e2e-secret-0123456789abcdef PLAYWRIGHT_MAIL_CAPTURE_PATH=./data/playwright-mail-capture.jsonl DELIVERY_HTTP_AUTH_LOGINRATELIMIT_ENABLED=false DELIVERY_HTTP_AUTH_GUESTUPGRADERATELIMIT_ENABLED=false DELIVERY_HTTP_AUTH_EMAILVERIFICATIONREQUESTRATELIMIT_ENABLED=false DELIVERY_HTTP_AUTH_PASSWORDRESETREQUESTRATELIMIT_ENABLED=false ADMIN_BOOTSTRAP_ENABLED=true ADMIN_BOOTSTRAP_USERNAME=playwright-admin ADMIN_BOOTSTRAP_PASSWORD=playwright-admin-password JOBS_ENABLED=false go run ./cmd"',
    url: 'http://127.0.0.1:18578',
    reuseExistingServer: false,
    timeout: 120000,
  },
});
