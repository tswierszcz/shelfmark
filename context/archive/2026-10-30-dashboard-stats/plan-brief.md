# Dashboard Statistics — Plan Brief

> Full plan: `context/changes/dashboard-stats/plan.md`

## What & Why

Every role lands on `/dashboard` after logging in, but the page only shows the user's email and role. This change turns it into an overview of the catalog and of redundancy: how many libraries are in which state, how many categories and projects exist, and which categories have several libraries doing the same job. Each role also gets one clear next step.

## Starting Point

`web/app/routes/dashboard.tsx` is a placeholder. The data exists (`libraries.status`, `categories`, `project_libraries.is_direct`, `strategic_marks`), but no endpoint serves aggregates, and the list endpoints are split by role (catalog for admin only; organization and projects for architect and viewer only).

## Desired End State

Admin, architect and viewer all see the same set of statistics: KPI tiles, a status breakdown, a per-ecosystem table and the top 5 redundancy hotspots with a reviewed or no-decision badge. Admin also gets a link to libraries needing manual review, and architect gets a link to redundant categories without a strategic decision. Loading, error and empty states are explicit.

## Key Decisions Made

| Decision | Choice | Why (1 sentence) |
| --- | --- | --- |
| Data source | New `GET /api/dashboard` for all roles, aggregates computed in SQL | Existing list endpoints are role-restricted, and counting whole lists in the browser does not scale. |
| Hotspot definition | Category with ≥ 2 distinct libraries that are **direct** dependencies of at least one project | Matches the retire-candidate rule and shows redundancy the organization actually ships. |
| Hotspot list | Top 5 by library count desc, ties by category name A→Z | Deterministic and short; a 6th tied category is cut off. |
| "Reviewed" | Reuse `strategicIndex.isReviewed` (any active strategic mark, global) | One owner of the rule; per-ecosystem scope stays open (K1). |
| Role differences | Same statistics for everyone; only the call to action differs (viewer has none) | One endpoint, one layout, simple tests. |
| Extra v1 content | Per-ecosystem library counts | One cheap query, ready for a second ecosystem. |
| Deferred | Retire-candidate total, URL filters on target pages | Keeps v1 small; both need extra rule or routing work. |

## Scope

**In scope:**
- `Store.DashboardStats`, `DashboardHandler`, route plus role-matrix entry, Go integration and JSON-contract tests
- `lib/dashboard.ts`, a rewritten `dashboard.tsx` and `dashboard.test.tsx`

**Out of scope:**
- Retire-candidate total, per-ecosystem "reviewed" (K1), URL filters on `/catalog` or `/organization`, role-tailored layouts, caching or auto-refresh, migrations

## Architecture / Approach

One store method runs a few `COUNT … GROUP BY` queries and one `loadStrategicIndex` call, and returns a struct whose maps and slices are never nil. The handler encodes it for any signed-in role. The page fetches it once and renders sections with `@theme` tokens and the existing `Badge`/`Button` components.

## Phases at a Glance

| Phase | What it delivers | Key risk |
| --- | --- | --- |
| 1. Dashboard stats API | `GET /api/dashboard` with tested counting rules | Hotspot rule silently counting transitive or catalog-only libraries (guarded by a break-check) |
| 2. Dashboard view | Statistics page with 4 states and a role call to action | Wrong call to action shown to the wrong role (guarded by tests and a break-check) |

**Prerequisites:** local Postgres (`docker compose up -d db`) and Docker for the backend test container.
**Estimated effort:** ~2 sessions, one per phase.

## Open Risks & Assumptions

- "Reviewed" is global across ecosystems. A second ecosystem would make a category look reviewed because of another ecosystem's mark (decision K1).
- Every library is `maven` today, so the ecosystem table shows a single row until a second ecosystem appears.

## Success Criteria (Summary)

- Each role sees statistics that match what `/catalog`, `/organization` and `/redundancy` show.
- `GET /api/dashboard` never returns `null` fields and is listed in the role matrix.
- `go test ./...` (with the dashboard tests actually running, not skipped), `npm test`, `typecheck`, `build` and the E2E seed spec pass, and both break-checks turn the tests red.
