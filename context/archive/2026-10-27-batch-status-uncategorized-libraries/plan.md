# Surface failed and uncategorized libraries — Implementation Plan

## Overview

`/redundancy` reports "Batch <id> — done" and an empty report ("No libraries recorded for this project.") when categorization produced no categories, although every library was stored. This is test-plan risk #1 ("reports success on an incomplete result"). This plan makes the batch status and the project report tell the truth: architects can read real batch progress, the batch line names libraries that need review or failed, and the report lists every project library, with uncategorized ones shown by status.

## Current State Analysis

Three independent defects combine into the silent failure:

1. **Batch progress is unreachable for the uploader.** `GET /api/catalog/batches/{batchID}` is admin-only (`backend/cmd/shelfmark/routes.go:48`), but `POST /api/projects/upload` is architect-only. On `/redundancy` the architect's first poll gets a 403; `startPolling`'s `catch` (`web/app/routes/redundancy.tsx:121`) stops polling and calls `onSettled`, and the batch line renders `— done` because `polling` is false. A 60 s poll timeout also renders `— done`.
2. **The report drops uncategorized libraries.** `Store.ProjectReport` (`backend/internal/catalog/project.go`) inner-joins `library_categories` and `categories`, so a library in `pending`, `processing`, `needs_manual` or `failed` (no category rows) produces no row and vanishes from `ProjectReportHandler`'s response.
3. **"categorized" with zero categories.** `ProcessLibrary` (`backend/internal/catalog/pipeline.go`) marks a library `categorized` whenever the LLM reports `confident`, even if every returned name is outside the taxonomy and nothing was attached.

## Desired End State

- An architect polling a batch they uploaded gets real per-status counts.
- After the batch settles, the batch line reads `Batch <id> — done` only if every library is `categorized`; otherwise `Batch <id> — done: N need review, M failed` (only non-zero parts, destructive tone). A poll error reads `Batch <id> — status unavailable`; a poll timeout reads `Batch <id> — still processing`.
- `GET /api/projects/{id}/report` returns every library recorded against the project; libraries without categories carry `categories: []` (never `null`) and their `status`.
- In the report table, a library without categories shows a status badge in the Categories cell: `Needs review` (`needs_manual`), `Failed` (`failed`, destructive tone), `Processing` (`pending`/`processing`).
- A library whose LLM result attaches no taxonomy category ends `needs_manual`, not `categorized`.

### Key Discoveries:

- `backend/cmd/shelfmark/routes_test.go:28` pins the role set of every route — must change with the route.
- `ProjectReportHandler` already groups rows per library and initialises `Categories: []projectReportCategory{}` (`project_handlers.go`), so a LEFT JOIN only needs nullable category columns and a "no category" guard.
- The redundancy row's `RetireCandidate` is computed per category row; a category-less row must not be a retire candidate.
- The E2E spec asserts `^Batch .+ — done$` exactly (`web/tests/e2e/sbom-upload-report.spec.ts`), so the all-categorized wording must stay byte-identical.
- `Badge` (`web/app/components/badge.tsx`) supports only `accent` | `destructive`; CLAUDE.md says to extend its prop contract rather than duplicate it. `/dev/kitchen-sink-redundancy` is the living reference of tokenized states.
- `lessons.md`: list endpoints must marshal `[]`, not `null` — applies to the per-library `categories` of category-less libraries.

## What We're NOT Doing

