# Observability logging contract and queue supervision Implementation Plan

## Overview

Make every server-side failure after boot reach stdout (collected by Fly.io) as a structured JSON log line that a responder can act on, and stop the background categorization queue from losing work or crashing the process. This implements fixes 1 and 2 of the observability audit `context/audits/observability/2026-10-26_catalog-seeding-redundancy-report-login-strategic-marking.md`.

## Current State Analysis

- Only `backend/cmd/shelfmark/main.go` imports `log`; nothing under `internal/` can log. Boot failures `log.Fatalf` (`main.go:21-38`), then a single `listening on` line (`main.go:63`).
- `http.ListenAndServe(addr, mux)` (`main.go:64`): default server, no timeouts, no middleware, no access log, no request id, no graceful shutdown. Handler panics are recovered by net/http per connection and printed to stderr without method/route/user.
- `writeCatalogError(w, status, msg)` (`catalog/handlers.go:51-55`) is called 40 times across `handlers.go`/`project_handlers.go`, 21 of them with 500; it never receives the `err`. `auth/handlers.go` has `writeUserError(w, status, msg)` (`:167-171`) plus 5 inline 500 writes that also drop `err` and skip `Content-Type` (`:51, :66, :132, :142, :157`).
- `RequireRole` (`auth/middleware.go:28-32`) turns every `VerifySession` error into a bare 401 — including `ErrNoSessionSecret` (deploy misconfiguration) and bad HMAC signatures — with no trace. `VerifySession` (`auth/session.go:84-125`) returns `ErrInvalidSession` for missing, malformed and bad-signature cookies alike, `ErrSessionExpired` for expiry.
- `Session` carries only `Email` and `Role` (`auth/session.go:23-26`).
- `db.HealthCheck` returns 503 on ping failure and discards the error (`db/health.go:19-21`).
- `catalog.Queue` (`queue.go:39-98`): 5 fire-and-forget goroutines with no `recover`; in-memory channel (buffer 1000); `process` returns silently when the `processing` status write fails (`:76-78`) and discards the `failed` status-write error (`:97`); only the last attempt's error is kept, in `libraries.error_detail`. No shutdown hook, no reconciliation of rows left `pending`/`processing` by a restart.
- `libraries` has `status` and `updated_at` (`db/migrations/0002_catalog_schema.up.sql:28-36`, index on `status`), so stuck rows can be found without a schema change. `UpdateLibraryStatus` bumps `updated_at` (`store.go:264-272`).

## Desired End State

- The server writes JSON log lines (`log/slog`) to stdout. Every `/api/*` request produces one access-log line with `request_id`, `method`, `route`, `status`, `duration_ms`, `role`, `user` (hashed). Non-API requests (static SPA, `/healthz`) are logged only when they end ≥ 500.
- Every 5xx response logs its underlying error (`error` attribute, full wrapped chain) together with the request context; client-facing messages and status codes are unchanged.
- A panic in a handler returns a JSON 500 and logs the panic value plus stack with request context.
- `RequireRole` logs `ErrNoSessionSecret` (error) and bad-signature cookies (warn); missing/expired sessions stay silent. `/healthz` logs ping failures.
- The queue never takes the process down: a panic while processing a library is recovered, logged with library id/purl/batch, and the library ends `failed` with `error_detail` describing the panic. Every failed attempt is logged (warn) and every exhausted library and every status-write failure is logged (error).
- On boot, libraries in `pending` or `processing` with `updated_at` older than 10 minutes are re-enqueued, and the count is logged.
- On SIGTERM/SIGINT the HTTP server stops accepting requests and drains, the queue stops taking new jobs, in-flight attempts get up to 25 s total to finish, and anything left stays in the DB for the next boot's sweep. The process exits 0 after a clean shutdown.
- Verify: `go build ./... && go vet ./... && go test ./...` from `backend/`; run the binary locally and observe JSON lines (see Manual Verification per phase).

### Key Discoveries:

