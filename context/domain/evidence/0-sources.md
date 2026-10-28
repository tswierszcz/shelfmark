# Sources: code

- repo_root: /opt/src/shelfmark
- code_scope: . (backend/ + web/; 93 source files .go/.ts/.tsx/.sql)
- head: 24a8920 (clean)
- path_alias: none

## Stack
- Backend: Go 1.27, net/http mux, pgx/v5 on PostgreSQL 16, single binary serving `/api` and the built SPA.
- Frontend: React Router 8 SPA (TypeScript, Tailwind v4 tokens), plain `fetch` clients.
- External calls from the backend: deps.dev, GitHub API, OpenRouter (LLM).
- Tests: Go `testing` with a real Postgres container per test; Playwright E2E in `web/tests/e2e/`.

## Layers
| layer | path patterns | notes |
|---|---|---|
| entry points | `backend/cmd/shelfmark/main.go`, `backend/cmd/shelfmark/routes.go` (route table with role sets), `backend/internal/*/handlers.go`, `backend/internal/catalog/project_handlers.go`, `backend/internal/catalog/queue.go` (in-process worker queue and boot sweep), `backend/cmd/hashpw/` (CLI helper) | HTTP handlers live in the same package as persistence |
| application logic | `backend/internal/catalog/pipeline.go`, `queue.go`, `csv.go`, `sbom.go`, `project_sbom.go`, `purl.go`, `depsdev.go`, `github.go`, `openrouter.go`; `backend/internal/auth/session.go`, `middleware.go`, `accounts.go` | no separate service layer; logic sits in handlers, pipeline and store methods |
| domain types | Go structs in `backend/internal/catalog/store.go`, `project.go`, `strategic.go`, `sbom.go`; `backend/internal/auth/*.go`; TS types in `web/app/lib/*.ts` | no separate domain package — a finding, not a gap |
| persistence | `backend/internal/db/migrations/*.sql` (7 numbered pairs), `backend/internal/catalog/store.go`, `project.go`, `strategic.go` (SQL in Go strings), `backend/internal/auth/users_store.go` | one `Store` type over a pgx pool |
| UI | `web/app/routes/*.tsx` (registered in `web/app/routes.ts`), `web/app/components/*.tsx`, `web/app/root.tsx` | no translation files; copy is inline in routes |
| tests | `backend/**/*_test.go` next to sources (`internal/catalog` tests start Postgres in Docker), `backend/internal/auth/authtest/`, `backend/hooktest/`, `web/tests/e2e/*.spec.ts` + `web/tests/e2e/mocks/` | |

## Excluded
| path | reason |
|---|---|
| `context/domain/`, `context/archive/*-domain-distillation/` | this distillation's own output (none archived yet) |
| `context/map/`, `context/changes/redundancy-flow-analysis/`, `context/archive/2026-10-27-refactor-opportunities/research.md` | earlier analyses of this codebase; would feed prior conclusions back in as evidence (synthesis may cite them as priors, sub-agents do not read them) |
| `.claude/`, `web/.agents/` | agent configuration and skills, not the product |
| `backend/hooktest/` | tests of agent hooks, developer tooling |
| `.github/`, `Dockerfile`, `fly.toml`, `docker-compose.yml`, `.dockerignore`, `.editorconfig` | build, CI and deploy configuration |
| `web/build/`, `web/node_modules/`, `web/.react-router/` | generated or vendored |
| `README.txt`, `test.csv`, `idea-check.md` | stray root files: one-word readme, ad-hoc sample, pre-project idea assessment (the latter is listed as a document in 0-documents.md) |
