---
project: shelfmark
checked_at: 2026-10-26T16:05:00Z
health_status: needs-attention
context_type: brownfield
language_family: multi
stack_assessment_available: false
checks_run:
  - lockfile
  - dependency_audit
  - outdated_deps
  - test_runner
  - ci_cd
  - configuration
audit_findings:
  critical: 0
  high: 1
  moderate: 1
  low: 0
test_runner_detected: true
ci_provider: null
recommended_fixes: 9
---

# Health Check: Shelfmark

> Two codebases in one repo: Go backend (`backend/`, Go 1.27, pgx, golang-migrate) and a React Router 8 SPA (`web/`, Vite, Tailwind v4, built as static files and served by the Go binary). Checked on branch `shape/shelfmark` after milestone M-1 closed, as the first step towards a production release.

## Dependency Health

### Lockfile

```
Status: present (backend/go.sum, web/package-lock.json)
Package manager: go, npm
```

### Security Audit

```
Tool: govulncheck v1.9.0 (backend, `go run golang.org/x/vuln/cmd/govulncheck@latest ./...`), npm audit (web)
Summary: 0 CRITICAL, 1 HIGH, 1 MODERATE, 0 LOW (+1 informational, not reachable)
Direct vs transitive: npm finding is transitive; Go finding is in the standard library and reachable from our code
```

#### HIGH findings

- **body-parser** 2.3.0 (web, transitive via `@react-router/serve` 8.4.0 → `express` 5.3.0) — DoS via trailing bytes after a compressed request body. **Not shipped to production**: `react-router.config.ts` sets `ssr: false`, the Dockerfile copies only `web/build/client`, and the Go binary serves it — `@react-router/serve` is only used by the unused `npm start` script. Fix: remove `@react-router/serve` and the `start` script (see Fix 2).

#### MODERATE findings

- **Go standard library** `crypto/x509@go1.27.2` — GO-2026-6809: distrust compromised intermediate certificates. **Reachable**: `catalog.DepsDevClient.FindRepo` (internal/catalog/depsdev.go:66) → outbound HTTPS → `x509.Certificate.Verify`. Same path applies to the OpenRouter and GitHub clients. Fixed in go1.27.3 (see Fix 1).
- Informational, not reachable: `golang.org/x/crypto` v0.58.0 — GO-2026-5932 (`openpgp` package unmaintained). Shelfmark only uses `bcrypt`; no fix available upstream, no action needed.

### Outdated Dependencies

```
Packages with major version gaps: 2 (both dev/type-only)
```

- **typescript**: 5.9.3 → 7.0.2 (2 major versions behind) — dev-only; hold until React Router's typegen supports TS 7.
- **@types/node**: 22.20.5 → 26.6.5 (devDependency) — should track the runtime used in the Dockerfile (`node:24`), not latest.
- Patch/minor drift (not major): `react-router`, `@react-router/*` 8.4.0 → 8.5.0, `react`/`react-dom` 19.3.0 → 19.3.1, `vite` 8.3.4 → 8.3.5.
- Go direct dependencies (`cyclonedx-go`, `golang-migrate`, `pgx/v5`, `packageurl-go`, `x/crypto`): all at latest.

## Test Suite

```
Test runner: go test (backend) | not detected (web)
Tests found: 76 Go tests across catalog/auth/db | 0 frontend tests
Test execution: passing (go test ./internal/... green on 2026-10-26: auth 14s, catalog 115s, db 4s)
```

```
Configuration: none needed (go test built-in); integration tests start a throwaway postgres:16-alpine per test (tmpfs, removed with -v)
Framework: Go stdlib testing, Docker required for integration tests
```

Frontend: `npm run typecheck` (react-router typegen + `tsc`, `strict: true`) passes, but there is no unit/component/e2e runner — every frontend change is verified by hand. Already planned as test-plan §3 Phase 4 / `/10x-e2e-setup`.

## CI/CD

```
Provider: not detected
Configuration: not found (no .github/workflows, no other CI files)
```

