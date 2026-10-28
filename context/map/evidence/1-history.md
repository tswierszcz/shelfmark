# Evidence: history

Window: full history, effective 2026-10-10..2026-10-27 (HEAD 5d99d11, branch shape/shelfmark). Buckets: ISO weeks W41-W44. Commits counted: 125 (merges excluded, 0 bots, 0 mass commits). Noise filter per contract (go.sum, package-lock.json, *.ico/*.png/*.svg). Raw per-commit attribution: `.work/history-raw.tsv`; fix subjects per capability: `.work/history-fixsubjects.json`.

Notes on the counts:
- 1 human (two identities, merged). Every concentration figure is a baseline (rule 6).
- W41 = 3 commits (initial README/idea notes). W44 covers only 2 days (Oct 26-27) but holds 52 commits. Per-week numbers are uneven and do not describe long-term trends.
- Total counted changes = 575. The contract's 579 includes the 4 deleted/added `web/app/welcome/*.svg` files under unmapped. This file applies the noise filter to them, so unmapped is 2 here. (evidence)

## Activity per capability

| Capability | Changes | Commits | Agent co-authored commits | Share of code changes (294) |
|---|---|---|---|---|
| docs (planning/prose) | 281 | 121 | 96 | n/a (49% of all 575) |
| platform | 78 | 33 | 32 | 27% |
| catalog | 53 | 22 | 19 | 18% |
| redundancy | 44 | 25 | 25 | 15% |
| signin | 37 | 18 | 17 | 13% |
| enrichment | 28 | 10 | 10 | 10% |
| foundations | 19 | 12 | 11 | 6% |
| users | 11 | 7 | 7 | 4% |
| landing | 9 | 8 | 8 | 3% |
| observability | 6 | 2 | 2 | 2% |
| orgview | 5 | 4 | 3 | 2% |
| strategic | 2 | 1 | 0 | <1% |
| unmapped | 2 | 2 | 2 | <1% (deleted scaffold) |

- Agent share: 100 of 125 commits (80%) carry an AI Co-authored-by trailer. They count as human commits per the contract and are shown here as context. (evidence)
- docs appears in 121 of 125 commits because every 10x change writes its plan/progress next to the code (rule 8). It is not a hot area. (evidence + inference)
- **Known distortion:** strategic marks has only 2 changes because its code lives in `catalog/store.go` and `catalog/handlers.go`. Part of catalog's 53 changes is strategic-marks work (scopes `mark-strategic-libraries` and `testing-access-control-data-integrity` "Strategic cap"). How much cannot be separated by path. (evidence)

## Top files in top capabilities

| Capability | Top files (changes) |
|---|---|
| platform | backend/cmd/shelfmark/main.go (16), .gitignore (5), backend/go.mod (5), web/package.json (4), backend/internal/db/migrate_test.go (4) |
| catalog | catalog/handlers_test.go (11), catalog/store.go (11), catalog/handlers.go (9), catalog/store_test.go (8), web/app/routes/catalog.tsx (4) |
| redundancy | web/app/routes/redundancy.tsx (8), catalog/project_handlers_test.go (7), catalog/project.go (6), catalog/project_handlers.go (5), dev.kitchen-sink-redundancy.tsx (3), e2e/sbom-upload-report.spec.ts (3) |
| signin | auth/handlers.go (6), auth/handlers_test.go (6), auth/session.go (4), auth/middleware_test.go (4), web/app/lib/require-auth.tsx (4) |
| enrichment | catalog/openrouter.go (4), catalog/queue_test.go (4), catalog/pipeline.go (3), catalog/queue.go (3) |
| foundations | web/app/routes.ts (8), web/app/root.tsx (3) |

## Trend

| Capability | W41 | W42 | W43 | W44 (2 days) | Label | Campaign (from commit scopes) |
|---|---|---|---|---|---|---|
| platform | 0 | 12 | 4 | 17 | seasonal | W42 data-layer-connectivity / scaffold; W44 prod-deploy + observability-logging-queue-supervision |
| catalog | 0 | 7 | 5 | 10 | constant | W42 seed-classification-catalog, mark-strategic-libraries; W44 testing-* hardening |
| redundancy | 0 | 1 | 13 | 11 | rising | W43 project-redundancy-report, projects-empty-list-null-fix; W44 batch-status-uncategorized-libraries, empty-batch-status |
| signin | 0 | 5 | 6 | 7 | constant | W42 minimal-auth-role-scaffold; afterwards touched by other changes |
| enrichment | 0 | 2 | 0 | 8 | rising (spike) | W44 testing-pipeline-input-hardening, batch-status-*, observability |
| foundations | 0 | 4 | 7 | 1 | fading | W43 redundancy-view-brand-tokens / views-brand-tokens-rollout |
| users | 0 | 1 | 5 | 1 | seasonal | W43 org-data-fixes-and-user-management |
| landing | 0 | 3 | 5 | 0 | fading | W43 app-nav-menu, home-landing-page |
| orgview | 0 | 1 | 3 | 0 | fading | brand-token and null-fix spillover |
| observability | 0 | 0 | 0 | 2 | new | W44 observability-logging-queue-supervision |
| strategic | 0 | 1 | 0 | 0 | one-off (path-only; real work under catalog) | mark-strategic-libraries |

These labels describe a 17-day window with four weekly buckets, and their confidence is low. (inference)

## Fix pressure

Heuristic: a commit counts as a fix if its type is `fix`, or if its subject, with the `type(scope):` prefix removed, matches `fix|bug|hotfix|revert|regression|broken|incident`. `chore(archive)` commits are excluded, and so is the phrase "regression tests". The scope is stripped because change-ids contain "fix" (`projects-empty-list-null-fix`, `org-data-fixes-and-user-management`). Matching the full raw subject gives 31 fix commits. The refined count is 23. Of those 23, 11 are **review fixes**, follow-up commits that address `impl-review`/`plan-review` findings. The 10x workflow produces one for most changes, so they are a process step and not field defects. Reverts: **0**. (evidence)

| Capability | Commits | Fix commits | Fix share | Review fixes | Other fixes | Fixes touching >3 caps |
|---|---|---|---|---|---|---|
| redundancy | 25 | 11 | 0.44 | 3 | 8 | 4 |
| users | 7 | 3 | 0.43 | 1 | 2 | 2 |
| enrichment | 10 | 4 | 0.40 | 2 | 2 | 4 |
| catalog | 22 | 8 | 0.36 | 4 | 4 | 4 |
| signin | 18 | 6 | 0.33 | 4 | 2 | 4 |
| platform | 33 | 8 | 0.24 | 5 | 3 | 5 |
| orgview | 4 | 1 | 0.25 | 0 | 1 | 1 |
| foundations | 12 | 2 | 0.17 | 1 | 1 | 1 |
| landing | 8 | 1 | 0.12 | 1 | 0 | 0 |
| observability | 2 | 1 | 0.50 | 1 | 0 | 1 |
| docs | 121 | 22 | 0.18 | 11 | 11 | 7 |

- **redundancy** has the strongest non-mechanical fix signal. Its fix commits are product-behaviour fixes aimed at the capability: empty batch settles as done (23ecc11), batch-status truthfulness backend + UI (ad2e7dd, 4b1fc0b), list endpoints never null (199542d), frontend null fallbacks (0fa5df8), cancel stale polling on new upload (c5da1df), check session before DB writes in upload (94016a6). The two dedicated fix changes `projects-empty-list-null-fix` and `batch-status-uncategorized-libraries` / `empty-batch-status` both centre on redundancy. Some of them spill into catalog, users, orgview and enrichment (rule 12). (evidence)
- **catalog** fixes are mainly about the strategic cap (aabdac3, 74daf0f) and null lists. Strategic marks shows no fixes only because its code is attributed to catalog. (evidence)
- **users** shows 0.43 on 7 commits. Two of its three fixes are cross-cutting commits (4838ab7 prod-deploy safety, 0fa5df8 null fallbacks), so the signal is weak. (inference)
- **platform** fixes are prod-deploy and data-layer impl-review findings plus "release-readiness quick fixes". This is iteration work (rule 5), so it is demoted. (evidence)
- Most fix commits in enrichment, signin and observability are multi-capability impl-review fixes (426a486, b142e00, 88be15a, each touching 5-6 capabilities). Their spillover inflates small capabilities (rule 12). (evidence)

## Co-change between capabilities

Commits touching more than 6 of 13 capabilities are dropped (none reached 7; the largest touched 6). Confidence = shared ÷ commits of the less active side. Pairs with docs are omitted from the ranking because docs rides along with almost every commit (rule 8): confidence 0.9-1.0 with every capability.

| Pair | Support | Confidence | Note |
|---|---|---|---|
| platform – signin | 12 | 0.67 | mostly via main.go route/middleware wiring (rule 2) |
| catalog – redundancy | 11 | 0.50 | shared `backend/internal/catalog` package: store.go/handlers.go and project*.go |
| catalog – platform | 8 | 0.36 | main.go route wiring, migrations |
| catalog – signin | 7 | 0.39 | role checks in catalog handlers/tests |
| enrichment – platform | 7 | 0.70 | queue started from main.go; migration 0007 |
| platform – redundancy | 6 | 0.24 | |
| enrichment – redundancy | 5 | 0.50 | batch status: report shows pipeline state |
| signin – users | 5 | 0.71 | same `internal/auth` package; DB-backed users replaced .env accounts |
| catalog – enrichment | 5 | 0.50 | same package, pipeline writes catalog rows |
| foundations – landing | 5 | 0.62 | routes.ts + nav/dashboard |
| catalog – orgview | 3 | 0.75 | orgview UI reads catalog data |

Small counts (3-12): treat as indicative. (evidence)

## Hub files and freshness

Partners count only if the capability was seen in at least 2 commits with the file.

| File | Own capability | Distinct partner caps (≥2) | What it is | Verdict |
|---|---|---|---|---|
| backend/cmd/shelfmark/main.go | platform | 9 | server bootstrap + route table | rule 2: wiring by design, mechanical. It is still the single most-changed file (16) |
| web/app/routes/dashboard.tsx | landing | 6 | dashboard linking to every view | UI aggregator, low risk |
| web/app/routes.ts | foundations | 5 | client route table | rule 2: config as code, mechanical |
| backend/internal/catalog/store.go | catalog | 5 | single store for catalog + strategic + project queries | real shared-code hub: three capabilities share one file (inference) |
| backend/internal/catalog/handlers.go / handlers_test.go | catalog | 5 | HTTP handlers for catalog + strategic marks | same as above |
| backend/internal/catalog/project_handlers_test.go | redundancy | 5 | redundancy API tests | |
| backend/internal/auth/handlers_test.go | signin | 5 | auth handler tests also covering users | |
| web/app/routes/catalog.tsx, organization.tsx | catalog / orgview | 5 | views touched by brand-token rollout and null-fix campaigns | rule 4: shared cause (campaigns), count once |

Freshness: `git cat-file -e HEAD:<path>` was run for all 29 named files. All exist except `web/app/welcome/welcome.tsx`, a deleted scaffold file (rule 11, history only). (evidence)

## Mechanical signals

- docs: 49% of changes and 121 of 125 commits. This is planning churn from the 10x workflow plus 17 `chore(archive)` renames (rule 8). It is never ranked.
- main.go and routes.ts hub status: route tables and wiring (rule 2).
- Platform fix share: iteration on deploy and CI (rule 5).
- 11 review-fix commits: a process step of the 10x chain. Each one touches many capabilities and inflates small ones (rule 12).
- Raw keyword matching inflated the fix count from 23 to 31 because change-ids contain "fix". The scope was stripped.
- Agent co-authorship (80%) is activity, not people (rule 7). With a single human, ownership and concentration are a baseline (rule 6).

## Unknowns

- **Strategic marks split:** its activity and fixes are hidden inside catalog/store.go and catalog/handlers.go. Function-level attribution such as `git log -L` per function would be needed to separate them. (unknown)
- **Missing co-change:** history cannot show backend and frontend twins that should change together but did not. Candidates: Go JSON structs in `catalog/project.go` / `store.go` vs the hand-kept TS types in `web/app/lib/projects.ts`, `catalog.ts`, `organization.ts`, `users.ts`. A `json_contract_test.go` exists for catalog only, so whether a guard covers the other pairs is unknown (rule 3 not verifiable from history).
- **Short window:** 17 days and 4 buckets (the last one only 2 days long), so no long-run trend is possible.
- Issue-label-based fix detection was not used. The forge was not queried for bug labels (that belongs to the discussion agent).
- Reverts = 0. There is no evidence about rollback behaviour.

Commands run:
- read `.work/scan-contract.md`, `.work/capabilities.tsv`, `.work/attribute.py`, `references/false-signals.md`, `references/agent-history.md`
- `python3 -I hist.py` (scratchpad): parses `.work/git-log.txt` with the same attribution as `attribute.py` (globs first, then the longest prefix; for renames, the new path), noise filter; changes/commits/agent/fix per capability, weekly buckets, co-change pairs (≤6 caps), hub files → `.work/history-raw.tsv`, `.work/history-fixsubjects.json`
- `grep -n welcome .work/git-log.txt`; `awk` over the raw TSV for fix subjects and weekly campaign scopes; per-day commit counts
- refined fix classification (scope stripped, archive excluded, review vs other) via `python3 -I`
- `git -c core.quotepath=off cat-file -e HEAD:<path>` for 29 named files
