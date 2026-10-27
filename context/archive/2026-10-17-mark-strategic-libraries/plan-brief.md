# Architect marks strategic libraries — Plan Brief

> Full plan: `context/changes/mark-strategic-libraries/plan.md`

## What & Why

Architects need a view of the organization's library catalog where they can designate one (or more) library per category/ecosystem as the strategic/standard choice. This is S-02 on the roadmap — it has to exist before S-03 (the redundancy report) can tell an architect which of a project's direct dependencies are candidates to retire.

## Starting Point

S-01 built the catalog (`categories`, `libraries`, `library_categories`) and an Admin-only catalog page/API. Nothing in the schema distinguishes direct from transitive libraries yet, and nothing records a "strategic" designation.

## Desired End State

A new `/organization` page, reachable by Architect (full access) and Viewer (read-only), lists catalog libraries with their categories and current strategic marks. An Architect can mark or unmark a library as strategic within a category, entering a justification each time.

## Key Decisions Made

| Decision | Choice | Why (1 sentence) | Source |
| --- | --- | --- | --- |
| "Direct" semantics this slice | `libraries.is_direct` added now, defaulted `true`, never set `false` by this change | No extraction source exists yet (SBOM/CSV parsing doesn't carry it); S-03's project SBOMs are expected to populate it meaningfully | Plan |
| Justification | Required on both mark and unmark | PRD Access Control: Architect "sets and changes recommendations... with justification" | Plan |
| Data model | New `strategic_marks` table, keyed `(library_id, category_id)`, with an `active` flag rather than row deletion | Keeps the latest justification/actor readable for an unmark without a full audit-log table | Plan |
| View access | Architect (full) + Viewer (read-only) | Matches PRD's Viewer definition: "views reports and current recommendations; no edit capabilities" | Plan |
| Cap per category+ecosystem | **Unbounded** — no limit enforced | Explicit user decision; deviates from PRD FR-007's literal "up to two" | Plan |
| Unmark action | Explicit unmark control, justification required, same as marking | Matches "sets and changes recommendations with justification" — removal is still a recommendation change | Plan |

## Scope

**In scope:**
- `is_direct` column + `strategic_marks` table (migration)
- Store methods: `MarkStrategic`, `UnmarkStrategic`, `StrategicMarksByLibrary`, plus `Library.IsDirect` / `LibraryFilter.Strategic`
- New endpoints: `GET /api/organization/libraries`, `POST`/`DELETE .../strategic`
- Widened `/api/catalog/categories` access (Admin+Architect+Viewer)
- New `/organization` frontend page + API client
- `RequireAuth` role-array support

**Out of scope:**
- Real direct/transitive detection (deferred to S-03)
- Any cap enforcement on strategic marks per category+ecosystem
- Full audit history of every mark/unmark event (only the latest action is kept)
- Changes to the existing Admin catalog page/endpoints beyond the categories-access widening

## Architecture / Approach

One migration adds the schema. The existing `catalog.Store` and `catalog` handler package gain new methods/handlers rather than a parallel package, reusing `ListLibraries` and the `AttachCategory` upsert idiom. A new frontend page mirrors `catalog.tsx`'s structure (filters + table) but targets Architect/Viewer roles and swaps upload/categorize controls for mark/unmark controls.

## Phases at a Glance

| Phase | What it delivers | Key risk |
| --- | --- | --- |
| 1. Schema migration | `is_direct` column, `strategic_marks` table | Migration must be reversible and additive-only |
| 2. Store layer | `MarkStrategic`/`UnmarkStrategic`/`StrategicMarksByLibrary`, filter support | Upsert/not-found semantics must match existing store conventions |
| 3. HTTP handlers + routing | New organization endpoints, widened categories access | Role gating must correctly separate Architect (write) from Viewer (read-only) |
| 4. Frontend | `/organization` page, API client, `RequireAuth` role-array support | `RequireAuth` change must not break existing single-role callers |

**Prerequisites:** S-01, F-01 (both done). **Estimated effort:** small-medium — one migration, ~4 new store methods, 3 new handlers, 1 new page; all following established patterns from S-01.
