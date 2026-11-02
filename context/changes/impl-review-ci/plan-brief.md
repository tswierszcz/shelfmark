# Implementation review in CI (impl-review-ci) — Plan Brief

> Full plan: `context/changes/impl-review-ci/plan.md`

## What & Why

Run `/10x-impl-review-ci` in GitHub Actions on labelled pull requests, so a PR that implements a plan gets an automatic review against that plan: a committed report, inline and summary comments, and a verdict status that blocks only on `REJECTED`. This is step 7 of the M1 release path, parked on 2026-10-27 until an Anthropic API key was available; the key now exists.

## Starting Point

The skill and its workflow template sit only in the local, gitignored `.claude/skills/10x-impl-review-ci/`. The template targets pnpm and Opus. `ci.yml` runs Go and web checks but no E2E. The skill would write `reviews/impl-review.md`, the same file the local `/10x-impl-review` uses for triaged decisions. The repo has neither the labels nor the `ANTHROPIC_API_KEY` secret.

## Desired End State

Adding the `impl-review` label to a same-repo PR runs the review with Sonnet 5.5. The bot commits `reviews/impl-review-ci.md` next to the plan with `[skip ci]`, comments on the PR and sets `impl-review-ci/verdict`. `impl-review-override` bypasses a `REJECTED` verdict. Local reviews are never overwritten, and Playwright steps are reported as "not run in CI".

## Key Decisions Made

| Decision | Choice | Why (1 sentence) |
| --- | --- | --- |
| CI vs local review file | CI writes `reviews/impl-review-ci.md` (small patch in the committed skill) | Keeps triaged local reviews intact and gives a second, independent opinion. |
| Model | `claude-sonnet-5-5` | Much cheaper per run; diff-vs-plan review does not need Opus depth. |
| E2E in CI | Not run; reported as "not run in CI" | Matches `ci.yml`, which has no E2E; a full E2E job is a separate change. |
| Trigger | `impl-review` label on any same-repo PR | Cost stays under control; docs/archive PRs cost nothing. |
| Verdict source | File from the latest `claude[bot]` commit in `origin/<base>..HEAD` | The template's newest-mtime lookup can pick another change's review; merged bot commits sit in base history. |
| Skip guard | `PRIOR_CLAUDE` scoped to `origin/<base>..HEAD` | Unscoped, a merged bot commit makes `gh pr create --label` skip the first review. |
| Archive compatibility | Report gets a `Reviewed phases` line listing only phases with SHAs in the PR range | `/10x-archive` counts it as coverage, so it must not claim phases the PR didn't contain. |

## Scope

**In scope:**
- `.gitignore` exception for this one skill; patched skill copy committed
- `.github/workflows/impl-review.yml` for Go + npm, Sonnet, 30 min timeout
- Runbook section and one AGENTS.md bullet
- Labels, secret (set by the user), smoke PR into `shape/shelfmark`

**Out of scope:**
- E2E environment in CI; changes to `ci.yml` or deploy
- Branch protection / required checks
- Reviewing several plans in one PR; committing other course skills

## Architecture / Approach

The PR gets the label → the workflow installs Go and npm deps → it stages the skill from the base branch into `~/.claude/skills/` → claude-code-action runs `/10x-impl-review-ci` with a Shelfmark context block → the bot pushes the report with `[skip ci]` and posts comments → the Validate step checks the output contract → the verdict step reads the bot's report and posts `impl-review-ci/verdict`. Label toggles skip Claude and only re-evaluate the verdict.

## Phases at a Glance

| Phase | What it delivers | Key risk |
| --- | --- | --- |
| 1. Workflow and skill in the repo | Tracked, patched skill; `impl-review.yml`; docs | Patch drifts from future skill updates (marked at top of SKILL.md) |
| 2. GitHub setup and smoke run | Labels, secret, one verified review on a real PR | First run surfaces action/model or permission issues |

**Prerequisites:** Paid Anthropic API key; phase 1 pushed to `shape/shelfmark` before the smoke PR.
**Estimated effort:** ~1 session (phase 1 short; phase 2 mostly waiting on the Actions run).

## Open Risks & Assumptions

- The first `shape/shelfmark → main` PR carrying this change must not be labelled, because `main` does not have the skill yet.
- The pinned claude-code-action version must accept the `claude-sonnet-5-5` model id; the smoke run proves it.
- Base-branch `.claude/settings.json` hooks also run in CI; the review writes only Markdown, so they should stay cheap.

## Success Criteria (Summary)

- A labelled PR gets a report, comments and a verdict status without a bot-commit loop.
- Local `impl-review.md` files are never changed by CI.
- `REJECTED` blocks until `impl-review-override` is added.
