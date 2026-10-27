# Test stack

## E2E

<!-- Written by /10x-e2e-setup. Re-run it to change this section; other skills only read it. -->

- runner: Playwright Test, @playwright/test 1.64.1
- config: web/playwright.config.ts
- single-spec command: cd web && npx playwright test tests/e2e/<name>.spec.ts
- full-suite command: cd web && npx playwright test
- base URL: http://localhost:8080
- port: 8080 (detected from backend/cmd/shelfmark/main.go envOr("PORT", "8080"); detected default 8080, override with E2E_PORT)
- web server command: npm run build && cd ../backend && go build -o shelfmark ./cmd/shelfmark && ./shelfmark (PORT=$E2E_PORT or 8080, STATIC_DIR=../web/build/client, DATABASE_URL=$E2E_DATABASE_URL, architect seed account = E2E user); reuseExistingServer outside CI; needs the local Postgres from docker-compose.yml (`docker compose up -d db`) with database shelfmark_e2e
- auth setup project: setup (web/tests/e2e/auth.setup.ts), credentials from E2E_USERNAME / E2E_PASSWORD in the repo-root .env (E2E_PASSWORD_HASH seeds the architect account into the empty shelfmark_e2e database)
- storageState: web/playwright/.auth/user.json (gitignored)
- seed: web/tests/e2e/seed.spec.ts — protects role-gate-redirect: a signed-in architect opening the admin-only /catalog is sent to /dashboard and never sees the catalog (UI side of test-plan risks #3 / #5)
- browser CLI: playwright-cli (global, 0.1.22, use -s=shelfmark), command skill at .claude/skills/playwright-cli/SKILL.md
- updated: 2026-10-27