- `http.ServeMux.Handler(r)` returns the matched registered pattern — usable by middleware wrapped *outside* the mux to label the route without relying on the mux mutating `r.Pattern` (`net/http` docs, Go 1.27).
- The session is attached by `RequireRole` *inside* the mux (`auth/middleware.go:46-47`), after any outer middleware has already built its context — so user identity must flow back to the access logger through a mutable holder, not a context value read after the fact.
- `ErrInvalidSession` is returned from 6 branches of `VerifySession` (`session.go:92-120`); distinguishing a bad signature needs a new sentinel that still satisfies `errors.Is(err, ErrInvalidSession)` so existing callers and tests keep working.
- Test infrastructure to reuse: `startTestPostgres` (tmpfs, `store_test.go`), `pollUntilStatus` and fake finders (`queue_test.go`, `pipeline_test.go`), `httptest` handler tests (`handlers_test.go`).

## What We're NOT Doing

- No external error tracker (Sentry/OTel) — stdout + Fly log collection only; tracker choice belongs to `/10x-infra-research`.
- No change to any client-facing error message, status code, JSON shape or frontend file.
- No fix for `SESSION_SECRET` fail-fast at boot, hash validation at seed, or 401→500 for missing secret (audit fix 5: P4, LU3 status change, LU5, LU6) — we only *log* them here.
- No upstream reclassification (deps.dev non-404, unparseable LLM output, zero valid categories — audit fix 4) and no redundancy-report changes (audit fix 3).
- No periodic/background sweep — the stuck-row sweep runs once at boot. No multi-machine coordination (advisory locks); the app is deployed as a single Fly machine, and the 10-minute age threshold keeps a second machine from stealing fresh work if that ever changes.
- No change to `CreateLibraries`/`ListLibraries` in upload handlers (audit CS4): rows orphaned by that path are now logged and picked up by the boot sweep.
- No metrics endpoint, tracing, or log sampling.

## Implementation Approach

Add one small package, `backend/internal/obs`, that owns the logger setup and HTTP middleware (request id, access log, panic recovery) plus a per-request `RequestInfo` holder. `catalog` and `auth` depend on `obs`; `obs` depends on neither. Then thread `r` and `err` through the two error-writer helpers so each 5xx logs its cause with the request context. Finally, give `Queue` a lifecycle (`Shutdown`, boot `Sweep`) and per-job supervision, and wire everything in `main.go` with an `http.Server` and signal handling.

Logs are emitted with `slog.Default()` (set once in `main`) so tests can swap in a buffer-backed JSON handler to assert on emitted lines.

## Critical Implementation Details

- **Timing & lifecycle** — Shutdown order matters: stop the HTTP server first (`srv.Shutdown`, so no handler can `Enqueue` after the queue stops), then `queue.Shutdown(ctx)`, then close the pool. `Queue.Shutdown` must not `close(q.ch)` — that would make workers drain the entire buffer; workers instead stop taking jobs when the stop signal fires, and the retry loop must not start a new attempt after it fires (the library stays `processing`, aged out by the next boot sweep). Run the boot sweep after `NewQueue` and before `ListenAndServe`, in a goroutine, so a large backlog cannot block startup or the Fly health check (enqueue blocks when the channel is full).
- **Debug & observability** — Never log passwords, raw emails or cookie values. The `user` attribute is the first 12 hex chars of HMAC-SHA256(email, SESSION_SECRET), computed in `auth`; when the secret is missing, omit `user` rather than hashing with an empty key.

## Phase 1: Logging foundation

### Overview

Create `internal/obs` (JSON logger, request id, access log, panic recovery, `RequestInfo`), make `RequireRole` record role + hashed user into it, and switch `main.go` to a configured `http.Server` wrapped in the middleware.

### Changes Required:

#### 1. Observability package

**File**: `backend/internal/obs/obs.go` (new), `backend/internal/obs/middleware.go` (new)

**Intent**: One place that knows how to log a request-scoped event, so every package logs the same fields the same way.

