# Shelfmark operations runbook

Status: draft. Commands marked **(verify on first use)** were not yet run against a real Fly account. Replace each marker with the exact command that worked once it has been run.

## Overview

- **App**: `shelfmark` on Fly.io, served at https://shelfmark.fly.dev.
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
2. Create the app, keeping the committed `fly.toml`: `fly apps create shelfmark`. If the name is taken, choose another, update `app` in `fly.toml`, and use the new `*.fly.dev` host everywhere.
3. Validate the config: `fly config validate`.
4. Re-check MPG's release status, then create the cluster in `fra` on PG 16 with the smallest plan and attach it (sets `DATABASE_URL`): `fly mpg create` then `fly mpg attach` **(verify on first use: flags, plan names)**.
5. Stage the remaining secrets so they apply on the first deploy: `fly secrets set --stage SESSION_SECRET=... OPENROUTER_API_KEY=... ADMIN_EMAIL=... ADMIN_PASSWORD_HASH='...' ...` (see [Secrets](#secrets)).
6. First deploy from a clean working tree: see [Deploy](#deploy) → Manual.
7. Smoke test: `curl -fsS https://shelfmark.fly.dev/healthz` returns `ok`; log in as admin and check the `shelfmark_session` cookie is `Secure; HttpOnly; SameSite=Lax`; upload a small catalog and confirm libraries get categorized (no 401 in logs); upload an SBOM as architect and see the redundancy report.
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
curl -fsS https://shelfmark.fly.dev/healthz
```

- Every log line carries `version`; confirm it matches the deployed sha.
- Database: `fly mpg connect` or a local proxy, and `fly mpg status` **(verify on first use: subcommand names)**.
- A boot failure logs `db: ...` or `auth: ...` and exits 1. These messages never contain the `DATABASE_URL` password or seed emails.

## Rollback

**(verify on first use)** — the exact commands get recorded here after the rollback drill.

1. Find the previous image reference: `fly releases --image`.
2. Deploy it: `fly deploy --image <previous-ref>`. Expect about 1–2 min.
3. Confirm `/healthz` returns `ok` and `fly logs` shows the older `version`.
4. To roll forward again, re-run the CI deploy or `fly deploy --image <current-ref>`.

The schema is not rolled back. If the newer release ran a non-additive migration, the old image may fail on it; fix forward instead.

## Backup & restore

MPG takes backups automatically. Its GA status is unconfirmed, so a restore must be tested once before relying on it.

**(verify on first use)** — the exact commands get recorded here after the restore drill.

1. List backups and restore the latest into a new temporary cluster or fork with `fly mpg` (check `fly mpg --help` for the backup/restore subcommands).
2. Connect via `fly mpg connect` or a proxy and compare row counts of `users`, `libraries` and `projects` with production.
3. Destroy the temporary cluster (human; it is billed while it exists).

Last drill: not yet run.

## Known limits

- One Machine: brief downtime on every deploy and no failover. Running two Machines needs the double boot-sweep case tested first.
- Migrations run on boot (no `release_command`); a bad one crash-loops the only Machine.
- No staging app and no PR previews. Verify changes locally with the production image.
- MPG major version (PG 16) is fixed; upgrading means dump and restore.
- App and DB must both stay in `fra`: cross-region private traffic is billed.
- Fly health checks don't follow redirects; `/healthz` must keep answering 200 directly.
- `fly mcp server` is experimental; use plain `flyctl` commands in scripts.
