# Data layer connectivity & migration tooling — Plan Brief

> Full plan: `context/changes/data-layer-connectivity/plan.md`

## What & Why

Shelfmark needs a real data layer before any slice can persist anything. This is roadmap foundation F-02: the app can connect to Postgres and run migrations, so S-01 (seeding the library catalog) and everything after it can create and use their own tables instead of improvising in-memory state.

## Starting Point

Today the backend (`backend/cmd/shelfmark/main.go`) has no database dependency at all — `go.mod` only has `golang.org/x/crypto` from F-01's auth work. `/healthz` is a no-op liveness check. There's no local Postgres setup, no `.env.example`, no `docker-compose.yml`.

## Desired End State

The backend connects to Postgres via a `DATABASE_URL` env var at startup, automatically applies pending migrations (currently one, enabling `pgcrypto`), and `/healthz` now actually reflects whether the database is reachable (200 vs 503). A developer runs `docker compose up -d` for a local Postgres and the backend picks it up via `.env.example`'s template.

## Key Decisions Made

| Decision | Choice | Why (1 sentence) |
| --- | --- | --- |
| Postgres driver | `pgx` (jackc/pgx) | Modern, pure-Go, actively maintained — the de facto standard over the maintenance-mode `lib/pq`. |
| Migration tool | `golang-migrate/migrate` | Industry-standard migration library with `embed.FS` support, so migrations ship inside the single binary. |
| Connection config | Single `DATABASE_URL` env var | Matches what Fly's managed Postgres and most hosting platforms inject directly — no manual DSN assembly. |
| Migration execution | Automatically at app startup | Single binary, single deploy step — fits the solo-dev speed goal; a bad migration fails startup loudly rather than needing a separate CI step. |
| Local dev Postgres | `docker-compose.yml` with a Postgres service | Docker is already used for the production image and available in this environment — one command, no manual install. |
| Health check scope | `/healthz` pings the DB | Lets Fly's own health checks catch a broken DB connection, which matters once every future slice depends on persistence. |
| DB testing strategy | Docker-gated integration test | Proves the real connect+migrate path works, not just that the code compiles; skips cleanly where Docker isn't available. |

## Scope

**In scope:**
- Backend: `pgx` connection pool, `golang-migrate`-based runner reading embedded `.sql` files, one baseline migration (`pgcrypto`), DB-aware `/healthz`.
- Local dev: `docker-compose.yml`, `.env.example`, a short README note (only if `README.md` already exists).

**Out of scope:**
- Any application-specific schema (libraries, projects, users tables) — that's S-01's job, using this same migration directory.
- A separate `cmd/migrate` binary or manual migration step.
- Connection retry/backoff beyond `pgxpool`'s own defaults; an ORM.

## Architecture / Approach

The Go binary owns its own database lifecycle: on startup it opens a `pgx` connection pool from `DATABASE_URL`, runs any pending migrations from an embedded directory via `golang-migrate`, and keeps the pool alive for the process's lifetime. `/healthz` pings that same pool on each request rather than caching a stale status, so it always reflects current reachability.

## Phases at a Glance

| Phase | What it delivers | Key risk |
| --- | --- | --- |
| 1. Backend DB core | Connection pool, migration runner, baseline `pgcrypto` migration | Getting the `golang-migrate` driver wiring right (pgx + iofs adapters) — a one-time integration cost |
| 2. Health check + wiring | `/healthz` reflects real DB state; pool wired into `main.go` startup | None significant once Phase 1's primitives exist |
| 3. Local dev setup | `docker-compose.yml`, `.env.example`, README note | None — purely additive tooling |

**Prerequisites:** None (F-02 has no roadmap prerequisites; runs in parallel with F-01, already done).
**Estimated effort:** ~1 session across 3 phases — small, well-bounded scaffold, same size class as F-01.

## Open Risks & Assumptions

- Assumes Fly's managed Postgres (or whatever production Postgres is eventually chosen) injects a standard `DATABASE_URL`-shaped connection string — if not, Phase 2's wiring needs a small adjustment, not a redesign.
- Assumes migrations running automatically at startup remains acceptable as the team grows past solo — flagged as an explicit tradeoff in the plan, revisit if it ever becomes risky.

## Success Criteria (Summary)

- The backend connects to Postgres and applies the baseline migration automatically at startup, failing fast and loudly if it can't.
- `/healthz` accurately reflects database reachability (200 when up, 503 when down).
- A developer can get a local Postgres running with one `docker compose up -d` command.