**Contract**:
- `func NewLogger(w io.Writer) *slog.Logger` — JSON handler, level Info, adds `service=shelfmark` and `version` (from a package var `Version`, default `"dev"`, settable via `-ldflags -X`).
- `type RequestInfo struct` with fields `ID, Method, Route string` and `Role, User string` (mutex-guarded setters), `func (ri *RequestInfo) SetUser(role, userHash string)`, `func FromContext(ctx) *RequestInfo` (returns nil-safe value when absent), and `func (ri *RequestInfo) Attrs() []any` returning the non-empty fields as slog key/values.
- `func LogError(r *http.Request, msg string, err error, attrs ...any)` — logs at Error with request attrs + `error` (= `err.Error()` of the full chain) + extra attrs. A `LogWarn` twin with the same shape.
- `func Middleware(mux *http.ServeMux) http.Handler` — single wrapper that: generates a request id (16 random hex chars; reuses the `Fly-Request-Id` header only when it matches `^[A-Za-z0-9-]{1,64}$`, otherwise generates one), sets `X-Request-Id` on the response, resolves the route via `mux.Handler(r)` (pattern, empty when unmatched), stores a `*RequestInfo` in the request context, records the status via a `ResponseWriter` wrapper that implements `Unwrap() http.ResponseWriter`, recovers panics (re-panics `http.ErrAbortHandler`; otherwise logs `panic` + `stack` via `runtime/debug.Stack()` at Error and writes a JSON `{"error":"internal server error"}` 500 if headers are not yet written), and finally writes the access-log line `msg="request"` with `status`, `duration_ms` and request attrs — for every path starting with `/api/`, and for any other path only when status ≥ 500.

#### 2. Auth fills user identity

**File**: `backend/internal/auth/middleware.go`, `backend/internal/auth/session.go`

**Intent**: Let the access log and every error log say who made the request without logging their email.

**Contract**: After a session is verified in `RequireRole`, call `obs.FromContext(r.Context()).SetUser(string(session.Role), UserHash(session.Email))`. New exported `func UserHash(email string) string` in `session.go`: first 12 hex chars of HMAC-SHA256(lowercased email, SESSION_SECRET); returns `""` when the secret is unset.

#### 3. Server wiring

**File**: `backend/cmd/shelfmark/main.go`

**Intent**: Install the logger and middleware, and replace the bare `ListenAndServe` with a server that has timeouts.

**Contract**: `slog.SetDefault(obs.NewLogger(os.Stdout))` first thing in `main`; existing `log.Fatalf` boot failures become `slog.Error(...)` + `os.Exit(1)` with the same messages; the `listening on` line becomes `slog.Info("listening", "addr", …, "static_dir", …, "version", obs.Version)`. Serve with `&http.Server{Addr, Handler: obs.Middleware(mux), ReadHeaderTimeout: 10s, ReadTimeout: 60s, WriteTimeout: 60s, IdleTimeout: 120s}` (Read/Write timeouts sized for 10MB uploads). Graceful shutdown is Phase 3.

#### 4. Release identity in the build

**File**: `Dockerfile`

**Intent**: Make `version` in every log line meaningful in production.

**Contract**: Add `ARG VERSION=dev` to the backend stage and `-X github.com/tswierszcz/shelfmark/backend/internal/obs.Version=${VERSION}` to the existing `-ldflags`. Fly deploy passes `--build-arg VERSION=$(git rev-parse --short HEAD)` (documented in the deploy change, not here).

#### 5. Tests

**File**: `backend/internal/obs/middleware_test.go` (new), `backend/internal/auth/middleware_test.go` (extend or new)

**Intent**: Prove the access-log, recovery and identity contracts with a captured logger.

**Contract**: Table-driven tests with a `slog` JSON handler over a `bytes.Buffer` set as default for the test: `/api/x` 200 → one `request` line with method/route/status/request_id; `/assets/x.js` 200 → no line; `/assets/x.js` 500 → one line; a panicking handler → response 500 JSON, one Error line with `panic` and `stack`, plus the access line with status 500; `Fly-Request-Id` header is reused. Auth: a request through `RequireRole` with a valid session yields an access line with `role` and a 12-char `user` that does not contain the email.

