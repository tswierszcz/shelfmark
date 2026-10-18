# Architect marks strategic libraries — Implementation Plan

## Overview

Give the Architect role a view onto the organization's library catalog and the ability to mark a library within a category as strategic, with a justification note. This is S-02 on the roadmap: the prerequisite for S-03's redundancy report, which needs to know which direct library per category/ecosystem is the "keep" choice.

## Current State Analysis

S-01 built the catalog schema (`categories`, `libraries`, `library_categories`) and an Admin-only catalog page (`web/app/routes/catalog.tsx`) backed by Admin-only endpoints under `/api/catalog/*`. There is no concept of "direct" vs "transitive" anywhere in the data model yet — neither `ParseCycloneDXSBOM` (`backend/internal/catalog/sbom.go`) nor `ParsePurlCSV` extracts that signal, because it is a per-project property that only becomes meaningful once S-03 ingests a project's own SBOM. There is also no concept of a "strategic" designation yet.

### Key Discoveries:

- `library_categories` is a many-to-many join (`backend/internal/db/migrations/0002_catalog_schema.up.sql:38`), so a library's strategic status is naturally scoped to a `(library, category)` pair, not to the library as a whole.
- `AttachCategory` (`backend/internal/catalog/store.go:245`) is the existing idiom for an idempotent upsert into a join-ish table — `MarkStrategic`/`UnmarkStrategic` follow the same shape.
- `RequireRole` (`backend/internal/auth/middleware.go:25`) takes a variadic role list per-route already; widening or adding routes with a different role set requires no change there. The frontend's `RequireAuth` (`web/app/lib/require-auth.tsx:11`) currently accepts only a single `role`, and needs to accept a set.
- `ListLibrariesHandler` (`backend/internal/catalog/handlers.go:145`) and its `libraryResponse` shape are the pattern to extend (not fork) for the new endpoint, so both Admin's catalog view and the new organization view share one query path.
- Migration numbering is sequential with paired `.up.sql`/`.down.sql` files (`backend/internal/db/migrations/0001_*`, `0002_*`); this change adds `0003_strategic_marks.{up,down}.sql`.
- Store integration tests spin up a throwaway Postgres via Docker and skip (not fail) when Docker is unavailable (`backend/internal/catalog/store_test.go:24`); new store tests follow that pattern.

## Desired End State

