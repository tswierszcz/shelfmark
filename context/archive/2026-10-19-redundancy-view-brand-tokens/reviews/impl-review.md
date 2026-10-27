<!-- IMPL-REVIEW-REPORT -->
# Implementation Review: Shelfmark brand design tokens — redundancy report view

- **Plan**: context/changes/redundancy-view-brand-tokens/plan.md
- **Scope**: Full plan
- **Reviewed phases**: 1, 2, 3, 4
- **Date**: 2026-10-20
- **Verdict**: NEEDS ATTENTION
- **Findings**: 0 critical, 2 warnings, 2 observations

## Verdicts

| Dimension | Verdict |
|-----------|---------|
| Plan Adherence | PASS |
| Scope Discipline | WARNING |
| Safety & Quality | WARNING |
| Architecture | PASS |
| Pattern Consistency | WARNING |
| Success Criteria | PASS |

## Findings

### F1 — Kitchen-sink dev route ships unguarded to production

- **Severity**: ⚠️ WARNING
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Safety & Quality
- **Location**: web/app/routes.ts:11, web/app/routes/dev.kitchen-sink-redundancy.tsx
- **Detail**: `/dev/kitchen-sink-redundancy` has no `RequireAuth` wrapper (correct per plan — it needs to be reachable without auth for review) and renders only static fixture strings, so no real org/project data leaks. But it's also unconditionally registered with no `import.meta.env.DEV` guard, unlike `root.tsx`'s `ErrorBoundary`, which already gates dev-only output the same way. The route and its internal design-system scaffolding ship and are publicly reachable in production builds.
- **Fix**: Register the route conditionally (`...(import.meta.env.DEV ? [route("dev/kitchen-sink-redundancy", "routes/dev.kitchen-sink-redundancy.tsx")] : [])`) or strip it at build time, matching the existing dev-gating idiom in `root.tsx`.
- **Decision**: FIXED (applied the recommended fix in web/app/routes.ts; verified production build no longer emits the kitchen-sink chunk; typecheck/build pass)

### F2 — Logo SVG hardcodes hex fills instead of referencing theme tokens

- **Severity**: ⚠️ WARNING
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Pattern Consistency
- **Location**: web/app/components/logo.tsx:5-9
- **Detail**: The SVG rects use literal `#c9a227` / `#ece7df` fills that duplicate `--color-accent` and `--color-ink` from `app.css`'s `@theme` block, rather than referencing the CSS variables. CLAUDE.md's "no literal hex" rule technically only binds route files, so this isn't a rule violation per the letter, but it undercuts the point of this change: if `--color-accent`/`--color-ink` are ever rebranded, the logo mark silently drifts out of sync with the rest of the UI.
- **Fix**: Change the SVG fills to `fill="var(--color-accent)"` / `fill="var(--color-ink)"`.
- **Decision**: FIXED (applied in web/app/components/logo.tsx; typecheck/build pass)

### F3 — Logo SVG size drift (40×40 vs. planned 28×28)

- **Severity**: 👁 OBSERVATION
- **Impact**: 🏃 LOW — cosmetic, no functional impact
- **Dimension**: Plan Adherence
- **Location**: web/app/components/logo.tsx
- **Detail**: Plan's contract specified `width=28 height=28`; implementation uses `width="40" height="40"`. Purely cosmetic sizing difference, everything else about the component (markup, fills, opacities, wrapper, wordmark) matches the plan exactly.
- **Decision**: SKIPPED

### F4 — Unplanned .gitignore change

- **Severity**: 👁 OBSERVATION
- **Impact**: 🏃 LOW — self-explanatory, harmless
- **Dimension**: Scope Discipline
- **Location**: .gitignore
- **Detail**: Commit 6fadde2 (inside the reviewed range) changed the ignored path from `.claude/settings.local.json` to `.claude/` ("stop tracking .claude/ (course skill downloads, not project source)"). Unrelated to the brand-tokens plan content, but low-risk, well-explained, and touches no plan-governed file.
- **Decision**: SKIPPED

## Notes

- Success criteria: `npm run typecheck` and `npm run build` both pass (re-verified live in this review). Hardcoded-value scan against `web/app/routes/redundancy.tsx` at commit 605e8f3 returns `0`, as the plan requires.
- Progress item 4.5 (desktop/mobile screenshots) is unchecked with the note "skipped by user decision — live review in this session substituted" — accepted, not a finding.
- "What We're NOT Doing" boundaries respected: this change's own commits (04e17d2^..605e8f3) did not touch catalog.tsx, organization.tsx, login.tsx, dashboard.tsx, or home.tsx. The later, separate change `views-brand-tokens-rollout` (e911de1) touched those routes as its own follow-up — correctly out of scope here.
- No security/injection/XSS/data-safety issues found in require-auth.tsx's loading-state change — protected content remains gated by the unchanged `!me`/`wrongRole` check.
