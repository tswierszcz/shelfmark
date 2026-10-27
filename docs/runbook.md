# Shelfmark operations runbook

Status: draft. Commands marked **(verify on first use)** were not yet run against a real Fly account. Replace each marker with the exact command that worked once it has been run.

## Overview

- **App**: `shelfmark-tsw` on Fly.io, served at https://shelfmark-tsw.fly.dev.
- **Compute**: one always-on `shared-cpu-1x` / 512 MB Machine in `fra`. Config lives in `fly.toml` (repo root).
- **Database**: Fly Managed Postgres (MPG), PG 16, in `fra`. The major version is fixed at creation; there is no in-place upgrade.
- **Image**: the root `Dockerfile` builds one distroless image. The Go binary serves the API and the built SPA on port 8080. `VERSION` (git sha) is baked in at build time and appears as `version` on every JSON log line.
- **Boot**: connect to Postgres → run migrations → seed accounts (only if `users` is empty) → start the queue's boot sweep → listen. `/healthz` pings the DB and returns `200 ok`.
- **Shutdown**: Fly sends SIGTERM and waits `kill_timeout = 30` s. The app drains HTTP and the catalog queue within a 25 s budget. Libraries left unfinished are re-enqueued by the boot sweep on the next start.
- **HTTPS**: TLS terminates at Fly's proxy; `force_https` redirects plain HTTP. The session cookie is still `Secure` because the app honors `X-Forwarded-Proto: https` from the proxy.

### Who runs what

- **Agent may run unattended**: `fly deploy` to the existing app, `fly logs`, `fly status`, `fly releases`, `fly secrets list`.
- **Human only**: anything that costs money or destroys data — creating or destroying the MPG cluster, `fly apps destroy`, scaling or resizing Machines, rotating DB credentials, and the first production deploy.

## Secrets

Set with `fly secrets set` (stored encrypted, injected as env vars at boot, not readable back through the CLI). Setting a secret restarts the Machine unless `--stage` is used. Never commit values or paste them into logs or chat.

| Name | Required | Notes |
|---|---|---|
| `DATABASE_URL` | yes | Set by `fly mpg attach`. Do not set by hand. |
| `SESSION_SECRET` | yes | Random, at least 32 bytes, e.g. `openssl rand -base64 48`. Rotating it logs everyone out. |
| `OPENROUTER_API_KEY` | yes | Must be a **new** key; the old one returns 401 and every library fails categorization. |
| `OPENROUTER_MODEL` | no | Overrides the default model. |
| `GITHUB_TOKEN` | no | Raises GitHub API rate limits for the catalog queue. |
| `ADMIN_EMAIL`, `ADMIN_PASSWORD_HASH` | yes | Seed account (admin). |
| `ARCHITECT_EMAIL`, `ARCHITECT_PASSWORD_HASH` | yes | Seed account (architect). |
| `VIEWER_EMAIL`, `VIEWER_PASSWORD_HASH` | yes | Seed account (viewer). |

Seed accounts are created only when the `users` table is empty (first boot). Changing the seed secrets later has no effect on existing users.

Generate a password hash (reads the password on stdin, prints a bcrypt hash):

```sh
cd backend && go run ./cmd/hashpw
```

**Pitfall: always single-quote bcrypt hashes.** A hash looks like `$2a$10$...`. Unquoted or in double quotes, the shell expands `$2`, `$10` etc. and silently corrupts it, so login then fails.

```sh
fly secrets set ADMIN_PASSWORD_HASH='$2a$10$...'   # correct
fly secrets set ADMIN_PASSWORD_HASH="$2a$10$..."   # WRONG: hash is mangled
```

## First-time setup

Human-run, once. Check each command with `fly <cmd> --help` first.

