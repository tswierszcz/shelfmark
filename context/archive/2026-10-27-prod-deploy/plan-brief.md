# Production Deploy (Fly.io) — Plan Brief

> Full plan: `context/changes/prod-deploy/plan.md`
> Infrastructure decision (acts as research): `context/foundation/infrastructure.md`

## What & Why

Ship Shelfmark M1 to production on Fly.io (`fra`, one Machine plus Managed Postgres 16) at `https://shelfmark.fly.dev`. The goal is a real, repeatable release: no secrets in logs, a session cookie that is safe over HTTPS, every later deploy coming from `main` via CI, and rollback and backup restore proven once before anyone relies on them.

## Starting Point

There is a working distroless `Dockerfile` with version stamping and SIGTERM-aware graceful shutdown (25 s budget). There is no `fly.toml`, no CI, and `flyctl` is not installed. Two code issues only bite in production: boot errors can print the `DATABASE_URL` password and a seed email (impl-review F3), and the cookie's `Secure` flag depends on `r.TLS`, which is always nil behind Fly's proxy. `main` has one README commit and no shared history with `shape/shelfmark`.

## Desired End State

`/healthz` on `shelfmark.fly.dev` is green on one Machine in `fra` running the `main` image, with the git SHA in its logs. Login → catalog upload (live OpenRouter key) → SBOM report works with a `Secure` cookie. PRs run Go and web checks, and a push to `main` auto-deploys. `docs/runbook.md` holds verified deploy, rollback and restore commands.

## Key Decisions Made

| Decision | Choice | Why (1 sentence) | Source |
| --- | --- | --- | --- |
| Platform / topology | Fly.io `fra`, 1 `shared-cpu-1x` Machine + MPG PG16 | Always-on queue, CLI-first ops, DB co-located; PG16 matches dev | Research |
| Shutdown | `kill_signal=SIGTERM`, `kill_timeout=30` | Fly defaults (SIGINT/5 s) would cut the 25 s queue drain | Research |
| Getting code onto `main` | Merge with `--allow-unrelated-histories` via PR | No force-push; PR is the release record and the CI hook | Plan |
| CI/CD scope | Tests on PR + `flyctl deploy` on push to `main` | Matches `tech-stack.md` auto-deploy-on-merge; deploys stop depending on a laptop | Plan |
| Migrations | Keep on boot (no `release_command`) | No new code path for M1; migrations kept additive instead | Plan |
| Pre-prod verification | Local run of the exact prod image, no Fly staging app | Catches image and config bugs at zero cost; Fly-specific bits are verified in Phase 4 | Plan |
| Backup restore drill | In scope, after the first deploy | MPG GA status is unconfirmed; the first restore shouldn't happen mid-incident | Plan |
| Domain | `shelfmark.fly.dev` | No DNS work; enough for M1 and the course submission | Plan |
| Secure cookie behind proxy | `Secure` if `r.TLS != nil` or `X-Forwarded-Proto: https` | Fly terminates TLS; a spoofed header only makes the cookie stricter | Plan |

## Scope

**In scope:**
- F3 redaction (DSN and seed email), Secure cookie fix, F4 comment, each with a regression test
- `fly.toml`, `.dockerignore` hygiene, `docs/runbook.md`
- Joining `shape/shelfmark` with `main` (`--allow-unrelated-histories`) so a PR can be opened
- `.github/workflows/ci.yml` (backend, web and deploy jobs)
- Provisioning the app, MPG and secrets (new OpenRouter key), plus the first manual deploy and smoke test
- Merging to `main`, the CI auto-deploy, and the rollback and restore drills

**Out of scope:** Fly staging app, `release_command`, multiple Machines or bluegreen deploys, custom domain, E2E in CI, the impl-review-ci workflow, PR previews, Postgres upgrade.

## Architecture / Approach

Code fixes come first, so nothing unsafe ever reaches production logs. Next comes a reproducible deploy (`fly.toml` + CI), verified locally against the real image. The billed and irreversible steps follow with a human at the keyboard: the first deploy is manual from the branch, then `FLY_API_TOKEN` goes into GitHub, and the merge to `main` triggers a second deploy through CI against the already-healthy app. The drills run last, against real data.

## Phases at a Glance

| Phase | What it delivers | Key risk |
| --- | --- | --- |
| 1. Prod-safety code fixes | No secrets in boot errors; Secure cookie behind proxy | Seed-failure test needs a reliable way to force an insert error |
| 2. fly.toml + local prod-image check | Fly config, verified image (version, healthz, drain), runbook draft | `fly.toml` keys unvalidated until `flyctl` is installed |
| 3. History join + CI/CD workflow | Branch joined with `main` (PR possible), PR checks + deploy-on-main job | Docker-in-CI test runtime; action version pins |
| 4. Provision + first deploy (human) | Live app with MPG, secrets, smoke-tested loop | App name taken; MPG commands differ from docs; billing starts |
| 5. Merge PR + ops drills | `main` = full history (merge commit, not squash), CI deploy proven, rollback and restore verified | Rollback leaves the schema ahead; temporary cluster cost |

**Prerequisites:** a Fly account with a card (human), a fresh OpenRouter API key, chosen seed account emails and passwords, and GitHub repo admin rights for the secret and the merge.
**Estimated effort:** ~3–4 sessions; Phases 1–3 agent-driven, Phases 4–5 paired with the human.

## Open Risks & Assumptions

- MPG release status is unconfirmed (was "beta" in June 2026); it is re-checked before creating the cluster.
- `fly mpg …` and `fly deploy --image` flags come from docs, not from runs; each is verified with `--help` before use.
- The app name `shelfmark` might be taken on Fly; the fallback is renaming in `fly.toml` and the runbook.
- Single-Machine deploys mean brief downtime on every deploy; accepted for M1.
- pgx's DSN redaction is only best effort, so `Connect` gets the same fixed-message treatment as `Migrate`.

## Success Criteria (Summary)

- A real user can log in, categorize a catalog and see a redundancy report on `https://shelfmark.fly.dev`.
- Merging to `main` deploys automatically, with the commit SHA visible in production logs.
- Rollback and backup restore have each been performed once and documented.
