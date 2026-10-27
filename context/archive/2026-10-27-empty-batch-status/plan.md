# Settle an empty upload batch — Implementation Plan

## Overview

An upload whose purls are all already in the catalog creates no libraries under its batch, so batch progress is `{}`. `/redundancy` treats `{}` as "not settled", polls for 60 s, and then shows `Batch <id> — still processing` although nothing was enqueued. Settle such a batch immediately with the user-approved wording `— done: no new libraries to categorize`.

## Current State Analysis

- `Store.CreateLibraries` inserts with `ON CONFLICT (purl) DO NOTHING` (`backend/internal/catalog/store.go:103-120`), so a known purl keeps its original `upload_batch_id`.
- `ProjectUploadHandler` writes the libraries before responding, so an empty progress map on the first poll can only mean "no new libraries"; it is not a race.
- `isBatchSettled` returns false for an empty map (`web/app/routes/redundancy.tsx:23-28`), and `describeBatch` has no empty-batch case.

## Desired End State

Uploading an SBOM of only known libraries shows `Batch <id> — done: no new libraries to categorize` on the first poll, in the normal (non-destructive) tone. The report is refreshed as usual. All other batch-line states stay unchanged.

## What We're NOT Doing

- No backend behaviour change. Batch progress stays `{}` for such a batch, and the upload response is unchanged.
- No change to how known libraries are re-processed. They are still not re-enqueued.
- No poll-retry change (impl-review F3 of batch-status-uncategorized-libraries stays a separate follow-up).

## Phase 1: Empty batch settles as done

### Changes Required:

#### 1. Polling and batch line

**File**: `web/app/routes/redundancy.tsx`

**Intent**: Treat an empty progress map as settled, and describe it as "no new libraries".

**Contract**: `isBatchSettled({}) === true`. `describeBatch` returns `" — done: no new libraries to categorize"` (problem: false) when settled and the progress map is empty.

#### 2. Kitchen sink

**File**: `web/app/routes/dev.kitchen-sink-redundancy.tsx`

**Intent**: Add the new batch line to section 8.

#### 3. Tests

**Files**: `backend/internal/catalog/project_handlers_test.go`, `web/tests/e2e/sbom-upload-report.spec.ts`

**Intent**: Lock in the backend contract the UI relies on, and the UI behaviour.

**Contract**:
- Go: re-uploading an SBOM whose purls all exist yields `BatchProgressHandler` body `{}` for the new batch, while the report still lists the project's libraries.
- E2E: after the existing first upload settles, re-uploading the same SBOM under the same project shows `— done: no new libraries to categorize` well under the 60 s poll timeout.

### Success Criteria:

#### Automated Verification:

- Catalog tests pass: `cd backend && go test ./internal/catalog/...`
- Typecheck and build pass: `cd web && npm run typecheck && npm run build`
- E2E suite passes: `cd web && npx playwright test`
- The E2E re-upload assertion fails when the empty-batch handling is reverted

#### Manual Verification:

- On production after deploy, re-uploading `docs/samples/smoke-sbom.cdx.json` shows `— done: no new libraries to categorize`

## References

- Previous change: `context/archive/2026-10-27-batch-status-uncategorized-libraries/`

## Progress

> Convention: `- [ ]` pending, `- [x]` done. Append ` — <commit sha>` when a step lands. Do not rename step titles. See `references/progress-format.md`.

### Phase 1: Empty batch settles as done

#### Automated

- [x] 1.1 Catalog tests pass — 23ecc11
- [x] 1.2 Typecheck and build pass — 23ecc11
- [x] 1.3 E2E suite passes — 23ecc11
- [x] 1.4 The E2E re-upload assertion fails when the empty-batch handling is reverted — 23ecc11

#### Manual

- [x] 1.5 Production re-upload shows the no-new-libraries line — 23ecc11