An Architect (or Viewer, read-only) can open `/organization`, see every catalog library with its categories and current strategic marks, and an Architect can mark or unmark a library as strategic within a given category, entering a short justification each time. There is no limit on how many libraries can be marked strategic within one category+ecosystem (see "What We're NOT Doing" — this is a deliberate departure from the PRD's literal "up to two," confirmed with the user during planning).

Verification: `go test ./...` and `go vet ./...` pass in `backend/`; `npm run typecheck` and `npm run build` pass in `web/`; manually, an Architect account can mark/unmark a library strategic with a justification and see the change reflected immediately, and a Viewer account can see the same data but has no mark/unmark controls.

## What We're NOT Doing

- Not implementing a real direct/transitive signal. `libraries.is_direct` is added now, defaulted `true` for every row, and nothing in this change ever sets it to `false`. S-03 is expected to populate it meaningfully once real per-project SBOM data exists.
- Not enforcing any cap on the number of strategic marks per category+ecosystem. The PRD (FR-007) states "up to two"; this plan ships unbounded instead, per an explicit decision made during planning. If the cap is wanted later, it is a follow-up change, not a silent addition here.
- Not touching the existing Admin catalog page or its endpoints beyond widening `/api/catalog/categories` to allow Architect/Viewer (shared reference data, no sensitive content).
- Not building an audit history of every mark/unmark event — the `strategic_marks` row keeps only the latest action (active mark or most recent unmark reason), not a full timeline.
- Not adding ecosystem as its own column on `strategic_marks` — ecosystem is already derivable via `libraries.ecosystem` through the `library_id` join.

## Implementation Approach

Add the schema in one migration, extend the existing `catalog.Store` (not a new store) with the new methods and fields, add new HTTP handlers reusing `ListLibraries`/`libraryResponse` conventions, and add a parallel frontend page that mirrors `catalog.tsx`'s structure but targets Architect/Viewer and renders strategic controls instead of upload/categorize controls.

## Phase 1: Schema migration

### Overview

Add the `is_direct` column to `libraries` and the new `strategic_marks` table.

### Changes Required:

#### 1. Migration files

**File**: `backend/internal/db/migrations/0003_strategic_marks.up.sql`

**Intent**: Add `is_direct` to `libraries` so the catalog can eventually distinguish direct from transitive libraries, and create `strategic_marks` to record an Architect's strategic designation for a `(library, category)` pair.

**Contract**:
```sql
ALTER TABLE libraries ADD COLUMN is_direct boolean NOT NULL DEFAULT true;

CREATE TABLE strategic_marks (
    library_id   uuid NOT NULL REFERENCES libraries (id) ON DELETE CASCADE,
    category_id  uuid NOT NULL REFERENCES categories (id) ON DELETE RESTRICT,
    active       boolean NOT NULL DEFAULT true,
    justification text NOT NULL,
    marked_by    text NOT NULL,
    marked_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (library_id, category_id)
);

CREATE INDEX strategic_marks_active_idx ON strategic_marks (category_id) WHERE active;
```
A row always exists once a library has ever been marked strategic in a category; `active` distinguishes a current mark from a since-removed one, so the most recent justification/actor for an unmark is still readable without a separate audit table.

**File**: `backend/internal/db/migrations/0003_strategic_marks.down.sql`

**Intent**: Reverse the up migration.

**Contract**:
```sql
DROP TABLE strategic_marks;
ALTER TABLE libraries DROP COLUMN is_direct;
```

### Success Criteria:

#### Automated Verification:

- Migration applies cleanly: `cd backend && go test ./internal/db/...`
- Full migration round-trip (up then down) succeeds locally via `migrate` tooling already exercised by `internal/db/migrate_test.go`

---

## Phase 2: Store layer

### Overview

Teach `catalog.Store` about `is_direct` and strategic marks.

### Changes Required:

#### 1. `Library` and `LibraryFilter`

**File**: `backend/internal/catalog/store.go`

**Intent**: Surface `is_direct` on every `Library`, and let callers filter the library list by current strategic status.

**Contract**: Add `IsDirect bool` to the `Library` struct (json tag `is_direct`). Add `Strategic *bool` to `LibraryFilter` (nil = no filter; `true`/`false` = has/has-not at least one row in `strategic_marks` with `active = true`). `ListLibraries`'s SELECT and scan gain the new column; a non-nil `Strategic` filter adds an `EXISTS`/`NOT EXISTS` subquery against `strategic_marks` rather than a join, so it doesn't interact with the existing `DISTINCT` + category join.

#### 2. Strategic mark methods

**File**: `backend/internal/catalog/store.go`

**Intent**: Mark, unmark, and read back strategic designations, following `AttachCategory`'s upsert idiom.

**Contract**:
- `MarkStrategic(ctx, libraryID, categoryID, justification, markedBy string) error` — upserts into `strategic_marks` with `ON CONFLICT (library_id, category_id) DO UPDATE SET active = true, justification = EXCLUDED.justification, marked_by = EXCLUDED.marked_by, marked_at = now()`.
- `UnmarkStrategic(ctx, libraryID, categoryID, justification, markedBy string) error` — `UPDATE strategic_marks SET active = false, justification = $3, marked_by = $4, marked_at = now() WHERE library_id = $1 AND category_id = $2 AND active = true`; returns an error (library/category not currently marked strategic) when `RowsAffected() == 0`, matching `UpdateLibraryStatus`'s existing not-found convention.
- `StrategicMarksByLibrary(ctx, libraryIDs []string) (map[string][]StrategicMark, error)` — returns only `active = true` rows, keyed by library ID, each entry carrying `CategoryID`, `CategoryName`, `Justification`, `MarkedBy`. Mirrors `CategoryNamesByLibrary`'s shape and "missing key = empty" contract.

New exported type:
```go
type StrategicMark struct {
    CategoryID   string `json:"category_id"`
    CategoryName string `json:"category_name"`
    Justification string `json:"justification"`
    MarkedBy     string `json:"marked_by"`
}
```

### Success Criteria:

#### Automated Verification:

- `cd backend && go test ./internal/catalog/...` passes (new cases in `store_test.go` for `MarkStrategic`, `UnmarkStrategic` happy path, `UnmarkStrategic` on a never-marked pair returns an error, `StrategicMarksByLibrary` reflects only active marks, `ListLibraries` with `Strategic` filter)
- `go vet ./...` passes

---

## Phase 3: HTTP handlers and routing

### Overview

Expose the new store capabilities over HTTP, scoped to Architect (mark/unmark) and Architect+Viewer (read).

### Changes Required:

#### 1. New handlers

**File**: `backend/internal/catalog/handlers.go`

**Intent**: List libraries enriched with `is_direct` and strategic marks for the organization view, and let an Architect mark/unmark a library as strategic within a category.

**Contract**:
- `ListOrganizationLibrariesHandler(store *Store) http.HandlerFunc` for `GET /api/organization/libraries` — same query-param filter surface as `ListLibrariesHandler` (`status`, `category`, `ecosystem`) plus `strategic` (`"true"`/`"false"`, absent = no filter), reusing `ListLibraries` + `CategoryNamesByLibrary` + the new `StrategicMarksByLibrary`. Response is `libraryResponse` (already has `Categories`) extended with a `Strategic []StrategicMark` field.
- `MarkStrategicHandler(store *Store) http.HandlerFunc` for `POST /api/organization/libraries/{libraryID}/categories/{categoryID}/strategic` — body `{"justification": "..."}`, 400 if blank; reads the acting Architect's email via `auth.SessionFromContext(r.Context())` for `markedBy`; 204 on success.
- `UnmarkStrategicHandler(store *Store) http.HandlerFunc` for `DELETE /api/organization/libraries/{libraryID}/categories/{categoryID}/strategic` — same body contract as mark; 404 if the pair isn't currently marked strategic (store method's not-found error).

