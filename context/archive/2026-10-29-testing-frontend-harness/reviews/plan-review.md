<!-- PLAN-REVIEW-REPORT -->
# Plan Review: Frontend Functional Test Harness

- **Plan**: context/changes/testing-frontend-harness/plan.md
- **Mode**: Deep (risky claims verified inline against `react-router` exports and `backend/hooktest/agent_hooks_test.go`)
- **Date**: 2026-10-29
- **Verdict**: REVISE → SOUND after fixes
- **Findings**: 0 critical, 3 warnings, 1 observation

## Verdicts

| Dimension | Verdict |
|-----------|---------|
| End-State Alignment | PASS |
| Lean Execution | PASS |
| Architectural Fitness | PASS |
| Blind Spots | WARNING |
| Plan Completeness | WARNING |

## Grounding
9/9 paths ✓, 3/3 symbols ✓ (`createRoutesStub` exported by react-router 8; `describeBatch`, `isBatchSettled` in `redundancy.tsx`), brief↔plan ✓; `docs/reference/contract-surfaces.md` absent (check skipped).

## Findings

### F1 — Hook test misdescribes the fixture: npx is a stub

- **Severity**: ⚠️ WARNING
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Plan Completeness
- **Location**: Phase 4 — #2 Hook test
- **Detail**: hooktest puts a fake `npx` on PATH (`agent_hooks_test.go:22-36`) whose default branch is `exit 0`, so `npx vitest run` would pass silently. The fixture also writes no `web/vitest.config.ts`, so the hook's gate would skip vitest.
- **Fix**: Extend `npxStub` with a `*vitest*` branch (`BROKEN_TEST` → FAIL, `STUB_VITEST=missing` → missing binary), write `web/vitest.config.ts` in `newFixture`, and assert the block and the skip message.
- **Decision**: FIXED

### F2 — Unmatched fetch would be swallowed by route catch blocks

- **Severity**: ⚠️ WARNING
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Blind Spots
- **Location**: Phase 1 — #3 Test support (stubFetch)
- **Detail**: If `stubFetch` throws on an unmatched URL, the route's own `catch` (e.g. `users.tsx:23`) turns it into an error state, so an error-state test can pass because of a stub typo.
- **Fix**: Record unmatched URLs and answer 599; `setup.ts` fails the test in `afterEach` when any were recorded.
- **Decision**: FIXED

### F3 — Loading row vs "keep rows during refetch" is ambiguous

- **Severity**: ⚠️ WARNING
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Plan Completeness
- **Location**: Phase 3 — #1 Loading / empty / error / data
- **Detail**: "Loading… row while loading" and "rows stay visible during refetch" contradict each other when loading is true on refetch.
- **Fix**: Loading row only when `loading && list.length === 0`; empty row only when `!loading && !error && list.length === 0`.
- **Decision**: FIXED

### F4 — Console noise check is manual; risk #5 §2 backport missing

- **Severity**: OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Blind Spots
- **Location**: Phase 1 Manual 1.5; Phase 4 #3
- **Detail**: The act()/unhandled-rejection check was manual, so it could regress silently. Research open question 3 (reword risk #5 evidence in test-plan §2) was not handled by any phase.
- **Fix**: `setup.ts` fails tests on `console.error` / `unhandledrejection` (new Automated 1.5; Manual renumbered 1.6); Phase 4 #3 backports the §2 risk #5 wording.
- **Decision**: FIXED
