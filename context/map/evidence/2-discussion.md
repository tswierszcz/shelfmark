# Evidence: discussion
Sources: gh (tswierszcz/shelfmark, read-only) + git. Window 2026-10-10..2026-10-27 (full history), HEAD 5d99d11. Limits hit: `gh pr view 2 --json commits` returned 100 commits (API cap) — PR #2 is larger than shown. Raw data: `.work/gh-prs.json`, `.work/gh-issues.json`, `.work/pr-commits.txt`, `.work/commits-caps.json`, `.work/change-scope-caps.txt`, `.work/fix-commits.txt`, `.work/debt-markers.txt`.

Baseline: 1 human (false-signal rule 6); 100/125 commits co-authored by an AI agent. Forge has 12 PRs, 0 issues, 0 reviews, 0 PR comments.

## Buzz per capability

Commits = commits touching the capability (noise removed, cross-cutting counted per capability). PRs: #1 (closed) and #2 (M1 release) each span every capability — shown separately as "mega"; "small PRs" = #3–#12, where #6/#8 and #9/#10 are the same change landed on two branches. Plans = 10x change folders whose commits are primarily about the capability (inference from commit scopes in `change-scope-caps.txt`), archived + open.

| Capability | Commits | fix commits | Small PRs | Closed-unmerged | Human review | Reverts | Open bugs | Plans (arch+open) | Impl-review NEEDS ATTENTION | Markers /kLOC |
|---|---|---|---|---|---|---|---|---|---|---|
| redundancy | 25 | 10 (40%) | 5 (#5,6,8,9,10; 3 distinct) | #1 (mega) | 0 | 0 | 0 (no issues) | 4 + 1 open (paused) | 0 of 4 | 0 |
| catalog | 22 | 8 (36%) | 0 | #1 | 0 | 0 | 0 | 2 + 0 | 1 (testing-access-control) | 0 |
| signin | 18 | 6 (33%) | 1 (#5) | #1 | 0 | 0 | 0 | 1 + 0 | 0 | 0 |
| platform | 33 | 7 (21%) | 4 (#3,5,6,8) | #1 | 0 | 0 | 0 | 2 + 2 open (1 parked) | 1 (prod-deploy) | 0 |
| enrichment | 10 | 4 (40%) | 3 (#5,6,8) | #1 | 0 | 0 | 0 | 1 + 0 | 1 (pipeline-input-hardening) | 0 |
| foundations | 12 | 2 | 2 (#6,8) | #1 | 0 | 0 | 0 | 2 + 0 | 1 (redundancy-view-brand-tokens) | 0 |
| users | 7 | 3 | 0 | #1 | 0 | 0 | 0 | 1 + 0 | 0 | 0 |
| landing | 8 | 0 | 0 | #1 | 0 | 0 | 0 | 2 + 0 | 0 | 0 |
| orgview | 4 | 1 | 0 | #1 | 0 | 0 | 0 | 0 | — | 0 |
| observability | 2 | 1 | 0 | #1 | 0 | 0 | 0 | 1 + 0 | 1 | 0 |
| strategic | 1 (code counted under catalog) | 0 | 0 | #1 | 0 | 0 | 0 | 1 + 0 (+2 as secondary) | 1 (mark-strategic-libraries) | 0 |
| docs | 121 | 19 | 10 | — | — | — | — | — | — | 0 |

Evidence: **no human review exists on any PR** (12/12 PRs have empty `reviews` and `comments`). Review happens in-repo as agent-run `/10x-plan-review` and `/10x-impl-review` reports, not on the forge.

## Friction

- **Reverts: 0**, hotfix/rollback subjects: 0 (evidence, `git-log.txt` grep).
- **Closed-unmerged PRs: 1** — #1 "Milestone M-1: First usable redundancy report" (46 commits, all capabilities), closed 2026-10-19 and superseded by #2 (inference: same branch `shape/shelfmark`). Not friction about one capability.
- **Fix commits: 20 of 125.** 9 of 20 are "impl-review fixes" from the agent review loop (process, not production defects — see Mechanical); 7 of 20 touch >3 capabilities (spillover, rule 12).
- **Second attempts (evidence from subjects/dates):**
  - *redundancy / enrichment — batch status*: `batch-status-uncategorized-libraries` (2 fix commits, PR #6/#8) then `empty-batch-status` (PR #9/#10) the same day 2026-10-27, the latter verified by a production re-upload. Two consecutive fix changes on batch-status truthfulness right after the prod release; a follow-up `testing-redundancy-report-correctness` is opened and paused. Strongest friction signal in the repo.
  - *redundancy / catalog — null lists*: `projects-empty-list-null-fix` (2026-10-21, 2 fix commits + impl-review fix) then `testing-access-control-data-integrity` p2 "List endpoints never null" (2026-10-26). The only entry in `context/foundation/lessons.md` is "Go nil slices marshal to JSON `null`, not `[]`" — the team codified it as a recurring pitfall.
  - *strategic (code in catalog) — strategic cap*: FR-007 strategic cap in `org-data-fixes-and-user-management` p1 (2026-10-20), then "Strategic cap — contract, edges, concurrency" in `testing-access-control-data-integrity` p3 (2026-10-26); original change `mark-strategic-libraries` had impl-review NEEDS ATTENTION.
  - *redundancy UI*: `fix(redundancy): cancel stale polling loop on new upload` (2026-10-19), same polling/report surface later fixed by both batch-status changes (inference).
- Open bugs / issue age: unknown — the issue tracker is unused (0 issues ever).

## Planning attention

17 archived changes + 3 open folders (`impl-review-ci` parked on missing API key, `testing-redundancy-report-correctness` paused until research, `bootstrap-verification` with only `verification.md`, no change.md).

- **redundancy** draws the most plans (4 archived, 1 open), and 3 of the last 4 product changes (2026-10-27) — evidence. It is the core value and is also where the post-release fixes land; both apply (rule 4: "being built" and "keeps breaking" share a cause until production data separates them).
- **catalog/strategic** attract planning mostly as *secondary* scope of fix/test changes (org-data-fixes, testing-access-control) — inference from commit scopes.
- **platform**: prod-deploy (9 commits, 2 fix, impl-review NEEDS ATTENTION, plan-review REVISE with 5 findings) + 2 open infra-ish changes.
- Plan reviews: 7 of 7 `plan-review.md` reports start at REVISE (most "SOUND after triage"); impl reviews: 6 NEEDS ATTENTION, 7 APPROVED (13 reports). Process baseline, not capability-specific.
- Foundation docs churn (`roadmap` 5, `test-plan`, `health-check`, `audit` commits) is planning about the product as a whole — rule 8, not ranked.

## Debt markers

`TODO|FIXME|HACK|XXX` at HEAD: **0** in every capability (0.0 per kLOC; lines per capability in `debt-markers.txt`). `git grep` only matches one binary PNG. Evidence that markers are not used, not that there is no debt (unknown).

## Mechanical signals

- PR #1 and #2 touch every capability: one milestone branch merged as a single PR (rule 12). Kept out of per-capability ranking.
- PRs #6/#8 and #9/#10 are duplicates (same change into `main` and `shape/shelfmark`) — count once.
- 17 `chore(archive)` commits and all `context/` churn: mechanical renames / prose (rule 8); attributed to the change they discuss above.
- 9 "impl-review fixes" commits: self-review loop of the 10x workflow, not defects found in use (rule 5/7 analogue).
- prod-deploy fixes (2) and CI/deploy commits: trial-and-error infra work (rule 5).
- Fix shares with n<10 (users, orgview, observability, strategic) are not meaningful.
- strategic shows 1 commit only because its code lives in `catalog/store.go`/`handlers.go` (contract distortion); its discussion (3 changes) is real.

## Unknowns

- Human review culture: unknown — 0 forge reviews; single developer reviewing via agents.
- Issue tracker: none on GitHub; bugs may be tracked elsewhere or not at all (unknown).
- Production errors: no error tracker found in `go.mod`, `package.json`, `fly.toml`, `.github` (grep for sentry/datadog/rollbar/bugsnag/honeybadger) — production defect rate is unknown; only signal is the "production re-upload verified" note in empty-batch-status.
- PR #2 commit list truncated at 100 by the API.
- Change → capability mapping for plans is inferred from commit scopes, not declared in change.md.

Commands run: `gh pr list --state all --limit 100 --json …`; `gh issue list --state all --limit 100 --json …`; `gh pr view <n> --json commits,mergeCommit` (×12, one at a time); `git log --merges`; Python parse of `.work/git-log.txt` with capabilities.tsv attribution (scope grouping, fix/revert counts); `ls`/`grep` over `context/changes`, `context/archive/*/reviews`, `context/foundation/lessons.md`; `git ls-files | attribute.py` + marker regex count; `git grep` for markers and error trackers.
