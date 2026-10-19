<!-- PLAN-REVIEW-REPORT -->
# Plan Review: Project Redundancy Report Implementation Plan

- **Plan**: context/changes/project-redundancy-report/plan.md
- **Mode**: Deep
- **Date**: 2026-10-19
- **Verdict**: REVISE (SOUND after triage — all 4 findings fixed)
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

7/7 paths ✓, claims verified against code (CycloneDX JSON tags, pipeline idempotency, Enqueuer blast radius, RequireAuth array support, catalog.tsx polling constants) ✓, brief↔plan ✓

## Findings

### F1 — "is_direct" means two different things on two pages

- **Severity**: ⚠️ WARNING
- **Impact**: 🔎 MEDIUM — real tradeoff; pause to reason through it
- **Dimension**: Blind Spots
- **Location**: Phase 4 / Implementation Approach
- **Detail**: `/organization` (S-02) already shows an `is_direct` field sourced from the global `libraries.is_direct` column, which is always `true` for every library. This plan's `/redundancy` page introduces a second, correctly per-project `is_direct` with real values. Same field name, two meanings, on two pages an architect will use side by side.
- **Fix A ⭐ Recommended**: Document as a known limitation, don't touch /organization
  - Strength: No scope creep into a previously-shipped S-02 page.
  - Tradeoff: The misleading signal stays live on /organization indefinitely.
  - Confidence: HIGH — this plan's scope anchors (FR-005/FR-008/US-01) don't cover /organization.
  - Blind spot: Whether architects actually rely on /organization's Direct column today is unverified.
- **Fix B**: Relabel or hide the Direct column on /organization in this change
  - Strength: Removes a misleading signal at the source.
  - Tradeoff: Reopens a shipped S-02 page, expanding this slice's scope.
  - Confidence: MEDIUM — straightforward UI change, but touches another slice's surface.
  - Blind spot: Whether S-02's acceptance already depends on that column being present as-is.
- **Decision**: FIXED (Fix A — added "Known Limitation" section to plan.md and a matching Open Risks bullet to plan-brief.md)

### F2 — No dependency-graph SBOM test fixtures exist yet

- **Severity**: ⚠️ WARNING
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Plan Completeness
- **Location**: Phase 2
- **Detail**: No JSON SBOM fixtures exist anywhere in the repo; the only existing fixture (`sbom_test.go`) is an inline, components-only literal with no `dependencies`/`metadata.component`. Phase 2's three test cases need brand-new fixtures authored from scratch — the plan doesn't flag this as new test-infrastructure work.
- **Fix**: Add a line to Phase 2 noting new inline CycloneDX JSON literals (following `sbom_test.go`'s existing style) must be authored for all three graph scenarios.
- **Decision**: FIXED (added to Phase 2 Automated Verification in plan.md)

### F3 — Pending-only enqueue guard isn't stated as deliberate

- **Severity**: ⚠️ WARNING
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Blind Spots
- **Location**: Phase 3 — Upload handler
- **Detail**: `Queue.process` unconditionally reruns the full categorization cascade for anything handed to `Enqueue`, and `AttachCategory` is an upsert that would silently overwrite a manual correction's `source`. The plan's "enqueue only `Status == pending`" rule avoids this but doesn't say why. A narrow, low-cost race exists: two concurrent project uploads sharing a brand-new purl could both enqueue it (harmless — `AttachCategory` upserts — just doubles external API calls).
- **Fix**: Add one sentence to Phase 3's upload-handler contract stating the pending-only guard explicitly and why; note the concurrent-upload race as an accepted, non-destructive cost.
- **Decision**: FIXED (expanded the upload handler Intent in plan.md)

### F4 — No stated rollback/ordering rationale across 4 sequential store calls

- **Severity**: ⚬ OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Blind Spots
- **Location**: Phase 3 — Upload handler
- **Detail**: `ProjectUploadHandler` calls `CreateLibraries` → `LibrariesByPurls` → `FindOrCreateProject` → `ReplaceProjectLibraries` with no transaction spanning all four. Safe as designed (earlier steps are idempotent; the one all-or-nothing step is deliberately last) but unstated.
- **Fix**: Add one sentence to Phase 3 confirming this ordering is intentional — earlier steps are idempotent, so a failure at any point is safely retryable.
- **Decision**: FIXED (expanded the upload handler Intent in plan.md)
