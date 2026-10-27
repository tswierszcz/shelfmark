<!-- IMPL-REVIEW-REPORT -->
# Implementation Review: Settle an empty upload batch

- **Plan**: context/changes/empty-batch-status/plan.md
- **Scope**: Full plan
- **Reviewed phases**: 1
- **Date**: 2026-10-27
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

Review was done inline (5 changed files, about 80 lines of code), without parallel review agents. All planned items match: the `isBatchSettled` empty case, the new `describeBatch` case in the normal tone, the kitchen-sink line, the Go contract test and the E2E re-upload step. Nothing outside the plan's scope changed. Success criteria: typecheck and build re-run OK. The catalog Go suite, the E2E suite (3 passed) and the break-check (red, stuck on "processing…") ran on this same commit, 23ecc11. Row 1.5 (production re-upload) is pending until deploy, as planned.

## Findings

### F1 — An SBOM with no usable components also reads "no new libraries"

- **Severity**: 💡 OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Safety & Quality
- **Location**: web/app/routes/redundancy.tsx (describeBatch) · backend/internal/catalog/project_sbom.go
- **Detail**: A valid CycloneDX BOM whose components are all skipped (no or invalid purl) parses without error, so its batch is empty too and shows "— done: no new libraries to categorize". The statement is true, but the upload response's `skipped` rows are never shown on /redundancy, so the user isn't told why. This gap existed before the change; it previously showed "still processing" after 60 s.
- **Fix**: As a follow-up, show the upload response's skipped count and reasons under the batch line.
- **Decision**: SKIPPED — possible follow-up (show skipped rows on /redundancy)

### F2 — The E2E smoke test now covers two behaviours

- **Severity**: 💡 OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Pattern Consistency
- **Location**: web/tests/e2e/sbom-upload-report.spec.ts
- **Detail**: The re-upload step is appended to the existing smoke test instead of a separate test, because `fullyParallel: true` and the file-level token and clean-up would make two tests collide. A failure in the first upload therefore hides the re-upload check. The test header documents both behaviours.
- **Fix**: None needed now; split it if the file gets per-test tokens.
- **Decision**: ACCEPTED — documented in the test header

## Triage summary

- Skipped: F1
- Accepted: F2
