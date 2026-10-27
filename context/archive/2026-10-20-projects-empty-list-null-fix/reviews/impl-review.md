<!-- IMPL-REVIEW-REPORT -->
# Implementation Review: Fix nil-slice → JSON `null` on list endpoints

- **Plan**: context/changes/projects-empty-list-null-fix/plan.md
- **Scope**: Full plan (Phase 1 of 2, Phase 2 of 2)
- **Reviewed phases**: 1, 2
- **Date**: 2026-10-21
- **Verdict**: APPROVED
- **Findings**: 0 critical, 0 warnings, 1 observation

## Verdicts

| Dimension | Verdict |
|-----------|---------|
| Plan Adherence | PASS |
| Scope Discipline | PASS |
| Safety & Quality | PASS |
| Architecture | PASS |
| Pattern Consistency | PASS |
| Success Criteria | PASS |

## Findings

### F1 — `refreshProjects()` return value not guarded like its own state

- **Severity**: OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Pattern Consistency / Reliability
- **Location**: web/app/routes/redundancy.tsx:57-58
- **Detail**: `refreshProjects()` does `setProjects(result ?? [])` on line 57 but then `return result` on line 58 — the raw, possibly-null value — while its own `catch` branch returns `[]`. The function guards its own state but not its return value, which is an inconsistent contract. Currently harmless: neither call site (`useEffect` at line 80, upload-flow `await refreshProjects()` at line 145) consumes the returned value. Latent footgun if a future caller does `(await refreshProjects()).length` or `.map()`.
- **Fix**: Change `return result;` to `return result ?? [];` for consistency with the rest of the fix's intent.
- **Decision**: FIXED — `return result;` → `return result ?? [];` (web/app/routes/redundancy.tsx:58)
