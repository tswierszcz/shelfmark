---
bootstrapped_at: 2026-10-10T20:38:03Z
starter_id: custom
starter_name: "Go (standard library) API + React Router v7 (SPA mode, TypeScript)"
project_name: shelfmark
language_family: multi
package_manager: npm
cwd_strategy: subdir-then-move
bootstrapper_confidence: best-effort
phase_3_status: ok
audit_command: "null"
---

## Hand-off

```yaml
starter_id: custom
custom_starter:
  name: "Go (standard library) API + React Router v7 (SPA mode, TypeScript)"
  docs_url: https://go.dev/doc/
package_manager: npm
project_name: shelfmark
hints:
  language_family: multi
  team_size: solo
  deployment_target: fly
  ci_provider: github-actions
  ci_default_flow: auto-deploy-on-merge
  bootstrapper_confidence: best-effort
  path_taken: custom
  quality_override: false
  self_check_answers:
    typed: true
    from_official_starter: true
    conventions: true
    docs_current: true
    can_judge_agent: true
  has_auth: true
  has_payments: false
  has_realtime: false
  has_ai: true
  has_background_jobs: false
```

**Why this stack**: A solo developer is building Shelfmark, a customer-hosted web app for architects that turns Maven SBOMs into capability-redundancy reports, on a six-week after-hours budget. The user chose a Go backend with a React frontend, so the hand-off pairs the Go standard-library card with React Router v7 (https://reactrouter.com) in SPA mode rather than plain Vite + React: React Router brings file-based routing and passes the convention gate that the bare Vite template fails, and both halves are typed, mainstream within their families and well documented, so no quality override was needed. The SPA builds to static files served by the Go binary, which keeps one deployable unit in a Docker image: the same image runs as a cloud demo on Fly now and inside customer infrastructure later, matching the PRD rule that SBOM data never leaves the customer. PostgreSQL is the preferred database. Auth (three config-provisioned roles) and AI (external categorization that receives only public Maven coordinates) are in scope. The npm package manager applies to the frontend; Go modules need no choice. Scaffolding is best-effort because no single registry card covers this pairing.

## Pre-scaffold verification

| Signal       | Value                                             | Severity | Notes                                                                                   |
| ------------ | ------------------------------------------------- | -------- | --------------------------------------------------------------------------------------- |
| npm package  | create-react-router v8.4.0 published 2026-09-15   | fresh    | checked for the frontend generator (custom starter; informational)                      |
| GitHub repo  | not run                                           | —        | `custom_starter.docs_url` is go.dev, not GitHub; `gh` is also not installed              |
| Toolchain    | Go 1.27.1 at /usr/lib/go-1.27/bin (not on PATH); Node v24.21.0; npm 12.2.0 | — | Go used by absolute path for this run                                  |

## Scaffold log

**Resolved invocation**:
1. `npx -y create-react-router@latest .bootstrap-scaffold/web --yes --no-git-init --package-manager npm --install`
2. `cd .bootstrap-scaffold && /usr/lib/go-1.27/bin/go mod init github.com/tswierszcz/shelfmark`

**Source**: `create-react-router --help` (v8.4.0) and go.dev module docs; shown to and approved by the user (latest React Router, v8, instead of the v7 named in the hand-off).
**Strategy**: subdir-then-move
**Exit code**: 0 (both commands)
**Post-generator edits (approved before run)**:
- `web/react-router.config.ts`: `ssr: true` → `ssr: false` (SPA mode per hand-off).
- `web/package.json`, `web/package-lock.json`: package name `bootstrap-scaffold-web` → `shelfmark-web` (temp-dir name leaked into the generated name).
- Added `cmd/shelfmark/main.go`: minimal `net/http` server with `GET /healthz`, so the Go module builds.
**Checks**: `go build ./...` ok; `npm run typecheck` ok; `npm run build` ok ("SPA Mode: Generated build/client/index.html"). Build outputs removed afterwards.
**Files moved**: 28 (excluding `web/node_modules/`) — `go.mod`, `cmd/shelfmark/main.go`, `web/**` (incl. `web/.agents/skills/react-router/`, `web/Dockerfile`, `web/.dockerignore`, `web/.gitignore`)
**Conflicts (.scaffold siblings)**: none
**.gitignore handling**: absent in scaffold root (only `web/.gitignore`, moved silently); absent in cwd
**.bootstrap-scaffold cleanup**: deleted

## Post-scaffold audit

**Tool**: skipped — no built-in audit tool for multi
**Recommended external tool**: no single audit tool covers this multi-language stack.

## Hints recorded but not acted on

| Hint                    | Value                |
| ----------------------- | -------------------- |
| bootstrapper_confidence | best-effort          |
| quality_override        | false                |
| path_taken              | custom               |
| self_check_answers      | typed: true, from_official_starter: true, conventions: true, docs_current: true, can_judge_agent: true |
| team_size               | solo                 |
| deployment_target       | fly                  |
| ci_provider             | github-actions       |
| ci_default_flow         | auto-deploy-on-merge |
| has_auth                | true                 |
| has_payments            | false                |
| has_realtime            | false                |
| has_ai                  | true                 |
| has_background_jobs     | false                |

## Next steps

Next: a future skill will set up agent context (CLAUDE.md, AGENTS.md). For now, your project is scaffolded and verified — happy hacking.

Useful manual steps in the meantime:
- `git init` (if you have not already) to start your own repo history.
- Review any `.scaffold` siblings the conflict policy created and decide which version of each file to keep.
- Address audit findings per your project's risk tolerance — the full breakdown is in this log.
