---
type: observability-audit
date: 2026-10-26 16:44
mode: audit
commit: cbe6625
branch: shape/shelfmark
dirty_tree: false
areas: [catalog-seeding, redundancy-report, login-users, strategic-marking]
area_source: user
runtime_proof: partial
error_tracker: none
previous_report: null
findings: { critical: 5, high: 14, medium: 13, low: 7 }
---

# Observability audit — catalog seeding, redundancy report, login & users, strategic marking (2026-10-26)

## 1. TL;DR

- **Nothing below `main` can log.** The stdlib `log` package is imported only in `backend/cmd/shelfmark/main.go`; every handler error writer takes a message but not the `err` (`catalog/handlers.go:51`, `auth/handlers.go:167`). All 26 explicit 500 paths reach stdout as **nothing**.
- **No server layer.** Bare `http.ListenAndServe(addr, mux)` (`main.go:64`): no access log, no request id, no recovery with context, no graceful shutdown. Fly.io's log stream will contain boot lines and panics only.
- **The background queue is a black hole.** Worker goroutines have no `recover` (one bad upstream response can crash the whole server), status-write failures are discarded (`queue.go:76-78`, `:97`), and nothing re-enqueues rows left `pending`/`processing` after a restart or deploy.
- **Upstream failures are reclassified as normal outcomes.** deps.dev 5xx/429 → "not found", unparseable LLM reply → "not confident", confident-but-invalid categories → `categorized` with zero categories. An outage looks like ordinary data.
- **The redundancy report shows only what succeeded.** Report SQL inner-joins categories (`project.go:202`), so failed/pending libraries vanish; the architect's upload page polls an admin-only endpoint, gets 403, and announces "done". **An architect can act on retire recommendations computed from a fraction of the project, with no signal anywhere.**
- **The frontend makes outages look like user error.** Any login failure shows "Invalid email or password"; a `/api/me` 500 redirects to `/login`.

## 2. Capture model

- **Runtime:** one long-running Go binary (`backend/cmd/shelfmark/main.go`) serving the JSON API and the static SPA (`web/build/client`, `react-router.config.ts:6` `ssr:false`). Background work: in-process goroutine pool `catalog.Queue` (`queue.go:39-66`), 5 workers, in-memory channel (buffer 1000), no persistence.
- **Error tracker:** none — no Sentry/OTel/Datadog/etc. in `backend/go.mod` or `web/package.json`.
- **Capture boundaries:** none in code. net/http's per-connection recover logs handler panics to stderr (`http: panic serving <addr>` + stack, no method/path/user). Worker goroutines are outside even that boundary (`queue.go:62-66`, no `recover` anywhere in the repo).
- **Logging:** stdlib `log`, only in `main.go` — `log.Fatalf` on boot failures (`:21-38`), `log.Printf("listening on …")` (`:63`). No `log`/`slog` import under `internal/`. No access log.
- **Persistent diagnostics:** `libraries.error_detail` (migration 0007, `queue.go:95-97`) — last error of an exhausted-retry library; admin-only via `GET /api/catalog/libraries`, not rendered by the UI (`web/app/lib/catalog.ts:6-15` type lacks it).
- **Health:** `GET /healthz` pings the DB (2s) and returns 503 on failure, discarding the ping error (`db/health.go:14-24`); does not reflect queue health.
- **Frontend:** single root `ErrorBoundary` (`web/app/root.tsx:53`) renders a generic message (stack only in DEV), reports nowhere; no `window.onerror` / `unhandledrejection` / `vite:preloadError`; zero `console.*` calls; fetch helpers check `res.ok` but throw plain `Error` without status.
- **Deploy identity:** none — no version via `-ldflags -X` (Dockerfile builds with `-s -w` only), nothing in boot log, header or `/healthz`.

**Assumptions (outside the repo, not findings):**
- Fly.io collects the process stdout/stderr (`fly logs`, log shipper) — **confirmed by user** as the target platform; Fly behaviour is standard.
- Fly health checks, log retention, alerting and edge-proxy 5xx metrics — **unconfirmed** (no `fly.toml` in repo yet).
- Postgres-side logging and external uptime monitoring — **unconfirmed**.

## 3. What reaches the logs