### Success Criteria:

#### Automated Verification:

- `go build ./...` and `go vet ./...` pass
- `go test ./internal/obs/...` passes
- `go test ./internal/auth/...` passes
- Full suite passes: `go test ./...`

#### Manual Verification:

- Running the server locally and calling `/api/me` (logged in) and `/` prints one JSON `request` line for `/api/me` with `route`, `status`, `role`, hashed `user` and `request_id`, and no line for `/`; the response carries `X-Request-Id`.

**Implementation Note**: After completing this phase and all automated verification passes, pause here for manual confirmation from the human that the manual testing was successful before proceeding to the next phase.

---

## Phase 2: Handlers log their causes

### Overview

Thread the request and the error into every error-writing helper so each 5xx logs its cause with request context, and make `RequireRole` and `/healthz` log the failures they currently swallow.

### Changes Required:

#### 1. Catalog error writer

**File**: `backend/internal/catalog/handlers.go`, `backend/internal/catalog/project_handlers.go`

**Intent**: Every catalog 500 records why it happened (audit P1, CS9, RR8, SM1, SM2) while clients keep receiving the same message.

**Contract**: `writeCatalogError(w http.ResponseWriter, r *http.Request, status int, msg string, err error, attrs ...any)`. When `status >= 500` it calls `obs.LogError(r, msg, err, attrs...)` before writing the unchanged JSON body; 4xx calls pass `err` when they have one but are not logged. Update all 40 call sites; every 500 call site passes its `err` and, where in scope, the relevant ids as attrs (`batch_id`, `library_id`, `category_id`, `project_id`, and for `ListOrganizationLibrariesHandler` a `query` attr naming which of its three store calls failed). Also log (warn) the ignored `json.NewEncoder(w).Encode` errors at the end of list handlers.

#### 2. Auth error writers

**File**: `backend/internal/auth/handlers.go`

**Intent**: Login and user-management outages leave a trace (audit LU2) and all 500s send JSON consistently.

**Contract**: `writeUserError(w, r, status, msg, err)` with the same ≥500 logging rule; replace the 5 inline 500 writes (`:51, :66, :132, :142, :157`) with it, keeping their messages. `writeInvalidCredentials` unchanged (no logging of failed logins in this change). Never pass the request body, email or password as attrs.

#### 3. Session verification failures

**File**: `backend/internal/auth/session.go`, `backend/internal/auth/middleware.go`

**Intent**: A missing signing key or forged/rotated-key cookies become visible instead of looking like ordinary logouts (partial LU3).

**Contract**: New sentinel `ErrInvalidSignature` returned from the HMAC-mismatch branch of `VerifySession`, wrapped so `errors.Is(err, ErrInvalidSession)` stays true (e.g. `fmt.Errorf("%w: %w", ErrInvalidSession, ErrInvalidSignature)`). In `RequireRole`: `ErrNoSessionSecret` → `obs.LogError(r, "session secret not configured", err)`; `ErrInvalidSignature` → `obs.LogWarn(r, "session signature invalid", err)`; all other errors → no log. Response stays a bare 401.

#### 4. Health check

**File**: `backend/internal/db/health.go`

**Intent**: A DB outage seen by Fly's health check is explained in the logs (audit P5).

**Contract**: On ping failure, `slog.Error("healthz: db ping failed", "error", err)` before writing the unchanged 503.

#### 5. Tests

**File**: `backend/internal/catalog/handlers_test.go`, `backend/internal/auth/handlers_test.go`, `backend/internal/auth/middleware_test.go`, `backend/internal/db/health_test.go` (new or extend)

**Intent**: Prove that a 5xx logs its cause and a 4xx does not, without asserting on every call site.

