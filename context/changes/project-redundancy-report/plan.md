# Project Redundancy Report Implementation Plan

## Overview

Implements S-03, the roadmap's north star: an architect uploads a project's CycloneDX SBOM and gets back a per-category report showing each direct dependency's strategic/non-strategic status, with non-strategic direct libraries flagged as candidates to retire (FR-005, FR-008, US-01).

## Current State Analysis

- `GET /api/organization/libraries` (`backend/internal/catalog/handlers.go:195-255`) already joins libraries + categories + active strategic marks into one response shape — the template this report's query reuses.
- `libraries.purl` is globally unique (`ON CONFLICT (purl) DO NOTHING` in `CreateLibraries`, `store.go:95-120`), so a library has one identity across the whole catalog — but `libraries.upload_batch_id` is set once at first insert and never updated, so it cannot be used to scope "which libraries belong to project X."
- `libraries.is_direct` (added in S-02, migration `0003_strategic_marks.up.sql:1`) is a **global** column, always `true`, never set to `false` anywhere. Direct-vs-transitive is actually a **per-project** property (the same library can be direct in one project and transitive in another), so this column cannot correctly represent it — this plan does not populate it and instead adds project-scoped storage.
- `ParseCycloneDXSBOM` (`sbom.go:24-59`) only reads `bom.Components[].PackageURL`; it never reads `bom.Dependencies`, so no direct/transitive signal is extracted from a CycloneDX SBOM today.
- `cyclonedx-go` v0.12.0 models the dependency graph as `BOM.Dependencies *[]Dependency{Ref string, Dependencies *[]string}` (BOMRef → list of BOMRefs it depends on), with the SBOM's root component at `BOM.Metadata.Component` (its own `BOMRef`). The root's `Dependency.Dependencies` entry (matched by `Ref == root.BOMRef`) is exactly the set of direct dependencies.
- `UploadHandler` (`handlers.go:55-115`) is the multipart-upload + batch-insert + enqueue template for a new project-upload endpoint.
- Role gating is a decorator applied per-route in `main.go:42-55`: `auth.RequireRole(role...)(handler)`. Roles are `RoleAdmin`, `RoleArchitect`, `RoleViewer` (`backend/internal/auth/accounts.go:12-15`).
- No frontend test framework exists (`web/package.json` has no `test` script); both prior slices verified via `npm run typecheck`, `npm run build`, and manual browser testing only.

### Key Discoveries:

- `store.go:62` `LibraryFilter.BatchID` exists but doesn't help scope a project report, for the reason above — new storage is needed, not a filter tweak.
- `strategic_marks` PK is `(library_id, category_id)` (`0003_strategic_marks.up.sql:3-11`) — since the catalog is Maven-only and a library's ecosystem is fixed, no additional ecosystem scoping is needed when checking strategic status for a category.
- The codebase has no existing multi-statement transaction (`MarkStrategic`/`UnmarkStrategic` are both single-statement). The project-library replace-on-reupload needs the first one (delete + bulk insert), via `pool.Begin(ctx)`.

## Desired End State

An Architect can open `/redundancy`, type a project name, upload that project's CycloneDX SBOM, and see a report of every library in that project grouped by category: each row shows the library, whether it's a direct dependency, its categories, whether it's the strategic pick in each category, and whether it's flagged as a candidate to retire (a direct library in a category where a *different* library is strategic). Viewers can see the same report read-only. Re-uploading the same project name replaces its reported library set with the new upload's contents.

### Verification: the above is true when `go test ./... && go vet ./...` pass in `backend/`, `npm run typecheck && npm run build` pass in `web/`, and the manual walkthrough in each phase's Manual Verification succeeds.

## Known Limitation

`/organization`'s existing `is_direct` field (from the global `libraries.is_direct` column, S-02) stays always-`true` and unrelated to this plan's new per-project `is_direct`. The same field name now means two different things on two pages an architect may use side by side — accepted as out of scope rather than reopening a shipped S-02 page; revisit if this causes real confusion in practice.

