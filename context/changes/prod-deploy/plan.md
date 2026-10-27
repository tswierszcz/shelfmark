# Production Deploy (Fly.io) Implementation Plan

## Overview

Ship Shelfmark M1 to production at `https://shelfmark.fly.dev`: one always-on Fly Machine in `fra` plus Fly Managed Postgres (PG 16) in the same region, as recommended in `context/foundation/infrastructure.md`. Before the first deploy, fix the code-level blockers that only bite in production (secrets in boot logs, session cookie without `Secure` behind Fly's TLS-terminating proxy). Add a GitHub Actions pipeline that tests every PR and auto-deploys on push to `main`, matching `tech-stack.md` (`ci_default_flow: auto-deploy-on-merge`). Finally, bring `main` up to date and prove the operational loop: deploy, rollback, backup restore.

## Current State Analysis

- **Image is ready, deploy config is not.** The root `Dockerfile` builds a distroless image (SPA + Go binary, `ARG VERSION` → `obs.Version`, `PORT=8080`). There is no `fly.toml`, no `.github/workflows/`, and `flyctl` is not installed locally.
- **Graceful shutdown already exists** for SIGINT and SIGTERM (`backend/cmd/shelfmark/main.go`), with a 25 s `shutdownBudget` for the HTTP drain plus the queue drain. Fly defaults to `kill_signal = SIGINT` and `kill_timeout = 5 s`, so `fly.toml` has to set both explicitly.
- **Migrations run on boot** (`db.Migrate` in `main.go`). That stays as-is for M1 (decision below).
- **Boot errors leak secrets (impl-review F3, skipped in the observability change).** `pgx5DatabaseURL` returns the raw `url.Parse` error, which contains the full URL including the password, and `Migrate` wraps it with `%w` (`backend/internal/db/migrate.go:31`, `:51-54`). The seed error embeds the account email via `%q` (`backend/internal/auth/users_store.go:136`). Both reach `fatal()` in `main.go` and land in JSON logs.
- **The session cookie loses `Secure` in production.** `backend/internal/auth/session.go:95` sets `Secure: r.TLS != nil`. On Fly, TLS terminates at the edge proxy and the app sees plain HTTP, so `r.TLS` is always nil.
- **Comment drift (impl-review F4).** The `shutdownBudget` comment says it stays "inside Fly's default kill timeout", but that default is 5 s.
- **Git: `main` and `shape/shelfmark` share no history.** `git merge-base` prints nothing. `main` holds a single commit with `README.txt`, the same one-line file the branch has. PR #1 (M-1 milestone) was closed unmerged.
- **Tests start their own Postgres via `docker run`** (`backend/internal/catalog/store_test.go:51`, `auth/handlers_test.go`), so CI on `ubuntu-latest` (which has Docker) needs no Postgres service.
- **Required runtime env:** `DATABASE_URL`, `SESSION_SECRET`, `OPENROUTER_API_KEY`, `ADMIN_EMAIL`/`ADMIN_PASSWORD_HASH`, `ARCHITECT_EMAIL`/`ARCHITECT_PASSWORD_HASH`, `VIEWER_EMAIL`/`VIEWER_PASSWORD_HASH`. Optional: `GITHUB_TOKEN`, `OPENROUTER_MODEL`. The current OpenRouter key returns 401 and must be replaced.

## Desired End State

- `https://shelfmark.fly.dev/healthz` returns `200 ok` from one Machine in `fra` that runs the image built from `main`. JSON logs carry `version=<git sha>`.
- An admin can log in, upload a catalog that gets categorized (live OpenRouter key), and an architect can upload an SBOM and see the redundancy report. The `shelfmark_session` cookie is `Secure; HttpOnly; SameSite=Lax`.
- `main` contains the full project history. Every PR runs Go tests plus a web typecheck and build, and every push to `main` deploys to Fly automatically with `VERSION=<sha>`.
- A rollback to the previous release has been performed once with the exact command recorded, and an MPG backup has been test-restored once. Both are documented in `docs/runbook.md`.
- Boot failures never print the `DATABASE_URL` password or a seed email.

### Key Discoveries:

- `backend/internal/db/migrate.go:31,51-54`: `url.Parse` error wrapped with `%w` leaks the DSN.
- `backend/internal/auth/users_store.go:136`: the seed error includes the email. `Account.Role` is available in the loop for a safe replacement.
- `backend/internal/auth/session.go:95`: `Secure` is derived only from `r.TLS`. The logout cookie at `:156` must get the same treatment.
- `backend/cmd/shelfmark/main.go`: `shutdownBudget = 25s` and its comment, plus the SIGTERM handling already in place.
- `backend/internal/obs/obs.go:16-22`: `Version` is injected by ldflags and attached to every log line.
- `backend/internal/db/health.go:17`: `/healthz` pings the DB and answers 200 with no redirect, so it works as a Fly HTTP check.
- `infrastructure.md` §Unknown Unknowns: Fly checks don't follow redirects, private cross-region traffic is billed, the MPG major version is fixed at creation, and the distroless image has no shell.

## What We're NOT Doing

- No throwaway Fly staging app. Verification is a local run of the exact production image.
- No `release_command` or `shelfmark migrate` subcommand. Migrations stay on boot.
- No second Machine, no `bluegreen` strategy, no multi-region setup. The brief downtime on every deploy is accepted for M1.
- No custom domain. Production uses `shelfmark.fly.dev`.
- No E2E/Playwright in CI (release-path step 6) and no `/10x-impl-review-ci` workflow (step 7).
- No PR preview environments.
- No Postgres upgrade: MPG is created on PG 16 to match `docker-compose.yml`.

## Implementation Approach

Fix the code first so that nothing unsafe ever reaches production logs, then make the deploy reproducible (`fly.toml` + CI), then do the irreversible, billed steps with a human at the keyboard. The first production deploy is manual from the branch (per the `infrastructure.md` approval rules). After it is healthy, the `FLY_API_TOKEN` goes into GitHub and the merge to `main` triggers the second deploy through CI, which proves the pipeline against a live app. Rollback and restore drills run last, against real data.

## Critical Implementation Details

- **Ordering of Phase 4 vs Phase 5.** The deploy job must not run against a Fly app that doesn't exist yet. Provision the app, MPG and secrets and do the first manual deploy (Phase 4) before adding `FLY_API_TOKEN` to GitHub and merging to `main` (Phase 5). The deploy job also skips cleanly when the token secret is absent, so an early merge cannot fail red for the wrong reason.
- **Health check grace.** Boot runs migrations before the server listens. Give the `/healthz` check a `grace_period` (≈30 s) so a first-boot migration isn't killed as unhealthy.
- **`X-Forwarded-Proto` trust.** Only *adding* `Secure` is driven by the header. A spoofed `https` value merely makes the cookie stricter, so no proxy allowlist is needed.

## Phase 1: Production-safety code fixes

### Overview

Close impl-review F3 and F4 and make the session cookie `Secure` behind a TLS-terminating proxy, each covered by a regression test.

### Changes Required:

#### 1. DSN redaction in migrations

**File**: `backend/internal/db/migrate.go`

**Intent**: When `DATABASE_URL` can't be parsed, return a fixed message without the underlying `url.Parse` error, so the password never reaches logs.

**Contract**: `Migrate(databaseURL string) error`. On a parse failure the error string is exactly `db: failed to parse DATABASE_URL` and does not wrap the `*url.Error`. Other error paths stay unchanged.

#### 2. DSN redaction in Connect

**File**: `backend/internal/db/pool.go`

**Intent**: pgx documents its parse-error redaction as best effort only (`pgconn/config.go:332-336`), so don't rely on it. Parse the DSN explicitly and return a fixed message on failure.

**Contract**: `Connect(ctx) (*pgxpool.Pool, error)` parses via `pgxpool.ParseConfig`. On error it returns exactly `db: failed to parse DATABASE_URL`, without wrapping the parse error, then builds the pool with `pgxpool.NewWithConfig`. The missing-env and ping error paths are unchanged (connect errors from pgconn name user, host and database, never the password).

#### 3. Seed error without email

**File**: `backend/internal/auth/users_store.go`

**Intent**: Identify the failing seed account by role instead of email.

**Contract**: The seed insert error reads `auth: failed to seed <role> account: %w`. No email appears in any `SeedFromEnvIfEmpty` error.

#### 4. Secure cookie behind proxy

**File**: `backend/internal/auth/session.go`

**Intent**: Mark the session and logout cookies `Secure` when the request came over HTTPS, either directly (`r.TLS != nil`) or via a proxy (`X-Forwarded-Proto: https`).

**Contract**: One unexported helper `requestIsHTTPS(r *http.Request) bool`, used by the session cookie (`:95`) and by `ClearSession`. `ClearSession` currently takes no request, so its signature becomes `ClearSession(w http.ResponseWriter, r *http.Request)`, and its only caller, `LogoutHandler` (`backend/internal/auth/handlers.go:80`), passes `r`. The deletion cookie then carries the same attributes as the cookie it clears. Plain local HTTP keeps `Secure=false`, so local dev login still works.

#### 5. shutdownBudget comment

**File**: `backend/cmd/shelfmark/main.go`

**Intent**: Correct the comment: the 25 s budget must stay below `kill_timeout = 30` set in `fly.toml` (Fly's default is 5 s).

**Contract**: Comment only. The value stays 25 s.

#### 6. Tests

**Files**: `backend/internal/db/migrate_test.go`, `backend/internal/db/pool_test.go`, `backend/internal/auth/session_test.go`, `backend/internal/auth/handlers_test.go` (or the existing seed test file)

**Intent**: Regression-lock each fix with named tests:
- `TestMigrate_ParseErrorDoesNotLeakPassword`: a DSN with password `s3cr3t-pw` and an invalid port/escape fails, and the error contains neither `s3cr3t-pw` nor the host string.
- `TestConnect_ErrorDoesNotLeakPassword`: same assertion for `Connect`.
- `TestSeed_ErrorOmitsEmail`: on an empty `users` table in the test's throwaway DB, add `ALTER TABLE users ADD CONSTRAINT seed_reject CHECK (email <> '<sentinel>')`, then set `ADMIN_EMAIL=<sentinel>` (other seed env valid) and call `SeedFromEnvIfEmpty`. Assert the error names `admin` and does not contain the sentinel. Pre-inserting a row or closing the pool does not work: `count > 0` returns early (`users_store.go:113`), and a closed pool fails in `Count`, before any insert.
- `TestSessionCookie_SecureBehindProxy`: a table with no TLS and no header → `false`, `X-Forwarded-Proto: https` → `true`, `X-Forwarded-Proto: http` → `false`, and TLS set → `true`. It covers both the login and logout cookies.

**Contract**: Tests assert the error strings and cookie attributes directly, with no mocking of the code under test.

### Success Criteria:

#### Automated Verification:

- Named regression tests pass: `cd backend && go test ./internal/db/ ./internal/auth/ -run 'TestMigrate_ParseErrorDoesNotLeakPassword|TestConnect_ErrorDoesNotLeakPassword|TestSeed_ErrorOmitsEmail|TestSessionCookie_SecureBehindProxy' -v` (all four names appear as PASS)
- Full backend suite passes: `cd backend && go vet ./... && go test ./...`
- Break check: reverting the `migrate.go` fix makes `TestMigrate_ParseErrorDoesNotLeakPassword` fail (then restore)

#### Manual Verification:

- Local login over plain HTTP (`http://localhost:8080`) still works, so the cookie is not `Secure` there

**Implementation Note**: After automated verification passes, pause for manual confirmation before Phase 2.

---

## Phase 2: fly.toml and local production-image check

### Overview

Add the Fly app configuration and prove that the exact production image boots, reports its version, passes `/healthz` and drains cleanly on SIGTERM, all locally. Draft the operations runbook.

### Changes Required:

#### 1. Fly configuration

**File**: `fly.toml` (repo root, new)

**Intent**: Declare the single always-on Machine with the shutdown, health and HTTPS settings from `infrastructure.md`.

**Contract**:
- `app = "shelfmark"`, `primary_region = "fra"`, `kill_signal = "SIGTERM"`, `kill_timeout = 30` (seconds)
- `[build]` uses the root `Dockerfile`
- `[http_service]`: `internal_port = 8080`, `force_https = true`, `auto_stop_machines = "off"`, `auto_start_machines = true`, `min_machines_running = 1`
- One HTTP check: `GET /healthz`, interval 15 s, timeout 5 s, `grace_period = "30s"`
- `[[vm]]`: `shared-cpu-1x`, 512 MB
- No secrets in the file. `VERSION` is passed with `--build-arg` at deploy time, not hard-coded.

Validate the keys against the `fly.toml` reference and `fly config validate` once `flyctl` is installed.

#### 2. Docker build context hygiene

**File**: `.dockerignore`

**Intent**: The final distroless stage copies only the binary and `/app/public`, so the image itself can't carry local files. The exposure is the build context uploaded to Fly's remote builder. `.dockerignore` already excludes `.git`, `.claude`, `context`, `.env*` and `*.md`. Add `test.csv` (and any other local scratch data in the repo root).

**Contract**: An ignore list only. The build still succeeds.

#### 3. Runbook draft

**File**: `docs/runbook.md` (new)

**Intent**: The single operational reference: first-time provisioning, secrets list, deploy (CI and manual), logs and status, rollback, backup/restore, and the "migrations don't roll back, keep them additive" rule. Commands the infra research left unverified (`fly deploy --image`, `fly mpg …` subcommands) are marked "verify on first use" until Phases 4–5 confirm them.

**Contract**: Sections: Overview, Secrets, First-time setup, Deploy, Observe, Rollback, Backup & restore, Known limits. No secret values.

### Success Criteria:

#### Automated Verification:

- Image builds with a version stamp: `docker build --build-arg VERSION=$(git rev-parse --short HEAD) -t shelfmark:prod-check .`
- Image runs against a throwaway tmpfs Postgres (port 55432) with generated test env, and `curl -fsS localhost:8091/healthz` returns `ok`
- Container logs are JSON and contain `"version":"<short sha>"`
- `docker stop --time 30` on the container yields `shutting down` then `shutdown complete` in the logs, and the container exits with code 0 well under 30 s
- Build-context check: building a throwaway Dockerfile (`FROM busybox`, `COPY . /ctx`, `RUN find /ctx -maxdepth 2`) with the repo as context lists no `.env*`, `context/`, `.git`, `.claude` or `test.csv` paths

#### Manual Verification:

- Review `fly.toml` values against the infrastructure doc (region, SIGTERM, 30 s, single machine, `force_https`)
- Read `docs/runbook.md` and confirm it is understandable without this plan

**Implementation Note**: The throwaway instance follows the project's established local pattern (tmpfs Postgres on 55432, app on 8091, generated accounts in a scratchpad env file, never printed). Clean up containers afterwards. Pause for manual confirmation before Phase 3.

---

## Phase 3: CI/CD workflow

### Overview

Join the branch's history with `main`, so a PR can exist at all, and add one GitHub Actions workflow that tests every PR and push and deploys to Fly on push to `main` after the tests pass.

### Changes Required:

#### 0. Join the histories

**Intent**: GitHub refuses to compare or open a PR between branches with no common history. `shape/shelfmark` was rewritten by filter-repo (the `backup/shape-shelfmark-pre-filter-*` root is main's 4459e3c, while the current branch has none). The join has to happen before the PR in this phase.

**Contract**: On `shape/shelfmark`, run `git fetch origin` and `git merge origin/main --allow-unrelated-histories`, resolving `README.txt` by keeping the branch version. The result is a merge commit with two roots, so `git merge-base origin/main HEAD` now returns 4459e3c. No force-push anywhere.

#### 1. Workflow

**File**: `.github/workflows/ci.yml` (new)

**Intent**: Gate merges on the same checks the local hooks run, and make every production deploy come from `main` with a traceable version.

**Contract**:
- Triggers: `pull_request` (any branch) and `push` to `main`.
- Job `backend`: `actions/setup-go` with `go-version-file: backend/go.mod`, then `go vet ./...` and `go test ./...` in `backend/`. Docker is available on `ubuntu-latest` for the test-started Postgres containers.
- Job `web`: `actions/setup-node` (Node 24, npm cache), then `npm ci`, `npm run typecheck` and `npm run build` in `web/`.
- Job `deploy`: `needs: [backend, web]`, runs only when `github.event_name == 'push' && github.ref == 'refs/heads/main'`. Uses `concurrency: { group: fly-deploy, cancel-in-progress: false }`, `superfly/flyctl-actions/setup-flyctl` pinned to a released version, then `flyctl deploy --remote-only --build-arg VERSION=${{ github.sha }}` with `FLY_API_TOKEN` from repo secrets. If the secret is empty, the job prints a notice and exits 0 instead of failing.
- Action versions are pinned to existing release tags. Permissions are `contents: read`.

### Success Criteria:

#### Automated Verification:

- Workflow lints clean: `actionlint .github/workflows/ci.yml` (run via `docker run --rm -v "$PWD":/repo -w /repo rhysd/actionlint:latest` if not installed)
- The commands the jobs run pass locally: `cd backend && go vet ./... && go test ./...` and `cd web && npm ci && npm run typecheck && npm run build`

#### Manual Verification:

- After the branch is pushed, the PR to `main` shows green `backend` and `web` checks and a skipped `deploy` job

**Implementation Note**: Pushing the branch and opening the PR `shape/shelfmark` → `main` (a new PR; #1 stays closed) are outward-facing, so ask before doing either. The PR stays open, unmerged, until Phase 5. Pause for manual confirmation before Phase 4.

---

## Phase 4: Provisioning and first manual production deploy

### Overview

The human-gated, billed steps: Fly account, app, MPG cluster, secrets, then the first `fly deploy` from the branch and a full smoke test.

### Changes Required:

#### 1. Provisioning (human runs; agent prepares exact commands)

**Intent**: Create the production resources in `fra` exactly once, with nothing created outside this plan.

**Contract**, in this order:
1. Install `flyctl` and run `fly auth login` (human; card required).
2. Create the app with `fly apps create shelfmark` (or `fly launch --no-deploy` while keeping the committed `fly.toml`). If the name is taken, pick a fallback name, update `app` in `fly.toml`, and use the new `*.fly.dev` host everywhere.
3. Re-check MPG release status, then create the cluster in `fra` on PG 16 with the smallest plan (`fly mpg create …`) and attach it to the app (`fly mpg attach …`), which sets `DATABASE_URL`. Verify subcommand flags with `--help` first.
4. Set the remaining secrets with `fly secrets set` (staged with `--stage` so they apply on the first deploy): `SESSION_SECRET` (fresh random 32+ bytes), a **new** `OPENROUTER_API_KEY`, the three `*_EMAIL` / `*_PASSWORD_HASH` pairs (hashes from `backend/cmd/hashpw`), plus `GITHUB_TOKEN` if desired. Values are typed by the human, never echoed into the transcript.

#### 2. First deploy (human approves; agent may run)

**Intent**: Deploy the Phase 1–3 branch head and prove the full user loop in production.

**Contract**: `fly deploy --build-arg VERSION=$(git rev-parse --short HEAD)` from a clean working tree on `shape/shelfmark`.

#### 3. Runbook update

**File**: `docs/runbook.md`

**Intent**: Replace "verify on first use" markers with the commands that actually worked, and record the app name, region, MPG cluster name/plan, and the date of first deploy.

**Contract**: Runbook sections updated. Still no secret values.

### Success Criteria:

#### Automated Verification:

- `fly config validate` passes
- `fly status` shows exactly 1 Machine, in `fra`, state `started`, with the check passing
- `curl -fsS https://shelfmark.fly.dev/healthz` returns `ok`
- `curl -sI http://shelfmark.fly.dev/` redirects to `https://`
- `fly logs --no-tail` contains `listening` with `"version":"<short sha>"` and no `DATABASE_URL`, `postgres://` or seed-email strings

#### Manual Verification:

- Login as admin works, and DevTools shows `shelfmark_session` with `Secure`, `HttpOnly` and `SameSite=Lax`
- Admin uploads a small catalog and libraries reach a categorized state, meaning the OpenRouter key works (no 401 in logs)
- Architect uploads an SBOM and sees the redundancy report
- The MPG cluster and app are both in `fra` (`fly mpg list` / `fly status`)

**Implementation Note**: Everything that costs money or creates infrastructure is run or explicitly approved by the human (per `infrastructure.md` §Approval). Pause for manual confirmation before Phase 5.

---

## Phase 5: Merge to main and operational verification

### Overview

Bring `main` up to date through a reviewed PR, prove the CI auto-deploy against the live app, and run the rollback and backup-restore drills once.

### Changes Required:

#### 1. Merge the release PR

**Intent**: Make `main` carry the full project without force-pushing. The histories were already joined in Phase 3.

**Contract**: Push any Phase 4 runbook commits to the open PR from Phase 3. After `FLY_API_TOKEN` is set (§2), its checks are green and the human approves, merge it with a merge commit (not squash, which would drop the joined history). Pushing and merging each need an explicit go-ahead.

#### 2. CI deploy token

**Intent**: Let the `deploy` job run.

**Contract**: A human creates a deploy-scoped token (`fly tokens create deploy`) and stores it as the GitHub repo secret `FLY_API_TOKEN` before the merge.

#### 3. Rollback drill

**Intent**: Verify the rollback path the infra research flagged as unverified.

**Contract**: Get the previous release image with `fly releases --image`, run `fly deploy --image <previous-ref>`, confirm `/healthz` and the older `version` in the logs, then roll forward again (re-run the CI deploy or `fly deploy --image <current-ref>`). Record the exact commands and timings in the runbook.

#### 4. Backup restore drill

**Intent**: Prove MPG backups are restorable while MPG's GA status is unconfirmed.

**Contract**: Restore the latest backup into a new temporary cluster or fork (the exact `fly mpg` command is verified with `--help`). Connect via `fly mpg connect` or a proxy and compare row counts of `users`, `libraries` and `projects` with production. Destroy the temporary cluster (human). Record the date, commands and result in the runbook.

#### 5. Foundation docs touch-up

**Files**: `docs/runbook.md`, `context/foundation/infrastructure.md` (Risk Register "Mitigation" notes only, if a drill changed the picture)

**Intent**: Leave the final, verified operating procedure, and mark the two risk-register items (rollback unverified, restore untested) as addressed.

**Contract**: Text only.

### Success Criteria:

#### Automated Verification:

- Release commit is on main: after `git fetch`, `git merge-base --is-ancestor <PR head sha> origin/main` exits 0
- The GitHub Actions run for the merge commit on `main` succeeds on all three jobs (`gh run list --branch main --limit 1`)
- After the CI deploy, `fly logs --no-tail` / `curl` show the merge commit SHA as `version` and `/healthz` returns `ok`

#### Manual Verification:

- Rollback drill: the previous version served, then the current version restored, with the commands recorded in the runbook
- Backup restore drill: the restored row counts match production, the temporary cluster is destroyed, and the result is recorded in the runbook
- Final smoke test after the drills: login → upload → report still works on `https://shelfmark.fly.dev`

**Implementation Note**: The rollback and restore drills touch production. Run them only with the human present and approving each billed or destructive step.

---

## Testing Strategy

### Unit Tests:

- Error-string redaction for `Migrate`, `Connect` and the seed path, using a sentinel password and email so absence is unambiguous.
- Cookie `Secure` derivation as a table over TLS / `X-Forwarded-Proto` combinations, for both the login and logout cookies.

### Integration Tests:

- The existing suite (`go test ./...`, with Postgres via Docker) must stay green locally and in CI.
- Local prod-image run: healthz, version stamp, SIGTERM drain.

### Manual Testing Steps:

1. Local plain-HTTP login still works (cookie not `Secure`).
2. Production: login (check cookie flags), catalog upload → categorized, SBOM upload → report.
3. Rollback to the previous image and back.
4. Restore a backup to a temporary cluster and compare counts.

## Performance Considerations

A single `shared-cpu-1x` 512 MB Machine is enough for M1 traffic. The queue's 5 workers are bounded by OpenRouter/GitHub latency, not CPU. Watch memory in `fly status` / metrics during the first catalog upload, and bump to 1 GB only if an OOM shows up.

## Migration Notes

- No schema changes in this plan. Production starts empty: migrations run on first boot, and the seed accounts are created from secrets.
- Rolling back the image does not roll back the schema. Future migrations must stay additive (expand, then contract), as stated in the runbook.

## References

- Infrastructure decision: `context/foundation/infrastructure.md`
- Tech stack (CI/deploy flow): `context/foundation/tech-stack.md`
- F3/F4 origin: `context/archive/2026-10-26-observability-logging-queue-supervision/reviews/impl-review.md`
- Shutdown handling: `backend/cmd/shelfmark/main.go`
- Cookie: `backend/internal/auth/session.go:95,156`
- DSN parse: `backend/internal/db/migrate.go:31,50-57`
- Seed error: `backend/internal/auth/users_store.go:136`

## Progress

> Convention: `- [ ]` pending, `- [x]` done. Append ` — <commit sha>` when a step lands. Do not rename step titles. See `references/progress-format.md`.

### Phase 1: Production-safety code fixes

#### Automated

- [x] 1.1 Named regression tests pass (migrate/connect/seed redaction, Secure cookie behind proxy) — 4838ab7
- [x] 1.2 Full backend suite passes (go vet + go test ./...) — 4838ab7
- [x] 1.3 Break check: reverting migrate.go fix turns its test red — 4838ab7

#### Manual

- [x] 1.4 Local plain-HTTP login still works — 4838ab7

### Phase 2: fly.toml and local production-image check

#### Automated

- [x] 2.1 Image builds with VERSION build-arg — 3482203
- [x] 2.2 Image serves /healthz ok against throwaway Postgres — 3482203
- [x] 2.3 Container logs are JSON with version=<short sha> — 3482203
- [x] 2.4 SIGTERM drains cleanly and exits 0 under 30 s — 3482203
- [x] 2.5 Build context contains no .env, context/, .git, .claude or test.csv paths — 3482203

#### Manual

- [x] 2.6 fly.toml values reviewed against infrastructure.md — 3482203
- [x] 2.7 Runbook draft reviewed — 3482203

### Phase 3: CI/CD workflow

#### Automated

- [x] 3.1 actionlint passes on ci.yml — cf52dbf
- [x] 3.2 Backend and web job commands pass locally — cf52dbf

#### Manual

- [x] 3.3 PR shows green backend/web checks and skipped deploy — cf52dbf

### Phase 4: Provisioning and first manual production deploy

#### Automated

- [x] 4.1 fly config validate passes
- [x] 4.2 fly status shows 1 started Machine in fra with passing check
- [x] 4.3 Production /healthz returns ok
- [x] 4.4 HTTP redirects to HTTPS
- [x] 4.5 Production logs show version and no DSN or seed-email strings

#### Manual

- [x] 4.6 Admin login works and session cookie is Secure/HttpOnly/SameSite=Lax
- [x] 4.7 Catalog upload gets categorized (OpenRouter key valid)
- [x] 4.8 Architect SBOM upload shows redundancy report
- [x] 4.9 App and MPG both in fra

### Phase 5: Merge to main and operational verification

#### Automated

- [ ] 5.1 PR merged: release commit is on main
- [ ] 5.2 CI run on main succeeds on all three jobs
- [ ] 5.3 CI-deployed version equals merge commit and /healthz ok

#### Manual

- [ ] 5.4 Rollback drill done and recorded in runbook
- [ ] 5.5 Backup restore drill done, temp cluster destroyed, recorded in runbook
- [ ] 5.6 Final production smoke test passes
