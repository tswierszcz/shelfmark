# Shelfmark

Shelfmark tells an organization which of its open-source libraries do the same job,
so it can pick one standard per category and retire the rest.

Security tools report what is *vulnerable*; Shelfmark reports what is *redundant*.
Every duplicate library is one more thing to patch and track.

**Live:** https://shelfmark-tsw.fly.dev

## How it works

1. **Catalog.** An admin uploads a CSV of package URLs (purls). Each library is enriched
   in the background (deps.dev, GitHub) and an LLM assigns it a capability category.
2. **Strategic choices.** An architect marks up to two libraries per category and
   ecosystem as the strategic standard, always with a justification.
3. **Project report.** An architect uploads a project's CycloneDX SBOM. The report flags
   every direct dependency that is not the standard in a category where a standard
   was chosen: a candidate to retire.

Roles: admin (catalog, users), architect (strategic choices, SBOM upload), viewer (reports).
The first release supports Maven (Java) only, on purpose: the goal is a real category
taxonomy built from one organization's data first.

## Stack

Go backend (`backend/`) serving `/api` and the built React Router SPA (`web/`) from one
binary · PostgreSQL 16 · OpenRouter for categorization · Fly.io · GitHub Actions CI/CD.

## Run locally

```sh
docker compose up -d db                       # Postgres on localhost:5432
cp .env.example .env                          # fill in the required values
cd web && npm ci && npm run build && cd ..    # build the SPA
set -a; . ./.env; set +a                      # the server reads env vars, not .env
cd backend && go run ./cmd/shelfmark          # http://localhost:8080
```

For frontend work, run `cd web && npm run dev`; it proxies `/api` to `localhost:8080`.
Password hashes for the seed accounts: `cd backend && go run ./cmd/hashpw`.

## Tests

- `cd backend && go vet ./... && go test ./...` — `internal/catalog` starts Postgres in Docker (~3 min)
- `cd web && npm run typecheck && npm test` — Vitest unit tests
- `cd web && npx playwright test` — end-to-end, needs the `E2E_*` values from `.env`

## How the work is organized

Every change follows the 10x workflow in `context/`: plan → plan review → phased
implementation → implementation review. Product docs live in `context/foundation/`
(PRD, roadmap, lessons), active changes in `context/changes/`, finished ones in
`context/archive/`. Pull requests labelled `impl-review` get an automatic review
against their plan in CI.

More: `AGENTS.md` (contributor and agent guide), `docs/runbook.md` (operations).
