# Dashboard Statistics Implementation Plan

## Overview

Replace the empty `/dashboard` (it only shows the user's email and role) with a statistics page that every role lands on: library counts by categorization status, categories, projects, strategic marks, per-ecosystem counts, the top redundancy hotspots, and one role-specific call to action. A new aggregate endpoint, `GET /api/dashboard`, open to all three roles, feeds the page. The existing list endpoints are role-restricted, and pulling whole lists to count them in the browser would not scale.

## Current State Analysis

- `web/app/routes/dashboard.tsx` renders `Nav`, a heading and "Logged in as <email> (<role>)" inside `RequireAuth` with no role restriction.
- Data model (migrations 0002–0004): `libraries.status` is one of `pending`, `processing`, `categorized`, `needs_manual`, `failed` (CHECK constraint). `categories` holds a 14-row seeded taxonomy. `library_categories` links libraries to categories. `projects` links to libraries through `project_libraries(project_id, library_id, is_direct)`. `strategic_marks(library_id, category_id, active, …)` is capped at 2 active libraries per (category, ecosystem).
- Every library is `maven` today; `libraries.ecosystem` defaults to `'maven'`.
- None of the existing endpoints works for every role. `GET /api/catalog/libraries` is admin-only. `GET /api/organization/libraries` and `GET /api/projects` are architect/viewer only (`backend/cmd/shelfmark/routes.go:44-62`).
- `/catalog` and `/organization` keep their filters in `useState` and do not read them from the URL, so a link cannot pre-filter them.

## Desired End State

- `GET /api/dashboard` returns `200` with the JSON document below for admin, architect and viewer, and `401` without a session. It never contains `null` (lessons.md).
- `/dashboard` shows, for every role:
  - KPI tiles for libraries, categories (in use / in taxonomy), projects and active strategic marks
  - a status breakdown
  - a per-ecosystem table
  - the hotspot list

  Each section also has explicit loading, error and empty states.
- Admin sees "N libraries need manual review" linking to `/catalog` when `needs_manual > 0`. Architect sees "N redundant categories without a strategic decision" linking to `/organization` when that count is > 0. Viewer sees no call to action.

### Response contract

```json
{
  "libraries": {
    "total": 10,
    "by_status": { "pending": 0, "processing": 0, "categorized": 8, "needs_manual": 1, "failed": 1 },
    "by_ecosystem": [{ "ecosystem": "maven", "count": 10 }]
  },
  "categories": { "total": 14, "in_use": 6 },
  "projects": 4,
  "strategic_marks": 3,
  "hotspots": [
    { "category_id": "…", "category_name": "logging", "library_count": 3, "reviewed": false }
  ],
  "unreviewed_hotspots": 2
}
```

Contract rules:

- `by_status` always carries all five status keys, zero-filled.
- `by_ecosystem` is sorted by count descending, then ecosystem name A→Z. It is `[]` when there are no libraries.
- `categories.in_use` counts categories with at least one `library_categories` row.
- `strategic_marks` counts active `strategic_marks` rows.
- **Hotspot rule**: a category is a hotspot when it has **≥ 2 distinct libraries that are a direct dependency (`project_libraries.is_direct`) of at least one project**. `library_count` is that distinct count. Libraries that are only in the catalog, or only transitive, do not count.
- **Ordering**: `library_count` descending, then `category_name` A→Z. At most **5** rows. A 6th category tied with the 5th is cut off.
- `reviewed` uses the existing rule: the category has at least one active strategic mark in any ecosystem (`strategicIndex.isReviewed`, still open decision K1).
- `unreviewed_hotspots` counts **all** hotspot categories (not only the top 5) with `reviewed = false`.

### Key Discoveries:

- `strategicIndex` is the single owner of the "reviewed" rule (`backend/internal/catalog/strategic.go:206-247`) and is loaded with `loadStrategicIndex` (`strategic.go:249`). Reuse it rather than writing a second SQL definition.
- The count-by-status pattern is `Store.BatchProgress` (`backend/internal/catalog/store.go:311`). The handler pattern is `ListTaxonomyHandler` (`handlers.go:301`) together with `writeCatalogError` / `obs.LogWarn`.
- Raw-JSON contract helper: `assertNoNullArrays` (`backend/internal/catalog/json_contract_test.go:21`). DB test helpers: `startTestPostgres`, `newTestCategory`, `newTestLibraries` (`store_test.go:26, 399, 412`).
- Every route must appear in `expectedRoles` (`backend/cmd/shelfmark/routes_test.go:18`), or `TestRoutes_RoleSetsMatchExpected` fails.
- Frontend: API clients follow `web/app/lib/projects.ts` (`fetch` → `errorMessage` → typed JSON). Route tests use `renderRoute`/`stubFetch` (`web/app/test/`), and per lessons.md the client must still fall back with `?? []` / `?? {}`.

## What We're NOT Doing

- No retire-candidate total. It would need the `strategicIndex` retire rule across all projects and is deferred.
- No per-ecosystem scoping of "reviewed" (open decision K1). The dashboard uses today's global rule.
- No URL filters on `/catalog` or `/organization`. Call-to-action links go to the plain page.
- No role-tailored layouts. Every role sees the same statistics.
- No schema migration, caching, charts library or auto-refresh.

## Implementation Approach

The backend computes everything in a handful of aggregate queries plus one `loadStrategicIndex` call, inside one store method, and the handler encodes a response struct with non-nil slices and maps. The frontend fetches once on mount and renders sections with design tokens and the existing `Badge`/`Button` components.

## Phase 1: Dashboard stats API

### Overview

Add the aggregate store method, the handler and the route, and lock the counting rules and the JSON contract with tests.

### Changes Required:

#### 1. Store aggregates

**File**: `backend/internal/catalog/dashboard.go` (new)

**Intent**: Compute every dashboard number in one place, reusing the `strategicIndex` rule for `reviewed`.

**Contract**: `func (s *Store) DashboardStats(ctx context.Context) (DashboardStats, error)` returns a struct whose JSON shape is the response contract above. It returns non-nil `ByStatus` (all five keys), `ByEcosystem` and `Hotspots`. The hotspot query counts `DISTINCT l.id` over `library_categories ⋈ project_libraries` with `is_direct`, keeps `HAVING count >= 2` and orders by count desc then name. The full hotspot list feeds `unreviewed_hotspots`, and the response is then truncated to 5.

#### 2. Handler and route

**File**: `backend/internal/catalog/dashboard.go`, `backend/cmd/shelfmark/routes.go`

**Intent**: Expose the stats to every signed-in role.

**Contract**: `DashboardHandler(store *Store) http.HandlerFunc` maps a store error to `500` through `writeCatalogError` and success to `200 application/json`. The route is `{"GET", "/api/dashboard", everyone, catalog.DashboardHandler(d.Catalog)}`. The payload contains no `error_detail` or other admin-only field.

#### 3. Tests

**File**: `backend/internal/catalog/dashboard_test.go` (new), `backend/internal/catalog/json_contract_test.go`, `backend/cmd/shelfmark/routes_test.go`

**Intent**: Lock the counting rules and the contract.

**Contract**:
- `TestDashboardStats_EmptyDatabase`: `total` 0, all five status keys present with 0, `by_ecosystem` and `hotspots` empty, `categories.total` 14.
- `TestDashboardStats_HotspotCountsOnlyDirectProjectLibraries`: in one category, 3 catalog libraries, 2 direct in a project and 1 transitive only, gives `library_count` 2. A category with 1 direct library is absent.
- `TestDashboardStats_HotspotsTopFiveOrderedByCountThenName`: 6 hotspot categories, two of them tied, give 5 rows ordered by count then name, and `unreviewed_hotspots` 6.
- `TestDashboardStats_ReviewedFollowsActiveStrategicMark`: an active mark sets `reviewed` true and lowers `unreviewed_hotspots`, while an inactive (unmarked) one does not.
- `TestDashboardStats_StatusAndEcosystemCounts`: mixed statuses and two ecosystems give the right counts and ordering.
- `TestDashboardHandler_EmptyDatabase_NoNullArrays` in `json_contract_test.go`, using `assertNoNullArrays`.
- `TestDashboardHandler_StoreError_Returns500`: a store whose pool is already closed makes the handler answer `500` with the `{"error": …}` body from `writeCatalogError`.
- `expectedRoles` gains `"GET /api/dashboard": {admin, architect, viewer}`.

### Success Criteria:

#### Automated Verification:

- Backend vet passes: `cd backend && go vet ./...`
- Backend tests pass: `cd backend && go test ./...`
- Dashboard tests actually ran: `cd backend && go test ./internal/catalog -run 'TestDashboard' -v` shows PASS, not SKIP, for every dashboard test (Docker must be reachable)
- Counting `is_direct` regardless of its value (dropping the direct filter) makes `TestDashboardStats_HotspotCountsOnlyDirectProjectLibraries` fail, and restoring it makes it pass

#### Manual Verification:

- On a local server with the seed data, `curl` with an architect session cookie to `/api/dashboard` returns the documented shape, and without a cookie returns `401`

**Implementation Note**: After completing this phase and all automated verification passes, pause here for manual confirmation from the human that the manual testing was successful before proceeding to the next phase.

---

## Phase 2: Dashboard view

### Overview

Render the statistics on `/dashboard` for every role, with the role's call to action and explicit loading, error and empty states.

### Changes Required:

#### 1. API client

**File**: `web/app/lib/dashboard.ts` (new)

**Intent**: A typed client for the endpoint, in the style of `lib/projects.ts`.

**Contract**: Exports a `DashboardStats` type that mirrors the response contract and `getDashboardStats(): Promise<DashboardStats>`, which throws `Error(errorMessage(...))` on non-2xx.

#### 2. Dashboard route

**File**: `web/app/routes/dashboard.tsx`

**Intent**: Replace the placeholder with the statistics page.

**Contract**:
- Keeps `RequireAuth` (all roles), `Nav` and the "Dashboard" heading. The email/role line may stay as a subtitle.
- The h1 stays exactly `Dashboard`, and no link is labelled `Catalog` for architect or viewer: `web/tests/e2e/seed.spec.ts:10-13` asserts both.
- Fetches once on mount and renders four states:
  - **loading** ("Loading statistics…")
  - **error**: an inline message using `text-destructive`, with no stale numbers
  - **empty**: when `libraries.total === 0`, admin sees "Upload a catalog to get started" with a link to `/catalog`, and architect/viewer see "The catalog is empty"
  - **data**
- Data state has 4 KPI tiles, a status list in the order categorized, needs review, failed, processing (= `pending + processing`), the ecosystem table, the hotspot list (category, library count, `Badge` "Reviewed" / "No decision"; text "No redundancy found yet" when empty) and the role call to action described in Desired End State.
- Nested arrays and maps fall back with `?? []` / `?? {}`. Only `@theme` tokens are used, with no palette or hex classes.

#### 3. Route tests

**File**: `web/app/routes/dashboard.test.tsx` (new)

**Intent**: Cover the states and the role-specific call to action.

**Contract**: Uses `renderRoute(Dashboard, { path: "/dashboard", role, fetch: { "/api/dashboard": … } })`:
- pending shows the loading text
- 500 shows the error and no KPI numbers
- `total: 0` shows the empty state per role
- data shows the KPIs, the hotspot rows in payload order and the badges
- admin shows the needs-review link only when `needs_manual > 0`
- architect shows the no-decision link only when `unreviewed_hotspots > 0`
- viewer shows no call-to-action link
- `hotspots: null` / `by_ecosystem: null` render without crashing

### Success Criteria:

#### Automated Verification:

- Frontend tests pass: `cd web && npm test`
- Type checking passes: `cd web && npm run typecheck`
- Build passes: `cd web && npm run build`
- E2E seed spec passes: `cd web && npx playwright test tests/e2e/seed.spec.ts`
- Removing the role check on the architect call to action makes `npm test` fail, and restoring it makes it pass

#### Manual Verification:

- On a local build, admin, architect and viewer each see the statistics, and the numbers match the seed data (cross-check with `/catalog` and `/organization`)
- The page reads well at phone width (375 px) and in dark mode

**Implementation Note**: After completing this phase and all automated verification passes, pause here for manual confirmation from the human that the manual testing was successful.

---

## Testing Strategy

### Unit / Integration Tests:

- Backend: the store tests above against the Postgres container, the raw-JSON contract test and the role matrix entry.
- Frontend: `dashboard.test.tsx` per state and role.

### Manual Testing Steps:

1. `docker compose up -d db`, build the web app, run the server with `.env` loaded.
2. Log in as each seed account, open `/dashboard` and compare the numbers with `/catalog` (admin) or `/organization` and `/redundancy` (architect/viewer).
3. Check phone width and dark mode.

## Performance Considerations

There are four or five aggregate queries plus one strategic-index load per request, all over small tables (tens to thousands of rows). They use existing indexes (`libraries_status_idx`) and need no caching.

## References

- Status count pattern: `backend/internal/catalog/store.go:311`
- Reviewed rule: `backend/internal/catalog/strategic.go:206-247`
- JSON contract helper: `backend/internal/catalog/json_contract_test.go:21`
- Role matrix: `backend/cmd/shelfmark/routes_test.go:18`
- Route test harness: `web/app/test/render-route.tsx`
- Open decision K1: `context/domain/domain-distillation.md` (R-05)

## Progress

> Convention: `- [ ]` pending, `- [x]` done. Append ` — <commit sha>` when a step lands. Do not rename step titles. See `references/progress-format.md`.

### Phase 1: Dashboard stats API

#### Automated

- [x] 1.1 Backend vet passes: `cd backend && go vet ./...` — 5762628
- [x] 1.2 Backend tests pass: `cd backend && go test ./...` — 5762628
- [x] 1.3 Dashboard tests actually ran: `cd backend && go test ./internal/catalog -run 'TestDashboard' -v` shows PASS, not SKIP, for every dashboard test (Docker must be reachable) — 5762628
- [x] 1.4 Counting `is_direct` regardless of its value (dropping the direct filter) makes `TestDashboardStats_HotspotCountsOnlyDirectProjectLibraries` fail, and restoring it makes it pass — 5762628

#### Manual

- [x] 1.5 On a local server with the seed data, `curl` with an architect session cookie to `/api/dashboard` returns the documented shape, and without a cookie returns `401` — 5762628

### Phase 2: Dashboard view

#### Automated

- [x] 2.1 Frontend tests pass: `cd web && npm test` — b084d9e
- [x] 2.2 Type checking passes: `cd web && npm run typecheck` — b084d9e
- [x] 2.3 Build passes: `cd web && npm run build` — b084d9e
- [x] 2.4 E2E seed spec passes: `cd web && npx playwright test tests/e2e/seed.spec.ts` — b084d9e
- [x] 2.5 Removing the role check on the architect call to action makes `npm test` fail, and restoring it makes it pass — b084d9e

#### Manual

- [x] 2.6 On a local build, admin, architect and viewer each see the statistics, and the numbers match the seed data (cross-check with `/catalog` and `/organization`) — b084d9e
- [x] 2.7 The page reads well at phone width (375 px) and in dark mode — b084d9e