| Stage      | Status | Notes                                   |
|------------|--------|-----------------------------------------|
| Lint       | ✗      | not configured                          |
| Test       | ✗      | not configured (go test runs locally)   |
| Build      | ✗      | not configured (Dockerfile exists)      |
| Type check | ✗      | not configured (`npm run typecheck` local) |
| Security   | ✗      | not configured                          |

ℹ No CI/CD configuration detected. This is the next infrastructure step on the release path (see Category B). For agent collaboration today, the local `go test` + `npm run typecheck` gates are sufficient.

## Configuration

### High severity

- **`.env.example`** — documents only `DATABASE_URL`, while the server requires `SESSION_SECRET`, `ADMIN_EMAIL`/`ADMIN_PASSWORD_HASH`, `ARCHITECT_EMAIL`/`ARCHITECT_PASSWORD_HASH`, `VIEWER_EMAIL`/`VIEWER_PASSWORD_HASH`, `OPENROUTER_API_KEY` and optionally `OPENROUTER_MODEL`, `GITHUB_TOKEN`, `PORT`, `STATIC_DIR`. A production deploy (and any agent setting one up) has no single source of truth for required config; the server fails fast on a missing one. Fix: see Fix 3.

### Medium severity

- **Frontend linter** (`eslint.config.*`) — none. Agent-written TSX has no automated check beyond `tsc` (e.g. hooks rules, unused vars). Fix: see Fix 5.
- **Frontend formatter** (`.prettierrc*` / `biome.json`) — none; formatting drifts per edit. Go side is covered by `gofmt` (clean) and `go vet` (clean). Fix: see Fix 5.

### Low severity

- **`.editorconfig`** — none. Fix: see Fix 8.
- `.gitignore` present and correct (`.env`, `.env.*` ignored, `!.env.example` kept; `web/.env` ignored). `tsconfig.json` has `strict: true`.

## Stack Assessment Cross-Reference

No stack-assessment.md found. Run /10x-stack-assess for quality-gate analysis.

## Recommended Fixes

### Fix before agent work (Category A)

### 1. Build with a patched Go toolchain (GO-2026-6809)

**Impact**: Reachable stdlib vulnerability in certificate verification on every outbound HTTPS call (deps.dev, OpenRouter, GitHub). Production builds must not ship go1.27.2.
**Severity**: medium
**Effort**: quick (< 5 min)
**Fix**:

```bash
cd backend && go get toolchain@go1.27.3   # pins `toolchain go1.27.3` in go.mod
```

and pin the build image in `Dockerfile`: `FROM golang:1.27.3-alpine AS backend` (the floating `1.27-alpine` tag only picks up the fix if the image is re-pulled). Re-run `govulncheck ./...` to confirm 0 reachable findings.

### 2. Drop the unused SSR server package (body-parser HIGH)

**Impact**: Removes the only HIGH audit finding and an unused production dependency tree (express, body-parser). The app is a static SPA served by Go, so nothing uses it.
**Severity**: high (audit) / low (actual exposure)
**Effort**: quick (< 5 min)
**Fix**:

```bash
cd web && npm uninstall @react-router/serve
```

then delete the `"start"` script from `web/package.json`, run `npm run build && npm run typecheck`, and confirm `npm audit` reports 0 vulnerabilities.

### 3. Complete `.env.example`

**Impact**: The deploy step (and any agent configuring it) needs the full list of required variables; today it must be reverse-engineered from `main.go`, `auth/accounts.go`, `auth/session.go`, `catalog/openrouter.go`, `catalog/github.go`.
**Severity**: high
**Effort**: quick (< 5 min)
**Fix**: list every variable with a placeholder and a one-line comment — `DATABASE_URL`, `SESSION_SECRET` (e.g. `openssl rand -hex 32`), the three `*_EMAIL` / `*_PASSWORD_HASH` pairs (bcrypt; only used to seed an empty `users` table), `OPENROUTER_API_KEY`, optional `OPENROUTER_MODEL`, `GITHUB_TOKEN`, `PORT` (default 8080), `STATIC_DIR`. Never real values.

### 4. Frontend test coverage

