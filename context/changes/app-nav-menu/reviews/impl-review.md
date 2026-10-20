<!-- IMPL-REVIEW-REPORT -->
# Implementation Review: Shared navigation menu

- **Plan**: context/changes/app-nav-menu/plan.md
- **Scope**: Phase 1 of 1 (Full plan)
- **Reviewed phases**: 1
- **Date**: 2026-10-20
- **Verdict**: APPROVED
- **Findings**: 0 critical, 1 warning, 0 observations

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

### F1 — logout() doesn't check res.ok, so a failed server logout still redirects as if it succeeded

- **Severity**: ⚠️ WARNING
- **Impact**: 🔎 MEDIUM — real tradeoff; pause to reason through it
- **Dimension**: Safety & Quality
- **Location**: web/app/lib/auth.ts:30-32 (logout), exposed by web/app/components/nav.tsx:19-25 (handleLogout)
- **Detail**: This plan added error handling to `handleLogout` (try/catch around `logout()`, showing "Log out failed." on exception) to close plan-review F2. But `auth.ts`'s `logout()` never checks `res.ok` — unlike `login()` and `me()` in the same file, which both throw on a non-ok response. A 500 from `/api/logout` resolves normally, so `handleLogout`'s try succeeds, `navigate("/login")` fires, and the UI shows "logged out" while the session cookie is still valid server-side. Not introduced by this change (`auth.ts` untouched by it), but surfaced by it — Nav is the first caller that visibly depends on `logout()` failing loudly.
- **Fix A ⭐ Recommended**: Make `logout()` throw on `!res.ok`, matching the existing `login`/`me` pattern in the same file.
  - Strength: One-line change; makes Nav's already-written catch block actually reachable; consistent with every other function in `auth.ts`.
  - Tradeoff: `auth.ts` is a shared file outside this plan's declared scope — touching it here is scope creep, even though the fix is trivial.
  - Confidence: HIGH — identical throw-on-!ok pattern already used twice in the same file.
  - Blind spot: Not verified whether any other caller of `logout()` relies on it never throwing.
- **Fix B**: Leave `auth.ts` unchanged; record as a follow-up.
  - Strength: Keeps this change's diff exactly scoped to the nav menu.
  - Tradeoff: The error-handling UI added for plan-review F2 stays dead code for the one case it was meant to catch.
  - Confidence: MEDIUM — low real-world odds of `/api/logout` returning non-2xx in this app's current form.
  - Blind spot: Session-hygiene impact (stale server session shown as "logged out") not measured.
- **Decision**: FIXED via Fix A — `logout()` now throws on non-ok response.