1. Install `flyctl` and log in: `fly auth login` (a card is required; there is no free tier).
2. Create the app, keeping the committed `fly.toml`: `fly apps create shelfmark-tsw --org personal` (done 2026-10-27; `shelfmark` was already taken on Fly, so the app and host use `shelfmark-tsw`).
3. Validate the config: `fly config validate`.
4. Re-check MPG's release status, then create the cluster in `fra` on PG 16 with the smallest plan and attach it (sets `DATABASE_URL`): `fly mpg create` then `fly mpg attach` **(verify on first use: flags, plan names)**.
5. Stage the remaining secrets so they apply on the first deploy: `fly secrets set --stage SESSION_SECRET=... OPENROUTER_API_KEY=... ADMIN_EMAIL=... ADMIN_PASSWORD_HASH='...' ...` (see [Secrets](#secrets)).
6. First deploy from a clean working tree: see [Deploy](#deploy) → Manual.
7. Smoke test: `curl -fsS https://shelfmark-tsw.fly.dev/healthz` returns `ok`; log in as admin and check the `shelfmark_session` cookie is `Secure; HttpOnly; SameSite=Lax`; upload `docs/samples/smoke-catalog.csv` and confirm libraries get categorized (no 401 in logs); upload `docs/samples/smoke-sbom.cdx.json` as architect and see the redundancy report (jackson-databind/gson and guava/commons-lang3 overlap on purpose). Done 2026-10-27, first deploy of 55f360d.
8. For CI: create a deploy token (`fly tokens create deploy`) and store it as the GitHub repo secret `FLY_API_TOKEN`.

## Deploy

**CI (normal path).** Every push to `main` runs the backend and web checks, then deploys with:

```sh
flyctl deploy --remote-only --build-arg VERSION=<commit sha>
```

The workflow is `.github/workflows/ci.yml`. The deploy job skips with a notice when `FLY_API_TOKEN` is not set.

**Manual.** From a clean working tree on the commit you want live:

```sh
fly deploy --build-arg VERSION=$(git rev-parse --short HEAD)
```

Every deploy has a short outage: there is one Machine and the `rolling` strategy. Migrations run on boot; a failing migration crash-loops the Machine (see [Rollback](#rollback)).

**Migrations don't roll back.** Rolling back the image leaves the schema where the newer release put it. Keep every migration additive and backward-compatible with the previous release (expand, then contract in a later release). Test `migrate up` against a copy of the data before shipping anything risky.

## Observe

The image is distroless: there is no shell, so `fly ssh console` and in-Machine `psql` are not available. Use logs, the health check and a DB proxy instead.

```sh
fly status                      # Machine state, region, check status
fly checks list                 # /healthz check results
fly logs                        # live JSON logs (use --no-tail for a snapshot)
fly releases                    # release history
fly secrets list                # names and digests only, no values
curl -fsS https://shelfmark-tsw.fly.dev/healthz
```

- Every log line carries `version`; confirm it matches the deployed sha.
- Database: `fly mpg connect` or a local proxy, and `fly mpg status` **(verify on first use: subcommand names)**.
- A boot failure logs `db: ...` or `auth: ...` and exits 1. These messages never contain the `DATABASE_URL` password or seed emails.

## Rollback

Verified by a drill on 2026-10-27 (v2 → v1 image → back to v2; each step ~20 s including the single-Machine restart).

1. Find the previous image reference: `fly releases --app shelfmark-tsw --image`.
2. Deploy it: `fly deploy --app shelfmark-tsw --image registry.fly.io/shelfmark-tsw:deployment-<id>`. No build happens; it only replaces the Machine's image (~20 s, brief outage).
3. Confirm `curl -fsS https://shelfmark-tsw.fly.dev/healthz` returns `ok` and `fly logs --app shelfmark-tsw --no-tail` shows the older `version` on the `listening` line.
4. To roll forward again, deploy the newer image the same way, or re-run the CI deploy for the commit on `main`.

The schema is not rolled back. If the newer release ran a non-additive migration, the old image may fail on it; fix forward instead.

## Backup & restore

MPG takes backups automatically (daily full, plus diff/incremental). Its GA status is unconfirmed, so restores are drilled. A restore always creates a **new** cluster and leaves the source untouched.

1. Optionally take a fresh backup and list them: `fly mpg backup create <cluster-id>`, `fly mpg backup list <cluster-id>`.
2. Restore into a temporary cluster: `fly mpg restore <cluster-id> --backup-id <id> --name shelfmark-db-restore-drill` (or `--pitr-time <RFC3339>` for point-in-time). It is billed separately while it exists; in the drill it was ready in 71 s.
3. Compare row counts with production through `fly mpg connect <cluster-id>`, which needs a local `psql` (any `psql` on PATH works, including a small wrapper that runs `docker run --rm -i --network host postgres:16-alpine psql "$@"`):
   `SELECT 'users', count(*) FROM users UNION ALL SELECT 'libraries', count(*) FROM libraries UNION ALL SELECT 'projects', count(*) FROM projects UNION ALL SELECT 'project_libraries', count(*) FROM project_libraries;`
4. Destroy the temporary cluster (human approves): `fly mpg destroy <restored-cluster-id>`.

To actually recover production, restore into a new cluster as above, then point the app at it (`fly mpg detach` the old one, `fly mpg attach` the new one, which updates `DATABASE_URL` and restarts the Machine).

Last drill: 2026-10-27, full backup `20261027-124033F` restored into `shelfmark-db-restore-drill` (fra, Basic); row counts matched production exactly (users 3, libraries 10, projects 4, project_libraries 27); temporary cluster destroyed afterwards.

## Known limits

- One Machine: brief downtime on every deploy and no failover. Running two Machines needs the double boot-sweep case tested first. **The first `fly deploy` of a new app creates 2 Machines (HA) by default**: pass `--ha=false` on that deploy, or run `fly scale count 1 --app shelfmark-tsw` right after (done on 2026-10-27). Later deploys keep the current count.
- `GITHUB_TOKEN` is not set yet, so repo lookups use GitHub's anonymous limit (60 requests/hour). Add a read-only fine-grained token with `fly secrets set GITHUB_TOKEN=... --app shelfmark-tsw` before uploading large catalogs.
- Migrations run on boot (no `release_command`); a bad one crash-loops the only Machine.
- No staging app and no PR previews. Verify changes locally with the production image.
- MPG major version (PG 16) is fixed; upgrading means dump and restore.
- App and DB must both stay in `fra`: cross-region private traffic is billed.
- Fly health checks don't follow redirects; `/healthz` must keep answering 200 directly.
- `fly mcp server` is experimental; use plain `flyctl` commands in scripts.
