# Observability logging contract and queue supervision — Plan Brief

> Full plan: `context/changes/observability-logging-queue-supervision/plan.md`
> Research: `context/audits/observability/2026-10-26_catalog-seeding-redundancy-report-login-strategic-marking.md` (observability audit, fixes 1 and 2)

## What & Why

After boot, Shelfmark writes almost nothing to stdout: every 500 drops its cause, there is no access log, and the background categorization queue can crash the whole process or leave libraries stuck forever without a trace. Fly.io collects stdout, so this change makes that stream useful before the first production deploy.

## Starting Point

Only `main.go` logs (boot failures + "listening on"). `writeCatalogError`/`writeUserError` take a message but not the error; `RequireRole` turns every session failure into a silent 401; the `Queue` runs 5 unsupervised goroutines over an in-memory channel with no shutdown and no recovery of stuck rows.

## Desired End State

Every `/api/*` request produces one JSON access-log line with request id, route, status, latency, role and a hashed user id; every 5xx logs its underlying error with that context; panics return JSON 500 and log a stack. The queue survives panics, logs every failed attempt and status-write failure, re-enqueues stuck rows at boot, and shuts down cleanly on SIGTERM.

## Key Decisions Made

| Decision | Choice | Why (1 sentence) | Source |
| --- | --- | --- | --- |
| What to fix | Audit fixes 1 (logging contract + server layer) and 2 (queue supervision) | Highest blindness removed per unit of effort across all four audited flows | Research |
| Logger | stdlib `log/slog` JSON to stdout, new `internal/obs` package | No new dependency; Fly collects stdout; tests can capture the default logger | Plan |
| User identity in logs | Role + first 12 hex of HMAC-SHA256(email, SESSION_SECRET) | Correlates events per user without PII and without changing the session cookie | Plan |
| Access-log scope | Every `/api/*`; static files and `/healthz` only when ≥ 500 | Full API picture without Fly health-check and asset noise | Plan |
| Stuck-row recovery | Boot-only sweep of `pending`/`processing` older than 10 min; single Fly machine | No schema change; age threshold avoids stealing fresh work if a second machine ever runs | Plan |
| Shutdown | Stop HTTP, stop taking jobs, finish in-flight attempts within 25 s, then cancel what is still running; leftovers wait for the sweep | Always-bounded shutdown that fits Fly `kill_timeout` ≥ 30 s; nothing is lost | Plan (+ plan-review F1) |
| Client contract | Messages, status codes and JSON shapes unchanged | Logging change only; frontend fixes are a separate audit item | Research |

## Scope

**In scope:** `internal/obs` (logger, request id, access log, panic recovery, `RequestInfo`); `RequireRole` fills role + hashed user; `http.Server` with timeouts; version via ldflags; error writers in `catalog` and `auth` log 5xx causes with ids; `RequireRole` logs missing secret / bad signature; `/healthz` logs ping failure; queue recover, attempt and status-write logging, `Shutdown`, boot `Sweep`, `ListStuckLibraries`; signal-driven graceful shutdown in `main`.

**Out of scope:** external tracker; frontend; `SESSION_SECRET` fail-fast and hash validation (audit fix 5); upstream reclassification (fix 4); redundancy report (fix 3); periodic sweep; multi-machine locking; failed-login events; metrics/tracing.

## Architecture / Approach

`obs.Middleware(mux)` wraps the mux: it creates a request id and a mutable `RequestInfo` in the context, resolves the route with `mux.Handler(r)`, recovers panics and writes the access line. `RequireRole` (inside the mux) fills role and user into that holder. Handlers call `writeCatalogError`/`writeUserError(w, r, status, msg, err, attrs...)`, which log via `obs.LogError` for ≥ 500. The `Queue` gains a stop signal + WaitGroup (`Shutdown`) and `Sweep`; `main` orders shutdown as HTTP server → queue → pool.

## Phases at a Glance

| Phase | What it delivers | Key risk |
| --- | --- | --- |
| 1. Logging foundation | JSON logger, request id, access log, panic recovery, hashed user, server timeouts, version | Route/user labelling from outside the mux — solved with `mux.Handler` + mutable holder |
| 2. Handlers log their causes | Every 5xx in catalog/auth logs its error with context; session and healthz failures logged | 45 call-site edits — risk of a missed one, guarded by a grep criterion |
| 3. Queue supervision & lifecycle | Panic-safe worker, attempt/status logging, boot sweep, graceful shutdown | Shutdown ordering and not draining the whole buffer |

**Prerequisites:** Docker for integration tests; Go 1.27.3 toolchain (pinned).
**Estimated effort:** ~3 sessions, one per phase.

## Open Risks & Assumptions

- Assumes a single Fly machine and `kill_timeout` ≥ 30 s — to be set in the upcoming `prod-deploy` change.
- A library interrupted by a deploy waits up to 10 min before the next boot's sweep picks it up (only at boot).
- Fly log retention/alerting is outside the repo; this change only guarantees the lines exist.

## Success Criteria (Summary)

- A DB outage on any API route shows up in `fly logs` as one Error line with route, request id, user hash and the pgx cause — while users see the same messages as today.
- A panic in categorization no longer restarts the server; the library ends `failed` with a reason.
- After a deploy mid-batch, no library stays `pending`/`processing` forever.
