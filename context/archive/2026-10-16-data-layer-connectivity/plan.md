# Data layer connectivity & migration tooling Implementation Plan

## Overview

Give the Go backend a PostgreSQL connection pool and an automatic migration runner, so every future slice (starting with S-01) can persist its own tables instead of improvising in-memory state. No slice-specific schema is created here — this is pure connectivity scaffolding plus one baseline migration that proves the pipe works.

## Current State Analysis

- `backend/go.mod` has one dependency (`golang.org/x/crypto`, from F-01's auth work). No database driver, no migration tooling.
- `backend/cmd/shelfmark/main.go` loads auth accounts and registers routes directly in `main()`; there is no structured startup/shutdown sequence yet.
- `backend/internal/auth/` is the only `internal/` package — it establishes the project's convention of one package per domain concern, each with its own small, focused files and a matching `_test.go`.
- `GET /healthz` currently just writes `"ok"` unconditionally — it has no dependency on any backend state.
- No `.env.example`, no `docker-compose.yml`, no `fly.toml` exist yet. The `Dockerfile` builds the Go binary and copies the built SPA, expecting `PORT` and `STATIC_DIR` env vars (already wired).
- No Postgres server is installed in this environment; Docker is available and already used for the production image.

### Key Discoveries:

- `golang-migrate/migrate/v4` ships separate driver packages for the database (`database/pgx/v5`) and the migration source (`source/iofs`, which reads an `embed.FS`) — both are added as explicit imports for their side-effecting `init()` registration, not used directly by name elsewhere in the code. This is a minor but non-obvious wiring detail worth calling out so the implementer doesn't drop the "unused" imports.
- `pgxpool.New` parses a `DATABASE_URL`-style connection string directly — no manual DSN assembly needed, consistent with the single-env-var decision.

## Desired End State

The backend connects to Postgres via `DATABASE_URL` at startup, applies any pending migrations (currently just one, enabling `pgcrypto`), and exposes that connectivity through `GET /healthz`, which now returns 503 if the database is unreachable. A solo developer can run `docker compose up -d` to get a local Postgres matching production, copy `.env.example` to `.env.local` (or export the vars), and run the backend with migrations applying automatically.

Verification: `go build ./...` and `go test ./...` pass in `backend/` (the Postgres-dependent test is skipped if Docker is unavailable, otherwise it runs against a Docker-started Postgres); manually confirm `docker compose up -d`, run the backend, and see `/healthz` return 200, then stop the DB and see `/healthz` return 503.

## What We're NOT Doing

- No application-specific schema (no `libraries`, `projects`, `users` tables, etc.) — that belongs to S-01 and later slices, which will add their own migration files to the same directory.
- No separate `cmd/migrate` binary or manual migration step — migrations run automatically at app startup, per the chosen tradeoff (single binary, single deploy step, consistent with the solo-dev speed goal).
- No connection retry/backoff loop beyond what `pgxpool` already does internally — if Postgres is unreachable at startup, the app fails fast with a clear error, matching the existing `LoadAccounts` fail-fast pattern from F-01.
- No ORM — raw SQL via `pgx`, consistent with the project's minimal-dependency bias (one pragmatic driver dependency, not a framework).

## Implementation Approach

Backend-first, same ordering logic as F-01: build the connectivity + migration primitive, then wire it into the app's observable behavior (`/healthz`), then give the developer a way to run it locally.

## Phase 1: Backend DB core

### Overview

Add a PostgreSQL connection pool, a `golang-migrate`-based migration runner reading embedded `.sql` files, and one baseline migration.

### Changes Required:

#### 1. Dependency additions

**File**: `backend/go.mod`, `backend/go.sum`

**Intent**: Add `pgx` (connection pool) and `golang-migrate` (migration runner + its Postgres/pgx and iofs driver packages).

**Contract**: `go get github.com/jackc/pgx/v5/pgxpool github.com/golang-migrate/migrate/v4 github.com/golang-migrate/migrate/v4/database/pgx/v5 github.com/golang-migrate/migrate/v4/source/iofs` run from `backend/`, pinning latest compatible versions.

#### 2. Connection pool

**File**: `backend/internal/db/pool.go`

**Intent**: Open a `pgx` connection pool from `DATABASE_URL`, failing fast with a clear error if the env var is missing or the connection string is invalid — matching the `LoadAccounts` fail-fast convention from `internal/auth/accounts.go`.

**Contract**: `Connect(ctx context.Context) (*pgxpool.Pool, error)` reads `DATABASE_URL` from the environment (error if empty), calls `pgxpool.New(ctx, url)`, and does one `Ping(ctx)` before returning, so a bad connection string or unreachable server is caught immediately rather than on the first real query.

#### 3. Migration runner

**File**: `backend/internal/db/migrate.go`

**Intent**: Apply any pending migrations from an embedded directory of `.sql` files against the pool's underlying connection, using `golang-migrate`.

**Contract**: `Migrate(pool *pgxpool.Pool) error` wraps the pool's connection config in `golang-migrate`'s `pgx/v5` database driver, wraps the embedded migrations directory in its `iofs` source driver, and calls `migrate.Up()`. Treat `migrate.ErrNoChange` as success (no pending migrations is not an error). The embedded migrations live in `backend/internal/db/migrations/*.sql`, embedded via `//go:embed migrations` into a package-level `embed.FS`.

#### 4. Baseline migration

**File**: `backend/internal/db/migrations/0001_enable_pgcrypto.up.sql`, `backend/internal/db/migrations/0001_enable_pgcrypto.down.sql`

**Intent**: Prove the migration pipe works end-to-end with a real, useful change — enabling the `pgcrypto` extension, which future slices (e.g. S-01's catalog entries) will likely want for `gen_random_uuid()`-style primary keys.

**Contract**: Up: `CREATE EXTENSION IF NOT EXISTS pgcrypto;`. Down: `DROP EXTENSION IF EXISTS pgcrypto;`.

### Success Criteria:

#### Automated Verification:

- `go build ./...` succeeds in `backend/`
- `go vet ./...` succeeds in `backend/`
- Unit tests pass: `go test ./backend/internal/db/...` — covering: `Connect` returns a clear error when `DATABASE_URL` is unset; `Connect` returns an error for a malformed connection string.
- Integration test (skipped automatically if Docker is unavailable, via `testing.Short()`-style capability check — see Phase 1 of `references/task-orchestration.md` conventions is not applicable here; implementer checks `docker info` or attempts a container start and calls `t.Skip` on failure): start a throwaway Postgres container, run `Connect` + `Migrate` against it, assert no error and that the `pgcrypto` extension is present (`SELECT 1 FROM pg_extension WHERE extname = 'pgcrypto'`).

#### Manual Verification:

- With a local Postgres reachable via `DATABASE_URL` (ahead of Phase 3's docker-compose convenience, a manually-run `docker run postgres` is fine for this phase's own verification), running a small throwaway `go run` snippet (or the integration test) against it applies the migration and the `pgcrypto` extension appears in `pg_extension`.

---

## Phase 2: Health check + wiring

### Overview

Wire the connection pool into `main.go`'s startup sequence (connect, migrate, keep the pool for the life of the process) and extend `/healthz` to report real DB connectivity.

### Changes Required:

#### 1. DB-aware health check

**File**: `backend/internal/db/health.go`

**Intent**: Give `/healthz` a way to verify the database is actually reachable, not just that the process is running.

**Contract**: `HealthCheck(pool *pgxpool.Pool) http.HandlerFunc` — pings the pool with a short-timeout context (e.g. 2s); on success writes `200 "ok"` (same body as today, to avoid changing the happy-path contract); on failure writes `503` with a short error body.

#### 2. Wire into main

**File**: `backend/cmd/shelfmark/main.go`

**Intent**: Connect to the database and run migrations before serving traffic; replace the existing no-op `/healthz` handler with the DB-aware one.

**Contract**: After loading auth accounts and before building the mux: `pool, err := db.Connect(ctx)` (fail fast on error, matching the `LoadAccounts` pattern), then `db.Migrate(pool)` (fail fast on error). Replace `mux.HandleFunc("GET /healthz", ...)`'s inline handler with `mux.HandleFunc("GET /healthz", db.HealthCheck(pool))`. The pool is not explicitly closed on shutdown — the process has no graceful-shutdown handling today (F-01 didn't add one either), so this stays consistent with the existing lifecycle rather than introducing one now.

### Success Criteria:

#### Automated Verification:

- `go build ./...` succeeds in `backend/`
- `go vet ./...` succeeds in `backend/`

#### Manual Verification:

- With a reachable Postgres and all required env vars set, starting the backend succeeds and `GET /healthz` returns 200.
- Stopping/blocking the Postgres connection and calling `GET /healthz` again returns 503.
- Starting the backend with an unreachable `DATABASE_URL` fails fast at startup with a clear log message (not a hang or a generic panic).

---

## Phase 3: Local dev setup

### Overview

Give a solo developer a one-command way to run a local Postgres matching the driver/version used elsewhere, plus documented env vars.

### Changes Required:

#### 1. Local Postgres via Docker Compose

**File**: `docker-compose.yml` (repo root, alongside `Dockerfile`)

**Intent**: One `docker compose up -d` starts a local Postgres for development.

**Contract**: A single `db` service using `postgres:16-alpine`, exposing `5432:5432`, with `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB` set to simple local-only defaults (e.g. `shelfmark`/`shelfmark`/`shelfmark`), and a named volume for data persistence across restarts.

#### 2. Documented env vars

**File**: `.env.example` (repo root)

**Intent**: Give the developer a copyable template for the env vars this and the previous change (F-01) require.

**Contract**: Add `DATABASE_URL=postgres://shelfmark:shelfmark@localhost:5432/shelfmark?sslmode=disable` to the file (creating it if it doesn't exist; if F-01 already created one with auth vars, append this line rather than replacing the file).

#### 3. README note

**File**: `README.md` (repo root, if one exists; otherwise skip this change — do not create a README solely for this note)

**Intent**: Point a future reader at the two-command local setup (`docker compose up -d`, copy `.env.example`).

**Contract**: A short "Local development" section naming the two commands. If `README.md` doesn't exist in the repo root, skip this file entirely rather than creating one — this phase documents, it doesn't establish project documentation structure.

### Success Criteria:

#### Automated Verification:

- `docker compose config` succeeds (validates `docker-compose.yml` syntax) if Docker Compose is available in this environment; otherwise this check is a PREFLIGHT skip, reported at phase start, not a silent pass.

#### Manual Verification:

- `docker compose up -d` starts a Postgres container; `docker compose down` stops and the named volume persists data across an `up`/`down`/`up` cycle (insert-independent check: the container comes back healthy without re-running `CREATE EXTENSION` manually — the migration runner's `ErrNoChange` path on the second startup confirms this).

---

## Testing Strategy

### Unit Tests:

- `Connect`: missing `DATABASE_URL` → clear error; malformed connection string → error (no panic).

### Integration Tests:

- One Docker-gated integration test (Phase 1) exercising `Connect` + `Migrate` against a real, throwaway Postgres container, asserting the `pgcrypto` extension is present afterward. Skips cleanly (not a failure) when Docker isn't available.

### Manual Testing Steps:

1. `docker compose up -d` (Phase 3) to start local Postgres.
2. Copy `.env.example` to `.env.local` (or export its vars), run `go run ./backend/cmd/shelfmark`.
3. Confirm startup logs show no errors and `curl localhost:8080/healthz` returns 200.
4. `docker compose stop db`, re-run the health check, confirm 503.
5. `docker compose start db`, confirm 200 again without restarting the Go process (proves the pool reconnects, not just that startup worked once).

## Performance Considerations

None beyond `pgxpool`'s own defaults — a single solo-dev-scale instance has no connection-pool tuning needs yet.

## Migration Notes

Not applicable — no existing data to migrate; this establishes the migration mechanism itself.

## References

- Roadmap item: `context/foundation/roadmap.md` § F-02
- PRD: `context/foundation/prd.md` § Functional Requirements (FR-001, FR-002 — the consumers this unlocks)
- Tech stack hand-off: `context/foundation/tech-stack.md` (PostgreSQL as preferred database)
- Existing fail-fast config pattern: `backend/internal/auth/accounts.go` (`LoadAccounts`)
- Current backend entrypoint: `backend/cmd/shelfmark/main.go`
- Existing Docker build: `Dockerfile`

## Progress

> Convention: `- [ ]` pending, `- [x]` done. Append ` — <commit sha>` when a step lands. Do not rename step titles. See `references/progress-format.md`.

### Phase 1: Backend DB core

#### Automated

- [x] 1.1 go build ./... succeeds in backend/ — e653c43
- [x] 1.2 go vet ./... succeeds in backend/ — e653c43
- [x] 1.3 Unit tests pass: go test ./backend/internal/db/... — e653c43
- [x] 1.4 Docker-gated integration test applies the baseline migration and confirms pgcrypto is present (or skips cleanly without Docker) — e653c43

#### Manual

- [x] 1.5 Running Connect + Migrate against a manually-started local Postgres applies the migration and pgcrypto appears in pg_extension — e653c43

### Phase 2: Health check + wiring

#### Automated

- [x] 2.1 go build ./... succeeds in backend/ — 98711fe
- [x] 2.2 go vet ./... succeeds in backend/ — 98711fe

#### Manual

- [x] 2.3 GET /healthz returns 200 with a reachable Postgres and correct env vars — 98711fe
- [x] 2.4 GET /healthz returns 503 when Postgres is unreachable — 98711fe
- [x] 2.5 Starting the backend with an unreachable DATABASE_URL fails fast with a clear error, not a hang — 98711fe

### Phase 3: Local dev setup

#### Automated

- [x] 3.1 docker compose config succeeds, or is reported as a PREFLIGHT skip if Docker Compose is unavailable — 7a807c5

#### Manual

- [x] 3.2 docker compose up -d starts a working local Postgres — 7a807c5
- [x] 3.3 Data persists across a docker compose down / up cycle (named volume works) — 7a807c5
