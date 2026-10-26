<!-- PLAN-REVIEW-REPORT -->
# Plan Review: Observability logging contract and queue supervision

- **Plan**: context/changes/observability-logging-queue-supervision/plan.md
- **Mode**: Deep
- **Date**: 2026-10-26
- **Verdict**: REVISE → SOUND after triage
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
7/7 paths ✓, 4/4 symbols ✓ (writeCatalogError, writeUserError, VerifySession hmac branch session.go:108, pgxpool.Close), brief↔plan ✓, Progress 17/17 ✓

## Findings

### F1 — In-flight attempts can outlive the 25 s shutdown budget

- **Severity**: ⚠️ WARNING
- **Impact**: 🔎 MEDIUM — real tradeoff; pause to reason through it
- **Dimension**: Blind Spots
- **Location**: Phase 3 §2 Shutdown, §4 Lifecycle wiring
- **Detail**: Attempts use a context from `context.Background()` with a 30 s timeout (queue.go:84), longer than the 25 s budget; `pgxpool.Close()` waits for acquired connections, so shutdown can hang until SIGKILL.
- **Fix A ⭐ Recommended**: Derive attempt ctx from a queue-owned cancellable ctx; cancel it when the shutdown budget expires, wait ≤ 2 s more.
  - Strength: Shutdown always bounded; interrupted library stays for the sweep.
  - Tradeoff: Interrupted attempt logs a `context canceled` warn.
  - Confidence: HIGH — standard Go shutdown pattern.
  - Blind spot: None significant.
- **Fix B**: Lower `processAttemptTimeout` to 20 s.
  - Strength: One line.
  - Tradeoff: More failures on slow OpenRouter calls in normal operation.
  - Confidence: MED.
  - Blind spot: Real OpenRouter latency distribution unknown.
- **Decision**: FIXED (Fix A) — Phase 3 §2 contract and shutdown test updated; brief decision row updated.

### F2 — Global slog capture vs workers leaked from earlier tests

- **Severity**: ⚠️ WARNING
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Blind Spots
- **Location**: Testing (all phases)
- **Detail**: Existing queue tests never stop workers; after this change they log into whatever default logger a later test installs — cross-test noise and a data race on `bytes.Buffer`.
- **Fix**: `captureLogs(t)` helper with mutex-guarded writer restored in Cleanup; assertions filter by own ids; new queue tests call `Shutdown` in Cleanup; run with `-race`.
- **Decision**: FIXED — "Log capture in tests" added to Testing Strategy.

### F3 — Criterion 2.2 could pass with no tests run

- **Severity**: ⚠️ WARNING
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Plan Completeness
- **Location**: Phase 2 §5 Tests, Success Criteria
- **Detail**: `-run 'Logs|5xx'` with unnamed tests passes vacuously ("no tests to run").
- **Fix**: Name Phase 2 tests and use exact names with `-v` in the criterion.
- **Decision**: FIXED — 5 tests named; criterion 2.2 uses exact names.

### F4 — Unvalidated Fly-Request-Id reused in logs

- **Severity**: ℹ️ OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Plan Completeness
- **Location**: Phase 1 §1 Observability package
- **Detail**: Client-controlled header copied into every log line without length/charset bounds.
- **Fix**: Reuse only when it matches `^[A-Za-z0-9-]{1,64}$`.
- **Decision**: FIXED — Middleware contract updated.
