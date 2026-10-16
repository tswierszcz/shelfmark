<!-- IMPL-REVIEW-REPORT -->
# Implementation Review: Data layer connectivity & migration tooling

- **Plan**: context/changes/data-layer-connectivity/plan.md
- **Scope**: Full plan
- **Reviewed phases**: 1, 2, 3
- **Date**: 2026-10-16
- **Verdict**: APPROVED
- **Findings**: 0 critical, 1 warning, 4 observations

## Verdicts

| Dimension | Verdict |
|-----------|---------|
| Plan Adherence | PASS |
| Scope Discipline | PASS |
| Safety & Quality | WARNING |
| Architecture | PASS |
| Pattern Consistency | WARNING |
| Success Criteria | PASS |

## Findings

### F1 — Unchecked `http.ResponseWriter.Write` return values in health check

- **Severity**: ⚠️ WARNING
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Safety & Quality
- **Location**: backend/internal/db/health.go:23, 27
- **Detail**: Both `w.Write([]byte(...))` calls ignore their error return. Every DB call elsewhere in this change (pool.go, migrate.go) is checked and wrapped — this is the one place that silently drops an error, which is a minor discipline inconsistency rather than a real risk (fixed-size write to an HTTP ResponseWriter).
- **Fix**: Assign to `_` explicitly or log on error, matching the checked-error discipline used elsewhere in this change.
- **Decision**: FIXED

### F2 — Test env-var helper duplicates `t.Setenv`

- **Severity**: ⚠️ WARNING
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Pattern Consistency
- **Location**: backend/internal/db/pool_test.go, backend/internal/db/migrate_test.go
- **Detail**: Both files roll a custom `withEnv(t, key, value)` helper for setting `DATABASE_URL`, while the sibling suite `backend/internal/auth/accounts_test.go` already uses Go's built-in `t.Setenv` (set + auto-restore in one call). `t.Setenv` can't unset a var, which likely motivated the custom helper for `TestConnect_MissingDatabaseURL` — but that case could use `t.Setenv("DATABASE_URL", "")` instead, since `Connect` already treats an empty string the same as unset.
- **Fix**: Consolidate on `t.Setenv` where sufficient; drop the bespoke `withEnv` helper to match the established test pattern in this repo.
- **Decision**: FIXED

### F3 — Pool not closed if `Migrate` fails after `Connect` succeeds

- **Severity**: 👁️ OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Safety & Quality
- **Location**: backend/cmd/shelfmark/main.go:23-29
- **Detail**: If `db.Connect` succeeds but the subsequent `db.Migrate` call fails, `pool` is never explicitly closed before `log.Fatalf` exits the process. Harmless in practice — process exit reclaims the OS socket, and `Connect` itself already closes the pool correctly on its own failure path (pool.go:25-27) — but worth noting as a theoretical gap, not a real leak.
- **Decision**: FIXED

### F4 — No retry/backoff on `Connect`

- **Severity**: 👁️ OBSERVATION
- **Impact**: 🔎 MEDIUM — real tradeoff; pause to reason through it
- **Dimension**: Architecture
- **Location**: backend/internal/db/pool.go
- **Detail**: `Connect` does a single `pgxpool.New` + one `Ping`, no retry/backoff. If the app starts before Postgres finishes initializing (e.g. an app service added later to `docker-compose.yml`, or a CI job), startup would fail hard rather than retrying briefly. Not an issue today — `docker-compose.yml` only defines the `db` service, no app service races it — but worth flagging for whoever adds app-in-compose or a CI integration step later.
- **Decision**: SKIPPED

### F5 — Trivial local-dev credentials in `docker-compose.yml` / `.env.example`

- **Severity**: 👁️ OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Safety & Quality
- **Location**: docker-compose.yml, .env.example
- **Detail**: Credentials are trivial and symmetric (`shelfmark`/`shelfmark`), mirrored exactly in `.env.example`'s `DATABASE_URL`. Nothing here is a real secret, and the file naming signals dev-only clearly — but this is exactly the kind of file that gets copy-pasted into a real `.env` without rotation. Confirmed no `.env` (non-example) file is committed and no prod compose overlay exists that could inherit these values silently.
- **Decision**: SKIPPED

## Not flagged (checked, no issue)

- SQL injection: migrations contain only static `CREATE/DROP EXTENSION IF EXISTS pgcrypto` — no user input; no app queries in this diff.
- CORS/authz: `/healthz` is intentionally unauthenticated (standard for health probes); `/api/me` correctly uses `auth.RequireRole`.
- Migration reversibility: up/down pair is symmetric, idempotent, no data-loss risk.
- Pool leak check: both `Connect` failure paths handled correctly — `pgxpool.New` failure returns before a pool exists; `Ping` failure explicitly closes the pool before returning.
