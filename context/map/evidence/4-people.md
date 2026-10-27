# Evidence: people

- Window: 2026-10-10..2026-10-27, 125 counted commits (no bots, no mass commits, merges excluded). Raw tallies: `.work/people-stats.txt`.
- Humans in window: **1**. Tomasz Świerszcz (tswierszcz), under two git identities (123 + 2 commits; the 2 are the initial README commits on 2026-10-10) [evidence]. The identities are merged here.
- Baseline (false-signal rule 6): with one human every capability is 100% one person. That is the baseline, not a finding.
- Agent co-authorship: 100 of 125 commits (80%) carry an AI `Co-authored-by` trailer (Claude Sonnet 5: 53, Claude Opus 5.5: 47) [evidence]. These count as human-steered commits (rule 7), not as extra people.

## People per capability

Who to ask is **Tomasz Świerszcz (tswierszcz)** for every capability. The useful part is the topics he worked on, taken from commit scopes (change-ids), and which planning folder holds the reasoning.

| Capability (crit.) | Commits | Active | Topic groups (scopes) | Planning record |
|---|---|---|---|---|
| redundancy (high) | 25 | 10-16..10-27 | report build-out, empty-list null fix, batch status / empty batch, hardening tests, brand tokens | archive/*-project-redundancy-report, *-projects-empty-list-null-fix, *-batch-status-uncategorized-libraries, *-empty-batch-status |
| catalog (high) | 22 | 10-16..10-27 | seed/CSV import, strategic marks, access-control & pipeline-input tests, logging | archive/*-seed-classification-catalog, *-testing-access-control-data-integrity |
| signin (high) | 18 | 10-16..10-27 | role scaffold, request logging middleware, prod-deploy cookie/session fixes, e2e auth setup | archive/*-minimal-auth-role-scaffold, *-observability-logging-queue-supervision, *-prod-deploy |
| enrichment (high) | 10 | 10-16..10-27 | pipeline seeding, input hardening, uncategorized batch status, queue supervision | archive/*-testing-pipeline-input-hardening, *-seed-classification-catalog |
| users (high) | 7 | 10-16..10-27 | user management, org data fixes, null-list fix | archive/*-org-data-fixes-and-user-management |
| strategic (high) | 1 (+7 under catalog) | 10-18 | marks schema → store → handlers → UI (p1–p4) | archive/2026-10-17-mark-strategic-libraries |
| platform (high) | 33 | 10-16..10-27 | data layer, prod deploy (Fly.io), agent hooks, e2e, health-check | archive/*-data-layer-connectivity, *-prod-deploy |
| orgview (medium) | 4 | 10-18..10-21 | strategic-mark column, nav, null fix | (no own change; inside the folders above) |
| observability (medium) | 2 | 10-26 | logging & queue supervision | archive/*-observability-logging-queue-supervision |
| foundations (top-5 by commits) | 12 | 10-16..10-27 | brand-token rollout | archive/*-redundancy-view-brand-tokens, *-views-brand-tokens-rollout |

Distortion: the strategic-marks code shares catalog/store.go and catalog/handlers.go, so most of its activity is counted under catalog (as the scan contract notes).

## Concentration

Small team: concentration is the baseline. Every capability has only ever been touched by one person. A bus-factor ranking would say nothing here [evidence + rule 6].

## Agent co-authorship

| Capability | Commits with an agent trailer |
|---|---|
| redundancy, enrichment, users, landing, observability | 100% |
| platform | 97% (32/33) |
| signin | 94% (17/18) |
| foundations | 92% (11/12) |
| catalog | 86% (19/22) |
| orgview | 75% (3/4) |
| strategic (migration only) | 0% (0/1) |
| docs/planning | 79% (96/121) |

The 25 commits without a trailer are 17 `chore(archive)` moves, 2 initial README commits, and the whole `mark-strategic-libraries` change (p1–p4, impl-review fix, close-out, 7 commits on 2026-10-18) [evidence]. Whether that change was written without an agent, or the trailer was just left off, is **unknown**.

## Implications

- [inference] Knowledge lives in the planning docs as much as in one head. 17 finished changes are archived under `context/archive/<date>-<change-id>/` (plan, plan-brief, reviews), and commit scopes name the change-id. To find the reasoning behind a commit, map its scope to the archive folder of the same name, using the table above.
- [inference] Before changing redundancy or enrichment (100% agent co-authored, most fixes): read the four redundancy archive folders and testing-pipeline-input-hardening first, then ask the owner about anything those plans do not cover.
- [inference] Before changing strategic marks: read archive/2026-10-17-mark-strategic-libraries. It is the one feature change with no agent trailer, so its plan may be the only written record.
- [evidence] Open, unarchived changes (`testing-redundancy-report-correctness`, `impl-review-ci`) are both parked. Check them before touching the redundancy report or CI review.

## Unknowns

- Whether anyone else (reviewers, a future team) has context: git shows one author and no formal ownership file. This was not checked against CODEOWNERS or the forge.
- How much of each change the human wrote and how much the agent wrote. The trailer shows only that an agent took part.
- Why the mark-strategic-libraries commits have no agent trailer.

Commands run: `cat` of the brief, scan contract, capabilities.tsv and false-signals; `head`/`wc` on `.work/git-log.txt`; `python3 -I scratchpad/people.py` (parses git-log.txt, attributes files via capabilities.tsv, writes `.work/people-stats.txt`); `awk` over git-log.txt to list commits without a trailer and the second identity's commits.
