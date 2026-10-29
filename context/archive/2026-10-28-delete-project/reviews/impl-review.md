<!-- IMPL-REVIEW-REPORT -->
# Implementation Review: Usuwanie projektu

- **Plan**: `context/changes/delete-project/plan.md`
- **Scope**: Full plan
- **Reviewed phases**: 1, 2
- **Date**: 2026-10-28
- **Verdict**: NEEDS ATTENTION
- **Findings**: 0 critical, 1 warning, 1 observation

## Verdicts

| Dimension | Verdict |
|-----------|---------|
| Plan Adherence | WARNING |
| Scope Discipline | PASS |
| Safety & Quality | WARNING |
| Architecture | PASS |
| Pattern Consistency | PASS |
| Success Criteria | WARNING |

## Verification

- `cd backend && go vet ./...` — PASS (combined command advanced to `go test`).
- `cd backend && go test ./... -count=1` — INCOMPLETE. Non-integration packages passed, but catalog integration tests did not finish. Docker access still fails with `permission denied` on `/var/run/docker.sock`; the run was interrupted after waiting without progress.
- `cd web && npm run typecheck && npm run build` — PASS.
- `cd web && npx playwright test` — PASS, 4 tests passed, including `project-delete.spec.ts`.
- `git diff origin/shape/shelfmark...HEAD --check` — PASS.
- Manual checks 1.4 and 2.4–2.6 are recorded as completed by the user.
- Post-triage verification of F1 and F2 was rerun by the user; pass/fail details were not provided in this chat.

## Findings

### F1 — In-flight deletion can clear a different project selection

- **Severity**: ⚠️ WARNING
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Safety & Quality
- **Location**: `web/app/routes/redundancy.tsx:217-221`
- **Detail**: The delete request captures the selected project implicitly, then its success handler unconditionally clears `selectedProjectId` and `report`. The project picker remains enabled while `deleting` is true. If the user selects another project before the request resolves, the response for the old deletion clears the new selection and report. Cancel also only resets `confirmDelete` (`redundancy.tsx:338-340`), so an earlier delete error remains visible after cancellation.
- **Fix**: Capture the ID being deleted and, after success, clear selection/report only if the current selection still equals that ID; clear `deleteError` when Cancel is clicked.
  - Strength: Preserves a selection made while the request is in flight and returns Cancel to the initial state.
  - Tradeoff: Small state-guard change in one route.
  - Confidence: HIGH — follows directly from the current async handler and state updates.
  - Blind spot: No browser test currently exercises changing selection during a delayed delete response.
- **Decision**: FIXED — disabled the project picker and Cancel while deletion is in flight; Cancel clears the prior delete error.

### F2 — Deletion test omits the planned direct-status regression assertion

- **Severity**: ℹ️ OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Plan Adherence
- **Location**: `backend/internal/catalog/project_handlers_test.go:563-643`
- **Detail**: The plan requires proving that a library direct only in deleted project A is no longer direct in the organization library view. The integration fixture deletes project A but only checks the shared library, which remains direct through project B; there is no assertion for a library whose only direct membership was in A.
- **Fix**: Assert that `ListLibraries` reports the direct-only-A library as not direct after deletion.
- **Decision**: FIXED — added an assertion that Jackson Databind remains in the catalog but is no longer direct after deleting its only project membership.

