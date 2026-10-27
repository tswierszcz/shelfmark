<!-- IMPL-REVIEW-REPORT -->
# Implementation Review: Observability logging contract and queue supervision

- **Plan**: context/changes/observability-logging-queue-supervision/plan.md
- **Scope**: Full plan
- **Reviewed phases**: 1, 2, 3
- **Date**: 2026-10-26
- **Verdict**: NEEDS ATTENTION
- **Findings**: 0 critical, 4 warnings, 5 observations

## Verdicts

| Dimension | Verdict |
|-----------|---------|
| Plan Adherence | PASS |
| Scope Discipline | PASS |
| Safety & Quality | WARNING |
| Architecture | PASS |
| Pattern Consistency | WARNING |
| Success Criteria | PASS |

Success criteria re-run 2026-10-26 at e7dc4e2: `go build ./... && go vet ./...` OK; `go test -race` obs/auth/db OK; 11 named catalog tests PASS (handler logging, queue, ListStuckLibraries); `go test ./...` OK; `_ = q.store` grep empty; 500 grep matches only `internal/obs/middleware.go:45` (panic recovery — see F9). Manual 1.5, 2.7, 3.5 verified on throwaway local instances (evidence in session: JSON access line with role/hashed user; pgx cause logged on 500 with Postgres stopped; SIGTERM → shutdown complete exit 0, boot sweep requeued 55).

## Findings

### F1 — Blocked Enqueue during shutdown can consume the whole budget

- **Severity**: ⚠️ WARNING
- **Impact**: 🔎 MEDIUM — real tradeoff; pause to reason through it
- **Dimension**: Safety & Quality
- **Location**: backend/cmd/shelfmark/main.go:107-114; backend/internal/catalog/handlers.go:138-140; project_handlers.go:103
- **Detail**: Upload handlers enqueue in a loop; `Enqueue` blocks on a full 1000-slot channel and is only released by the queue's stop signal, which closes after `srv.Shutdown` returns. A large upload in flight at SIGTERM makes `srv.Shutdown` wait for the handler until the 25 s budget expires, then `queue.Shutdown` gets an expired ctx and cancels in-flight attempts without any drain.
- **Fix A ⭐ Recommended**: Run `srv.Shutdown` and `catalogQueue.Shutdown` concurrently with the shared budget, then `pool.Close()` after both return.
  - Strength: The queue's stop releases blocked `Enqueue` calls (they drop safely; rows stay pending for the sweep), so the HTTP drain can finish; both phases share the full budget.
  - Tradeoff: A handler that enqueues after stop drops jobs instead of queueing them — already the designed behaviour.
  - Confidence: HIGH — `Enqueue` already selects on `q.stop`.
  - Blind spot: None significant.
- **Fix B**: Split the budget (e.g. 15 s HTTP, 10 s queue) and keep the sequential order.
  - Strength: Keeps strict ordering.
  - Tradeoff: A blocked upload still burns its 15 s and the queue drain gets less time.
  - Confidence: MED.
  - Blind spot: Real upload sizes in production unknown.
- **Decision**: FIXED (Fix A) — main.go runs srv.Shutdown and catalogQueue.Shutdown concurrently under the shared 25 s budget; pool.Close after both.

### F2 — Panic after headers are written ends as a clean, truncated 200

- **Severity**: ⚠️ WARNING
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Safety & Quality
- **Location**: backend/internal/obs/middleware.go:43-46
- **Detail**: When a handler panics after writing headers, the middleware records status 500 and returns normally, so the client receives a truncated body that ends cleanly under 200 while the log says 500. Without the middleware net/http would abort the connection.
- **Fix**: In that branch, after logging, `panic(http.ErrAbortHandler)` so the connection is aborted.
- **Decision**: FIXED — middleware aborts the connection (panic(http.ErrAbortHandler)) after logging when headers were already sent; regression test TestMiddleware_PanicAfterHeadersAbortsConnection (chunked response; break-check red→restored).

### F3 — Boot errors can log DATABASE_URL password and a seed email

