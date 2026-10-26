---
project: shelfmark
researched_at: 2026-10-26
recommended_platform: Fly.io
runner_up: Render
context_type: mvp
tech_stack:
  language: Go 1.27 + TypeScript
  framework: Go standard library (net/http) + React Router v8 (SPA mode)
  runtime: single distroless Docker image (Go binary serves API + static SPA), PostgreSQL 16
---

## Recommendation

**Deploy on Fly.io** — one `shared-cpu-1x` Machine in `fra`, plus Fly Managed Postgres (MPG) in the same region.

Shelfmark needs a process that stays up between requests. The catalog queue runs as in-process goroutines making GitHub and LLM calls, and it needs a ~30 s graceful drain. That rules out Netlify, Vercel and Cloudflare. Of the three container PaaS left, Fly.io and Render tied on the five agent-friendly criteria (4 Pass + 1 Partial each). Fly.io wins the tie on the most heavily weighted criterion: the whole operational loop, rollback included, runs through `flyctl`, while Render's rollback lives only in the API or dashboard. `tech-stack.md` already names `deployment_target: fly`. Interview answers: always-on process required, DX over cost, one region, a co-located database preferred, no prior platform familiarity.

## Platform Comparison

| Platform | CLI-first | Managed/Serverless | Agent-readable docs | Stable deploy API | MCP / Integration | Total |
|---|---|---|---|---|---|---|
| Cloudflare | Partial | Partial | Pass | Pass | Pass | filtered out |
| Vercel | Pass | Pass | Pass | Pass | Pass | filtered out |
| Netlify | Partial | Pass | Pass | Pass | Pass | filtered out |
| **Fly.io** | Pass | Pass | Pass | Pass | Partial | **4P + 1Pa** |
| Render | Partial | Pass | Pass | Pass | Pass | 4P + 1Pa |
| Railway | Partial | Partial | Pass | Pass | Pass | 3P + 2Pa |

Notes per platform (all checked 2026-10-26):

- **Cloudflare**: Go runs on Workers only as WASM. Containers have been GA since 2026-04-13, but each instance sleeps after `sleepAfter` (default 10 min), so it stays up only through keepalive tricks. Placement isn't pinned to a region. There is no native Postgres (D1 is SQLite; Postgres comes via Hyperdrive or PlanetScale). It fails the always-on hard filter.
- **Vercel**: The Go runtime is **beta**, and a `Dockerfile.vercel` image runs as a request-scoped Function. Instances pause after the last request ("not permanent background daemons"). Queues and Daemons are both **beta**, and Queues has JS/Python SDKs only. Hobby is non-commercial. It fails the always-on hard filter.
- **Netlify**: No containers and no long-running server. Go works only through the Lambda-compat API, which is **deprecated** and stops accepting deploys on 2027-07-01. Background functions are capped at 15 min. It fails the always-on hard filter.
- **Fly.io**: Any Docker image runs as a Firecracker Machine. `auto_stop_machines` defaults to off, and `kill_timeout` goes up to 300 s. Docs are available as `llms.txt` and `.md`. `fly mcp server` is **experimental**. MPG release stage is **unconfirmed**: the 2026-06-09 changelog said "beta" and no GA announcement was found. MPG v2 is live in AMS/FRA/IAD/ORD/DFW/YYZ. Cost is about $45–50/mo (MPG Basic $38 + Machine ~$5).
- **Render**: Native `go` and `docker` runtimes. The CLI covers deploy and logs; **rollback is API/dashboard only**. Docs are available as `llms.txt` and `.md`. A hosted MCP exists, but it can't roll back or scale. Postgres in `frankfurt` is $6–19/mo. Deploys are zero-downtime (when the service has no disk). `maxShutdownDelaySeconds` defaults to 30. Cost is about $13–26/mo.
- **Railway**: Dockerfile is auto-detected. Rollback is dashboard-only. Postgres is a Railway-maintained template container, not fully managed, and its docs say backups are "still under development". `RAILWAY_DEPLOYMENT_DRAINING_SECONDS` defaults to **0**. The EU region (Amsterdam) is GA. Cost is about $5–15/mo on Hobby.

### Shortlisted Platforms

#### 1. Fly.io (Recommended)

