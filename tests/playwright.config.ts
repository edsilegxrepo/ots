import { defineConfig, devices } from '@playwright/test';
import path from 'path';
import { getChromiumExecutablePath } from './ui/chromium-resolver';

const executablePath = getChromiumExecutablePath();
if (executablePath) {
  console.log(`[Playwright] Using custom Chromium executable path: ${executablePath}`);
}

const rootDir = path.resolve(__dirname, '..');

export default defineConfig({
  testDir: './ui/e2e',
  outputDir: '../testfiles/ui/results',
  timeout: 30000,
  use: {
    baseURL: 'http://127.0.0.1:3000',
    headless: true,
    launchOptions: {
      executablePath: executablePath || undefined,
    },
  },
  webServer: {
    command: 'go run main.go',
    cwd: rootDir,
    url: 'http://127.0.0.1:3000',
    reuseExistingServer: true,
    timeout: 30000,
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
});
