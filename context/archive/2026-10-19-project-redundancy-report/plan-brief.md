# Project Redundancy Report — Plan Brief

> Full plan: `context/changes/project-redundancy-report/plan.md`

## What & Why

This is S-03, the roadmap's north star: an architect uploads a project's SBOM and sees which of its direct dependencies are redundant — i.e. not the organization's strategic pick for that category. It's the slice that finally confronts the categorized catalog (S-01) and the strategic marks (S-02) against a real project's own dependency data, proving the core product hypothesis.

## Starting Point

Today, the catalog can be seeded and categorized (S-01), and an architect can mark one library per category/ecosystem as strategic (S-02). But there is no concept of "a project" anywhere, and direct-vs-transitive dependency status — the whole point of flagging retire candidates — is never derived from an SBOM; a column added for it in S-02 (`libraries.is_direct`) is always `true` because it's architecturally the wrong shape (direct/transitive varies per project, not globally per library).

## Desired End State

An Architect opens `/redundancy`, types a project name, and uploads that project's CycloneDX SBOM. They (and any Viewer) then see a report: every library in the project, grouped by category, flagged direct or transitive, showing whether it's the strategic pick, and — for direct libraries in a category where a different library is strategic — flagged as a candidate to retire. Re-uploading the same project name refreshes that project's report in place.

## Key Decisions Made

| Decision | Choice | Why (1 sentence) |
| --- | --- | --- |
| Project identity | Free-text name, case-insensitive match-or-create on upload | Lightweight, no separate "create project" step, matches the PRD's casual "repeatedly upload" framing |
| Re-upload behavior | Replace the project's current library set, no history | A report is a current-state view; nothing in FR-005/FR-008/US-01 asks for change-over-time |
| Direct/transitive storage | New `project_libraries(project_id, library_id, is_direct)` table, not the existing global `libraries.is_direct` | Direct/transitive is inherently per-project; a global column would let one project's upload corrupt another's report |
| Unknown purl in project SBOM | Auto-ingest via the existing catalog pipeline (GitHub/LLM categorization) | Reuses proven S-01 machinery; report stays complete without a manual admin step first |
| Cross-project org report | Out of scope | Not one of S-03's PRD anchors (FR-005/FR-008/US-01); a future roadmap slice |
| Retire-candidate definition | Only flag a direct library when a *different* library is strategic in that category | Matches the literal "retire in favor of a standard" framing; an unreviewed category isn't noise-flagged |
| Access roles | Architect uploads; Architect + Viewer view | Mirrors FR-005 ("Architect can upload") and FR-008/Access Control's broader "User"/"Viewer views reports" |
| CycloneDX dependency parsing | Parse `bom.Dependencies` for real direct/transitive; fall back to all-direct if absent | Without this, every library would be "direct" and the retire-candidate flag would be meaningless |
| Project selection UX | Free-text name field, not a picker-or-create dropdown | Fewer clicks for the common case; typo risk accepted (no rename tooling) |
| Frontend testing | No new framework — typecheck + build + manual, matching S-01/S-02 | Consistent with the existing codebase convention |

## Scope

**In scope:**
- `projects` + `project_libraries` tables and store methods
- CycloneDX dependency-graph parsing for direct/transitive classification
- Project SBOM upload endpoint (auto-ingest unknown purls, replace project's library set)
- Project report endpoint (per-category strategic + retire-candidate flags) and a project-listing endpoint
- `/redundancy` frontend page: upload form + project picker + report table

**Out of scope:**
- Organization-wide cross-project redundancy report
- Project upload history/versioning
- Project rename/merge tooling
- CSV upload for projects (SBOM only, since direct/transitive needs the dependency graph)
- New frontend test tooling

## Architecture / Approach

New project-scoped tables sit alongside the existing catalog tables rather than reusing S-02's global `is_direct` column. The SBOM parser gains a dependency-graph walk (CycloneDX `dependencies` → root component's direct children). The upload endpoint reuses S-01's `CreateLibraries` for auto-ingestion, then resolves every purl by value (not by batch, since pre-existing purls don't get a new batch id) and replaces the project's `project_libraries` rows in one transaction. The report endpoint reuses the join shape already proven by S-02's `ListOrganizationLibrariesHandler`, scoped to one project, with a retire-candidate flag computed per (library, category) pair. All new backend code stays in the existing `catalog` package.

## Phases at a Glance

| Phase | What it delivers | Key risk |
| --- | --- | --- |
| 1. Project data model & store layer | `projects`/`project_libraries` schema + Store methods (find-or-create, replace, resolve-by-purl, report query) | Retire-candidate query must check *other* libraries' marks per category, not just the row's own — easy to get wrong |
| 2. SBOM dependency-graph parsing | `ParseProjectSBOM` classifies each component direct/transitive from `bom.Dependencies` | CycloneDX SBOMs without a clean root/dependency graph need a safe all-direct fallback |
| 3. Project upload & report endpoints | `POST /api/projects/upload`, `GET /api/projects`, `GET /api/projects/{id}/report`, wired with Architect/Viewer role gates | Re-upload must replace cleanly (first multi-statement transaction in the codebase) without disturbing shared catalog rows other projects also reference |
| 4. Frontend — redundancy report page | `/redundancy` page: upload form, project picker, category-grouped report table | Reusing the existing batch-progress polling pattern correctly for the new upload endpoint's `batch_id` |

**Prerequisites:** S-01 (seed-classification-catalog) and S-02 (mark-strategic-libraries), both done.
**Estimated effort:** ~4 sessions across 4 phases (1 backend-heavy data-model phase, 1 focused parsing phase, 1 endpoint-wiring phase, 1 frontend phase).

## Open Risks & Assumptions

- Assumes CycloneDX SBOMs produced by the organization's existing tooling populate `metadata.component` and `dependencies` meaningfully; if they don't, every project report silently falls back to all-direct (not wrong per the guardrail, but less useful).
- Assumes "retire candidate" should not fire for an unreviewed category (no strategic mark anywhere in it) — if usage reveals architects actually want those surfaced too, this is a one-line flip in the `ProjectReport` query, not a redesign.
- Free-text project matching means a name typo silently creates a duplicate project with no merge path; acceptable per this plan's explicit scope cut, revisit if it causes real confusion.
- `/organization`'s existing `is_direct` (global, always `true`, from S-02) and this plan's new per-project `is_direct` share a field name but mean different things on two pages an architect may use side by side; accepted as a known limitation rather than reopening S-02's shipped page.

## Success Criteria (Summary)

- An Architect can upload a project's SBOM and get back a category report with direct dependencies flagged, strategic status shown, and non-strategic direct libraries clearly marked as retire candidates (US-01's acceptance criterion).
- Re-uploading a project by name updates its existing report rather than creating a duplicate.
- A Viewer can see the same report without any edit/upload capability.