Fly.io gives a full CLI loop (`fly deploy`, `fly logs`, `fly releases`, redeploying an older image) with no dashboard dependency. A Machine runs the existing distroless image unchanged and stays up by default. `kill_timeout = 30` with `kill_signal = "SIGTERM"` covers the queue's drain, and the app already handles both SIGINT and SIGTERM. The database (MPG) sits in the same region as the app.

#### 2. Render

Render tied on score and is stronger on the data layer: Postgres is GA and cheaper, deploys are zero-downtime by default, and the default 30 s shutdown delay already matches the app. It lost the tie on the heavily weighted CLI criterion, because rollback isn't a CLI command.

#### 3. Railway

Railway is the cheapest and has good DX and a hosted MCP. It is weaker on managed-ness: Postgres runs as a template container and backups are "under development". Rollback is dashboard-only, and the drain default of 0 s would need configuring.

## Anti-Bias Cross-Check: Fly.io

### Devil's Advocate — Weaknesses

1. **MPG's maturity is unproven.** The last explicit label (2026-06-09) was "beta" and no GA announcement was found, yet MPG is about 80% of the monthly bill. Falling back to legacy unmanaged Fly Postgres would make backups and upgrades our job.
2. **A single Machine with the `rolling` strategy means a short outage on every deploy.** `bluegreen` needs a health check. Running two Machines would mean two in-process queues both running the boot `Sweep` over the same `pending` rows, a concurrency case nobody has tested.
3. **Migrations run on boot (`db.Migrate` in `main.go`), not as a `release_command`.** A bad migration crash-loops the only Machine. Rolling back the image does not roll back the schema.
4. **There is no free tier, only a 2 h / 7 day trial.** A card is required from day one, and a misconfigured MPG plan stays billed.
5. **The rollback command is unverified.** The releases docs list no `rollback` subcommand. The expected path is `fly deploy --image <previous-ref>`, which needs checking against `fly deploy --help` before anyone relies on it.

### Pre-Mortem — How This Could Fail

The team shipped Shelfmark on one Fly Machine with MPG in `fra`. They read "managed" as "GA" and never test-restored a backup. `kill_timeout` was left at the default 5 s, so every deploy cut LLM calls off mid-flight. The boot sweep re-enqueued them, which quietly doubled OpenRouter spend during busy upload weeks. In month three, migration 0009 hit a lock on a large table and failed on boot. The Machine crash-looped, and `/healthz` never went green. They rolled back the image, which put old code on a half-migrated schema. Debugging was slow: the distroless image has no shell, so `fly ssh console` gave them nothing, and the only signal was `fly logs`. Worse, a connection error during boot printed the full `DATABASE_URL`, password included, into those logs. They then learned the MPG Postgres major version is fixed at creation, so moving 16 → 17 to pick up a fix needed a dump and restore. None of it was Fly's fault alone: each assumption was left unchecked because the deploy was "just one `fly deploy`".

### Unknown Unknowns

- `kill_signal` defaults to **SIGINT**, not SIGTERM, and `kill_timeout` defaults to **5 s**. Both must be set explicitly in `fly.toml`.
- Private traffic between regions has been **billed since February 2026**. The app and MPG must both be in `fra`.
- Fly HTTP health checks **don't follow redirects**. `/healthz` has to answer 200 directly.
- The MPG Postgres major version is **fixed at creation**, and there's no in-place upgrade. Local dev runs `postgres:16-alpine`, so choose 16 to match, or move dev to 17 first.
- The distroless image has **no shell**. `fly ssh console` and one-off `psql` inside the Machine are unavailable. Use `fly mpg connect` / a proxy and `fly logs` instead.

## Operational Story

