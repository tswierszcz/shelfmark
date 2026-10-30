<!-- IMPL-REVIEW-REPORT -->
# Implementation Review: Frontend Functional Test Harness

- **Plan**: context/changes/testing-frontend-harness/plan.md
- **Scope**: Full plan
- **Reviewed phases**: 1, 2, 3, 4
- **Date**: 2026-10-29
- **Verdict**: NEEDS ATTENTION
- **Findings**: 0 critical, 3 warnings, 5 observations
- **Triage**: Fixed F1, F2, F4, F5, F7, F8; Skipped F3, F6

## Verdicts

| Dimension | Verdict |
|-----------|---------|
| Plan Adherence | PASS |
| Scope Discipline | PASS |
| Safety & Quality | WARNING |
| Architecture | PASS |
| Pattern Consistency | PASS |
| Success Criteria | PASS |

Success criteria at HEAD 9c2e07c: `npm test` 57/57, `npm run typecheck`, `npm run build`, duplication grep and TBD grep pass. E2E (4/4) and `go test ./hooktest/...` passed at the phase 3/4 commits; code is unchanged since 59fc885. All 23 Progress rows carry a SHA; manual rows 2.5–4.4 were exercised in the browser / against the real hook during implementation.

Plan drift agent: 0 MISSING, 0 EXTRA (auth.ts `.catch` is the approved adaptation). Minor drifts folded into F7/F8.

## Findings

### F1 — /catalog poll loop has no token or timer reset

- **Severity**: ⚠️ WARNING
- **Impact**: 🔎 MEDIUM — real tradeoff; pause to reason through it
- **Dimension**: Safety & Quality
- **Location**: web/app/routes/catalog.tsx:91-121
- **Detail**: Unlike `/redundancy` (`pollToken` + clearing `pollTimer`, redundancy.tsx:101-140), `/catalog`'s `startPolling` neither invalidates a previous chain nor clears its timer. Upload is disabled only during the request, so a second upload while batch A still polls runs two chains; A's late answers overwrite `batchProgress`/`pollOutcome` under B's id, and the new `pollOutcome` makes a wrong "done" label more likely. Pre-existing, but phase 2 only moved the pure helpers, so the two poll loops are still duplicated and have already diverged.
- **Fix A ⭐ Recommended**: Copy `/redundancy`'s token-and-clear pattern into `/catalog` `startPolling`
  - Strength: Small, local; reuses a pattern already proven in this repo.
  - Tradeoff: Two poll loops stay duplicated.
  - Confidence: HIGH — identical code exists in redundancy.tsx.
  - Blind spot: No test drives two overlapping uploads yet.
- **Fix B**: Extract a shared `useBatchPoll` hook into `web/app/lib/batch.ts` used by both routes
  - Strength: Removes the duplication that caused this drift class.
  - Tradeoff: Touches `/redundancy` again; larger diff.
  - Confidence: MED — needs care with the routes' different onSettled callbacks.
  - Blind spot: Hook-level tests would need fake timers.
- **Decision**: FIXED (Fix A) — token-and-clear in /catalog startPolling; test "a late answer for an earlier batch does not overwrite the current one" (stubFetch gained `after`)

### F2 — /redundancy report can show the wrong project's rows

- **Severity**: ⚠️ WARNING
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Safety & Quality
- **Location**: web/app/routes/redundancy.tsx:67-86
- **Detail**: `refreshReport` has no stale-response guard. Picking p-1 then p-2 quickly lets a slow p-1 response land last and show its rows (or error) under p-2; the first response's `finally` also clears `reportLoading` while p-2 is still in flight, so the empty row can flash. Phase 3 introduced the clear-and-load sequence; tests resolve instantly and cannot hit it.
- **Fix**: Track the latest requested project id in a ref and ignore responses (and the `finally`) for any other id.
- **Decision**: FIXED — reportToken in refreshReport ignores stale answers and their finally; test "a slow answer for an earlier project does not land under the current one"

### F3 — In-flight promises can leak into the next test

- **Severity**: ⚠️ WARNING
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Safety & Quality
- **Location**: web/app/test/setup.ts:24-35
- **Detail**: `cleanup()` unmounts, but a chain already in flight (e.g. `/catalog` `poll()` → `refreshLibraries()`) keeps running; if it fetches after the next test's `stubFetch`, it is recorded as unmatched and fails an unrelated test (a flake, not seen in runs so far).
- **Fix**: In the affected route tests, wait for the final settled state (e.g. the refreshed rows) before the test ends; F1/F2's guards also stop post-unmount updates.
- **Decision**: SKIPPED — F1/F2 guards removed the realistic leak source; revisit if a flake appears

### F4 — Hook skips vitest silently when tsc fails

- **Severity**: OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Safety & Quality
- **Location**: .claude/hooks/end-of-turn.sh:112-124
- **Detail**: When tsc fails, tests are not run and nothing says so, contrary to the header's "never silent". Also `tail -n 80` can cut the first assertion diff when many tests fail, and any `web/**/*.json` change (incl. package-lock) triggers the full suite.
- **Fix**: Append "web tests (not run: typecheck failed)" to the report when tsc blocks; use `--reporter=dot` so failures fit in 80 lines.
- **Decision**: FIXED — hook reports "Web tests not run (typecheck failed)" and runs `vitest run --reporter=dot`

### F5 — allowConsoleError is a blanket switch

- **Severity**: OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Safety & Quality
- **Location**: web/app/test/setup.ts:11-13
- **Detail**: Once called, every console.error in that test is allowed, so an unrelated act() warning in the `/api/me` 500 characterization would be hidden.
- **Fix**: Take a matcher (`allowConsoleError(/Failed to load the current user/)`) and still fail on non-matching errors.
- **Decision**: FIXED — allowConsoleError(matcher: RegExp); non-matching console.error still fails; caller updated

### F6 — Report table flashes "Loading…" on same-project refresh

- **Severity**: OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Safety & Quality
- **Location**: web/app/routes/redundancy.tsx:73-74
- **Detail**: After an upload settles, `refreshReport` blanks and reloads the same project's table. Correct, but a visible flash.
- **Fix**: Clear rows only when the project id changes.
- **Decision**: SKIPPED — correct behavior, cosmetic flash only

### F7 — Hook test lacks "tsc fails → vitest not run"; fake timers unused

- **Severity**: OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Success Criteria
- **Location**: backend/hooktest/agent_hooks_test.go:263-278; web/app/routes/catalog.test.tsx
- **Detail**: The ordering rule (vitest only after clean tsc) is not locked by a case. The plan asked for fake timers in route polling tests; none use them — harmless, both settle on the first poll (test-plan §6.4 already words it that way).
- **Fix**: Add a hooktest case with a BROKEN app file and a FAILING_TEST file expecting "Web typecheck fails" and no "Web tests fail".
- **Decision**: FIXED — hooktest subtest "type error stops web tests and says so" (break-checked against the previous hook)

### F8 — test-plan cosmetic drifts

- **Severity**: OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Plan Adherence
- **Location**: context/foundation/test-plan.md §5, §6.5
- **Detail**: §5 "post-edit hook" row still lists end-of-turn web checks as tsc only; the §6.5 Phase 4 note sits above Phase 2; the §4 row was renamed "unit/component (frontend)".
- **Fix**: Add vitest to the §5 hook row and move the Phase 4 note to the end of §6.5.
- **Decision**: FIXED — §5 hook row lists vitest, §6.5 Phase 4 note moved last, §6.4 allowConsoleError(/pattern/)
