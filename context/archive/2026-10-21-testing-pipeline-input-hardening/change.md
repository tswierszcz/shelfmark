---
change_id: testing-pipeline-input-hardening
title: Testing pipeline input hardening
status: archived
created: 2026-10-21
updated: 2026-10-26
archived_at: 2026-10-26T15:30:07Z
---

## Notes

<!-- Free-form notes for this change: links, ad-hoc context, decisions that don't belong in research/frame/plan. -->

Implementation adaptations (recorded from impl-review F9, 2026-10-26):

- Phase 1: both upload handlers call `r.ParseMultipartForm` explicitly and check `*http.MaxBytesError` there, because `r.FormValue` swallows parse errors and an oversized body would otherwise surface as a misleading "kind must be..." / missing-field 400.
- Phase 3: the reconciliation test uses a test-local `artifactRoutingFinder` (outcome per artifact) instead of combining `fakeRepoFinder` + `countingErrFinder` — one `Queue` shares a single finder list, so a single batch can't mix outcomes otherwise.
- Phase 3: `LibrariesByPurls` (project.go) also scans `error_detail`, per the plan's "any other SELECT that builds a Library". `ProjectReport` rows deliberately don't carry it.
- Impl-review: `error_detail` is admin-only — `GET /api/organization/libraries` strips it (F1).
