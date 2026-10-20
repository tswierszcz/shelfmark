<!-- IMPL-REVIEW-REPORT -->
# Implementation Review: Org data fixes and user management

- **Plan**: context/changes/org-data-fixes-and-user-management/plan.md
- **Scope**: Full plan
- **Reviewed phases**: 1, 2, 3, 4
- **Date**: 2026-10-20
- **Verdict**: APPROVED
- **Findings**: 0 critical, 2 warnings, 1 observation

## Verdicts

| Dimension | Verdict |
|-----------|---------|
| Plan Adherence | PASS |
| Scope Discipline | PASS |
| Safety & Quality | WARNING |
| Architecture | PASS |
| Pattern Consistency | PASS |
| Success Criteria | PASS |

## Findings

### F1 — Unbounded request body feeds bcrypt on /api/login and /api/users

- **Severity**: ⚠️ WARNING
- **Impact**: 🔎 MEDIUM — real tradeoff; pause to reason through it
- **Dimension**: Safety & Quality
- **Location**: backend/internal/auth/handlers.go:39-72 (LoginHandler), :111-124 (CreateUserHandler)
- **Detail**: Neither handler caps request body size (no `http.MaxBytesReader`) before decoding JSON and passing the password into `bcrypt.CompareHashAndPassword` / `bcrypt.GenerateFromPassword`. `LoginHandler` is fully unauthenticated — an oversized password string is a cheap CPU-exhaustion vector given bcrypt's cost amplification. Pre-existing on `/api/login`; this plan added a second handler (`CreateUserHandler`) with the same gap.
- **Fix**: Wrap `r.Body` with `http.MaxBytesReader(w, r.Body, 10<<10)` at the top of both handlers before decoding.
  - Strength: One line per handler, no behavior change for legitimate requests.
  - Tradeoff: None meaningful at this payload size.
  - Confidence: HIGH — standard Go idiom for this exact risk class.
  - Blind spot: Doesn't address a distributed flood of small requests (rate limiting is explicitly out of scope per the plan).
- **Decision**: FIXED — `http.MaxBytesReader(w, r.Body, 10<<10)` added to both `LoginHandler` and `CreateUserHandler`.

### F2 — SeedFromEnvIfEmpty can silently half-seed on a crash

- **Severity**: ⚠️ WARNING
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Safety & Quality
- **Location**: backend/internal/auth/users_store.go:111-132
- **Detail**: The 3 `Create` calls in the seed loop aren't transactional. If the process is killed after inserting account 1 of 3, the next restart's `Count(ctx)` returns 1 (`>0`) and seeding is skipped forever — two roles silently have no login, with no error surfaced. Low likelihood (only matters during the single first-boot window) but a real, silent data-safety gap.
- **Fix**: Wrap the 3 `Create` calls in one `s.pool.Begin`/`Commit` transaction, mirroring the pattern `MarkStrategic` already established in `catalog/store.go`.
- **Decision**: FIXED — `SeedFromEnvIfEmpty` now inserts all 3 accounts inside one transaction with `defer tx.Rollback(ctx)` and an explicit `tx.Commit(ctx)`.

### F3 — PRD role table wording not updated alongside the new sentence

- **Severity**: ℹ️ OBSERVATION
- **Impact**: 🏃 LOW
- **Dimension**: Plan Completeness
- **Location**: context/foundation/prd.md — Access Control section
- **Detail**: The plan's required sentence was added correctly, but the pre-existing role table still reads "Admin | ... (via configuration)" a few lines above it — now slightly inconsistent with the new sentence below. Cosmetic only.
- **Fix**: Not required by this plan's scope; a one-word tidy-up whenever the PRD is next touched.
- **Decision**: SKIPPED
