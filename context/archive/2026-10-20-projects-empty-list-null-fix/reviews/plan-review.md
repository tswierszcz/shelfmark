<!-- PLAN-REVIEW-REPORT -->
# Plan Review: Fix nil-slice → JSON `null` on list endpoints

- **Plan**: context/changes/projects-empty-list-null-fix/plan.md
- **Mode**: Deep
- **Date**: 2026-10-20
- **Verdict**: REVISE
- **Findings**: 0 critical, 2 warnings, 0 observations

## Verdicts

| Dimension | Verdict |
|-----------|---------|
| End-State Alignment | PASS |
| Lean Execution | PASS |
| Architectural Fitness | PASS |
| Blind Spots | WARNING |
| Plan Completeness | WARNING |

## Grounding

9/9 paths ✓, 7/7 symbols ✓, brief↔plan ✓

## Findings

### F1 — ProjectReport zero-row test fixture doesn't exist as described

- **Severity**: ⚠️ WARNING
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Plan Completeness
- **Location**: Phase 1, item 4 (Zero-row regression tests)
- **Detail**: The plan says the ProjectReport test can "upload a project, then query its report before attaching anything." But every existing fixture in `project_handlers_test.go` (`doProjectUpload`, `setUpProjectReportFixture`) uploads an SBOM that always contains libraries — there's no zero-library upload path to reuse.
- **Fix**: Name the exact helper to use: `store.FindOrCreateProject(ctx, "Empty Project", "test@example.com")` (`project.go:49`) creates a project row directly with no attached libraries — call `store.ProjectReport(ctx, project.ID)` on it without ever calling `ReplaceProjectLibraries`.
- **Decision**: FIXED (Phase 1 item 4 now names `store.FindOrCreateProject` as the fixture helper)

### F2 — Missed caller of ListTaxonomy outside the HTTP handler

- **Severity**: ⚠️ WARNING
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Blind Spots
- **Location**: Phase 1, item 2 (ListTaxonomy nil-guard)
- **Detail**: `ProcessLibrary` (`pipeline.go:33-82`) also calls `store.ListTaxonomy` (`pipeline.go:54`) and ranges over the result — not just `ListTaxonomyHandler` as the plan states. No functional risk (ranging over nil vs. `[]T{}` is a no-op difference in Go), but worth naming for completeness.
- **Fix**: Add one sentence to Phase 1 item 2's Intent noting `ProcessLibrary` (`pipeline.go:54`) also calls `ListTaxonomy` and is unaffected by the change.
- **Decision**: FIXED (note added to Phase 1 item 2's Intent)
