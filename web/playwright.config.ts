import { existsSync } from 'node:fs';
import { defineConfig, devices } from '@playwright/test';

// Local secrets (E2E_USERNAME, E2E_PASSWORD, app env) come from the gitignored
// repo-root .env. In CI the file is absent and the variables come from the job.
if (existsSync('../.env')) process.loadEnvFile('../.env');

// 8080 was detected from backend/cmd/shelfmark/main.go (envOr("PORT", "8080")).
// E2E_PORT overrides it when that port is taken on this machine, e.g. by a
// dev backend: outside CI a server already on the port is reused silently.
const PORT = Number(process.env.E2E_PORT ?? 8080);
const baseURL = `http://localhost:${PORT}`;

export default defineConfig({
  testDir: './tests/e2e',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: 'html',
  use: {
    baseURL,
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
  },
  projects: [
    { name: 'setup', testMatch: /.*\.setup\.ts/ },
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'], storageState: 'playwright/.auth/user.json' },
      dependencies: ['setup'],
    },
  ],
  webServer: {
    // Production-like: build the SPA, then build and run the Go binary that
    // serves it plus /api, exactly as the Docker image does. Needs the local
    // Postgres from docker-compose.yml (docker compose up -d db).
    command: 'npm run build && cd ../backend && go build -o shelfmark ./cmd/shelfmark && ./shelfmark',
    url: baseURL,
    reuseExistingServer: !process.env.CI,
    timeout: 180_000,
    env: {
      PORT: String(PORT),
      STATIC_DIR: '../web/build/client',
      // A separate local database keeps E2E data out of the dev database.
      DATABASE_URL: process.env.E2E_DATABASE_URL ?? '',
      // The backend seeds accounts only into an empty users table, so a fresh
      // E2E database gets the test user as its architect account.
      ARCHITECT_EMAIL: process.env.E2E_USERNAME ?? '',
      ARCHITECT_PASSWORD_HASH: process.env.E2E_PASSWORD_HASH ?? '',
    },
  },
});