- No new project-scoped batch-progress route; the existing route is widened to admin + architect (decision: architects may read any batch's status counts if they know its random ID).
- No change to `BatchProgress`'s response shape or to the `/catalog` admin page.
- No retry/re-categorize action from the report; no manual categorization from `/redundancy`.
- No change to the LLM prompt, OpenRouter client, or E2E mock.
- No schema migration.

## Implementation Approach

Fix the backend first so the API is truthful and covered by integration tests (Phase 1), then make the UI render that truth with the agreed wording (Phase 2), finishing with the existing E2E smoke test as a regression gate.

## Phase 1: Backend truthfulness

### Overview

Architects can read batch progress; the report includes every project library; zero attached categories means `needs_manual`.

### Changes Required:

#### 1. Batch progress access

**File**: `backend/cmd/shelfmark/routes.go`, `backend/cmd/shelfmark/routes_test.go`

**Intent**: Let the architect who uploads a project SBOM poll its batch.

**Contract**: `GET /api/catalog/batches/{batchID}` roles become `{admin, architect}`; the role-matrix test expects the same.

#### 2. Report includes uncategorized libraries

**File**: `backend/internal/catalog/project.go`

**Intent**: Return one row per project library even with no categories.

**Contract**: `ProjectReport` LEFT JOINs `library_categories`/`categories`/`strategic_marks`; `ProjectReportRow.CategoryID` and `CategoryName` become `*string` (nil for a category-less library); `RetireCandidate` stays false when `CategoryID` is nil. Ordering stays `l.purl, c.name`.

**File**: `backend/internal/catalog/project_handlers.go`

**Intent**: Group rows as today but skip appending a category for a nil-category row, so the library appears with `categories: []`.

**Contract**: `projectReportLibrary.Categories` is always non-nil.

#### 3. Zero attached categories → needs_manual

**File**: `backend/internal/catalog/pipeline.go`

**Intent**: A confident LLM answer whose names all fall outside the taxonomy attaches nothing; that library needs review, not `categorized`.

**Contract**: If no category was attached, `UpdateLibraryStatus(..., "needs_manual", &repo, nil)`; doc comment updated.

#### 4. Integration tests

**Files**: `backend/internal/catalog/pipeline_test.go`, `project_handlers_test.go`, `handlers_test.go` (or `project_handlers_test.go`) as fits existing patterns

**Intent**: Lock in the three behaviours against a real Postgres.

**Contract**:
- Pipeline: LLM returns only out-of-taxonomy names with `confident=true` → status `needs_manual`, no `library_categories` rows.
- Report: a project with one categorized, one `needs_manual` and one `failed` library returns all three; the uncategorized ones have `status` set and `categories` serialised as `[]` (raw JSON check, not `null`).
- Batch status: a project upload whose libraries end `needs_manual`/`failed` reports those counts from `BatchProgressHandler` for that batch (not only `categorized`).

### Success Criteria:

#### Automated Verification:

- Go build and vet pass: `cd backend && go build ./... && go vet ./...`
- Catalog and route tests pass, including the new integration tests: `cd backend && go test ./internal/catalog/... ./cmd/shelfmark/...`
- New tests fail when the fix is reverted (deliberate-break check on `ProjectReport` JOIN and the pipeline guard)

---

## Phase 2: /redundancy UI

### Overview

The batch line and the report render the truthful API data with the agreed wording.

### Changes Required:

#### 1. Honest polling outcome

**File**: `web/app/routes/redundancy.tsx`

**Intent**: Distinguish settled, timed-out and errored polls instead of collapsing all into "done".

**Contract**: polling state records an outcome `"settled" | "timeout" | "error"`; batch line text:
- processing → `— processing…` (unchanged)
- settled, all `categorized` → `— done` (unchanged; E2E depends on it)
- settled with `needs_manual`/`failed` → `— done: N need review, M failed` (omit zero parts; `1 needs review` singular) in `text-destructive`
- timeout → `— still processing`
- error → `— status unavailable` in `text-destructive`
The report is still refreshed on every outcome.

#### 2. Neutral badge tone

**File**: `web/app/components/badge.tsx`

**Intent**: Extend the shared Badge for a non-alarming status.

**Contract**: `tone: "accent" | "destructive" | "neutral"`; neutral uses tokens only (e.g. `border border-rule text-ink-dim`).

#### 3. Status badge for uncategorized libraries

**File**: `web/app/routes/redundancy.tsx`

**Intent**: Replace the bare `—` in the Categories cell of a category-less library with its status.

**Contract**: `needs_manual` → `<Badge tone="neutral">Needs review</Badge>`; `failed` → `<Badge tone="destructive">Failed</Badge>`; `pending`/`processing` → `<Badge tone="neutral">Processing</Badge>`; any other status → `—`.

#### 4. Kitchen-sink reference

**File**: `web/app/routes/dev.kitchen-sink-redundancy.tsx`

**Intent**: Keep the living reference current with the new badge tone and batch-line states.

**Contract**: shows the neutral badge, the three status badges and the done-with-issues / status-unavailable lines.

### Success Criteria:

#### Automated Verification:

- Typecheck passes: `cd web && npm run typecheck`
- Production build passes: `cd web && npm run build`
- No literal palette colors in touched route files: `grep -nE '(gray|red|amber|green|blue)-[0-9]' web/app/routes/redundancy.tsx web/app/routes/dev.kitchen-sink-redundancy.tsx` returns nothing
- E2E smoke passes: `cd web && npx playwright test tests/e2e/sbom-upload-report.spec.ts` (needs `docker compose up -d db`)

#### Manual Verification:

- On a local instance, an SBOM whose libraries the categorizer can't place shows `Batch … — done: N need review` and the report lists every library with a `Needs review` badge

**Implementation Note**: After automated verification passes, pause for manual confirmation before closing the phase.

---

## Testing Strategy

### Integration Tests:

- Pipeline out-of-taxonomy → `needs_manual` (Phase 1).
- Report lists categorized + `needs_manual` + `failed` libraries; `categories` is `[]` (Phase 1).
- Batch progress for a project upload reports `needs_manual`/`failed` counts (Phase 1).
- Route role matrix: architect allowed on batch progress (Phase 1).

### Manual Testing Steps:

1. Run the app locally with the E2E external-APIs mock, upload an SBOM whose artifacts don't start with `json-`/`http-` (mock answers 503 → retries → `failed`) or that the categorizer can't place.
2. Confirm the batch line and report badges match the Desired End State.

## References

- Change identity: `context/changes/batch-status-uncategorized-libraries/change.md`
- Test-plan risk #1: `context/foundation/test-plan.md`
- Lesson: `context/foundation/lessons.md` (nil slices marshal as `null`)
- E2E smoke: `web/tests/e2e/sbom-upload-report.spec.ts`

## Review Addendum

- 2026-10-27 impl-review F1: the status-to-badge mapping planned inline in `redundancy.tsx` (Phase 2 §3) lives in a shared component, `web/app/components/library-status-badge.tsx` (`LibraryStatusBadge`). It is used by `redundancy.tsx` and the kitchen sink, and its mapping is unchanged.
- 2026-10-27 impl-review F2: `web/tests/e2e/sbom-upload-report.spec.ts` now waits for the batch line to settle and then asserts `— done`, so an incomplete batch fails with its real status.

## Progress

> Convention: `- [ ]` pending, `- [x]` done. Append ` — <commit sha>` when a step lands. Do not rename step titles. See `references/progress-format.md`.

### Phase 1: Backend truthfulness

#### Automated

- [x] 1.1 Go build and vet pass — ad2e7dd
- [x] 1.2 Catalog and route tests pass, including the new integration tests — ad2e7dd
- [x] 1.3 New tests fail when the fix is reverted — ad2e7dd

### Phase 2: /redundancy UI

#### Automated

- [x] 2.1 Typecheck passes — 4b1fc0b
- [x] 2.2 Production build passes — 4b1fc0b
- [x] 2.3 No literal palette colors in touched route files — 4b1fc0b
- [x] 2.4 E2E smoke passes — 4b1fc0b

#### Manual

- [x] 2.5 Uncategorizable SBOM shows done-with-issues line and Needs review badges — 4b1fc0b
