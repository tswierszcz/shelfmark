<!-- IMPL-REVIEW-REPORT -->
# Implementation Review: Production Deploy (Fly.io)

- **Plan**: context/changes/prod-deploy/plan.md
- **Scope**: Full plan
- **Reviewed phases**: 1, 2, 3, 4, 5
- **Date**: 2026-10-27
- **Verdict**: NEEDS ATTENTION
- **Findings**: 0 critical, 2 warnings, 3 observations

## Verdicts

| Dimension | Verdict |
|-----------|---------|
| Plan Adherence | WARNING |
| Scope Discipline | WARNING |
| Safety & Quality | WARNING |
| Architecture | PASS |
| Pattern Consistency | WARNING |
| Success Criteria | PASS |

Success criteria re-run on 2026-10-27:
- Phase 1: the 4 named tests and the full backend suite pass.
- Phase 3: actionlint passes.
- Phase 4: `fly config validate` passes; there is 1 started Machine with a passing check; `/healthz` returns ok; HTTP returns 301 to HTTPS; logs contain no DSN, seed email or ERROR.
- Phase 5: a8e0751 is on main, and CI on main succeeded for 9c76949.
- Phase 2: the Docker gates were not re-run. That evidence comes from the implementation run, and the same image is live in production.

## Findings

### F1 — Deploy token exposed to every step of the deploy job

- **Severity**: ⚠️ WARNING
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Safety & Quality
- **Location**: .github/workflows/ci.yml:63-64
- **Detail**: `FLY_API_TOKEN` is set in the job-level `env`. That makes it visible to `actions/checkout` and to the third-party `superfly/flyctl-actions/setup-flyctl@1.6`, which is pinned to a mutable tag. If that action were compromised or retagged, it could read the production deploy token.
- **Fix**: Map the token through step-level `env` on the gate step and the `flyctl deploy` step only, and pin `setup-flyctl` to a full commit SHA, keeping the tag in a comment.
- **Decision**: FIXED — token mapped per step; setup-flyctl pinned to ed8efb3 (tag 1.6); actionlint PASS

### F2 — Runbook still marked draft; production MPG commands and cluster not recorded

- **Severity**: ⚠️ WARNING
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Plan Adherence
- **Location**: docs/runbook.md:3, :57, :97
- **Detail**: Phase 4 §3 asked to replace the "verify on first use" markers with the commands that actually worked, and to record the MPG cluster name and plan. Several things are still missing. Line 3 still says "Status: draft". The `fly mpg create`/`attach` and `fly mpg status`/`connect` lines still carry the markers. The production cluster (`shelfmark-db`, `d2gznoq7ew80pkm8`, fra, Basic, PG 16, 10 GB) is not recorded anywhere.
- **Fix**: Replace the markers with the verified commands, including `mpg create --name shelfmark-db --org personal --region fra --plan Basic --pg-major-version 16 --volume-size 10`, `mpg attach <id> --app shelfmark-tsw` and `mpg list --org personal`. Record the cluster ID and plan, and drop the draft status.
- **Decision**: FIXED — runbook status verified; mpg commands and shelfmark-db details recorded

### F3 — infrastructure.md body uses the old app name; F3-leak risk row not marked done

- **Severity**: ℹ️ OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Plan Adherence
- **Location**: context/foundation/infrastructure.md:81, :92, :108
- **Detail**: `fly logs --app shelfmark` and `curl https://shelfmark.fly.dev/healthz` are stale; production is `shelfmark-tsw`. The risk row about `DATABASE_URL`/seed-email leaks (impl-review F3) has been mitigated by Phase 1, but unlike the two rows next to it, it isn't marked done. The plan limited edits to Mitigation notes, so this is not drift, but the text is now operationally wrong.
- **Fix**: Update the two command lines to `shelfmark-tsw`, and add "Done 2026-10-27 (prod-deploy p1)" to the F3 risk row.
- **Decision**: FIXED — app name updated in infrastructure.md; leak risk row marked done

### F4 — VERSION is a short SHA on manual deploys and a full SHA from CI

- **Severity**: ℹ️ OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Pattern Consistency
- **Location**: docs/runbook.md:66 vs .github/workflows/ci.yml:77
- **Detail**: The `version` log field shows `55f360d` after a manual deploy and `9c76949d0e81…` after a CI deploy, so filtering logs by version depends on how the release was deployed.
- **Fix**: Use the full SHA everywhere: `git rev-parse HEAD` in the runbook's manual deploy command, matching `github.sha` in CI.
- **Decision**: FIXED — runbook manual deploy uses full SHA

### F5 — Unplanned smoke-test sample files

- **Severity**: ℹ️ OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Scope Discipline
- **Location**: docs/samples/smoke-catalog.csv, docs/samples/smoke-sbom.cdx.json
- **Detail**: These files were added during the Phase 4 smoke test at the user's request and are not in the plan. They are harmless: public Maven purls, no PII, nothing that reaches the image. The runbook's smoke test references them.
- **Fix**: Keep them; no change needed beyond acknowledging them here.
- **Decision**: ACCEPTED — kept as the standard smoke-test fixture