**Contract**: Uses the shared `captureLogs(t)` helper (see Testing Strategy). Named tests: `TestCatalogHandler_5xx_LogsCause`, `TestCatalogHandler_4xx_DoesNotLog`, `TestListUsers_5xx_LogsCause`, `TestRequireRole_LogsSessionFailures`, `TestHealthCheck_DBDown_Logs`. Catalog: a handler whose store call fails (e.g. closed pool from `startTestPostgres` then `pool.Close()`) returns the same 500 body as today and emits one Error line containing the handler's message, an `error` attribute with the pgx cause, and the request attrs; a 400 path (missing `kind`) emits no Error line. Auth: `ListUsersHandler` with a closed pool → 500 JSON with `Content-Type: application/json` and one Error line; `RequireRole` with `SESSION_SECRET` unset → 401 and one Error line; with a tampered signature → 401 and one Warn line; with no cookie → 401 and no line. Health: closed pool → 503 and one Error line.

### Success Criteria:

#### Automated Verification:

- `go build ./...` and `go vet ./...` pass
- `go test ./internal/catalog/... -run 'TestCatalogHandler_5xx_LogsCause|TestCatalogHandler_4xx_DoesNotLog' -v` passes and reports both tests as run
- `go test ./internal/auth/...` passes
- `go test ./internal/db/...` passes
- Full suite passes: `go test ./...`
- No remaining bare 500 writes: `grep -rn "WriteHeader(http.StatusInternalServerError)" backend/internal --include='*.go' | grep -v _test` returns nothing

#### Manual Verification:

- With the local server running, stopping the Postgres container and calling `/api/projects` as an Architect returns the same 500 message as before and prints one JSON Error line with `route`, `request_id`, `role`, `user` and the pgx error; `/healthz` returns 503 and logs the ping error.

**Implementation Note**: After completing this phase and all automated verification passes, pause here for manual confirmation from the human that the manual testing was successful before proceeding to the next phase.

---

## Phase 3: Queue supervision & lifecycle

### Overview

Make the queue survive panics, log every failure it currently swallows, recover stuck rows at boot, and shut down cleanly on SIGTERM together with the HTTP server.

### Changes Required:

#### 1. Per-job supervision and logging

**File**: `backend/internal/catalog/queue.go`

**Intent**: One bad upstream response can no longer crash the server, and no failure in the worker disappears (audit CS1, CS3).

**Contract**: `process` gets a deferred `recover`: on panic, log Error `queue: panic processing library` with `library_id`, `purl`, `batch_id`, `panic`, `stack`, then set status `failed` with `error_detail = "panic: <value>"` (using a fresh `context.Background()` with a short timeout). The `processing` status-write failure is logged (Error, with ids) instead of silently returning. Each failed attempt logs Warn `queue: attempt failed` with `attempt`, `max_attempts`, ids and `error`. Exhausting retries logs Error `queue: library failed` with ids and the last error. A failure to write the `failed` status is logged (Error) instead of `_ =`.

#### 2. Shutdown

**File**: `backend/internal/catalog/queue.go`

**Intent**: Deploys stop the queue in bounded time without starting work they cannot finish (audit P3).

**Contract**: `func (q *Queue) Shutdown(ctx context.Context) error` — signals workers to stop taking jobs (a stop channel/context, *not* closing `q.ch`), makes the retry loop skip further attempts once stopped, and waits for in-flight `process` calls via a `sync.WaitGroup` until `ctx` is done. Each attempt's context is derived from a queue-owned cancellable context (not `context.Background()`), still capped by `processAttemptTimeout`; when `ctx` expires, `Shutdown` cancels that context so in-flight attempts return promptly (the library stays `processing` for the next boot sweep and the attempt logs a Warn with `context canceled`), waits up to 2 s more for workers to exit, then returns `ctx.Err()`. This keeps `pool.Close()` from blocking on a connection held by a 30 s attempt. Logs Info `queue: shutdown` with `in_flight_abandoned` / `pending_in_channel` counts. `Enqueue` after shutdown drops the job and logs Warn (the row stays `pending` for the sweep).

#### 3. Boot sweep

**File**: `backend/internal/catalog/store.go`, `backend/internal/catalog/queue.go`

