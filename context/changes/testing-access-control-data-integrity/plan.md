# Access control and data-integrity regression tests Implementation Plan

## Overview

Rollout Phase 2 of `context/foundation/test-plan.md`: lock role gating (#3), the never-`null` list contract (#2) and the strategic-marks cap (#6) against regression, and fix the two live defects the research found — upload responses send `"skipped": null`, and concurrent marks can exceed the cap of 2 per (category, ecosystem).

## Current State Analysis

From `context/changes/testing-access-control-data-integrity/research.md`:

- **#3:** all 18 routes are registered inline in `main()` (`backend/cmd/shelfmark/main.go:59-77`) behind DB connect/migrate/seed and `NewOpenRouterClient()`, so no test can reach the real route→role wiring. `auth.RequireRole` (`auth/middleware.go:29-64`) is the only gate (401 empty body for any session failure, 403 for a valid session with a disallowed role). The middleware is unit-tested with stub handlers; the only route-specific gating test re-wraps `RequireRole` itself (`catalog/handlers_test.go:647`), proving nothing about `main.go`.
- **#2:** 9 endpoints return an array or contain one. Raw-body `[]` tests exist for 4 (`auth/handlers_test.go:225`, `catalog/handlers_test.go:275`, `project_handlers_test.go:592,612`). Both upload endpoints send `"skipped": null` today because `ParsePurlCSV` (`catalog/csv.go:22-25`), `ParseCycloneDXSBOM` (`catalog/sbom.go:33-40`) and `ParseProjectSBOM` (`catalog/project_sbom.go:36-39`) declare `var skipped []SkippedRow`; `project_handlers_test.go:161` exercises this and passes because it decodes and checks `len`. `ListLibraries` returns nil on no rows (`catalog/store.go:205`) and relies on handler guards.
- **#6:** `MarkStrategic` (`catalog/store.go:367-414`) runs ecosystem lookup → count of active marks per (category, ecosystem) excluding the library → reject at `>= 2` → upsert, in a `pool.Begin` transaction at READ COMMITTED with no lock; no DB-level cap (`db/migrations/0003_strategic_marks.up.sql`). Sequential "2 OK, 3rd rejected" is tested (`store_test.go:319`); 409 mapping (`handlers.go:374-376`), edge cases and concurrency are not.
- **PRD divergence:** `prd.md:97` lists "uploads project SBOMs" for Admin; code allows Architect only (`main.go:74`). User decision: keep the code, fix the PRD.

## Desired End State

- `go test ./cmd/shelfmark/...` fails if any protected route's allowed-role set differs from an independent expected table, if a route is added without an entry in that table, or if any disallowed cell (no session / wrong role) does not return 401 / 403 through the real router.
- Every endpoint whose body is or contains an array returns `[]` (never `null`) for empty results, asserted on the raw JSON body; upload responses return `"skipped": []` when nothing was skipped.
- Under concurrent requests, at most 2 active strategic marks exist per (category, ecosystem); the third is rejected with HTTP 409 and the explicit message.
- PRD §Access Control matches the enforced roles; `test-plan.md` §6.2 documents the three patterns shipped.
- Verify: `cd backend && go build ./... && go vet ./... && go test ./...`.

### Key Discoveries:

- Handlers only capture their dependencies at construction (`*catalog.Store`, `*auth.UsersStore`, `catalog.Enqueuer`), and `RequireRole` rejects before the handler runs (`auth/middleware.go:45,55-57`) — denial cases need no database (research §3).
- `auth.IssueSession(w, r, email, role)` is exported (`auth/session.go:78`) and can mint test cookies from any package once `SESSION_SECRET` is set.
- Go decodes JSON `null` and `[]` into the same nil slice, so only raw-body or `any`-decoded inspection can tell them apart.
- `github_repo` is a legitimately nullable scalar (`*string`, `catalog/store.go:54`); `error_detail` is `omitempty`.
- The advisory-lock key needs the ecosystem, which is only known after the first `SELECT` in `MarkStrategic` (`store.go:374`).

## What We're NOT Doing

- No change to any route's role set or to Admin's access to projects/reports (user decision: code stays, PRD is corrected).
- No testing of allowed-role success paths through the router (they need real dependencies; handler tests already cover happy paths per package).
- No ownership/IDOR checks — the PRD defines a single organization with role-only access.
- No 404 for unknown `/api/*` paths (SPA catch-all returns 200 HTML; research open question 3) — follow-up.
- No DB-level cap constraint or trigger; no change to transaction isolation beyond the advisory lock.
- No json tags for `SkippedRow` (`Index`/`Reason` field names stay as-is).
- No frontend changes.

## Implementation Approach

Make the route table data (`[]route`) built by one function and registered in a loop, so the test can both compare it with an independent oracle and drive the real `http.ServeMux`. Then add raw-JSON contract tests for the uncovered list endpoints (red), fix the nil sources (green). Then cover the cap's HTTP contract and edges, add a concurrent test that fails today, and serialize the check-then-insert per (category, ecosystem) with `pg_advisory_xact_lock` (green). Finish with docs.

## Critical Implementation Details

- **Timing & lifecycle** — In `MarkStrategic` the lock must be taken inside the same transaction, after the ecosystem lookup and before the count: `SELECT pg_advisory_xact_lock(hashtextextended($1 || ':' || $2, 0))` with (categoryID, ecosystem). The `_xact_` variant releases on commit/rollback, so no unlock path is needed. Taking it before the ecosystem lookup would require a second key scheme; taking it after the count leaves the race open.
- **Debug & observability** — The race window is one round trip, so red-before-fix comes from the deterministic `afterCapCheck` seam test, not from luck; the goroutine load test stays as an extra check. The seam's wait must time out (2 s) so that, once the lock exists, the transaction holding the lock is not stuck waiting for one that is blocked on that lock.

## Phase 1: Route wiring + role-denial matrix (#3)

### Overview

Expose the real route registration to tests and prove every disallowed (route, role) cell is denied through it.

### Changes Required:

#### 1. Route table as data

**File**: `backend/cmd/shelfmark/routes.go` (new), `backend/cmd/shelfmark/main.go`

**Intent**: Move route registration out of `main()` without changing any path, method, role set or handler, so tests can use the exact same wiring.

**Contract**: `type route struct { Method, Pattern string; Roles []auth.Role; Handler http.Handler }` (nil `Roles` = public; protected handlers wrapped as `auth.RequireRole(Roles...)(h.ServeHTTP)`, so the SPA's `http.Handler` fits the same type), `func routes(d deps) []route` returning all 18 current routes in the current order, and `func newMux(d deps) *http.ServeMux` registering each as `Method + " " + Pattern`, wrapping with `auth.RequireRole(Roles...)` when `Roles` is non-nil and registering the SPA handler for `GET /`. `type deps struct { Pool *pgxpool.Pool; Users *auth.UsersStore; Catalog *catalog.Store; Queue catalog.Enqueuer; StaticDir string }`. `main()` builds `deps` and calls `newMux`; `obs.Middleware(newMux(d))` stays the server handler.

#### 2. Role matrix test

**File**: `backend/cmd/shelfmark/routes_test.go` (new)

**Intent**: Catch a widened/narrowed role set, an unlisted new route, or a wiring mistake that the middleware unit tests cannot see (anti-pattern avoided: re-wrapping handlers with `RequireRole` inside the test; testing one role only). The expected table is a **role regression lock**: the PRD does not specify roles per route, so its correctness rests on the human review in Manual 1.5, not on the test.

**Contract**:
- `expectedRoles` — an independent table written from PRD §Access Control and the user decision (not read from `routes()`): the 14 protected `METHOD pattern` keys with their allowed roles and the 4 public ones (`GET /healthz`, `POST /api/login`, `POST /api/logout`, `GET /`).
- `TestRoutes_RoleSetsMatchExpected` — every entry of `routes(deps{})` has a key in `expectedRoles` with an equal role set (order-insensitive), and every `expectedRoles` key exists in `routes()` (catches an unlisted or removed route).
- `TestRoutes_DeniesDisallowedCallers` — table-driven over every protected route × {no session, admin, architect, viewer}: through `newMux(deps{StaticDir: t.TempDir()})` with concrete path values (UUIDs for `{batchID}`, `{libraryID}`, `{categoryID}`, `{projectID}`), no session → 401; a role not in the expected set → 403. Cookies minted via `auth.IssueSession` into an `httptest.Recorder` with `SESSION_SECRET` set by `t.Setenv`. Allowed cells are skipped (not executed).
- Boundary covered: `GET /api/me` (all three roles allowed → only the no-session cell); `DELETE` strategic shares a path with `POST` (both checked by method).

### Success Criteria:

#### Automated Verification:

- `go build ./...` and `go vet ./...` pass
- `go test ./cmd/shelfmark/ -run 'TestRoutes_RoleSetsMatchExpected|TestRoutes_DeniesDisallowedCallers' -v` passes and reports both tests as run
- Full suite passes: `go test ./...`

#### Manual Verification:

- Running the server locally, `POST /api/catalog/upload` with an Architect cookie returns 403 and `GET /api/projects` with an Admin cookie returns 403 (same as before the refactor); the SPA still loads at `/`.
- The user reviewed the `expectedRoles` table in `routes_test.go` against PRD §Access Control and confirmed each route's role set.

**Implementation Note**: After completing this phase and all automated verification passes, pause here for manual confirmation from the human that the manual testing was successful before proceeding to the next phase.

---

## Phase 2: List endpoints never `null` (#2)

### Overview

Assert the raw JSON contract on every uncovered list endpoint (red), then remove the nil sources (green).

### Changes Required:

#### 1. Raw-JSON contract helper and tests

**File**: `backend/internal/catalog/json_contract_test.go` (new), extending tests in `handlers_test.go` / `project_handlers_test.go` only where needed

**Intent**: Detect `null` where an array is the contract — at top level and in array fields of items — regardless of which layer owns the guarantee (anti-pattern avoided: decoding into Go structs and checking `len`).

**Contract**: test helper `assertNoNullArrays(t, body []byte, nullable ...string)` decodes into `any` and fails if the top level is `null` or any object field not listed in `nullable` is `null` (so `github_repo` is passed as nullable). Tests:
- `TestListLibrariesHandler_EmptyAndUncategorized_NoNullArrays` — empty DB → body is `[]`; one library with no categories/marks → `categories` and `strategic` are `[]`.
- `TestListOrganizationLibrariesHandler_EmptyAndUncategorized_NoNullArrays` — same two scenarios.
- `TestBatchProgressHandler_UnknownBatch_ReturnsEmptyObject` — raw body is `{}`.
- `TestUploadHandler_NothingSkipped_ReturnsEmptySkippedArray` — CSV and SBOM uploads with only valid rows → `"skipped": []`.
- `TestProjectUploadHandler_NothingSkipped_ReturnsEmptySkippedArray` — valid project SBOM → `"skipped": []`.
Expected to fail before change 2 on both upload tests (regression caught: `"skipped": null`).

#### 2. Remove nil sources

**File**: `backend/internal/catalog/csv.go`, `sbom.go`, `project_sbom.go`, `store.go`

**Intent**: Own the guarantee at the source so a new handler reusing these functions inherits it (lessons.md rule).

**Contract**: `ParsePurlCSV`, `ParseCycloneDXSBOM` (including its `bom.Components == nil` early return) and `ParseProjectSBOM` return a non-nil `[]SkippedRow{}` when nothing is skipped; `ListLibraries` returns `[]Library{}` on no rows. Existing handler guards stay.

### Success Criteria:

#### Automated Verification:

- `go build ./...` and `go vet ./...` pass
- `go test ./internal/catalog/ -run 'NoNullArrays|ReturnsEmptyObject|ReturnsEmptySkippedArray' -v` passes and reports all 5 tests as run
- Full suite passes: `go test ./...`

#### Manual Verification:

- Uploading a valid CSV through the running app returns a response containing `"skipped":[]` (devtools network tab).

**Implementation Note**: After completing this phase and all automated verification passes, pause here for manual confirmation from the human that the manual testing was successful before proceeding to the next phase.

---

## Phase 3: Strategic cap — contract, edges, concurrency (#6)

### Overview

Pin the cap's HTTP contract and edge semantics, prove the concurrent bypass with a failing test, then serialize the check per (category, ecosystem).

### Changes Required:

#### 1. Contract and edge tests

**File**: `backend/internal/catalog/store_test.go`, `backend/internal/catalog/handlers_test.go`

**Intent**: Cover the boundaries the existing sequential test does not (anti-pattern avoided: sequential-only and single-mark tests).

**Contract**:
- `TestMarkStrategicHandler_ThirdMarkReturns409` — two marks via the handler succeed (204), the third in the same (category, ecosystem) returns 409 with body `{"error":"already 2 strategic libraries marked for this category and ecosystem"}`.
- `TestMarkStrategic_CapEdges` — table of: another ecosystem in the same category at the cap → allowed; unmark one of two → a new third library allowed; re-activating a previously unmarked library while 2 others are active → `ErrStrategicCapReached`; same library marked in a second category → allowed.

#### 2. Concurrency test (red), then the lock (green)

**File**: `backend/internal/catalog/store_test.go`, `backend/internal/catalog/store.go`

**Intent**: Prove that concurrent marks cannot exceed 2 active per (category, ecosystem), then make it true.

**Contract**:
- Test seam in `store.go`: unexported package variable `afterCapCheck func()` (nil in production, no-op when nil), called in `MarkStrategic` right after the cap check passes and before the insert.
- `TestMarkStrategic_InterleavedMarksRespectCap` (deterministic) — one active mark in a (category, ecosystem); the test sets `afterCapCheck` to a function that records arrival and waits until two transactions have arrived or 2 s have passed, then starts two goroutines marking two different libraries in that pair. Expected: exactly one succeeds, the other returns `ErrStrategicCapReached`, and the active count is 2. Must fail on the unfixed code (both pass the check → 3 active). After the lock, the second goroutine blocks on the lock, the first times out at the seam and commits, the second then counts 2 and is rejected. Restore `afterCapCheck` in `t.Cleanup`.
- `TestMarkStrategic_ConcurrentMarksNeverExceedCap` (load check, kept) — per iteration (≥20, fresh category + ≥10 libraries of one ecosystem): ≥10 goroutines wait on one barrier channel and call `MarkStrategic` for distinct libraries; afterwards a direct SQL count of active marks is ≤ 2 and exactly `n - 2` calls returned `ErrStrategicCapReached`.
- `MarkStrategic`: after the ecosystem lookup and before the count, `SELECT pg_advisory_xact_lock(hashtextextended($1 || ':' || $2, 0))` with categoryID and ecosystem; error wrapped like the other steps. No signature change.

### Success Criteria:

#### Automated Verification:

- `go build ./...` and `go vet ./...` pass
- `go test ./internal/catalog/ -run 'TestMarkStrategicHandler_ThirdMarkReturns409|TestMarkStrategic_CapEdges|TestMarkStrategic_InterleavedMarksRespectCap|TestMarkStrategic_ConcurrentMarksNeverExceedCap' -v` passes and reports all 4 tests as run
- `go test -race ./internal/catalog/ -run TestMarkStrategic_ConcurrentMarksNeverExceedCap -count=3` passes
- Full suite passes: `go test ./...`

#### Manual Verification:

- None — covered by automated tests against real Postgres.

**Implementation Note**: After completing this phase and all automated verification passes, proceed to Phase 4.

---

## Phase 4: Docs — PRD and test-plan cookbook

### Overview

Align the PRD with the enforced roles and record the shipped test patterns.

### Changes Required:

#### 1. PRD Access Control

**File**: `context/foundation/prd.md`

**Intent**: Remove the divergence found in research (user decision: code is right).

**Contract**: In §Access Control, Admin's capabilities no longer include "uploads project SBOMs"; add one line that project SBOM upload is Architect-only and project lists/reports are Architect + Viewer.

#### 2. Test-plan cookbook

**File**: `context/foundation/test-plan.md`

**Intent**: Make the patterns reusable for future endpoints.

**Contract**: §6.2 gains three bullets — route × role denial matrix via `routes()`/`newMux` with an independent expected-roles table (`cmd/shelfmark/routes_test.go`); raw-JSON never-`null` assertions with `assertNoNullArrays` (`catalog/json_contract_test.go`); barrier-released goroutine test for row-spanning invariants (`TestMarkStrategic_ConcurrentMarksNeverExceedCap`). §6.5 gets a Phase 2 note. §3 status is left to the archive step.

### Success Criteria:

#### Automated Verification:

- `! grep -nE '^\| Admin \|.*project SBOMs' context/foundation/prd.md` succeeds (Admin row no longer lists project SBOM upload; Architect row unchanged)
- `grep -n "assertNoNullArrays\|routes_test.go\|ConcurrentMarksNeverExceedCap" context/foundation/test-plan.md` finds all three patterns

#### Manual Verification:

- PRD §Access Control and the route table read consistently.

---

## Testing Strategy

### Unit Tests:

- `assertNoNullArrays` helper behaviour is exercised through the endpoint tests (top-level `null`, nested `null`, allowed nullable field).

### Integration Tests:

- Router-level denial matrix (no DB) for all protected routes × roles.
- Handler-level raw-JSON tests with an empty Postgres for 5 endpoints.
- Store-level cap edges and a concurrent bypass test against real Postgres.

### Manual Testing Steps:

1. Local server: Architect → `POST /api/catalog/upload` = 403; Admin → `GET /api/projects` = 403; `/` serves the SPA.
2. Upload a valid CSV; response contains `"skipped":[]`.

## Performance Considerations

The advisory lock serializes only concurrent marks within one (category, ecosystem); marks are rare, human-initiated actions. The concurrency test adds ~20 iterations against one Postgres container (seconds).

## Migration Notes

No schema migration. The lock is transaction-scoped and needs no cleanup.

## References

- Research: `context/changes/testing-access-control-data-integrity/research.md`
- Test plan: `context/foundation/test-plan.md` §2 (#2, #3, #6), §3 Phase 2
- Routes: `backend/cmd/shelfmark/main.go:59-77`; middleware `backend/internal/auth/middleware.go:29-64`
- Cap: `backend/internal/catalog/store.go:367-414`; handler mapping `backend/internal/catalog/handlers.go:370-378`
- Lessons: `context/foundation/lessons.md` (nil slices → `null`)

## Progress

> Convention: `- [ ]` pending, `- [x]` done. Append ` — <commit sha>` when a step lands. Do not rename step titles. See `references/progress-format.md`.

### Phase 1: Route wiring + role-denial matrix (#3)

#### Automated

- [x] 1.1 go build and go vet pass — 7b6a342
- [x] 1.2 Route role-set and denial-matrix tests pass — 7b6a342
- [x] 1.3 Full suite passes — 7b6a342

#### Manual

- [ ] 1.4 Local server: Architect upload 403, Admin projects 403, SPA loads
- [ ] 1.5 User reviewed expectedRoles table against PRD Access Control

### Phase 2: List endpoints never null (#2)

#### Automated

- [x] 2.1 go build and go vet pass — 199542d
- [x] 2.2 Raw-JSON contract tests pass (5 run) — 199542d
- [x] 2.3 Full suite passes — 199542d

#### Manual

- [ ] 2.4 Valid CSV upload returns "skipped":[]

### Phase 3: Strategic cap — contract, edges, concurrency (#6)

#### Automated

- [x] 3.1 go build and go vet pass
- [x] 3.2 Cap contract, edge and concurrency tests pass (4 run)
- [x] 3.3 Concurrency test passes under -race three times
- [x] 3.4 Full suite passes

### Phase 4: Docs — PRD and test-plan cookbook

#### Automated

- [ ] 4.1 PRD Admin row no longer lists project SBOM upload
- [ ] 4.2 test-plan §6.2 documents all three patterns

#### Manual

- [ ] 4.3 PRD Access Control and route table read consistently