**Impact**: Agents change `web/app/routes/*` without any automated check of rendering, error/empty states or role mismatches (test-plan risk #5).
**Severity**: medium
**Effort**: significant (> 1 hour)
**Fix**: run `/10x-e2e-setup` (Playwright, smoke of login → upload → report), and later test-plan §3 Phase 4 for component tests (`npm init vitest@latest` + `@testing-library/react`).

### 5. Frontend lint + format

**Impact**: Consistent agent output and a cheap per-edit gate for `/10x-configure-hook`.
**Severity**: medium
**Effort**: moderate (15–30 min)
**Fix**:

```bash
cd web && npm i -D eslint @eslint/js typescript-eslint eslint-plugin-react-hooks prettier
```

add `eslint.config.js` (recommended + react-hooks), `.prettierrc`, and scripts `"lint": "eslint ."`, `"format": "prettier --write ."`. Run once and commit the formatting separately.

### 6. Patch/minor dependency refresh

**Impact**: Keeps React Router and Vite on current patch levels before release.
**Severity**: low
**Effort**: quick (< 5 min)
**Fix**: `cd web && npm update && npm run typecheck && npm run build`.

### 7. Align `@types/node` with the runtime

**Impact**: Type definitions should match the Node used to build (`node:24` in Dockerfile).
**Severity**: low
**Effort**: quick (< 5 min)
**Fix**: `cd web && npm i -D @types/node@24`. Leave TypeScript on 5.x for now.

### 8. Add `.editorconfig`

**Impact**: Consistent indentation (tabs for Go, 2 spaces for TS/CSS/MD) across editors and agents.
**Severity**: low
**Effort**: quick (< 5 min)
**Fix**: root `.editorconfig` with `[*.go] indent_style = tab`, `[*.{ts,tsx,js,json,css,md,yml}] indent_style = space, indent_size = 2`.

### 9. Reclaim leaked Docker volumes on the dev host

**Impact**: Before the test-helper fix (commit b142e00) every integration test left a volume; the host still has ~180 dangling volumes. A full disk silently breaks the whole Go suite ("postgres did not become ready").
**Severity**: low
**Effort**: quick (< 5 min)
**Fix**: `docker volume prune -f` (removes only anonymous, unused volumes).

### Addressed in upcoming lessons (Category B)

### No CI/CD pipeline

**Lesson**: [Sprint Zero z Agentem: infrastruktura, walking skeleton i pierwszy deploy (M1L5)](https://platforma.przeprogramowani.pl/external/10xdevs-3/m1-l5)
**What you'll do there**: Add a pipeline running `go vet`, `go test ./...` (Docker service), `npm run typecheck`, `npm run build`, `govulncheck` and `npm audit` on every PR — planned as release step 5 (`/10x-infra-research` → `prod-deploy` change).

### No deployment configuration

**Lesson**: [Sprint Zero z Agentem: infrastruktura, walking skeleton i pierwszy deploy (M1L5)](https://platforma.przeprogramowani.pl/external/10xdevs-3/m1-l5)
**What you'll do there**: Pick a platform for the existing Dockerfile + managed Postgres, wire secrets, migrations-on-start and backups.

### Agent instruction files

**Lesson**: [Agent Onboarding: Agents.md, AI Rules i feedback loops (M1L4)](https://platforma.przeprogramowani.pl/external/10xdevs-3/m1-l4)
**What you'll do there**: `CLAUDE.md` exists (course chain + brand tokens); review it with `/10x-rule-review` and generate an `AGENTS.md` with `/10x-agents-md` once lint/test commands are final.

## Summary

Health status: needs-attention

The backend is in good shape for agent work: lockfiles present, direct Go dependencies current, `gofmt`/`go vet` clean, and 76 Go tests passing against real Postgres. The gaps are concentrated on release hygiene and the frontend: a reachable Go stdlib advisory fixed by moving to go1.27.3, an unused SSR package carrying the only HIGH npm advisory, an incomplete `.env.example`, and no frontend tests, linter or formatter.

Next step: apply the quick fixes 1–3 (and 6–9) in one small change, then continue the release path with `/10x-observability-audit`; fixes 4–5 feed into `/10x-e2e-setup` and `/10x-configure-hook`.