Static analysis plus one runtime observation (2026-10-26, manual check 3.6 of `testing-pipeline-input-hardening` on a throwaway local instance). No error tracker exists, so the "Tracker" column is omitted.

| Failure shape | Response / UI | stdout on Fly | Verdict | Evidence |
|---|---|---|---|---|
| OpenRouter 401 for every library (bad key) | upload 200; batch `{"failed": 9}`; UI shows counts | only `listening on :8091` | **missed** (observed) | runtime 3.6; `error_detail` held the cause |
| DB error in any handler | generic 500 text | nothing | **missed** | `handlers.go:51`, 26 paths |
| Panic in HTTP handler | connection dropped | `http: panic serving …` + stack, no route/user | poor | `main.go:64` default server |
| Panic in queue worker | whole process exits, Fly restarts | panic + stack, no library/batch id | **missed context; outage** | `queue.go:62-66` |
| Deploy/restart mid-batch | rows stuck `pending`/`processing` forever | nothing | **missed** | no shutdown, no startup sweep |
| deps.dev 5xx/429 | falls through to LLM finder | nothing | **missed** (degrades data) | `depsdev.go:68-80` |
| LLM returns garbage | library `needs_manual` | nothing | **missed** | `openrouter.go:216-219` |
| Architect upload, batch poll | 403 → UI "— done", partial report | nothing | **missed** | `main.go:51`, `redundancy.tsx:120-126` |
| `SESSION_SECRET` unset | boots, `/healthz` 200; every login 500 → "Invalid email or password" | nothing | **missed** | `auth/session.go:41-47` |
| `/api/me` 500 | redirect to `/login` | nothing | **missed** | `auth.ts:65-73`, `require-auth.tsx:25` |
| Frontend render crash / chunk-load error | generic "Oops!" | nothing (browser only) | **missed** | `root.tsx:53` |

## 4. Systemic root causes

