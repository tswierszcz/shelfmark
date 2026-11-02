# Implementation review in CI (impl-review-ci) Implementation Plan

## Overview

Run `/10x-impl-review-ci` on labelled pull requests through GitHub Actions. The skill is committed to the repo with a small Shelfmark patch, and a new `.github/workflows/impl-review.yml` adapted to Go + npm runs it with Claude Code Action. The PR gets a committed review report, inline comments, a summary comment and an `impl-review-ci/verdict` commit status that blocks only on `REJECTED`, with an `impl-review-override` label to bypass.

## Current State Analysis

- The skill exists only locally: `.claude/skills/10x-impl-review-ci/` (`SKILL.md`, `references/impl-review-instructions.md`, `references/workflow-template.yml`). `.gitignore` ignores `.claude/*` except `settings.json` and `hooks/` (`.gitignore`, "Local Claude Code settings" block), so the skill and ~40 other course skills are untracked.
- The only workflow is `.github/workflows/ci.yml`: backend (`go vet`, `go test`, Docker-backed catalog tests, 15 min) and web (`npm ci`, `typecheck`, `npm test`, `build`), plus deploy on `main`. It never runs Playwright E2E.
- The workflow template (`references/workflow-template.yml`) is built for pnpm and Opus, gates on the `impl-review` label, blocks fork PRs, stages the skill into `~/.claude/skills/` from `origin/<base-ref>` (because claude-code-action replaces the PR's `.claude/` with the base branch's copy), validates the bot commit and posts the verdict status.
- The skill writes its report to `<change-dir>/reviews/impl-review.md` (`SKILL.md` Step 6 and Step 7 `REVIEW_PATH`). This is the same path the local `/10x-impl-review` uses. The local review carries triage decisions (e.g. `context/archive/2026-10-30-dashboard-stats/reviews/impl-review.md`), which the CI run would overwrite.
- The GitHub repo has no `impl-review` / `impl-review-override` labels and no `ANTHROPIC_API_KEY` secret (only `FLY_API_TOKEN`).
- PRs into `main` come from `shape/shelfmark` (PRs #16–19) and often bundle several changes; the skill reviews one plan per PR (newest `plan.md` in the diff, or a `Plan:` line in the PR body).

## Desired End State

- `.claude/skills/10x-impl-review-ci/` is tracked; every other `.claude/skills/*` stays ignored.
- Adding the `impl-review` label to a same-repo PR runs the review with `claude-sonnet-5-5`. The bot pushes `chore(review): impl-review for <change-id> [skip ci]` with `<change-dir>/reviews/impl-review-ci.md` (first line `<!-- IMPL-REVIEW-REPORT -->`, a `Reviewed phases` line, `Decision: PENDING` per finding), posts inline and summary comments, and the workflow sets `impl-review-ci/verdict` on the bot's commit.
- A local `reviews/impl-review.md` is never touched by CI.
- Playwright commands in a plan's Automated Verification are reported as "not run in CI", not graded as failures.
- Toggling a label after a review re-evaluates the verdict without a new Claude run; `impl-review-override` turns a `REJECTED` status green.
- `docs/runbook.md` and `AGENTS.md` say how to trigger, read and override the review.

### Key Discoveries:

- Template staging step: `git archive "origin/$BASE_REF" .claude/skills/10x-impl-review-ci` (`references/workflow-template.yml:119-122`). The skill must be on the PR's base branch, so the first working PR targets `shape/shelfmark`, not `main`.
- Template verdict step picks the newest `impl-review*.md` by mtime (`references/workflow-template.yml:253-254`). After a fresh checkout on a label-only re-run all files share one mtime, so it could read another change's local review. The bot's last commit names the right file.
- Validate step greps `context/changes/[^/]+/reviews/impl-review.*\.md` (`references/workflow-template.yml:202`), which already matches `impl-review-ci.md`.
- `/10x-archive` counts review coverage from `reviews/impl-review*.md` and does not accept a bare "Full plan" scope as proof; it reads a `Reviewed phases` line (`.claude/skills/10x-archive/SKILL.md`, "Kontrola pokrycia przeglądów").
- The skill's Step 6 mentions `oxfmt --check .`, which this repo does not use; the prompt says so instead of editing that line.
- `.claude/settings.json` hooks (gofmt/vet/tsc on edit and at end of turn) come back with the base `.claude/` in CI. The review writes only Markdown, so they only sweep `.md` files; `jq` is preinstalled on `ubuntu-latest`.

## What We're NOT Doing

- No Playwright/E2E environment in the review job (Postgres service, browsers, `E2E_*` credentials). E2E in CI would be a separate change to `ci.yml`.
- No automatic review of every PR or every PR into `main`; the label is the trigger.
- No branch protection / required-check settings; whether `impl-review-ci/verdict` is required stays a manual GitHub setting.
- No changes to `ci.yml`, the deploy job, or application code.
- No other course skills committed; no change to the local `/10x-impl-review` flow.
- No review of several plans in one PR; bundled release PRs use the `Plan:` line to pick one.
- Claude does not handle the API key value; the user sets the secret.

## Implementation Approach

Start from the template and change only what Shelfmark needs: toolchain (Go + npm), model (Sonnet 5.5), job timeout, a Shelfmark context block in the prompt (report filename, E2E rule, no oxfmt), and a verdict step that reads the bot's own file. Patch the committed skill copy in the three places that name the report path, add a `Reviewed phases` field to its template, and mark the patch at the top so a future skill update can re-apply it. Land phase 1 directly on `shape/shelfmark` so the skill is on the base branch; phase 2 verifies on a real PR into `shape/shelfmark`.

## Critical Implementation Details

- **Ordering**: the skill and workflow must be pushed to `shape/shelfmark` before the smoke PR opens, because the staging step reads the skill from the base branch. The first `shape/shelfmark → main` PR carrying this change must not get the `impl-review` label, since `main` does not have the skill yet.
- **Recursion guard**: keep the template's `[skip ci]` check, `claude[bot]` author guard and `head.repo.full_name == github.repository` fork block verbatim; they keep the bot from looping and keep secrets away from forks.

## Phase 1: Workflow and skill in the repo

### Overview

Track and patch the skill, add the workflow and document the flow. No GitHub settings change in this phase.

### Changes Required:

#### 1. Track only the impl-review-ci skill

**File**: `.gitignore`

**Intent**: Let git track `.claude/skills/10x-impl-review-ci/` while every other skill under `.claude/skills/` stays ignored.

**Contract**: The `.claude/*` block gains un-ignore rules for the `skills` directory, a re-ignore of `.claude/skills/*`, and an un-ignore of `.claude/skills/10x-impl-review-ci/`. The comment above the block names the exception and why (CI stages it from the base branch).

#### 2. Patch the committed skill copy

**File**: `.claude/skills/10x-impl-review-ci/SKILL.md`

**Intent**: Write the CI report to its own file so a local, triaged `reviews/impl-review.md` is never overwritten, and record which phases it covered so `/10x-archive` counts it.

**Contract**:
- A short note right after the H1 says this copy is patched for Shelfmark (report file `impl-review-ci.md`, `Reviewed phases` field) and must be re-applied when the skill is updated.
- Frontmatter `description` (line 6), Step 6 report path and "Katalog jest wyprowadzany…" paragraph: `<change-dir>/reviews/impl-review-ci.md`.
- Step 6 template header gains `- **Reviewed phases**: <comma-separated phase numbers>` after `- **Scope**`. It lists only the phases whose Progress rows carry a commit SHA that is in `git log origin/<base-ref>..HEAD`, or whose changes the PR diff visibly implements; with none it says `none`. Never every phase by default, because `/10x-archive` counts this field as review coverage.
- Step 7: `REVIEW_PATH="${CHANGE_DIR}/reviews/impl-review-ci.md"`.
- Nothing else changes (output marker, `Decision: PENDING`, commit message, comment markers stay as they are).

The two `references/` files are committed unchanged.

#### 3. Add the workflow

**File**: `.github/workflows/impl-review.yml`

**Intent**: Run the review on labelled same-repo PRs with the Shelfmark toolchain and gate on the verdict.

**Contract**: Copy of `references/workflow-template.yml` with these differences only:
- Header comment says it is adapted from the skill's template for Go + npm, Sonnet 5.5, and the `impl-review-ci.md` report file.
- Job `timeout-minutes: 30`.
- Toolchain steps replace pnpm: `actions/setup-go@v7` (`go-version-file: backend/go.mod`, `cache-dependency-path: backend/go.sum`, matching `ci.yml`), `actions/setup-node` (template pin, `node-version: "24"`, `cache: npm`, `cache-dependency-path: web/package-lock.json`), then `npm ci` in `web` and `go mod download` in `backend`.
- `claude_args`: `--model claude-sonnet-5-5`; `--max-turns 60` and the tool allowlist unchanged.
- Prompt keeps the template text and adds a "Shelfmark specifics" block: the report file is `<change-dir>/reviews/impl-review-ci.md`, never `impl-review.md`; `npx playwright test …` commands are not runnable here (no E2E database, credentials or browsers) and are listed as "not run in CI", not graded as failures; backend tests are `cd backend && go test ./...` (catalog tests start Postgres via Docker, ~3 min); there is no `oxfmt`.
- "Skip Claude run…" guard: `PRIOR_CLAUDE` looks only at the PR's own commits (`git log origin/"$BASE_REF"..HEAD --author='claude\[bot\]' -1`). PRs merge with merge commits, so bot commits from earlier PRs sit in base-branch history; an unscoped lookup would skip the first review of every `gh pr create --label impl-review` PR. The step gets `BASE_REF` in `env` and fetches it like the staging step.
- "Check review verdict": the review file is the `impl-review-ci.md` added or changed by the latest `claude[bot]` commit in `origin/$BASE_REF..HEAD`; with no such commit it passes without posting a status (as the template does with no file). Verdict parsing, override label and status posting stay as in the template.
- Triggers, label gate, fork guard, concurrency, permissions, staging step, Validate step: unchanged from the template.

```bash
REVIEW_FILE=$(git log -1 --author='claude\[bot\]' --name-only --format= \
  "origin/$BASE_REF..HEAD" \
  -- ':(glob)**/context/changes/*/reviews/impl-review-ci.md' | head -1)
```

#### 4. Document the flow

**File**: `docs/runbook.md`

**Intent**: Operators know how the CI review is triggered, read and bypassed.

**Contract**: New `## Implementation review in CI` section (after `## Deploy`): add the `impl-review` label to a PR (`gh pr create --label impl-review` or `gh pr edit <n> --add-label impl-review`); report lands in `reviews/impl-review-ci.md` next to the plan; `REJECTED` fails `impl-review-ci/verdict` until `impl-review-override` is added; release PRs bundling several changes pick one with a `Plan: context/changes/<id>/plan.md` line; `ANTHROPIC_API_KEY` is a GitHub repo secret, each run is billed to it; the skill is read from the PR's base branch.

**File**: `AGENTS.md`

**Intent**: Agents opening PRs know the label exists.

**Contract**: One bullet in "Commits & Pull Requests": PRs implementing a plan may get the `impl-review` label to run the CI implementation review (`.github/workflows/impl-review.yml`, details in @docs/runbook.md).

### Success Criteria:

#### Automated Verification:

- Skill is tracked and other skills are not: `git check-ignore -q .claude/skills/10x-impl-review-ci/SKILL.md` exits 1 and `git check-ignore -q .claude/skills/10x-plan/SKILL.md` exits 0
- Workflow passes actionlint: `cd backend && go run github.com/rhysd/actionlint/cmd/actionlint@latest ../.github/workflows/impl-review.yml`
- Skill writes only the CI report path: `grep -c 'reviews/impl-review-ci.md' .claude/skills/10x-impl-review-ci/SKILL.md` is at least 2 and `grep -n 'reviews/impl-review\.md' .claude/skills/10x-impl-review-ci/SKILL.md` prints nothing
- Workflow uses Sonnet and the label gate: `grep -q 'claude-sonnet-5-5' .github/workflows/impl-review.yml && grep -q "labels.\*.name, 'impl-review'" .github/workflows/impl-review.yml`
- Bot-commit lookups are scoped to the PR: `grep -c 'origin/"\?\$BASE_REF"\?\.\.HEAD' .github/workflows/impl-review.yml` is at least 2

#### Manual Verification:

- The prompt's Shelfmark block and the runbook section read correctly and match the decisions in this plan

**Implementation Note**: Commit this phase on `shape/shelfmark` and push it before Phase 2; the smoke PR needs the skill on its base branch.

---

## Phase 2: GitHub setup and smoke run

### Overview

Create the labels, have the user add the API key secret, and prove the workflow end to end on a real PR into `shape/shelfmark`.

### Changes Required:

#### 1. Labels

**Intent**: The gate and the override label exist in the repo.

**Contract**: `gh label create impl-review --description "Run the CI implementation review" --color 1d76db` and `gh label create impl-review-override --description "Bypass a REJECTED impl-review verdict" --color b60205`.

#### 2. API key secret (user)

**Intent**: The workflow can call Claude.

**Contract**: The user runs `gh secret set ANTHROPIC_API_KEY` (value pasted at the prompt, never into chat or a file). The agent only checks that the name appears in `gh secret list`.

#### 3. Smoke PR

**Intent**: Run the whole path once on a plan-touching PR.

**Contract**: Branch `chore/impl-review-ci-smoke` from `shape/shelfmark`, one commit that touches `context/changes/impl-review-ci/plan.md` (the Phase 2 Progress update), PR into `shape/shelfmark` opened with `--label impl-review`. The plan's own review is the subject; any verdict is acceptable, since this checks the mechanics. Merge the PR afterwards so the CI report lands next to the plan.

### Success Criteria:

#### Automated Verification:

- Both labels exist: `gh label list --search impl-review` lists `impl-review` and `impl-review-override`
- Secret is set: `gh secret list` lists `ANTHROPIC_API_KEY`

#### Manual Verification:

- On the smoke PR the "Impl Review" run finishes and the bot pushes `chore(review): impl-review for impl-review-ci [skip ci]` adding `context/changes/impl-review-ci/reviews/impl-review-ci.md`, whose first line is `<!-- IMPL-REVIEW-REPORT -->` and which has a `Reviewed phases` line; the Validate step is green
- The PR shows a summary comment and an `impl-review-ci/verdict` status on the bot's commit, and the bot commit does not start another full review
- Removing and re-adding the `impl-review` label re-runs only the verdict check (Claude step skipped); if the verdict is `REJECTED`, adding `impl-review-override` turns the status green
- The run's cost on the Anthropic console is acceptable for one review per labelled PR

**Implementation Note**: After the smoke PR is merged, delete the `chore/impl-review-ci-smoke` branch.

---

## Testing Strategy

### Unit Tests:

- None; no application code changes.

### Integration Tests:

- actionlint on the new workflow (Phase 1).
- The smoke PR is the end-to-end test (Phase 2).

### Manual Testing Steps:

1. Open the smoke PR with `--label impl-review` and follow the Actions run.
2. Check the bot commit, report file, summary comment and verdict status.
3. Toggle the label; confirm the Claude step is skipped and the status is re-posted.
4. If `REJECTED`, add `impl-review-override` and confirm the status turns green.

## Performance Considerations

One run installs Go and npm dependencies, runs the plan's backend tests (~3 min with Docker) and web checks, and uses up to 60 turns of Sonnet 5.5. The 30-minute job timeout caps a stuck run; the label gate keeps unlabelled PRs free.

## Migration Notes

None. Existing archived reviews keep the `impl-review.md` name; new CI reports sit beside local ones as `impl-review-ci.md`.

## References

- Skill: `.claude/skills/10x-impl-review-ci/SKILL.md`
- Template: `.claude/skills/10x-impl-review-ci/references/workflow-template.yml`
- Existing CI: `.github/workflows/ci.yml`
- E2E stack: `context/foundation/test-stack.md`

## Progress

> Convention: `- [ ]` pending, `- [x]` done. Append ` — <commit sha>` when a step lands. Do not rename step titles. See `references/progress-format.md`.

### Phase 1: Workflow and skill in the repo

#### Automated

- [x] 1.1 Skill is tracked and other skills are not: `git check-ignore -q .claude/skills/10x-impl-review-ci/SKILL.md` exits 1 and `git check-ignore -q .claude/skills/10x-plan/SKILL.md` exits 0
- [x] 1.2 Workflow passes actionlint: `cd backend && go run github.com/rhysd/actionlint/cmd/actionlint@latest ../.github/workflows/impl-review.yml`
- [x] 1.3 Skill writes only the CI report path: `grep -c 'reviews/impl-review-ci.md' .claude/skills/10x-impl-review-ci/SKILL.md` is at least 2 and `grep -n 'reviews/impl-review\.md' .claude/skills/10x-impl-review-ci/SKILL.md` prints nothing
- [x] 1.4 Workflow uses Sonnet and the label gate: `grep -q 'claude-sonnet-5-5' .github/workflows/impl-review.yml && grep -q "labels.\*.name, 'impl-review'" .github/workflows/impl-review.yml`
- [x] 1.5 Bot-commit lookups are scoped to the PR: `grep -c 'origin/"\?\$BASE_REF"\?\.\.HEAD' .github/workflows/impl-review.yml` is at least 2

#### Manual

- [x] 1.6 The prompt's Shelfmark block and the runbook section read correctly and match the decisions in this plan

### Phase 2: GitHub setup and smoke run

#### Automated

- [ ] 2.1 Both labels exist: `gh label list --search impl-review` lists `impl-review` and `impl-review-override`
- [ ] 2.2 Secret is set: `gh secret list` lists `ANTHROPIC_API_KEY`

#### Manual

- [ ] 2.3 On the smoke PR the "Impl Review" run finishes and the bot pushes `chore(review): impl-review for impl-review-ci [skip ci]` adding `context/changes/impl-review-ci/reviews/impl-review-ci.md`, whose first line is `<!-- IMPL-REVIEW-REPORT -->` and which has a `Reviewed phases` line; the Validate step is green
- [ ] 2.4 The PR shows a summary comment and an `impl-review-ci/verdict` status on the bot's commit, and the bot commit does not start another full review
- [ ] 2.5 Removing and re-adding the `impl-review` label re-runs only the verdict check (Claude step skipped); if the verdict is `REJECTED`, adding `impl-review-override` turns the status green
- [ ] 2.6 The run's cost on the Anthropic console is acceptable for one review per labelled PR
