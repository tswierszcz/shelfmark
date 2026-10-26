<!-- IMPL-REVIEW-REPORT -->
# Implementation Review: Access control and data-integrity regression tests

- **Plan**: context/changes/testing-access-control-data-integrity/plan.md
- **Scope**: Full plan
- **Reviewed phases**: 1, 2, 3, 4
- **Date**: 2026-10-26
- **Verdict**: NEEDS ATTENTION
- **Findings**: 0 critical, 3 warnings, 4 observations

## Verdicts

| Dimension | Verdict |
|-----------|---------|
| Plan Adherence | PASS |
| Scope Discipline | PASS |
| Safety & Quality | WARNING |
| Architecture | PASS |
| Pattern Consistency | WARNING |
| Success Criteria | PASS |

Success criteria re-run at 8195267: `go build ./... && go vet ./...` OK; Phase 1 named tests 2/2, Phase 2 5/5, Phase 3 4/4 PASS; `go test -race … ConcurrentMarksNeverExceedCap -count=3` OK; `go test ./...` OK; Phase 4: Admin row check OK, 3 cookbook patterns present. Manual 1.4 and 2.4 verified on a throwaway local instance (Architect upload 403, Admin projects 403, SPA 200; `"skipped":[]`); 1.5 and 4.3 confirmed by the user. Drift review: all 18 routes unchanged (paths, methods, roles, handlers, order); every planned item MATCH. Two test-plan edits outside the Phase 4 contract (§2 risk-guidance backport, §3 status) are the user-approved post-research backport and the orchestrator's status updates, not scope creep.

## Findings

### F1 — No lock_timeout on the strategic-cap advisory lock

- **Severity**: ⚠️ WARNING
- **Impact**: 🔎 MEDIUM — real tradeoff; pause to reason through it
- **Dimension**: Safety & Quality
- **Location**: backend/internal/catalog/store.go:391
- **Detail**: Waiters block on `pg_advisory_xact_lock` while holding a pool connection; with no `lock_timeout`/`statement_timeout`, a slow lock holder could tie up pool connections used by every route. Mitigated today: the transaction is short and the request ctx (cancelled on client disconnect) reaches pgx.
- **Fix A ⭐ Recommended**: `SET LOCAL lock_timeout = '5s'` in the transaction before taking the lock; map Postgres `lock_not_available` (55P03) to a new sentinel and HTTP 503 "please retry".
  - Strength: bounds the worst case per request; explicit, retryable error for the user.
  - Tradeoff: one more error path and a sentinel/handler mapping plus a test.
  - Confidence: HIGH — standard Postgres pattern.
  - Blind spot: pool size and real contention unknown; marks are rare human actions.
- **Fix B**: Accept the risk with a code comment and revisit if lock waits appear in logs.
  - Strength: no code change before production.
  - Tradeoff: worst case is unbounded until the client disconnects.
  - Confidence: MED — depends on traffic that is unknown pre-launch.
  - Blind spot: no metrics on lock waits yet.
- **Decision**: FIXED (Fix A) — SET LOCAL lock_timeout (strategicLockTimeout, default 5s) before the lock; 55P03 → ErrStrategicMarkBusy → HTTP 503 "strategic marking is busy, please retry"; TestMarkStrategic_LockTimeoutReturnsBusy + TestMarkStrategicHandler_BusyReturns503 (observed red before, break-check red→restored).

### F2 — Raw-JSON contract tests start one Postgres container per subtest

- **Severity**: ⚠️ WARNING
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Safety & Quality
- **Location**: backend/internal/catalog/json_contract_test.go
- **Detail**: Each `empty`/`uncategorized`/upload subtest calls `startTestPostgres`, adding ~6 container starts (tens of seconds) to every `go test ./...` run and to CI.
- **Fix**: Start one container per test function and reset state between subtests (TRUNCATE the library tables) instead of per subtest.
- **Decision**: FIXED — one startTestPostgres per test function, resetCatalogTables (TRUNCATE … CASCADE) between subtests; names and assertions unchanged.

### F3 — Session-cookie test helper duplicated across packages

- **Severity**: ⚠️ WARNING
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Pattern Consistency
- **Location**: backend/cmd/shelfmark/routes_test.go:147 (`mintSessionCookie`), backend/internal/catalog/handlers_test.go:446 (`sessionCookie`), auth test helpers
- **Detail**: Three copies; the catalog one returns `cookies[0]` without checking the name. `internal/obs/obstest` already set the pattern for shared test helpers.
- **Fix**: Add `internal/auth/authtest.SessionCookie(t, email, role)` (sets SESSION_SECRET, checks cookie name) and use it from catalog and cmd tests.
- **Decision**: FIXED — internal/auth/authtest.SessionCookie replaces mintSessionCookie (cmd) and sessionCookie (catalog).

### F4 — Interleaved cap test always waits the full 2 s seam timeout

- **Severity**: ℹ️ OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Safety & Quality
- **Location**: backend/internal/catalog/store_test.go (`TestMarkStrategic_InterleavedMarksRespectCap`)
- **Detail**: With the lock, the second transaction never reaches the seam, so the holder waits out the timeout. Deterministic, documented, catches regressions — only a fixed cost.
- **Fix**: None required; keep as designed.
- **Decision**: ACCEPTED — deterministic 2 s cost by design.

### F5 — Nullable allowlist matches field names anywhere in the document

- **Severity**: ℹ️ OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Safety & Quality
- **Location**: backend/internal/catalog/json_contract_test.go:20-52
- **Detail**: `github_repo: null` would be accepted in any nested object, not only library items.
- **Fix**: None now; acceptable while `github_repo` only exists on library items.
- **Decision**: ACCEPTED — github_repo only exists on library items today.

### F6 — afterCapCheck seam would race if MarkStrategic tests ever run in parallel

- **Severity**: ℹ️ OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Safety & Quality
- **Location**: backend/internal/catalog/store.go:368
- **Detail**: Package var mutated by one test; safe today (no `t.Parallel` in backend), a future `t.Parallel` would race.
- **Fix**: Add "do not use t.Parallel in tests that set afterCapCheck" to the var's comment.
- **Decision**: FIXED — comment on afterCapCheck and strategicLockTimeout (no t.Parallel).

### F7 — Ecosystem read before the lock assumes ecosystem is immutable

- **Severity**: ℹ️ OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Safety & Quality
- **Location**: backend/internal/catalog/store.go:381-396
- **Detail**: The lock key uses the ecosystem read before locking; correct while `libraries.ecosystem` is never updated (no UPDATE of it exists today).
- **Fix**: None now; note it in the comment above the lock.
- **Decision**: FIXED — comment above the lock (ecosystem must stay immutable).