- **Severity**: ⚠️ WARNING
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Safety & Quality
- **Location**: backend/internal/db/migrate.go:31; backend/internal/auth/users_store.go:136 (logged by main.go fatal)
- **Detail**: `url.Parse` errors embed the full URL including the password and are wrapped with `%w`; the seed error includes the account email via `%q`. Pre-existing, but now shipped as structured JSON to Fly logs.
- **Fix**: Return `db: failed to parse DATABASE_URL` without wrapping the parse error; drop the email from the seed error (keep role).
- **Decision**: SKIPPED

### F4 — shutdownBudget comment contradicts Fly's default kill timeout

- **Severity**: ⚠️ WARNING
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Safety & Quality
- **Location**: backend/cmd/shelfmark/main.go:119-121
- **Detail**: The comment says 25 s stays inside Fly's default kill timeout; Fly's default is 5 s. Without `kill_timeout = 30` in fly.toml the drain is SIGKILLed after ~5 s (the plan already lists `kill_timeout ≥ 30 s` as a prod-deploy prerequisite).
- **Fix**: Correct the comment to state the `kill_timeout ≥ 30 s` requirement; keep it as a prod-deploy checklist item.
- **Decision**: SKIPPED

### F5 — Boot sweep can double-process pending rows if two machines overlap

- **Severity**: ℹ️ OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Safety & Quality
- **Location**: backend/internal/catalog/queue.go:161-178
- **Detail**: `pending` rows sitting >10 min in another instance's channel would be re-enqueued by a new machine during a rolling/bluegreen deploy. Matches the plan's explicit single-machine decision.
- **Fix**: None now; keep "single machine + rolling strategy that stops the old machine first" in the prod-deploy change.
- **Decision**: ACCEPTED — single Fly machine with a non-overlapping deploy strategy is a prod-deploy requirement.

### F6 — logSink/captureLogs duplicated in four test packages

- **Severity**: ℹ️ OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Pattern Consistency
- **Location**: obs/middleware_test.go:16, auth/middleware_test.go:19, catalog/handlers_test.go:727, db/health_test.go:21
- **Detail**: ~60 lines copied four times, with slightly different `Find` semantics.
- **Fix**: Move to `internal/obs/obstest` and import it from the four test files.
- **Decision**: FIXED — new internal/obs/obstest (Sink, Capture, CaptureWith); the four test packages use it via CaptureWith(t, obs.NewLogger).

### F7 — "Stays pending" assertions are a single sample after a sleep

- **Severity**: ℹ️ OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Pattern Consistency
- **Location**: backend/internal/catalog/queue_test.go:334, 392, 432
- **Detail**: Fixed sleep then `pollUntilStatus(…"pending"…)`, which succeeds on the first read; weak under load.
- **Fix**: Assert the status directly after the sleep and additionally that no `attempt failed`/status log exists for that library id.
- **Decision**: FIXED — assertStaysStatus samples every 25 ms over a window; enqueue-after-shutdown cases also assert the dropped-job warning for that library_id.

### F8 — Minor logging-style inconsistencies

- **Severity**: ℹ️ OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Pattern Consistency
- **Location**: backend/internal/db/health.go:23; backend/internal/obs/middleware.go:41
- **Detail**: healthz logs `"error", err` without request attrs (plan specified `slog.Error`, so not a drift); panic recovery logs the raw panic value while the queue uses `fmt.Sprint`.
- **Fix**: Use `fmt.Sprint(rec)` in the middleware panic log.
- **Decision**: FIXED — panic value logged via fmt.Sprint(rec).

### F9 — Plan defects found during implementation

- **Severity**: ℹ️ OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Plan Adherence
- **Location**: plan.md criterion 2.6; main.go:91; queue.go:161-179
- **Detail**: Criterion 2.6's grep cannot pass literally because Phase 1's mandated panic recovery writes a 500; the `listening` line relies on the logger's `version` attr; `Sweep` can count a job dropped during a shutdown race in `requeued`.
- **Fix**: Record these in change.md `## Notes`.
- **Decision**: SKIPPED
