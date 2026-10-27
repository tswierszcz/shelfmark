<!-- PLAN-REVIEW-REPORT -->
# Plan Review: Production Deploy (Fly.io)

- **Plan**: context/changes/prod-deploy/plan.md
- **Mode**: Deep
- **Date**: 2026-10-27
- **Verdict**: REVISE → SOUND after triage (all 5 fixed)
- **Findings**: 1 critical, 3 warnings, 1 observation

## Verdicts

| Dimension | Verdict |
|-----------|---------|
| End-State Alignment | PASS |
| Lean Execution | PASS |
| Architectural Fitness | PASS |
| Blind Spots | WARNING |
| Plan Completeness | FAIL |

## Grounding
Grounding: 7/7 paths ✓, 3/3 symbols ✓ (ClearSession, SeedFromEnvIfEmpty, pgx5DatabaseURL), brief↔plan ✓. Riskiest claims verified inline against the code and the pgx source (no sub-agent needed).

## Findings

### F1 — PR to main cannot be opened before histories are joined

- **Severity**: ❌ CRITICAL
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Plan Completeness
- **Location**: Phase 3 (criterion 3.3) vs Phase 5 §1
- **Detail**: Criterion 3.3 expects a PR to `main` with green checks, but the `--allow-unrelated-histories` merge happens only in Phase 5. GitHub won't compare branches with entirely different histories. The branch was rewritten by filter-repo: `backup/shape-shelfmark-pre-filter-*` has root 4459e3c (= main), while the current branch has no merge-base with main.
- **Fix**: Move the unrelated-histories merge + README.txt resolution to the start of Phase 3 (before push/PR). Phase 5 keeps only merging the PR after the token is set.
- **Decision**: FIXED — applied in plan

### F2 — pgx DSN redaction is explicitly best-effort

- **Severity**: ⚠️ WARNING
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Blind Spots
- **Location**: Phase 1 §2 (Connect error check)
- **Detail**: The plan changes `Connect` only if a test proves a leak. The pgx docs (`pgconn/config.go:332-336`) say redaction "is best effort" and that apps should not assume parse errors are safe to expose. One green test doesn't cover other malformed DSNs.
- **Fix**: Split `Connect` unconditionally into `pgxpool.ParseConfig` (fixed, unwrapped `db: failed to parse DATABASE_URL`) and `NewWithConfig`. Keep `TestConnect_ErrorDoesNotLeakPassword`.
- **Decision**: FIXED — applied in plan

### F3 — Logout cookie site has no *http.Request

- **Severity**: ⚠️ WARNING
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Plan Completeness
- **Location**: Phase 1 §4, §6
- **Detail**: The contract applies `requestIsHTTPS` at `session.go:95` and `:156`, but `ClearSession(w http.ResponseWriter)` (`session.go:150`) receives no request. Its only caller is `LogoutHandler` (`handlers.go:80`).
- **Fix**: Change the signature to `ClearSession(w, r)`, update the one caller, and have the test cover both cookies.
- **Decision**: FIXED — applied in plan

### F4 — Seed-failure test method can't trigger the insert path

- **Severity**: ⚠️ WARNING
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Plan Completeness
- **Location**: Phase 1 §6 (TestSeed_ErrorOmitsEmail)
- **Detail**: A pre-inserted email makes `count > 0`, so `SeedFromEnvIfEmpty` returns early (`users_store.go:113`). A closed pool fails in `Count` before any insert. Neither approach reaches line 136.
- **Fix**: In the test DB, add a CHECK constraint that rejects a sentinel email, set `ADMIN_EMAIL` to that sentinel, and assert the error names `admin` and lacks the sentinel.
- **Decision**: FIXED — applied in plan

### F5 — Image-contents check (2.5) is nearly vacuous

- **Severity**: ℹ️ OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Plan Completeness
- **Location**: Phase 2 §2, criterion 2.5
- **Detail**: The final distroless stage copies only `/app/shelfmark` and `/app/public`, so the image can't contain `.env`, `context/` or `.git` whatever `.dockerignore` says. The real exposure is the build context uploaded to Fly's remote builder. `.dockerignore` already covers `.git`, `.claude`, `context` and `.env*`, but not `test.csv`.
- **Fix**: Keep the 2.5 title, check the build context instead (a throwaway `COPY . /ctx` listing), and add `test.csv` to `.dockerignore`.
- **Decision**: FIXED — applied in plan
