# Repository Guidelines

Shelfmark classifies libraries by capability and reports redundancy per project SBOM. Go 1.27 backend (`backend/`) serves `/api` and the built React Router 8 SPA (`web/`) from one binary; Postgres 16; deployed to Fly.io.

## Hard Rules

- Every Go handler returning a list must marshal `[]`, never `null` (`make([]T, 0)`); web callers still fall back with `?? []`. See @context/foundation/lessons.md.
- Route files use the `@theme` tokens in `web/app/app.css` (`bg-accent`, `text-ink-faint`, `border-rule`), never `gray-*`/`red-*` palette classes or hex values. Reuse `web/app/components/` before adding a component.
- Never commit `.env`; add new settings to @.env.example. `DATABASE_URL`, `SESSION_SECRET` and `OPENROUTER_API_KEY` are required.
- A merge to `main` deploys to production. Only run the agent-safe `fly` commands listed in @docs/runbook.md ("Who runs what").
- Add schema changes as a new numbered pair `backend/internal/db/migrations/NNNN_name.{up,down}.sql`; never edit an applied migration.

## Project Structure

- `backend/cmd/shelfmark` (server), `backend/cmd/hashpw` (bcrypt helper), `backend/internal/{auth,catalog,db,obs}`; tests sit next to sources as `*_test.go`.
- `web/app/routes/*.tsx` (registered in `web/app/routes.ts`), `web/app/lib/*.ts` (API clients), `web/tests/e2e/` (Playwright).
- `context/` holds the 10x change flow: `foundation/` (PRD, roadmap, lessons), `changes/<id>/` (plan.md with `## Progress`), `archive/` (read-only).
- Operations: @docs/runbook.md; deploy config: @fly.toml, @Dockerfile.

## Commands

- `docker compose up -d db` — local Postgres.
- `cd backend && go vet ./... && go test ./...` — CI backend gate; `internal/catalog` tests start a Postgres container via Docker (~3 min).
- `cd web && npm run typecheck && npm run build` — CI web gate.
- `cd web && npx playwright test [tests/e2e/<name>.spec.ts]` — E2E against the built binary plus a local mock of deps.dev/GitHub/OpenRouter; needs `E2E_*` from `.env`. See @context/foundation/test-stack.md.

Claude Code hooks in `.claude/hooks/` run gofmt/vet/tsc after edits and at end of turn; `backend/hooktest` tests them.

## Commits & Pull Requests

- Conventional Commits scoped to the change id: `fix(empty-batch-status): ... (p1)`, `chore(<id>): impl-review ...`, `test(e2e): ...`.
- Branch from and open PRs into `shape/shelfmark`; it reaches `main` by PR. CI (@.github/workflows/ci.yml) must pass.