#### 2. Routing

**File**: `backend/cmd/shelfmark/main.go`

**Intent**: Wire the new routes with the agreed role split, and widen taxonomy access.

**Contract**: Add three `mux.HandleFunc` lines for the organization endpoints — the list route gated `auth.RequireRole(auth.RoleArchitect, auth.RoleViewer)`, the mark/unmark routes gated `auth.RequireRole(auth.RoleArchitect)`. Change the existing `GET /api/catalog/categories` line's `RequireRole(auth.RoleAdmin)` to `RequireRole(auth.RoleAdmin, auth.RoleArchitect, auth.RoleViewer)` — taxonomy names are non-sensitive reference data needed by the new page too.

### Success Criteria:

#### Automated Verification:

- `cd backend && go test ./internal/catalog/... ./cmd/...` passes (new `handlers_test.go` cases: list reflects strategic marks, mark/unmark round-trip, role gating returns 403 for a Viewer attempting to mark, 401 with no session)
- `go build ./...` succeeds

#### Manual Verification:

- An Architect session can `curl` the new endpoints successfully; a Viewer session gets 403 on mark/unmark but 200 on list

---

## Phase 4: Frontend

### Overview

Add the `/organization` page for Architect (full access) and Viewer (read-only), and the API client it uses.

### Changes Required:

#### 1. API client

**File**: `web/app/lib/organization.ts`

**Intent**: Typed fetch wrappers for the new endpoints, mirroring `lib/catalog.ts`'s style (plain `fetch`, `errorMessage` helper, exported types).

**Contract**: Exports `OrganizationLibrary` (= `Library` + `is_direct: boolean` + `strategic: StrategicMark[]`), `StrategicMark` type, `listOrganizationLibraries(filter)`, `markStrategic(libraryId, categoryId, justification)`, `unmarkStrategic(libraryId, categoryId, justification)`.

#### 2. Role-set support in `RequireAuth`

**File**: `web/app/lib/require-auth.tsx`

**Intent**: Let a page allow more than one role, needed because this page is Architect+Viewer.

**Contract**: `role` prop becomes `"admin" | "architect" | "viewer" | Array<"admin" | "architect" | "viewer">`; `wrongRole` checks membership (`Array.isArray(role) ? !role.includes(me.role) : me.role !== role`) instead of strict equality. Existing single-role callers (`catalog.tsx`, `dashboard.tsx`) are unaffected since a bare string still works.

