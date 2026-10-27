# Fix nil-slice → JSON `null` on list endpoints — Plan Brief

> Full plan: `context/changes/projects-empty-list-null-fix/plan.md`

## What & Why

`GET /api/projects` returns the literal body `null` instead of `[]` when an organization has zero projects, because the Go handler marshals a nil slice straight from the DB query. The frontend assigns it directly into state with no fallback, so `/redundancy` crashes (`TypeError: Cannot read properties of null (reading 'map')`) for any project-less architect/viewer account. Research found this is one instance of a repeating pattern — 4 Go query functions share the same nil-prone declaration — so this plan closes the whole bug class, not just the one crash that was hit.

## Starting Point

`web/app/routes/redundancy.tsx` currently crashes on load for any account with zero projects (discovered 2026-10-20 during the `redundancy-view-brand-tokens` close-out). The codebase already has the correct fix pattern in `ListLibrariesHandler`/`ListOrganizationLibrariesHandler` (`backend/internal/catalog/handlers.go`) — it's just not applied consistently to the other four list functions.

## Desired End State

Every list endpoint returns `[]` for an empty result, never `null`. `/redundancy`, `/catalog`, `/organization`, and `/users` all render correctly for a brand-new org with no data yet, instead of crashing on first load.

## Key Decisions Made

| Decision | Choice | Why (1 sentence) | Source |
| --- | --- | --- | --- |
| Fix scope | Whole pattern (4 backend functions + 6 frontend call sites), not just `ListProjects` | Matches the rule already recorded in `context/foundation/lessons.md`; `ListTaxonomy` is the same bug waiting to happen | Plan (user confirmed) |
| Backend fix shape | Change `var x []T` to `x := []T{}` at declaration | Smallest possible diff; matches the `make()`/literal-`{}` idiom already used elsewhere in this codebase | Plan |
| Frontend fix shape | Add `?? []` at each `setState(result)` call site | Matches the pattern `refreshReport()` already uses in the same file (`redundancy.tsx:73`) | Plan |
| Test coverage | One zero-row test per guarded function, asserting `[]` body | Closes the test gap noted in the existing `ListProjects` test, which only covers the non-empty case | Plan |

## Scope

**In scope:**
- `backend/internal/catalog/project.go` — `ListProjects`, `ProjectReport`
- `backend/internal/catalog/store.go` — `ListTaxonomy`
- `backend/internal/auth/users_store.go` — `UsersStore.List`
- 6 frontend `setState` call sites across `redundancy.tsx`, `catalog.tsx`, `organization.tsx`, `users.tsx`
- One zero-row regression test per backend function

**Out of scope:**
- A lint rule or CI check to catch future instances of this pattern
- `ListLibrariesHandler` / `ListOrganizationLibrariesHandler` — already correct, cited only as precedent
- Any behavior change beyond null→`[]`

## Architecture / Approach

Two independent layers, each closing the gap from its own side: Phase 1 fixes the source (backend always returns `[]`), Phase 2 adds defense-in-depth (frontend never trusts that guarantee blindly). Same shape of fix applied four times on the backend, six times on the frontend — mechanical, low-risk, no behavior change beyond the empty-state response shape.

## Phases at a Glance

| Phase | What it delivers | Key risk |
| --- | --- | --- |
| 1. Backend nil-guards + regression tests | All 4 query functions return `[]` not `null`; zero-row tests lock it in | `ListTaxonomy`/`UsersStore.List` tests must force an artificially empty table since both are normally always seeded |
| 2. Frontend defensive fallbacks | All 6 call sites fall back to `[]` regardless of backend behavior | None — mechanical, one-line changes following an existing in-file pattern |

**Prerequisites:** None — self-contained bug fix, no dependency on other in-flight changes.
**Estimated effort:** Small — single session, 2 phases, ~10 total one-line/few-line edits plus 4 new tests.

## Open Risks & Assumptions

- Assumes the exact helper/fixture used for "project with zero attached libraries" in the `ProjectReport` test will need to be built from what's already in `project_handlers_test.go` (upload a project, don't attach any libraries) — implementer discretion, not pinned in the plan.
- Assumes deleting seeded rows mid-test (for the `ListTaxonomy`/`UsersStore.List` zero-row tests) is acceptable within this test suite's isolation model (each test gets its own throwaway Postgres container per `startTestPostgres`), so this doesn't affect other tests.

## Success Criteria (Summary)

- `go test ./backend/...` passes, including 4 new zero-row tests
- `/redundancy` loads without error for an architect account with zero projects (the exact scenario that crashed during `redundancy-view-brand-tokens` close-out)
- `GET /api/projects` on an empty table returns `[]`, confirmed via `curl`
