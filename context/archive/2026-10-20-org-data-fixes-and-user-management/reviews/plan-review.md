<!-- PLAN-REVIEW-REPORT -->
# Plan Review: Org data fixes and user management

- **Plan**: context/changes/org-data-fixes-and-user-management/plan.md
- **Mode**: Deep
- **Date**: 2026-10-20
- **Verdict**: REVISE
- **Findings**: 1 critical, 1 warning, 0 observations

## Verdicts

| Dimension | Verdict |
|-----------|---------|
| End-State Alignment | PASS |
| Lean Execution | PASS |
| Architectural Fitness | PASS |
| Blind Spots | FAIL |
| Plan Completeness | WARNING |

## Grounding
12/12 paths ✓, 7/7 symbols ✓, brief↔plan ✓

## Findings

### F1 — Plan breaks 3 existing tests and never runs `go test`

- **Severity**: ❌ CRITICAL
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Blind Spots
- **Location**: Phase 2 — Changes Required §4 (LoginHandler); Success Criteria (all phases)
- **Detail**: `backend/internal/auth/handlers_test.go` calls `LoginHandler(accounts)` with a static `Accounts` map at 3 call sites (lines 38, 71, 85). Phase 2 changes `LoginHandler`'s signature to `LoginHandler(store *UsersStore)` but never mentions updating this test file — it won't compile afterward. Every phase's "Automated Verification" only runs `go build ./...` and `go vet ./...`, neither of which runs tests, so this breakage would go undetected by the plan's own gate.
- **Fix**: Add a 5th item to Phase 2 Changes Required: update the 3 `LoginHandler(accounts)` call sites in `handlers_test.go` to build a `*UsersStore` fixture instead (mirror how `catalog`'s `store_test.go` sets up a `*Store` against a test database). Add `go test ./...` to every phase's Automated Verification list and `## Progress`.
- **Decision**: FIXED — added Changes Required §8 (Phase 2) for the test fixtures, added `go test ./...` to Phases 1/2/4 Automated Verification and renumbered Progress accordingly.

### F2 — No test coverage added for the new FR-007 cap

- **Severity**: ⚠️ WARNING
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Plan Completeness
- **Location**: Phase 1 — Changes Required §4 (MarkStrategic cap)
- **Detail**: `catalog/store_test.go` already has `MarkStrategic` tests but none test the new 2-per-category/ecosystem cap — a regression here would only be caught by manual verification, never by `go test`.
- **Fix**: Add a Changes Required item to Phase 1: a new `store_test.go` case asserting a 3rd distinct library's `MarkStrategic` call returns `ErrStrategicCapReached` when 2 are already active for the same category+ecosystem, and that re-marking one of the existing two still succeeds.
- **Decision**: FIXED — added as Changes Required §6 in Phase 1 (bundled with F1's fix since both touch Phase 1's Automated Verification).