1. **Error-writer contract drops the cause.** `writeCatalogError(w, status, msg)` (`catalog/handlers.go:51`) and `writeUserError` / inline 500s (`auth/handlers.go:51,66,132,142,157,167`) never receive `err`; combined with no logger below `main`, every handler failure ends at the HTTP status. Reasonable locally (don't leak internals to clients), but nothing records the internal side. Explains P1, CS10, RR9, LU2, SM1, SM2.
2. **No server layer around the mux.** `http.ListenAndServe(addr, mux)` (`main.go:64`) — no middleware chain, so no access log, request id, user/route context, panic recovery with context, timeouts or graceful shutdown. Explains P2, P3, LU9.
3. **Unsupervised in-memory queue.** Fire-and-forget goroutines (`queue.go:62-66`) without `recover`; status-write errors returned silently or discarded (`queue.go:76-78`, `:97`); state only in a channel (`queue.go:45`); no reconciliation of non-terminal rows on boot. Explains CS1-CS4, RR5.
4. **"Expected outcome" absorbs failures.** Upstream adapters map errors onto domain outcomes: `depsdev.go:68-80` (any non-200 or decode error → not found), `openrouter.go:216-219` (unparseable → not confident), `openrouter.go:103-105` (non-2xx body dropped), `pipeline.go:70-89` (confident with zero valid categories → `categorized`). Explains CS5-CS8, CS11.
5. **The report only shows successes.** `project.go:202` inner-joins `library_categories`; the UI never renders per-library `status`; the architect's progress poll hits an admin-only route and treats failure as "settled". Explains RR1-RR4, RR11.
6. **Frontend errors are terminal at the catch site.** Fetch helpers throw `Error` without status; `catch` blocks render a fixed string, an empty list or "done"; `AuthProvider` has no catch; no global handlers. Explains LU1, LU5, RR14, RR15, SM4-SM7, CS15-CS18, P7.

## 5. Findings by area

### Platform / plumbing

| # | Location | Category | Severity | What happens in production | Fix direction |
|---|---|---|---|---|---|
| P1 | `catalog/handlers.go:51`; `auth/handlers.go:51-167` (26 × 500) | swallowed | critical | Any DB/pgx/timeout failure → user sees a generic message; stdout empty; responder has no cause, route or id | Error writer takes `err`, logs it (slog JSON) with method, route pattern, request id, user id; client message unchanged |
| P2 | `main.go:43-64` | missing-context | high | No access log: 4xx/5xx rate, latency, route and user are unknowable from Fly logs | Middleware: request id, access log (method, pattern, status, ms, user id), panic recovery → 500 + log |
| P3 | `main.go:64` + `queue.go` | coverage-gap | high | SIGTERM on every `fly deploy` kills in-flight work silently (see CS2) | `http.Server` with timeouts, `signal.NotifyContext`, `srv.Shutdown`, drain queue |
| P4 | `auth/session.go:41-47`; `main.go:28-64` | config | high | `SESSION_SECRET` unset → app boots, `/healthz` 200, every login 500 shown as "Invalid email or password", every API call bare 401; zero log lines | Validate at boot and `log.Fatalf`, like the other required vars |
| P5 | `db/health.go:19-21` | logged-not-captured | low | DB down → `/healthz` 503 with no cause in logs | Log the ping error (rate-limited) |
| P6 | Dockerfile `-ldflags="-s -w"`; `main.go:63` | config | low | Errors can't be tied to a release/commit | `-X main.version=$GIT_SHA`; log at boot; expose in `/healthz` |
| P7 | `web/app/root.tsx:53`; no global handlers | coverage-gap | medium | Render crashes, unhandled rejections, chunk-load errors after deploy are seen only by the user | `window.onerror`/`unhandledrejection`/`vite:preloadError` → small `POST /api/client-errors` that logs server-side (or a tracker later) |

### Catalog seeding

| # | Location | Category | Severity | What happens in production | Fix direction |
|---|---|---|---|---|---|
| CS1 | `queue.go:62-66`, `:73-98` | coverage-gap | critical | A panic in pipeline/upstream parsing crashes the **whole server**; queue lost; stack has no library/batch id | `defer recover()` in `process`: log with lib id/batch/purl, mark `failed` |
| CS2 | `main.go:40-41`; `queue.go:25-28` | coverage-gap | critical | After any restart/deploy, `pending`/`processing` rows stay stuck forever; UI polls until timeout and says "done" | Boot sweep: re-enqueue `pending`, reset stale `processing`; log counts |
| CS3 | `queue.go:76-78`, `:96-97` | swallowed | critical | DB blip on the `processing`/`failed` write → library silently dropped or stuck in `processing`; `lastErr` lost | Log both errors with lib id; never `_ =` |
| CS4 | `handlers.go:118-128`; `store.go:111` | missing-throw | high | CreateLibraries ok, then ListLibraries fails → 500, nothing enqueued, rows `pending` forever; re-upload can't fix (`ON CONFLICT DO NOTHING` keeps old batch) | Use `RETURNING` ids; log with batch id |
| CS5 | `depsdev.go:68-80` | flattened-response | high | deps.dev 5xx/429/schema drift → "not found" → LLM may hallucinate a repo; outage invisible, data quality drops silently | Only 404 = not found; other statuses return error; log decode failures |
| CS6 | `openrouter.go:216-219` | flattened-response | high | Garbage/non-JSON reply (bad model, prompt regression) → every library `needs_manual`; raw reply discarded | Typed "unparseable" error or log truncated reply + model |
| CS7 | `pipeline.go:70-89` | missing-throw | high | LLM "confident" but all names outside taxonomy → `categorized` with **zero** categories (failure looks like success) | Count attachments; 0 → `needs_manual`, log rejected names |
| CS8 | `openrouter.go:103-105` | identity-lost | high | Non-2xx body (bad model, no credits, rate limit) dropped — why the known 401 case said only "status 401" | Include truncated body + model in the error |
| CS9 | `handlers.go:107-153, 186-188, 320-322` | swallowed | high | Every catalog 500 drops `err` (instance of root cause 1) | See P1 |
| CS10 | `github.go:180-182`; `queue.go:81-94` | noise | medium | GitHub 404 (moved/hallucinated repo) and 403/429 rate limits retried 3× then `failed`; 404 never `needs_manual`; earlier attempts' errors overwritten | Classify: 404 → `needs_manual`, 4xx permanent (no retry); log each attempt |
| CS11 | `store.go:111`; `handlers.go:123-137` | flattened-response | medium | Existing purls silently dropped from the batch (not in `created`, not in `skipped`); old `failed` rows never retried | Report duplicates; add re-process path for `failed` |
| CS12 | `handlers.go:129-131`; `queue.go:58-60` | coverage-gap | medium | Enqueue blocks when 1000-slot buffer is full, ignoring request ctx → large upload hangs/times out at proxy while work continues invisibly | Non-blocking enqueue / ctx-aware; log queue depth |
| CS13 | `handlers.go:147-148`; `catalog.tsx:21-27, 98-113, 204` | flattened-response | medium | Unknown batch → 200 `{}`; poll timeout and poll errors render "— done" | 404 for unknown batch; UI "timed out" / "polling failed" states |
| CS14 | `lib/catalog.ts:6-15`; `catalog.tsx:293-298` | coverage-gap | medium | `error_detail` exists but Admin UI never shows it | Add field; render for `failed` rows |
| CS15 | `catalog.tsx:140-153`; `handlers.go:220-223` | missing-throw | medium | Failed manual attach → unhandled rejection, no UI message; bad category id → FK → generic 500 | Catch + show; map FK to 400 |
| CS16 | `handlers.go:70-76` | swallowed | low | Non-size multipart parse errors → misleading "kind must be…" 400 | 400 with parse error (impl-review F6, skipped then) |
| CS17 | `catalog.tsx:62-71`; `lib/catalog.ts:68-86` | swallowed | low | Taxonomy load failure → empty dropdowns, no message; server `{error}` dropped | Error state; `errorMessage()` everywhere |

### Redundancy report

| # | Location | Category | Severity | What happens in production | Fix direction |
|---|---|---|---|---|---|
| RR1 | `main.go:51` vs `redundancy.tsx:120-126` | flattened-response | critical | Architect upload polls admin-only `/api/catalog/batches/{id}` → 403 → catch sets "done" and loads the report while new libraries are still `pending`; nothing logged or shown | Project-scoped progress endpoint for Architect; poll error ≠ settled |
| RR2 | `project.go:202` (inner `JOIN library_categories`) | coverage-gap | critical | Pending/processing/failed/needs_manual libraries vanish from the report; total failure = "No libraries recorded", partial = smaller "complete-looking" report | `LEFT JOIN`; return every library with status; per-status counts |
| RR3 | `redundancy.tsx:263-299` | coverage-gap | high | API sends `status`, UI never shows it; can't tell "uncategorized/failed" from "no category" | Status badge + "N of M uncategorized (k failed)" banner |
| RR4 | `project.go:224` | missing-context | high | Retire candidates computed from attached categories only — on partial data, without saying so | `complete:false`; suppress retire flags where libraries unsettled |
| RR5 | `project_handlers.go:100-104` | swallowed | high | Only `pending` re-enqueued; `failed`/stuck `processing` libraries never retried on re-upload, stay out of report | Covered by CS2 sweep + retry path |
| RR6 | `project_handlers.go:80-119` | missing-throw | high | 4 writes without a transaction; failure after enqueue → 500 with libraries created/queued, project empty or with old membership | One tx for project + membership; enqueue after commit |
| RR7 | `project_handlers.go:67-71, 112-116`; `project_sbom.go:43-44` | missing-throw | high | SBOM with 0 usable components → 200 and **wipes** the project's memberships | 400 on 0 parsed libraries |
| RR8 | `project_handlers.go:75-227` (8 × 500) | swallowed | high | Every report/upload 500 drops `err` (root cause 1) | See P1 |
| RR9 | `redundancy.tsx:111-116, 209` | flattened-response | medium | 60s poll timeout → "— done" on a partial report | "still processing (X pending)" + refresh |
| RR10 | `redundancy.tsx:22-28`; `project_handlers.go:80` | flattened-response | medium | All purls already known → empty batch → fake 60s "processing…"; reused pending/failed libraries not tracked | Track progress per project |
| RR11 | `redundancy.tsx:141-148`; `project_handlers.go:127-128` | coverage-gap | medium | Upload `skipped` rows never shown; `created` counts reused libraries | Show skipped + reasons; separate new/existing counts |
| RR12 | `redundancy.tsx:59-62` | flattened-response | medium | `listProjects` 401/403/500 → "Failed to load projects." + empty picker; status lost | Show message/status; empty state only on success |
| RR13 | `redundancy.tsx:70-77` | identity-lost | medium | Report fetch error leaves the **previous project's** rows under the error banner | Clear report on error |
| RR14 | `project_handlers.go:165-168` | noise | low | Non-UUID projectID → Postgres cast error → 500 instead of 404 | Validate UUID → 404/400 |

### Login & users

| # | Location | Category | Severity | What happens in production | Fix direction |
|---|---|---|---|---|---|
| LU1 | `web/app/lib/auth.ts:23-24`; `login.tsx:24-25, 68-69` | flattened-response | high | 401, 500 (DB down, secret missing), 413 and network errors all show "Invalid email or password."; users reset passwords during an outage | `ApiError(status)`; credentials message only on 401, "service unavailable" otherwise |
| LU2 | `auth/handlers.go:50-53, 65-68, 130-133, 142-144, 156-159` | swallowed | high | Every auth 500 drops the wrapped pgx error; login/users outage leaves no trace | See P1 (user id / hashed email only, never email/password) |
| LU3 | `auth/middleware.go:28-32` | flattened-response | high | `RequireRole` maps every `VerifySession` error to bare 401 — missing secret, forged/rotated-key signature, malformed cookie, expiry all identical | 500 + log on `ErrNoSessionSecret`; log invalid signatures separately from expiry |
| LU4 | `web/app/lib/auth.ts:65-78`; `require-auth.tsx:25-27` | missing-throw | high | `/api/me` 500/network error → unhandled rejection, `me=null`, redirect to `/login` → outage looks like logout (then LU1) | `error` state in AuthProvider; outage screen instead of redirect |
| LU5 | `auth/handlers.go:60-62` | flattened-response | medium | Any bcrypt error (malformed seeded hash) → 401; account can never log in, nothing points to the bad hash | Only `ErrMismatchedHashAndPassword` → 401; other errors logged as server fault |
| LU6 | `auth/users_store.go:112-145`; `accounts.go:28-54` | config | medium | Seed runs only on empty table and never validates hash format; later env "password rotation" silently ignored | Validate hashes at seed; log "users table non-empty, env seed skipped" |
| LU7 | `auth/handlers.go:55-63` | missing-context | medium | Failed logins leave no server event; credential stuffing indistinguishable from normal traffic | Info log `login_failed` with reason, hashed email, client IP; response unchanged |
| LU8 | `auth/handlers.go:44-46` | flattened-response | low | Malformed/oversized login body → 401 "invalid credentials" | 400/413 |
| LU9 | `web/app/routes/users.tsx:23-24` | swallowed | low | Users list failure → "No users yet." alongside generic error; status lost | Show error, hide empty state |
| LU10 | `auth/handlers.go:115-128` | missing-context | low | No audit event for "admin X created user Y with role Z" | Log actor id, new user id, role |

### Strategic marking

| # | Location | Category | Severity | What happens in production | Fix direction |
|---|---|---|---|---|---|
| SM1 | `catalog/handlers.go:363, 399` | swallowed | high | Mark/unmark store failure (tx begin/commit, deadlock, DB down) → "failed to mark strategic", nothing logged | See P1, with library/category/actor ids |
| SM2 | `catalog/handlers.go:246, 256, 261` | swallowed | high | Any of 3 queries behind the Organization page fails → whole page 500, no trace of which | See P1, name the failing query |
| SM3 | `store.go:340, 347-352, 388-394` | flattened-response | high | Malformed libraryID/categoryID → Postgres 22P02 → **500**; client bug looks like outage | Validate UUIDs → 400, or map `InvalidTextRepresentation` |
| SM4 | `organization.tsx:53-55, 243` | swallowed | high | Taxonomy load fails → `setCategories([])` → every Mark/Unmark button silently disappears | Error state; disabled controls with reason |
| SM5 | `organization.tsx:46-47` | flattened-response | medium | 401/400/500 all "Failed to load libraries."; stale rows stay visible | Show status/message; clear or flag stale rows |
| SM6 | `organization.tsx:83-86` + `:34-48` | missing-throw | medium | Mark succeeds, refresh fails silently → UI diverges from DB; retry overwrites justification via upsert | Propagate refresh failure |
| SM7 | `lib/organization.ts:22-29, 66-86`; `auth/middleware.go:30,42` | identity-lost | medium | Error has no status; 401/403 bodies empty → session expiry looks like a server failure | Typed error with status; JSON body on 401/403 |
| SM8 | `handlers.go:359-362`; `store.go:358-360` | logged-not-captured | medium | Cap rejections (409) correctly distinct, but neither 409 nor 5xx counted/logged — can't tell "users hit cap" from outage | Info log 409 with category/ecosystem/count |
| SM9 | `store.go:338` | swallowed | low | Rollback with request ctx; client disconnect → `context.Canceled` reported as 500 (noise) | `context.WithoutCancel` for rollback; treat Canceled as 499/no-log |
| SM10 | `handlers.go:282`; `project_handlers.go:121-128, 217` | swallowed | low | `json.Encode` errors ignored → truncated responses invisible | Check and log |

## 6. Recommended fix order

1. **Logging contract + server layer** (P1, P2, CS9, RR8, LU2, SM1, SM2; partially LU3, LU7, SM8, P5). One `slog` JSON logger on stdout; `writeError(w, r, status, clientMsg, err)` that logs `err` with method, route pattern, request id and user id; middleware for request id, access log and panic recovery; `http.Server` with timeouts. Highest blindness removed per effort: every flow at once. *Constraint:* never log passwords, raw emails or session cookies; keep client messages unchanged.
2. **Queue supervision** (CS1, CS2, CS3, CS4, RR5, P3). `recover` per job, log every attempt and every status-write failure with lib id/batch/purl, boot sweep for `pending`/`processing`, graceful shutdown that drains the channel. *Constraint:* sweep must be idempotent across multiple machines if Fly runs >1 instance.
3. **Honest redundancy report** (RR1, RR2, RR3, RR4, RR9, RR10). Architect-scoped progress endpoint, `LEFT JOIN` with per-library status, completeness flag that suppresses retire recommendations on partial data. *This is also a product correctness bug in the north-star flow* — worth its own change.
4. **Upstream classification** (CS5, CS6, CS7, CS8, CS10). Only 404 is "not found"; unparseable LLM output and zero valid categories become explicit errors/`needs_manual` with a logged reason; include OpenRouter body + model in errors.
5. **Fail-fast config** (P4, LU5, LU6). Validate `SESSION_SECRET` and seed hashes at boot; log when the env seed is skipped.
6. **Frontend error contract** (LU1, LU4, RR12, RR13, SM4-SM7, CS13-CS17, P7). Shared `ApiError(status, message)`, AuthProvider error state, login distinguishes 401 from 5xx, no "done"/empty on error, global handlers posting to a small client-error endpoint. *Constraint:* don't send PII from the browser.
7. **Release identity** (P6) — cheap, do alongside the deploy change.

Integrity fixes surfaced along the way (not observability, but found here): RR6 (non-atomic project upload), RR7 (empty SBOM wipes project), SM3/RR14 (UUID validation), CS11/CS12.

## 7. Changes since last audit

First run — no previous report.

## 8. Method and limits

- **Agents:** 4 area auditors (catalog seeding, redundancy report, login & users, strategic marking) + 1 plumbing auditor, all read-only, run in parallel.
- **Spot-checked by hand:** RR1 (`main.go:51` admin-only route; `redundancy.tsx:120-126` catch → `onSettled()`), RR2 (`project.go:202` inner join), CS5 (`depsdev.go:68-80`), CS6 (`openrouter.go:216-219`), CS7 (`pipeline.go:70-89` — auditor cited wrong lines 161-186, corrected), CS1 (no `recover` anywhere in `backend/`). All confirmed.
- **Runtime proof:** partial — one observed run (OpenRouter 401 on a local throwaway instance during change `testing-pipeline-input-hardening`, manual check 3.6): 9/9 libraries `failed`, cause only in `libraries.error_detail`, server stdout contained only the startup line. No probe harness was built; other rows in §3 are static.
- **Repo-wide sweep (non-test):** `log` imported in 1 file (`main.go`); 26 explicit 500 paths, 0 logging the cause; 3 `_ =` error discards (1 consequential: `queue.go:97`); ~40 handler `if err != nil { … return }` without logging; 12 `fmt.Errorf` without `%w` (all leaf errors); 20 frontend `catch` blocks (most replace the error with a fixed string, 2 default to `[]`); 0 `console.*`.
- **Not verifiable:** Fly log rendering/retention, health-check wiring (no `fly.toml` yet), whether pgx `SendBatch` in `CreateLibraries` is atomic.
