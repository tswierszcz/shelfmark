<!-- PLAN-REVIEW-REPORT -->
# Plan Review: Dashboard Statistics Implementation Plan

- **Plan**: context/changes/dashboard-stats/plan.md
- **Mode**: Deep
- **Date**: 2026-10-30
- **Verdict**: SOUND
- **Findings**: 0 critical, 2 warnings, 1 observation

## Verdicts

| Dimension | Verdict |
|-----------|---------|
| End-State Alignment | PASS |
| Lean Execution | PASS |
| Architectural Fitness | PASS |
| Blind Spots | WARNING |
| Plan Completeness | WARNING |

## Grounding

Grounding: 6/6 paths ✓, 7/7 symbols ✓, brief↔plan ✓

## Findings

### F1 — Backend gates can pass with the integration tests skipped

- **Severity**: ⚠️ WARNING
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Plan Completeness
- **Location**: Phase 1 — Success Criteria 1.2, 1.3
- **Detail**: `startTestPostgres` calls `t.Skip` when Docker is missing or unreachable (`backend/internal/catalog/store_test.go:30-33`). `go test ./...` then reports ok, and the break-check passes vacuously.
- **Fix**: Require that `go test ./internal/catalog -run 'TestDashboard' -v` shows PASS (not SKIP) for every dashboard test.
- **Decision**: FIXED

### F2 — The E2E seed spec depends on the dashboard markup

- **Severity**: ⚠️ WARNING
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Blind Spots
- **Location**: Phase 2 — Dashboard route
- **Detail**: `web/tests/e2e/seed.spec.ts:10-13` asserts an h1 "Dashboard" and zero links named "Catalog" for an architect on `/dashboard`. The plan neither names this constraint nor runs the E2E spec.
- **Fix**: State the constraint in the Phase 2 Contract and add `cd web && npx playwright test tests/e2e/seed.spec.ts` as an automated criterion.
- **Decision**: FIXED

### F3 — The handler's 500 path is untested

- **Severity**: 🔍 OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Blind Spots
- **Location**: Phase 1 — Tests
- **Detail**: A store error maps to 500 via `writeCatalogError`, but no test covers it. Existing list handlers don't test their 500 paths either.
- **Fix**: Leave as is, or add one test with a closed pool.
- **Decision**: FIXED