- **Preview deploys**: None built in. For M1, verify on a throwaway app (`fly deploy --app shelfmark-staging`) when needed. There are no per-PR previews.
- **Secrets**: `fly secrets set KEY=value` (stored encrypted in Fly's vault, set as env vars at boot, unreadable via the CLI after setting). Required: `DATABASE_URL` (set by `fly mpg attach`), `OPENROUTER_API_KEY`, `GITHUB_TOKEN` (if used), session secret, seed account env. To rotate, run `fly secrets set` again; it triggers a restart.
- **Rollback**: `fly releases --image` to find the previous image ref, then `fly deploy --image <ref>` (verify the flags against `fly deploy --help`). Expect about 1–2 min. **Migrations don't roll back.** Keep migrations backward-compatible with the previous release.
- **Approval**: A human runs anything that costs money or destroys data: creating or destroying the MPG cluster, `fly apps destroy`, changing the Machine size or plan, rotating the primary DB credentials, and the first production deploy. An agent may run `fly deploy` to an existing app after green CI, plus `fly logs`, `fly status`, `fly releases` and `fly secrets list` unattended.
- **Logs**: Read-only via `fly logs --app shelfmark` (stdout, JSON from `obs.NewLogger`), `fly status`, `fly checks list`, `fly machine status <id>`. For Postgres: `fly mpg status` (verify the subcommand name).

## Risk Register

| Risk | Source | Likelihood | Impact | Mitigation |
|---|---|---|---|---|
| MPG not GA; backup or restore behaves unexpectedly | Research finding / Devil's advocate | M | H | Re-check MPG status before creating the cluster. Run one test restore before go-live and record it in the prod-deploy change. |
| A deploy kills in-flight queue work (default 5 s timeout) | Unknown unknowns | H (if unset) | M | Set `kill_signal = "SIGTERM"` and `kill_timeout = 30` in `fly.toml`. |
| Downtime on every deploy with a single Machine | Devil's advocate | H | L | Accept for M1 (small user base). Revisit with 2 Machines only after the double-sweep concurrency is tested. |
| A bad boot migration crash-loops the only Machine | Pre-mortem | L | H | Keep migrations additive. Test `migrate up` against a copy first. Consider `release_command = "/app/shelfmark migrate"` later. |
| Rolling back the image leaves the schema ahead | Devil's advocate | M | M | Expand-then-contract migrations. Rollback runbook in the prod-deploy change. |
| `DATABASE_URL` password or seed email leaks into boot error logs (impl-review F3) | Pre-mortem | M | H | Redact the DSN and seed values in the `fatal()` paths before the first prod deploy. |
| App and DB in different regions (latency + billed private traffic) | Unknown unknowns | L | M | `primary_region = "fra"`, and create MPG in `fra`. |
| Postgres major version locked at creation | Unknown unknowns | M | L | Choose PG 16 to match dev and CI, or bump dev to 17 before creating MPG. |
| Rollback command unverified | Research finding | M | M | Run `fly deploy --help` and do a dry rollback on the first deploy. Record the exact command in the runbook. |
| Debugging blind because distroless has no shell | Unknown unknowns | M | L | Rely on structured stdout logs and `/healthz`. Use `fly mpg connect` / proxy for DB inspection. |
| `fly mcp server` experimental; agent tool surface changes | Research finding | M | L | Use plain `flyctl` commands in skills and hooks. Treat MCP as optional. |
| Invalid `OPENROUTER_API_KEY` fails every library with 401 | Research finding (test-plan Phase 1) | H (current key) | H | Set a fresh key via `fly secrets set` and smoke-test one upload after deploy. |

## Getting Started

`flyctl` is not installed on this machine yet, so verify every command against `fly <cmd> --help` before scripting it.

1. Install and log in: `curl -L https://fly.io/install.sh | sh`, then `fly auth login` (a human step; a card is required).
2. Create the app without deploying: `fly launch --no-deploy --name shelfmark --region fra`. The existing root `Dockerfile` is reused. Then edit `fly.toml`: `internal_port = 8080`, `auto_stop_machines = "off"`, `min_machines_running = 1`, `kill_signal = "SIGTERM"`, `kill_timeout = 30`, and an HTTP check on `GET /healthz`.
3. Create Postgres in the same region and attach it: `fly mpg create` (region `fra`, PG 16), then `fly mpg attach` to set the `DATABASE_URL` secret. Set the rest with `fly secrets set OPENROUTER_API_KEY=… …`.
4. Deploy with a version stamp: `fly deploy --build-arg VERSION=$(git rev-parse --short HEAD)`. Migrations run on boot, and the queue's boot sweep re-enqueues orphaned libraries.
5. Verify: `fly status`, `fly logs`, `curl https://shelfmark.fly.dev/healthz`, then one login → upload → report smoke test.

## Out of Scope

The following were not evaluated in this research:
- Docker image configuration (the existing `Dockerfile` is reused as-is)
- CI/CD pipeline setup (the GitHub Actions auto-deploy is decided in `tech-stack.md`, not here)
- Production-scale architecture (multi-region, HA, DR)
