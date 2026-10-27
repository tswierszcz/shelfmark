<!-- PLAN-REVIEW-REPORT -->
# Plan Review: Access control and data-integrity regression tests

- **Plan**: context/changes/testing-access-control-data-integrity/plan.md
- **Mode**: Deep
- **Date**: 2026-10-26
- **Verdict**: REVISE → SOUND after triage
- **Findings**: 0 critical, 2 warnings, 2 observations

## Verdicts

| Dimension | Verdict |
|-----------|---------|
| End-State Alignment | PASS |
| Lean Execution | PASS |
| Architectural Fitness | PASS |
| Blind Spots | WARNING |
| Plan Completeness | WARNING |

## Grounding
6/6 paths ✓, 18/18 routes in main.go ✓, 11/11 catalog handler constructors return closures without touching deps ✓, no caller relies on ListLibraries returning nil ✓, advisory-lock key expression prepares and executes on postgres:16 ✓, brief↔plan ✓, Progress 15/15 ✓ (16 after triage)

## Findings

### F1 — Red-before-fix concurrency test may never go red

- **Severity**: ⚠️ WARNING
- **Impact**: 🔎 MEDIUM — real tradeoff; pause to reason through it
- **Dimension**: Blind Spots
- **Location**: Phase 3 §2
- **Detail**: The COUNT→INSERT window is one round trip; a goroutine-only test is probabilistic, so an autonomous implementer may stall or ship a lock without proof the test catches the race.
- **Fix A ⭐ Recommended**: unexported `afterCapCheck func()` seam (nil in prod) between check and insert; deterministic interleaving test with a 2 s seam timeout; keep the goroutine load test.
  - Strength: deterministic red before the lock, green after.
  - Tradeoff: one-line test seam in production code.
  - Confidence: HIGH — standard seam technique; timeout avoids lock/seam deadlock.
  - Blind spot: timeout must exceed lock wait on slow hosts.
- **Fix B**: goroutines only; implementer must demonstrate red or stop.
  - Strength: no seam.
  - Tradeoff: may never be red on a fast host.
  - Confidence: MED.
  - Blind spot: host timing.
- **Decision**: FIXED (Fix A) — Phase 3 §2 contract, criteria (4 tests), Critical Implementation Details and brief updated.

### F2 — expectedRoles oracle can degrade into a copy of main.go

- **Severity**: ⚠️ WARNING
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Blind Spots
- **Location**: Phase 1 §2
- **Detail**: PRD has no per-route roles, so the table is written from current code; it locks regressions but does not prove correctness.
- **Fix**: State it as a role regression lock and add Manual 1.5 (user reviews the table against the PRD).
- **Decision**: FIXED — intent sentence, Manual 1.5 and Progress row added.

### F3 — Criterion 4.1 is not a pass/fail command

- **Severity**: ℹ️ OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Plan Completeness
- **Location**: Phase 4 Success Criteria
- **Detail**: `grep -c` returns a count, not a verdict.
- **Fix**: `! grep -nE '^\| Admin \|.*project SBOMs' context/foundation/prd.md`.
- **Decision**: FIXED.

### F4 — route.Handler type cannot hold the SPA handler

- **Severity**: ℹ️ OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Architectural Fitness
- **Location**: Phase 1 §1
- **Detail**: `spaHandler` returns `http.Handler` (`cmd/shelfmark/main.go:144`), plan typed the field `http.HandlerFunc`.
- **Fix**: field `http.Handler`; protected routes wrapped as `auth.RequireRole(...)(h.ServeHTTP)`.
- **Decision**: FIXED.
