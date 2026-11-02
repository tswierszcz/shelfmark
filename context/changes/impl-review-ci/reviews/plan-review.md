<!-- PLAN-REVIEW-REPORT -->
# Plan Review: Implementation review in CI (impl-review-ci)

- **Plan**: context/changes/impl-review-ci/plan.md
- **Mode**: Deep
- **Date**: 2026-11-02
- **Verdict**: REVISE → SOUND after triage
- **Findings**: 1 critical, 2 warnings, 0 observations

## Verdicts

| Dimension | Verdict |
|-----------|---------|
| End-State Alignment | PASS |
| Lean Execution | PASS |
| Architectural Fitness | PASS |
| Blind Spots | FAIL |
| Plan Completeness | WARNING |

## Grounding
9/9 paths ✓, 6/6 symbols ✓ (template lines 119-122, 202, 253-254; SKILL.md Step 6/7; archive coverage rule; ci.yml setup pins), brief↔plan ✓

## Findings

### F1 — Bot-commit lookups scan base-branch history

- **Severity**: ❌ CRITICAL
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Blind Spots
- **Location**: Phase 1 §3 (guard step "unchanged", new verdict lookup); Phase 2 §3
- **Detail**: The job checks out the PR head with `fetch-depth: 0`. The template guard (`PRIOR_CLAUDE=$(git log --author='claude\[bot\]' -1)`, workflow-template.yml:92) and the plan's verdict lookup both search all history, including the base branch. PRs are merged with merge commits (#9–#15), which keep the claude[bot] author. After the smoke PR merges into `shape/shelfmark`, every later PR inherits a bot commit: `gh pr create --label impl-review` → the `labeled` run sees PRIOR_CLAUDE → skip → the first review never runs; if Claude fails, the verdict step posts the smoke PR's verdict for another change.
- **Fix**: Scope both lookups to `origin/$BASE_REF..HEAD`; remove the guard step from the "unchanged from the template" list.
- **Decision**: FIXED (guard and verdict lookups scoped to origin/$BASE_REF..HEAD; criterion 1.5 added)

### F2 — "Reviewed phases" claims every phase in Progress

- **Severity**: ⚠️ WARNING
- **Impact**: 🔎 MEDIUM — real tradeoff; pause to reason through it
- **Dimension**: Blind Spots
- **Location**: Phase 1 §2 (SKILL.md Step 6 template); Phase 2 §3
- **Detail**: The patch fills `Reviewed phases` with every phase in `## Progress`. `/10x-archive` treats that field as proof of coverage (`.claude/skills/10x-archive/SKILL.md:109`). A per-phase PR, or the smoke PR (whose diff only touches plan.md), would mark phases it never reviewed as covered.
- **Fix A ⭐ Recommended**: List only phases whose Progress SHAs are in `origin/<base>..HEAD` (or that the diff visibly implements); write "none" if there are none
  - Strength: Coverage stays honest; the data is already in Progress and git.
  - Tradeoff: One more sentence in the patch; the model has to map SHAs to phases.
  - Confidence: HIGH — /10x-implement always records the SHA in Progress.
  - Blind spot: Phases committed without a SHA would be left out (too few listed, which is the safe direction).
- **Fix B**: Keep "all phases"; note in the runbook that CI coverage does not count for /10x-archive
  - Strength: No extra skill logic.
  - Tradeoff: Archive still counts it automatically; the note is only advice.
  - Confidence: MED — depends on people reading the runbook.
  - Blind spot: The archive skill cannot see this note.
- **Decision**: FIXED (Fix A)

### F3 — SKILL.md frontmatter still names impl-review.md

- **Severity**: ⚠️ WARNING
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Plan Completeness
- **Location**: Phase 1 §2 contract vs criterion 1.3
- **Detail**: `SKILL.md:6` (frontmatter description) contains `reviews/impl-review.md`. The patch says "Nothing else changes", but criterion 1.3 requires the grep for `reviews/impl-review\.md` to print nothing, so the patch as written fails its own check.
- **Fix**: Add the frontmatter description (line 6) to the patch list.
- **Decision**: FIXED