## What We're NOT Doing

- No organization-wide cross-project redundancy report (PRD Secondary success criteria) — out of scope per S-03's roadmap anchors (FR-005, FR-008, US-01 only); a future roadmap slice.
- No versioned history of a project's past uploads — re-uploading replaces the project's current library set; there is no way to see what it looked like before.
- No project rename/merge tooling — project identity is purely free-text name matched case-insensitively; a typo creates a new project with no fix-up UI.
- No CSV upload path for projects — CycloneDX only, since CSV carries no dependency graph and direct/transitive is the point of this report.
- No frontend test framework — verified via `npm run typecheck`, `npm run build`, and manual testing, matching both prior slices.
- No new health-metrics enrichment — parked per roadmap.

## Implementation Approach

New project-scoped storage (`projects`, `project_libraries`) is added alongside the existing catalog tables rather than reusing `libraries.is_direct`, since direct/transitive must vary per project. The SBOM parser gains a dependency-graph walk to classify each component as direct or transitive. The upload endpoint reuses S-01's `CreateLibraries` so a project SBOM's purls are auto-ingested into the shared catalog and categorized through the existing pipeline if not already known, then resolves all of them by purl (not by batch) to populate `project_libraries`. The report endpoint reuses the join shape from `ListOrganizationLibrariesHandler`, scoped to one project, with a retire-candidate flag computed per (library, category) pair. All new backend code lives in the existing `catalog` package, matching how `strategic_marks` (conceptually "organization", S-02) was kept there rather than split into a new package.

## Phase 1: Project data model & store layer

### Overview

Adds the `projects` and `project_libraries` tables and the `Store` methods the upload and report endpoints need: finding/creating a project by name, replacing a project's library set, resolving libraries by purl (regardless of batch), and computing the report.

### Changes Required:

#### 1. Migration

**File**: `backend/internal/db/migrations/0004_project_reports.up.sql` / `.down.sql`

**Intent**: Add project identity and per-project library membership (including per-project direct/transitive), without touching the existing `libraries.is_direct` column (left as dead weight — never read by this feature).

**Contract**:
```sql
CREATE TABLE projects (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name       text NOT NULL,
    created_by text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX projects_name_lower_idx ON projects (lower(name));

CREATE TABLE project_libraries (
    project_id uuid NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    library_id uuid NOT NULL REFERENCES libraries (id) ON DELETE CASCADE,
    is_direct  boolean NOT NULL,
    PRIMARY KEY (project_id, library_id)
);
```
The down migration drops both tables (and the unique index, dropped implicitly with the table).

#### 2. Store types & methods

**File**: `backend/internal/catalog/project.go` (new)

**Intent**: Give the upload/report handlers the operations they need without embedding SQL in handlers, matching `Store`'s existing convention.