**Intent**: Libraries orphaned by a restart, deploy, panic or failed status write are picked up again instead of staying `pending`/`processing` forever (audit CS2, CS4, RR5).

**Contract**: `func (s *Store) ListStuckLibraries(ctx context.Context, olderThan time.Duration) ([]Library, error)` — libraries with `status IN ('pending','processing') AND updated_at < now() - olderThan`, ordered by `updated_at`, returning a non-nil slice (lessons: nil slices). `func (q *Queue) Sweep(ctx context.Context, olderThan time.Duration) (int, error)` — lists stuck libraries and enqueues each (stopping early if the queue is shut down); logs Info `queue: boot sweep` with `requeued` count (also when 0) and Error on list failure. Const `stuckLibraryAge = 10 * time.Minute` in `queue.go`.

#### 4. Lifecycle wiring

**File**: `backend/cmd/shelfmark/main.go`

**Intent**: SIGTERM from Fly drains the HTTP server and the queue in the right order; the sweep runs once at boot.

**Contract**: `ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)`; start `go catalogQueue.Sweep(ctx, stuckLibraryAge)` (exported constant or passed value) after `NewQueue`; run `srv.ListenAndServe()` in a goroutine; on `ctx.Done()` log Info `shutting down`, then with a 25 s budget: `srv.Shutdown`, then `catalogQueue.Shutdown`, then `pool.Close()`; log Info `shutdown complete` and exit 0. `http.ErrServerClosed` is not an error. A non-`ErrServerClosed` serve error logs Error and exits 1.

#### 5. Tests

**File**: `backend/internal/catalog/queue_test.go`, `backend/internal/catalog/store_test.go`

**Intent**: Prove supervision, sweep and shutdown against the real queue and Postgres.

**Contract**:
- `TestQueue_PanicInPipeline_MarksFailedAndKeepsServing` — a finder that panics for one library and succeeds for another: the panicking one ends `failed` with `error_detail` starting `panic:`, the other still ends `categorized` (process survives), and one Error log line carries `library_id` and `stack`.
- `TestQueue_AttemptFailures_AreLogged` — `countingErrFinder`: 3 Warn `attempt failed` lines and 1 Error `library failed` line with the library id.
- `TestStore_ListStuckLibraries_RespectsAgeAndStatus` — rows in `pending`/`processing`/`categorized` with `updated_at` set via SQL to 20 min and 1 min ago: only the old `pending`/`processing` rows are returned; empty result is `[]`, not nil.
- `TestQueue_Sweep_RequeuesStuckLibraries` — an old `processing` row is swept and reaches `categorized`; a fresh `pending` row is not touched by the sweep.
- `TestQueue_Shutdown_WaitsForInFlightThenStops` — a finder that blocks until its ctx is cancelled or it is released: (a) released before the deadline → `Shutdown` returns nil; (b) never released → `Shutdown` with a short ctx returns `context.DeadlineExceeded` within deadline + 2 s, the finder observed cancellation, and the library is still `processing`; in both scenarios a library enqueued after shutdown stays `pending`.

### Success Criteria:

#### Automated Verification:

- `go build ./...` and `go vet ./...` pass
- `go test ./internal/catalog/... -run 'TestQueue_|TestStore_ListStuckLibraries'` passes
- Full suite passes: `go test ./...`
- No discarded queue errors remain: `grep -n "_ = q.store" backend/internal/catalog/queue.go` returns nothing

#### Manual Verification:

- Local server: upload a CSV, send SIGTERM while libraries are processing → logs show `shutting down`, `queue: shutdown`, `shutdown complete`, exit code 0; restart with a DB whose stuck rows are older than 10 min (or temporarily lower the threshold) → `queue: boot sweep` with `requeued > 0` and those libraries settle.

**Implementation Note**: After completing this phase and all automated verification passes, pause here for manual confirmation from the human that the manual testing was successful.

---

## Testing Strategy

### Log capture in tests:

