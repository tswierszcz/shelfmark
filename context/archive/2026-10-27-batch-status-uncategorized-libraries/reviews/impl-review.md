<!-- IMPL-REVIEW-REPORT -->
# Implementation Review: Surface failed and uncategorized libraries

- **Plan**: context/changes/batch-status-uncategorized-libraries/plan.md
- **Scope**: Full plan
- **Reviewed phases**: 1, 2
- **Date**: 2026-10-27
- **Verdict**: APPROVED
- **Findings**: 0 critical, 1 warning, 3 observations

## Verdicts

| Dimension | Verdict |
|-----------|---------|
| Plan Adherence | PASS |
| Scope Discipline | WARNING |
| Safety & Quality | PASS |
| Architecture | PASS |
| Pattern Consistency | PASS |
| Success Criteria | PASS |

Success criteria re-run on 2026-10-27: `go build ./... && go vet ./...` OK; `go test ./internal/catalog/... ./cmd/shelfmark/...` OK; the new queue-driven test also passes under `-race`; `npm run typecheck` and `npm run build` OK; palette grep empty. The E2E suite (3 passed) and manual row 2.5 were verified during implementation (4b1fc0b).

## Findings

### F1 — Unplanned shared LibraryStatusBadge component

- **Severity**: ⚠️ WARNING
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Scope Discipline
- **Location**: web/app/components/library-status-badge.tsx
- **Detail**: Plan Phase 2 §3 put the status-to-badge mapping inline in `redundancy.tsx`. The implementation extracted it into a new shared component, also used by the kitchen sink. The mapping matches the plan exactly, and CLAUDE.md asks for shared components, but the plan does not mention the file.
- **Fix**: Record the component as a review addendum in the plan, so future reviews treat it as planned.
- **Decision**: FIXED — review addendum added to plan.md

### F2 — An incomplete E2E batch fails with an unhelpful timeout

- **Severity**: 💡 OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Success Criteria
- **Location**: web/tests/e2e/sbom-upload-report.spec.ts:72
- **Detail**: `^Batch .+ — done$` now matches only a fully categorized batch, as intended. If the batch ends incomplete, the test waits 30 s and reports "element not found" instead of the real line ("done: N need review"). The E2E categorizer is a deterministic local mock, so this is a diagnostics problem, not flakiness.
- **Fix**: Wait for the batch line to settle (not "processing…"), then assert its exact text, so a failure shows the actual status.
- **Decision**: FIXED — E2E waits for the settled batch line, then asserts "— done"; verified: with the deliberate break it fails in ~13 s with Received "Batch … — done: 3 need review"

### F3 — One failed poll ends polling for good

- **Severity**: 💡 OBSERVATION
- **Impact**: 🔎 MEDIUM — real tradeoff; pause to reason through it
- **Dimension**: Safety & Quality
- **Location**: web/app/routes/redundancy.tsx:156-166
- **Detail**: Any single failed `getBatchProgress` call stops polling and shows "— status unavailable", even though a later poll might succeed. This existed before; it is now visible instead of being hidden behind "done". Not a regression.
- **Fix**: Retry a few times (for example 3 consecutive failures) before settling on "status unavailable". Do it as a separate follow-up.
  - Strength: Avoids a false alarm on a single network blip.
  - Tradeoff: A little more polling state, and a new wording decision if retries should be visible.
  - Confidence: MED — no evidence yet of transient errors in practice.
  - Blind spot: Fly.io proxy error rates are unknown.
- **Decision**: SKIPPED — possible follow-up (retry before "status unavailable")

### F4 — Architects can read any batch's status counts

- **Severity**: 💡 OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Safety & Quality
- **Location**: backend/cmd/shelfmark/routes.go:51
- **Detail**: The widened route also exposes counts of admin catalog-upload batches. The response holds only `{status: count}`, and batch IDs carry 122 bits of crypto/rand (`handlers.go` `newBatchID`). This was an explicit planning decision, recorded in the plan under "What We're NOT Doing".
- **Fix**: None needed. If ownership checks are ever wanted, scope the check through `project_libraries` or the uploader.
- **Decision**: ACCEPTED — explicit planning decision

## Triage summary

- Fixed: F1, F2
- Skipped: F3
- Accepted: F4
