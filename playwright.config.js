// @ts-check
const { defineConfig, devices } = require('@playwright/test');

const launchOptions = {};
if (process.env.CHROMIUM_EXECUTABLE_PATH) {
  launchOptions.executablePath = process.env.CHROMIUM_EXECUTABLE_PATH;
}

module.exports = defineConfig({
  testDir: './tests/ui',
  timeout: 60_000,
  expect: {
    timeout: 10_000,
  },
  reporter: [['list'], ['html', { open: 'never' }]],
  use: {
    baseURL: process.env.UI_URL || 'http://127.0.0.1:8081',
    trace: 'retain-on-failure',
  },
  projects: [
    {
      name: 'chromium',
      use: {
        ...devices['Desktop Chrome'],
        launchOptions,
      },
    },
  ],
});
