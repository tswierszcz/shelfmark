---
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
  has_background_jobs: true
---

## Why this stack

A solo developer is building Shelfmark, a web app for architects that classifies libraries by capability (via GitHub repository signals and LLM categorization) and confronts that classification against uploaded project SBOMs to surface functional redundancy, on a twelve-week after-hours budget targeting a 2027-01-10 submission. The user chose a Go backend with a React frontend, so the hand-off pairs the Go standard-library card with React Router v7 (https://reactrouter.com) in SPA mode rather than plain Vite + React: React Router brings file-based routing and passes the convention gate that the bare Vite template fails, and both halves are typed, mainstream within their families and well documented, so no quality override was needed. The SPA builds to static files served by the Go binary, which keeps one deployable unit in a Docker image, deployed to Fly. PostgreSQL is the preferred database. Auth (three config-provisioned roles), AI (GitHub-sourced tags validated/extended by an LLM), and background jobs (per-library GitHub lookups and LLM calls during catalog seeding, run asynchronously so the admin isn't blocked) are all in scope for this revised flow. The npm package manager applies to the frontend; Go modules need no choice. Scaffolding is best-effort because no single registry card covers this pairing.
