---
change_id: projects-empty-list-null-fix
title: Fix GET /api/projects returning null instead of [] when empty
status: impl_reviewed
created: 2026-10-20
updated: 2026-10-21
archived_at: null
---

## Notes

Discovered 2026-10-20 while manually verifying `/redundancy` during the `redundancy-view-brand-tokens` close-out (see `context/archive/2026-10-19-redundancy-view-brand-tokens/`). Logged as a lesson in `context/foundation/lessons.md` ("Go nil slices marshal to JSON `null`, not `[]`").

`GET /api/projects` returns the literal body `null` (not `[]`) when an organization/account has zero projects, because the Go handler marshals a nil slice straight from the DB query. The frontend (`web/app/routes/redundancy.tsx:53-63`, `refreshProjects()`) does `setProjects(result)` with no `?? []` fallback, so `projects.map(...)` (line ~239) throws `TypeError: Cannot read properties of null (reading 'map')` and crashes the whole `/redundancy` page for any project-less architect/viewer account — including a freshly provisioned org, which is not an edge case.

Fix needs both ends per the lesson:
- Backend: guarantee a non-nil slice before marshaling in the `GET /api/projects` handler (and audit sibling list handlers for the same pattern).
- Frontend: defensive `?? []` fallback in `refreshProjects()` regardless, matching the pattern `refreshReport()` already uses on the same file.