- One helper per package that needs it, `captureLogs(t *testing.T) *logSink`: installs `slog.SetDefault` with a JSON handler over a mutex-guarded writer, restores the previous default in `t.Cleanup`, and exposes `Lines()` / `Find(msg string, attrs ...any)` that parse JSON lines. Assertions always filter by the test's own `request_id` / `library_id`, never by total line count, because workers started by earlier queue tests keep running and may log.
- Every new queue test calls `queue.Shutdown` in `t.Cleanup` so it does not leak workers into later tests. Run the queue tests with `-race` at least once per phase.

### Unit Tests:

- `obs.Middleware`: access-log scope (`/api/*` vs static vs ≥500), request-id reuse, panic recovery response and log.
- `auth.UserHash`: stable, 12 hex chars, never contains the email, empty without secret.
- `RequireRole` log levels per error class.

### Integration Tests:

- Catalog and auth handlers against a closed pool: unchanged 500 body + one Error log with cause and request attrs.
- Queue against real Postgres: panic recovery, attempt logging, sweep by age, shutdown wait/timeout.

### Manual Testing Steps:

1. Run locally with JSON logs, exercise `/api/me` and `/` — one access line, route + hashed user, no static noise.
2. Stop Postgres, hit an API route and `/healthz` — Error lines with the pgx cause.
3. Upload a batch, SIGTERM mid-processing, restart after the threshold — shutdown lines, then boot sweep requeues and settles the leftovers.

## Performance Considerations

One log line per API request plus error lines; JSON encoding cost is negligible at this app's traffic. Static assets and health checks are excluded from the access log to keep Fly log volume low.

## Migration Notes

No schema migration. Deploy note for the later `prod-deploy` change: set Fly `kill_timeout` ≥ 30 s so the 25 s shutdown budget fits, run a single machine, and pass `--build-arg VERSION=<git sha>`.

## References

- Audit: `context/audits/observability/2026-10-26_catalog-seeding-redundancy-report-login-strategic-marking.md` (fixes 1 and 2)
- Existing error writers: `backend/internal/catalog/handlers.go:51`, `backend/internal/auth/handlers.go:167`
- Queue: `backend/internal/catalog/queue.go:39-98`
- Session: `backend/internal/auth/session.go:23-125`, `backend/internal/auth/middleware.go:25-49`
- Test patterns: `backend/internal/catalog/store_test.go` (`startTestPostgres`), `backend/internal/catalog/queue_test.go` (`pollUntilStatus`, `countingErrFinder`)
- Lessons: `context/foundation/lessons.md` (nil slices → `[]`)

## Progress

> Convention: `- [ ]` pending, `- [x]` done. Append ` — <commit sha>` when a step lands. Do not rename step titles. See `references/progress-format.md`.

### Phase 1: Logging foundation

#### Automated

- [x] 1.1 go build and go vet pass
- [x] 1.2 internal/obs tests pass
- [x] 1.3 internal/auth tests pass
- [x] 1.4 Full suite passes

#### Manual

- [x] 1.5 Local /api/me logs one JSON request line with route, status, role, hashed user and request_id; / logs nothing; X-Request-Id returned

### Phase 2: Handlers log their causes

#### Automated

- [ ] 2.1 go build and go vet pass
- [ ] 2.2 Catalog logging tests pass
- [ ] 2.3 internal/auth tests pass
- [ ] 2.4 internal/db tests pass
- [ ] 2.5 Full suite passes
- [ ] 2.6 No bare 500 writes remain in internal/

#### Manual

- [ ] 2.7 With Postgres stopped, /api/projects returns the same 500 and logs one Error line with route, request_id, role, user and pgx cause; /healthz 503 logs the ping error

### Phase 3: Queue supervision & lifecycle

#### Automated

- [ ] 3.1 go build and go vet pass
- [ ] 3.2 Queue and ListStuckLibraries tests pass
- [ ] 3.3 Full suite passes
- [ ] 3.4 No discarded queue store errors remain

#### Manual

- [ ] 3.5 SIGTERM mid-batch logs shutting down, queue shutdown and shutdown complete with exit 0; restart after threshold logs boot sweep with requeued > 0 and leftovers settle