#### 3. Organization page

**File**: `web/app/routes/organization.tsx`

**Intent**: List libraries with their categories, `is_direct` flag, and current strategic marks; for an Architect, render a mark/unmark control per `(library, category)` pair that prompts for a justification before submitting.

**Contract**: `RequireAuth role={["architect", "viewer"]}` wrapping a content component analogous to `CatalogContent` — same filter inputs (status/category/ecosystem) plus a strategic filter, same table-based listing. The mark/unmark control (button + justification `<textarea>` in a small inline form, matching the existing category-attach `<select>` + button pattern in `catalog.tsx`) only renders when `me.role === "architect"`.

#### 4. Route registration

**File**: `web/app/routes.ts`

**Intent**: Register the new page.

**Contract**: Add `route("organization", "routes/organization.tsx")` to the exported route array.

### Success Criteria:

#### Automated Verification:

- `cd web && npm run typecheck` passes
- `cd web && npm run build` passes

#### Manual Verification:

- Logged in as Architect, `/organization` lists libraries, marking a library strategic with a justification shows it as strategic immediately, and unmarking it (with a justification) removes the strategic badge
- Logged in as Viewer, `/organization` shows the same list with no mark/unmark controls
- Logged in as Admin, `/organization` redirects away (role not permitted), matching `RequireAuth`'s existing redirect-to-dashboard behavior

---

## Testing Strategy

### Unit Tests:

- `MarkStrategic` upsert semantics (re-marking updates justification/actor, doesn't duplicate rows)
- `UnmarkStrategic` on a pair never marked strategic returns an error
- `StrategicMarksByLibrary` excludes `active = false` rows
- `ListLibraries` with `Strategic: ptr(true)`/`ptr(false)` filters correctly

### Integration Tests:

- Full handler round-trip: mark → list shows it → unmark → list no longer shows it, across Architect/Viewer/Admin sessions for role gating

### Manual Testing Steps:

1. As Architect, mark a library strategic in a category with a justification; confirm it appears in the list with that justification.
2. Mark a second (and third) library strategic in the same category+ecosystem; confirm no cap error (per "What We're NOT Doing").
3. Unmark one of them with a different justification; confirm the list updates and the removed justification isn't silently lost from view if the UI still shows a "last changed" note.
4. As Viewer, load `/organization` and confirm no mark/unmark controls are rendered and a direct `fetch` to the mark endpoint from the browser console returns 403.

## References

- Existing catalog store/handlers: `backend/internal/catalog/store.go`, `backend/internal/catalog/handlers.go`
- Existing catalog page: `web/app/routes/catalog.tsx`, `web/app/lib/catalog.ts`
- Role model: `backend/internal/auth/accounts.go`, `backend/internal/auth/middleware.go`
- Roadmap entry: `context/foundation/roadmap.md` (S-02)
- PRD refs: `context/foundation/prd.md` FR-006, FR-007

## Progress

> Convention: `- [ ]` pending, `- [x]` done. Append ` — <commit sha>` when a step lands. Do not rename step titles.

### Phase 1: Schema migration

#### Automated

- [x] 1.1 Migration applies cleanly — bcb3eb0
- [x] 1.2 Full migration round-trip (up then down) succeeds — bcb3eb0

### Phase 2: Store layer

#### Automated

- [x] 2.1 `go test ./internal/catalog/...` passes — bf57e29
- [x] 2.2 `go vet ./...` passes — bf57e29

### Phase 3: HTTP handlers and routing

#### Automated

- [x] 3.1 `go test ./internal/catalog/... ./cmd/...` passes
- [x] 3.2 `go build ./...` succeeds

#### Manual

- [ ] 3.3 Architect/Viewer role gating verified via curl

### Phase 4: Frontend

#### Automated

- [ ] 4.1 `npm run typecheck` passes
- [ ] 4.2 `npm run build` passes

#### Manual

- [ ] 4.3 Architect mark/unmark flow verified in browser
- [ ] 4.4 Viewer read-only access verified in browser
- [ ] 4.5 Admin redirect-away verified in browser
