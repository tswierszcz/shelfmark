# Fix nil-slice → JSON `null` on list endpoints — Implementation Plan

## Overview

Four Go query functions declare their result as `var x []T` and return it unchanged when zero rows match, so `encoding/json` marshals the response body as the literal `null` instead of `[]`. One of them (`ListProjects`) already crashes `/redundancy` in production for any project-less architect/viewer account (`TypeError: Cannot read properties of null (reading 'map')`). This plan applies the same one-line guard the codebase already uses elsewhere (`ListLibrariesHandler`'s `make([]responseType, len(...))` / `if names == nil { names = []string{} }` idiom) to all four functions, and adds a defensive `?? []` at every frontend call site that consumes one of these endpoints without it — closing the bug class recorded in `context/foundation/lessons.md` ("Go nil slices marshal to JSON `null`, not `[]`"), not just the one symptom that was hit.

## Current State Analysis

- `backend/internal/catalog/project.go:171` — `ListProjects`: `var projects []Project`, returned as-is. Consumed by `ListProjectsHandler` (`backend/internal/catalog/project_handlers.go:203-214`), which `json.Encode`s it directly with no guard. **Confirmed crash path**: `GET /api/projects` on an empty table returns body `null`; `web/app/routes/redundancy.tsx:57` does `setProjects(result)` with no fallback, so `projects.map(...)` (`redundancy.tsx:239`) throws on first render.
- `backend/internal/catalog/project.go:215` — `ProjectReport`: `var report []ProjectReportRow`, same unguarded pattern. `web/app/lib/projects.ts`'s `getProjectReport` return value is already consumed defensively (`redundancy.tsx:73`, `result ?? []`), so this is an API-contract inconsistency (empty project ships `null`, not `[]`), not a live crash.
- `backend/internal/catalog/store.go:135` — `ListTaxonomy`: `var categories []Category`, unguarded in `ListTaxonomyHandler` (`store.go:259-271`). The `categories` table is permanently seeded by migration `backend/internal/db/migrations/0002_catalog_schema.up.sql` (14 fixed rows, see `TestListTaxonomy_SeededByMigration`, `backend/internal/catalog/store_test.go:104`), so this path is not reachable in normal operation today — fixed for defense-in-depth and lesson compliance, not an active crash.
- `backend/internal/auth/users_store.go:81` — `UsersStore.List`: `var users []UserSummary`, unguarded in `ListUsersHandler` (`backend/internal/auth/handlers.go:153-164`). At least one admin account always exists (env-seeded on first boot), so this path is also not reachable today — same defense-in-depth rationale.
- **Already-correct precedent in the same codebase**: `ListLibrariesHandler` (`backend/internal/catalog/handlers.go:150-183`) builds `resp := make([]libraryResponse, len(libraries))` — a `make` with length 0 is a non-nil empty slice — and `ListOrganizationLibrariesHandler` (`handlers.go:195-253`) additionally guards its per-library nested slices with `if names == nil { names = []string{} }` / `if marks == nil { marks = []StrategicMark{} }`. This plan applies the same shape of fix, not a new idiom.
- **Frontend call sites lacking a `?? []` fallback** (will crash on `setState(null)` the same way `redundancy.tsx` did):
  - `web/app/routes/redundancy.tsx:57` — `setProjects(result)`
  - `web/app/routes/catalog.tsx:61` — `setLibraries(result)`
  - `web/app/routes/catalog.tsx:70` — `listTaxonomy().then(setCategories)...`
  - `web/app/routes/organization.tsx:45` — `setLibraries(result)`
  - `web/app/routes/organization.tsx:54` — `listTaxonomy().then(setCategories)...`
  - `web/app/routes/users.tsx:22` — `setUsers(result)`
- **Existing test gap**: `TestListProjectsHandler_ReturnsUploadedProjects` (`backend/internal/catalog/project_handlers_test.go:511-548`) only covers the "one project exists" case; no test anywhere in the backend asserts the zero-row response body shape for any of the four functions.

## Desired End State

Every one of the four Go query functions returns a non-nil empty slice when zero rows match, so their handlers serialize `[]` instead of `null`. Every frontend call site that consumes one of these endpoints falls back to `[]` defensively regardless of what the backend sends. `/redundancy` (and `/catalog`, `/organization`, `/users`) render correctly for a brand-new, project-less/library-less org instead of crashing.

**Verification**: `go test ./backend/...` passes including new zero-row tests; `cd web && npm run typecheck` passes; manually confirmed `GET /api/projects` on an empty table returns `[]` not `null` (`curl`), and `/redundancy` loads without error for an architect account with zero projects.

### Key Discoveries:

- The fix idiom already exists in this codebase (`handlers.go:176`, `handlers.go:180`, `handlers.go:246-248`) — this plan is consistency, not invention.
- `ListTaxonomy` and `UsersStore.List` are currently unreachable in the null state (seeded taxonomy, seeded admin), so their phase-1 tests must force the empty condition explicitly (delete the seeded rows within the test transaction/fixture) rather than relying on a naturally empty table.

## What We're NOT Doing

- Not adding a lint rule or CI check to catch future instances of `var x []T` — out of scope per `context/foundation/lessons.md`'s rule, which is documentation-level guidance for reviewers/agents, not tooling.
- Not touching `CategoryNamesByLibrary` (`store.go:222`) or `StrategicMarksByLibrary` (`store.go:406-436`) — both already initialize as `map[string][]string{}` / `map[string][]StrategicMark{}` literals, not nil-prone.
- Not changing `ListLibrariesHandler` or `ListOrganizationLibrariesHandler` — both already guard correctly; cited only as the precedent pattern.
- Not adding pagination, caching, or any behavior change beyond the null→`[]` fix.

## Implementation Approach

Fix the backend at the source (the four query functions) first, verified by tests, then add frontend defense-in-depth on top — matching the two-layer guarantee the lesson calls for ("every Go handler... every frontend consumer... should still defensively fall back"). Backend and frontend are independently testable and committable, hence two phases.

## Phase 1: Backend nil-guards + regression tests

### Overview

Change each of the four query functions' zero-value slice declaration from `var x []T` (nil-prone) to `x := []T{}` (non-nil empty slice) — a one-line change per function, no control-flow added, since `append` on a `[]T{}` literal behaves identically to `append` on a nil slice. Add one test per function asserting the zero-row response is `[]`, not `null`.

### Changes Required:

#### 1. `ListProjects` + `ProjectReport` nil-guard

**File**: `backend/internal/catalog/project.go`

**Intent**: Guarantee both project-listing functions return a non-nil empty slice when no rows match, closing the confirmed `/redundancy` crash and the matching API inconsistency on the report endpoint.

**Contract**: Line 171, change `var projects []Project` to `projects := []Project{}`. Line ~215 (inside `ProjectReport`), change `var report []ProjectReportRow` to `report := []ProjectReportRow{}`. No other lines in either function change — `append` behaves the same on both declarations.

#### 2. `ListTaxonomy` nil-guard

**File**: `backend/internal/catalog/store.go`

**Intent**: Apply the same guarantee to the taxonomy listing, for defense-in-depth and to close the lesson's documented bug class even though the table is always migration-seeded today. `ListTaxonomy` has one other caller besides `ListTaxonomyHandler` — `ProcessLibrary` (`backend/internal/catalog/pipeline.go:54`), which ranges over the result — but ranging over nil vs. `[]Category{}` is behaviorally identical in Go, so it's unaffected by this change.

**Contract**: Line 135, change `var categories []Category` to `categories := []Category{}`.

#### 3. `UsersStore.List` nil-guard

**File**: `backend/internal/auth/users_store.go`

**Intent**: Apply the same guarantee to the user listing, for the same defense-in-depth reason (an admin account always exists today, but the function itself shouldn't rely on that).

**Contract**: Line 81, change `var users []UserSummary` to `users := []UserSummary{}`.

#### 4. Zero-row regression tests

**Files**: `backend/internal/catalog/project_handlers_test.go`, `backend/internal/catalog/store_test.go` (or `handlers_test.go`, matching wherever `TestListTaxonomyHandler_ReturnsSeededCategories` already lives), `backend/internal/auth/handlers_test.go`

**Intent**: Lock in the `[]` response shape at the HTTP-handler boundary (what the frontend actually receives) for all four functions, closing the test gap noted in `project_handlers_test.go:511-548`.

**Contract**: One new test per function, each asserting the raw response body for a zero-row case is `[]` (e.g. `rec.Body.String() == "[]\n"`, matching `json.Encoder`'s trailing-newline behavior), not `null`:
- `ListProjectsHandler` on a fresh store with no uploads (natural zero state, same fixture pattern as `TestListProjectsHandler_ReturnsUploadedProjects`).
- `ProjectReportHandler` for a project with zero attached libraries: use `store.FindOrCreateProject(ctx, "Empty Project", "test@example.com")` (`project.go:49`) to create the project row directly, without ever calling `ReplaceProjectLibraries` — existing fixtures (`doProjectUpload`, `setUpProjectReportFixture`) all upload an SBOM that attaches at least one library, so they can't produce the zero-library case.
- `ListTaxonomyHandler` after deleting all seeded category rows within the test (the table is otherwise never empty — this test must force the condition).
- `ListUsersHandler` after deleting all seeded user rows within the test (same — an admin always exists in practice).

### Success Criteria:

#### Automated Verification:

- Backend tests pass: `go test ./backend/internal/catalog/... ./backend/internal/auth/...`

#### Manual Verification:

- `curl -s http://localhost:8080/api/projects` (with a valid architect/viewer session, zero projects) returns `[]`, not `null`.

---

## Phase 2: Frontend defensive fallbacks

### Overview

Add a `?? []` fallback at every frontend call site that assigns a list-endpoint response straight into component state, matching the pattern `refreshReport()` already uses in the same file (`redundancy.tsx:73`). This is defense-in-depth on top of Phase 1, not a replacement for it — per the lesson, the frontend should not rely on the backend's guarantee holding forever.

### Changes Required:

#### 1. `redundancy.tsx` projects fallback

**File**: `web/app/routes/redundancy.tsx`

**Intent**: Stop `projects.map(...)` from crashing if `listProjects()` ever again resolves to `null`.

**Contract**: Line 57, change `setProjects(result);` to `setProjects(result ?? []);`.

#### 2. `catalog.tsx` libraries + categories fallback

**File**: `web/app/routes/catalog.tsx`

**Intent**: Same guarantee for the catalog view's two list endpoints.

**Contract**: Line 61, change `setLibraries(result);` to `setLibraries(result ?? []);`. Line 70, change `listTaxonomy().then(setCategories).catch(...)` to `listTaxonomy().then((result) => setCategories(result ?? [])).catch(...)` (catch clause unchanged).

#### 3. `organization.tsx` libraries + categories fallback

**File**: `web/app/routes/organization.tsx`

**Intent**: Same guarantee for the organization view's two list endpoints.

**Contract**: Line 45, change `setLibraries(result);` to `setLibraries(result ?? []);`. Line 54, change `listTaxonomy().then(setCategories).catch(...)` to `listTaxonomy().then((result) => setCategories(result ?? [])).catch(...)` (catch clause unchanged).

#### 4. `users.tsx` users fallback

**File**: `web/app/routes/users.tsx`

**Intent**: Same guarantee for the admin users list.

**Contract**: Line 22, change `setUsers(result);` to `setUsers(result ?? []);`.

### Success Criteria:

#### Automated Verification:

- Type checking passes: `cd web && npm run typecheck`
- Build succeeds: `cd web && npm run build`

#### Manual Verification:

- `/redundancy` loads without error for an architect account with zero projects (re-test the exact scenario that crashed during `redundancy-view-brand-tokens` close-out).

**Implementation Note**: Pause here for manual confirmation before closing the plan.

---

## Testing Strategy

### Unit Tests:

- One zero-row test per guarded function (see Phase 1, item 4) — asserts response body is `[]`, not `null`.

### Manual Testing Steps:

1. Start backend + frontend locally against a database with zero projects for the test architect account.
2. Log in as that architect, visit `/redundancy` — page renders the empty project-selector state instead of crashing.
3. `curl -s http://localhost:8080/api/projects` with that session cookie — body is `[]`.

## Performance Considerations

None — single-line slice-initialization change, no new queries or allocations beyond what already happens on a non-empty result.

## Migration Notes

Not applicable — no schema or data changes.

## References

- Lesson: `context/foundation/lessons.md` ("Go nil slices marshal to JSON `null`, not `[]`")
- Discovered during: `context/archive/2026-10-19-redundancy-view-brand-tokens/` close-out
- Precedent fix pattern: `backend/internal/catalog/handlers.go:176`, `:180`, `:246-248`

## Progress

> Convention: `- [ ]` pending, `- [x]` done. Append ` — <commit sha>` when a step lands. Do not rename step titles. See `references/progress-format.md`.

### Phase 1: Backend nil-guards + regression tests

#### Automated

- [x] 1.1 Backend tests pass: `go test ./backend/internal/catalog/... ./backend/internal/auth/...` — 4a6d826

#### Manual

- [x] 1.2 `curl -s http://localhost:8080/api/projects` (zero projects) returns `[]`, not `null` — 4a6d826

### Phase 2: Frontend defensive fallbacks

#### Automated

- [x] 2.1 Type checking passes: `cd web && npm run typecheck`
- [x] 2.2 Build succeeds: `cd web && npm run build`

#### Manual

- [x] 2.3 `/redundancy` loads without error for an architect account with zero projects
