<!-- IMPL-REVIEW-REPORT -->
# Implementation Review: Dashboard Statistics Implementation Plan

- **Plan**: context/changes/dashboard-stats/plan.md
- **Scope**: Full plan
- **Reviewed phases**: 1, 2
- **Date**: 2026-10-30
- **Verdict**: APPROVED
- **Findings**: 0 critical, 0 warnings, 2 observations

## Verdicts

| Dimension | Verdict |
|-----------|---------|
| Plan Adherence | PASS |
| Scope Discipline | PASS |
| Safety & Quality | PASS |
| Architecture | PASS |
| Pattern Consistency | PASS |
| Success Criteria | PASS |

Diff since plan start (c2f0157..HEAD) matches the plan's file list: 0 MISSING, 0 EXTRA. The hotspot rule (≥ 2 distinct direct project libraries), top-5 ordering (count desc, name A→Z), `reviewed` through `strategicIndex.isReviewed`, zero-filled `by_status`, non-nil slices, the route for all three roles plus its role-matrix entry, the four UI states and the per-role call to action are all implemented as planned. The payload carries no admin-only fields, and the route uses only `@theme` tokens. Success criteria at HEAD: `go vet`, 9 dashboard/route tests PASS with 0 SKIP (full `go test ./...` green at 5762628; backend code unchanged since), `npm test` 77/77, typecheck, build. The E2E seed spec (2/2) and both break-checks ran during implementation. Manual rows 1.5, 2.6 and 2.7 were confirmed by the user on a local build with demo data.

## Findings

### F1 — Hotspot EXISTS probes project_libraries by library_id without a leading index

- **Severity**: 🔍 OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Safety & Quality
- **Location**: backend/internal/catalog/dashboard.go (hotspot query)
- **Detail**: `project_libraries` has only `PRIMARY KEY (project_id, library_id)` (migration 0004), so `EXISTS (… WHERE pl.library_id = lc.library_id AND pl.is_direct)` cannot use an index on `library_id`. At today's size (tens of rows) this is irrelevant. With many projects and thousands of libraries it becomes a sequential scan per category attachment.
- **Fix**: None now. If the dashboard becomes slow, add `CREATE INDEX ON project_libraries (library_id) WHERE is_direct` in a new migration.
- **Decision**: PENDING

### F2 — errorMessage is duplicated once more in lib/dashboard.ts

- **Severity**: 🔍 OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Pattern Consistency
- **Location**: web/app/lib/dashboard.ts
- **Detail**: The same private `errorMessage` helper now exists in 5 files (`catalog.ts`, `users.ts`, `projects.ts`, `organization.ts`, `dashboard.ts`). This follows the existing pattern but extends the duplication.
- **Fix**: Leave as is. Extract a shared `lib/http.ts` helper in a separate refactor change if wanted.
- **Decision**: PENDING