**Contract**:
- `type Project struct { ID, Name, CreatedBy string; CreatedAt time.Time }`
- `type ProjectLibraryEntry struct { LibraryID string; IsDirect bool }`
- `type ProjectReportRow` — one row per (library, category) pair: library fields (id, purl, name, ecosystem, github_repo, status), `IsDirect bool`, `CategoryID`, `CategoryName`, `Strategic bool` (this library has an active mark on this category), `RetireCandidate bool`.
- `func (s *Store) FindOrCreateProject(ctx context.Context, name, createdBy string) (*Project, error)` — looks up by `lower(name)`; creates if absent; touches `updated_at` if found (so a project's "last uploaded" time is visible).
- `func (s *Store) LibrariesByPurls(ctx context.Context, purls []string) ([]Library, error)` — resolves full `Library` rows (with IDs) for a purl list, independent of `upload_batch_id`. Needed because `CreateLibraries` + a `BatchID` filter (the existing catalog upload pattern) misses purls that already existed under a different batch.
- `func (s *Store) ReplaceProjectLibraries(ctx context.Context, projectID string, entries []ProjectLibraryEntry) error` — inside one `pool.Begin(ctx)` transaction: `DELETE FROM project_libraries WHERE project_id = $1`, then bulk-insert `entries`. This is the codebase's first multi-statement transaction; commit only after both statements succeed, rollback on any error.
- `func (s *Store) ListProjects(ctx context.Context) ([]Project, error)` — ordered by `updated_at DESC`, for the frontend's project picker.
- `func (s *Store) ProjectReport(ctx context.Context, projectID string) ([]ProjectReportRow, error)` — joins `project_libraries` + `libraries` + `library_categories` (category name) + `strategic_marks` (active). `RetireCandidate` for a row is `is_direct AND <category has at least one other active strategic mark> AND NOT strategic` — this requires knowing, per category, whether *any* library (not just this one) holds an active mark there; compute this as a `category_id → bool` lookup (e.g. a `EXISTS` subquery per category or one pre-pass query over `strategic_marks WHERE active`) rather than only checking this row's own `Strategic` flag, or every never-reviewed category's direct libraries will be wrongly flagged.

### Success Criteria:

#### Automated Verification:

- Migration applies cleanly: `go run ./cmd/shelfmark` boots against a fresh DB (or equivalent migrate-only check already used by prior slices)
- Unit tests pass: `go test ./internal/catalog/... ./internal/db/...`
- `go vet ./...` passes

#### Manual Verification:

- None (no user-facing surface yet)

---

## Phase 2: SBOM dependency-graph parsing

### Overview

Extends SBOM parsing to classify each component as direct or transitive by walking CycloneDX's `dependencies` graph, with an all-direct fallback when the SBOM doesn't carry that graph (pre-negotiated guardrail).

### Changes Required:

#### 1. Project SBOM parser

**File**: `backend/internal/catalog/project_sbom.go` (new)

**Intent**: Produce, per purl, both the `NewLibrary` fields `ParseCycloneDXSBOM` already extracts and an `IsDirect` classification, so the upload handler doesn't need its own CycloneDX-walking logic.

**Contract**: `func ParseProjectSBOM(r io.Reader) ([]NewLibrary, map[string]bool, []SkippedRow, error)` where the returned `map[string]bool` is keyed by purl → `IsDirect`.
Reuses `ParsePurl`/skip semantics identically to `ParseCycloneDXSBOM` (same error/skip behavior for malformed purls or non-Maven types), but additionally:
```
rootRef := bom.Metadata != nil && bom.Metadata.Component != nil ? bom.Metadata.Component.BOMRef : ""
if bom.Dependencies == nil || rootRef == "" {
    // fallback: every parsed purl is direct
}
directRefs := the Dependency entry in *bom.Dependencies whose Ref == rootRef; its *Dependencies (dependsOn) set
bomRefToPurl := map built from bom.Components[].BOMRef -> PackageURL
for each parsed purl: IsDirect = component's BOMRef is in directRefs (components with no BOMRef, or a BOMRef absent from directRefs, default to IsDirect=true — conservative, matches the "show distinction only when input supports it" guardrail rather than silently calling an unresolvable component transitive)
```

### Success Criteria:

#### Automated Verification:

- Unit tests pass: `go test ./internal/catalog/... -run ParseProjectSBOM` covering: a CycloneDX SBOM with a full dependency graph (correct direct/transitive split), one with no `dependencies` field (all-direct fallback), and one with a `dependencies` graph but a root with no matching `Dependency` entry (all-direct fallback). No existing fixture has a `dependencies` array or `metadata.component` (the only current fixture, `sbom_test.go`, is components-only) — author new inline CycloneDX JSON literals for all three cases, following that file's existing style.
- `go vet ./...` passes

#### Manual Verification:

- None (no user-facing surface yet)

---

## Phase 3: Project upload & report endpoints

### Overview

Wires the new storage and parser into two HTTP endpoints: uploading a project's SBOM (auto-ingesting unknown purls into the shared catalog and categorizing them) and fetching that project's redundancy report. Also adds a project-listing endpoint so the frontend can offer a picker of previously uploaded projects.

### Changes Required:

#### 1. Upload handler

**File**: `backend/internal/catalog/project_handlers.go` (new)

**Intent**: Accept a project name + CycloneDX file, auto-ingest any purls the catalog doesn't already know (through the existing categorization pipeline), and record this project's current library membership (direct/transitive) — replacing whatever it recorded last time.

**Contract**: `func ProjectUploadHandler(store *Store, queue Enqueuer) http.HandlerFunc` for `POST /api/projects/upload`. Reads `r.FormValue("project_name")` (required, 400 if blank) and `r.FormFile("file")` (CycloneDX only — no `kind` field, unlike `UploadHandler`). Flow: `ParseProjectSBOM` → `store.CreateLibraries` (new batch id, existing no-op-on-conflict semantics) → `store.LibrariesByPurls` (resolves every purl's `Library`, new or pre-existing) → enqueue only the ones with `Status == "pending"` to `queue` (freshly inserted; already-categorized ones are not re-processed) — this guard is deliberate: `Queue.process` unconditionally reruns the full categorization cascade for anything enqueued, and `AttachCategory` upserts, so enqueueing an already-categorized or manually-corrected library would silently overwrite its `library_categories.source`. A narrow race where two concurrent project uploads both see a brand-new purl as `pending` and both enqueue it is accepted as a harmless cost (duplicate external API calls, no data corruption — `AttachCategory` upserts to the same result either way), not something this phase needs to prevent → `store.FindOrCreateProject(name)` → `store.ReplaceProjectLibraries(project.ID, entries built from the resolved libraries + the parser's direct/transitive map)`. This ordering is intentional and needs no spanning transaction: `CreateLibraries` and `FindOrCreateProject` are each independently idempotent/retry-safe, and `ReplaceProjectLibraries` — the one all-or-nothing step — is deliberately last, so a failure anywhere in the sequence is safely retryable without partial corruption. Responds `{"project_id", "project_name", "batch_id", "created", "skipped"}` (reuses `SkippedRow` shape).

#### 2. Report & listing handlers

**File**: `backend/internal/catalog/project_handlers.go`

**Intent**: Serve the computed report and the project picker list.

**Contract**:
- `func ProjectReportHandler(store *Store) http.HandlerFunc` for `GET /api/projects/{projectID}/report` — calls `store.ProjectReport`, groups `[]ProjectReportRow` by library into a JSON shape mirroring `ListOrganizationLibrariesHandler`'s library response (library fields + `is_direct` + `categories: [{category_id, category_name, strategic, retire_candidate}]`). 404 (not an empty 200) if the project ID doesn't exist — unlike `BatchProgressHandler`'s empty-map-on-unknown-batch behavior, since a report for a nonexistent project is a real error, not a legitimately-empty batch.
- `func ListProjectsHandler(store *Store) http.HandlerFunc` for `GET /api/projects` — calls `store.ListProjects`, returns `[]Project` as JSON.

#### 3. Routing

**File**: `backend/cmd/shelfmark/main.go`

**Intent**: Wire the three new routes with the role gating agreed for this slice: Architect uploads, Architect + Viewer view/list.

**Contract**: Add after the existing `/api/organization/*` lines:
```go
mux.HandleFunc("POST /api/projects/upload", auth.RequireRole(auth.RoleArchitect)(catalog.ProjectUploadHandler(catalogStore, catalogQueue)))
mux.HandleFunc("GET /api/projects", auth.RequireRole(auth.RoleArchitect, auth.RoleViewer)(catalog.ListProjectsHandler(catalogStore)))
mux.HandleFunc("GET /api/projects/{projectID}/report", auth.RequireRole(auth.RoleArchitect, auth.RoleViewer)(catalog.ProjectReportHandler(catalogStore)))
```

### Success Criteria:

#### Automated Verification:

- Unit tests pass: `go test ./internal/catalog/...` covering upload (new project, re-upload replacing a prior set, a purl already in the catalog under a different batch, an unknown purl triggering enqueue) and report (direct+strategic → not flagged, direct+non-strategic in a category with a strategic mark → flagged, direct in a never-reviewed category → not flagged, transitive → never flagged)
- `go vet ./...` passes

#### Manual Verification:

- Uploading a project SBOM via `curl`/Postman as an Architect account succeeds and the report reflects the uploaded components
- Uploading as a Viewer account is rejected with 403; viewing a report as a Viewer succeeds
- Re-uploading the same project name (case-insensitive) updates the existing project's report rather than creating a second one

**Implementation Note**: After completing this phase and all automated verification passes, pause here for manual confirmation from the human that the manual testing was successful before proceeding to the next phase.

---

## Phase 4: Frontend — redundancy report page

### Overview

Adds the `/redundancy` page: an upload form (project name + SBOM file, Architect-only) and a project picker + report table (Architect and Viewer), following the combined upload+table layout already used by `catalog.tsx` and `organization.tsx`.

### Changes Required:

#### 1. API client

**File**: `web/app/lib/projects.ts` (new)

**Intent**: Typed fetch wrappers for the three new endpoints, matching the pattern in `lib/catalog.ts`/`lib/organization.ts` (own `errorMessage` helper, no shared client).

**Contract**:
- `type Project = { id: string; name: string; updated_at: string }`
- `type ProjectReportCategory = { category_id: string; category_name: string; strategic: boolean; retire_candidate: boolean }`
- `type ProjectReportLibrary = { id: string; purl: string; name: string; ecosystem: string; github_repo: string | null; status: string; is_direct: boolean; categories: ProjectReportCategory[] }`
- `uploadProjectSbom(projectName: string, file: File): Promise<{ project_id: string; project_name: string; batch_id: string; created: number; skipped: SkippedRow[] }>`
- `listProjects(): Promise<Project[]>`
- `getProjectReport(projectId: string): Promise<ProjectReportLibrary[]>`
- Re-export/reuse `getBatchProgress` from `lib/catalog.ts` for the post-upload polling step (same `batch_id` shape).

#### 2. Route

**File**: `web/app/routes.ts`

**Intent**: Register the new page.

**Contract**: Add `route("redundancy", "routes/redundancy.tsx")` to the exported array.

#### 3. Page component

**File**: `web/app/routes/redundancy.tsx` (new)

**Intent**: Architect sees an upload form (project name text input + file input) above a project picker (`<select>` populated from `listProjects()`) and, once a project is selected, a report table grouped by category showing each direct dependency's strategic/retire-candidate status; Viewer sees the same picker and table without the upload form.

**Contract**: Default export wraps content in `<RequireAuth role={["architect", "viewer"]}>`, same as `organization.tsx:310-316`. On upload submit: call `uploadProjectSbom`, then poll `getBatchProgress(batch_id)` with the same interval/timeout constants as `catalog.tsx:19-25` until settled, then select the returned `project_id` and call `getProjectReport`. On picker change: call `getProjectReport(projectId)` directly. Table render follows the plain-`<table>` + inline-Tailwind convention of `organization.tsx`/`catalog.tsx` — one row per library, with a nested list or repeated sub-rows for its categories showing strategic/retire-candidate flags (e.g. a "Retire?" column rendered only for rows where `retire_candidate` is true in at least one category). Upload form itself is conditionally rendered only when `me?.role === "architect"`, matching `organization.tsx:15, 242, 255`'s `isArchitect` gating.

### Success Criteria:

#### Automated Verification:

- Type checking passes: `npm run typecheck`
- Build succeeds: `npm run build`

#### Manual Verification:

- As Architect: upload a project SBOM, see the upload settle, see the report populate with correct direct/transitive and retire-candidate flags
- As Architect: re-upload the same project name with a different SBOM and confirm the report now reflects only the new upload's contents
- As Viewer: see the project picker and report table, with no upload form visible
- As a logged-out user: redirected to `/login` when visiting `/redundancy`

**Implementation Note**: After completing this phase and all automated verification passes, pause here for manual confirmation from the human that the manual testing was successful before proceeding to the next phase.

---

## Testing Strategy

### Unit Tests:

- SBOM dependency-graph parsing: full graph, no graph (fallback), graph present but root unresolved (fallback)
- `ReplaceProjectLibraries` transactional replace: re-upload removes libraries no longer present, keeps the project row stable (same ID across uploads)
- `ProjectReport` retire-candidate computation: all four cases (strategic, non-strategic-in-reviewed-category, non-strategic-in-unreviewed-category, transitive)
- Upload handler: unknown purl triggers enqueue with `Status == "pending"`; already-categorized purl is not re-enqueued

### Integration Tests:

- Full upload → report round trip against a real Postgres (via the existing Docker-based test helper), covering a re-upload that changes which libraries are direct

### Manual Testing Steps:

1. Seed the catalog with a few Maven libraries across 2+ categories, mark one strategic per category (via `/organization`, as in S-02)
2. As Architect, upload a project SBOM containing: a strategic library, a non-strategic direct library in a reviewed category, a direct library in an unreviewed category, and a transitive-only library
3. Confirm the report flags only the non-strategic direct library in the reviewed category as a retire candidate
4. Re-upload the same project name with a trimmed-down SBOM and confirm the report shrinks accordingly
5. As Viewer, confirm read-only access to the same report

## Migration Notes

Additive only — new tables, no changes to existing `libraries`/`categories`/`strategic_marks` rows or columns. `libraries.is_direct` is left in place but unused by this feature (no migration removes it, since S-02 already shipped it and other code may still read its default).

## References

- Similar implementation: `backend/internal/catalog/handlers.go:195-255` (`ListOrganizationLibrariesHandler`), `backend/internal/catalog/handlers.go:55-115` (`UploadHandler`)
- Prior plan: `context/archive/2026-10-17-mark-strategic-libraries/plan.md`
- Prior plan: `context/archive/2026-10-16-seed-classification-catalog/plan.md`

## Progress

> Convention: `- [ ]` pending, `- [x]` done. Append ` — <commit sha>` when a step lands. Do not rename step titles. See `references/progress-format.md`.

### Phase 1: Project data model & store layer

#### Automated

- [x] 1.1 Migration applies cleanly — d8b6f2d
- [x] 1.2 Unit tests pass — d8b6f2d
- [x] 1.3 go vet passes — d8b6f2d

### Phase 2: SBOM dependency-graph parsing

#### Automated

- [x] 2.1 Unit tests pass — 565e9c6
- [x] 2.2 go vet passes — 565e9c6

### Phase 3: Project upload & report endpoints

#### Automated

- [x] 3.1 Unit tests pass
- [x] 3.2 go vet passes

#### Manual

- [x] 3.3 Upload as Architect succeeds and report reflects uploaded components
- [x] 3.4 Upload as Viewer rejected with 403; view as Viewer succeeds
- [x] 3.5 Re-upload with same project name updates existing project

### Phase 4: Frontend — redundancy report page

#### Automated

- [ ] 4.1 Type checking passes
- [ ] 4.2 Build succeeds

#### Manual

- [ ] 4.3 Architect upload → settle → report populates correctly
- [ ] 4.4 Re-upload same project name reflects only new contents
- [ ] 4.5 Viewer sees picker and table, no upload form
- [ ] 4.6 Logged-out user redirected to /login
