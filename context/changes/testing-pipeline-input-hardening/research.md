---
date: 2026-10-21T22:00:23+02:00
researcher: Claude (Sonnet 5)
git_commit: 9a3dc902cb6e607ca1e7a5254e3de4ea8bea9d0a
branch: shape/shelfmark
repository: shelfmark
topic: "Critical-path pipeline & input hardening — grounding for context/foundation/test-plan.md §3 Phase 1 (Risks #1 and #7)"
tags: [research, codebase, catalog, pipeline, sbom, csv, llm, openrouter, queue]
status: complete
last_updated: 2026-10-21
last_updated_by: Claude (Sonnet 5)
---

# Research: Critical-path pipeline & input hardening

**Date**: 2026-10-21T22:00:23+02:00
**Researcher**: Claude (Sonnet 5)
**Git Commit**: 9a3dc902cb6e607ca1e7a5254e3de4ea8bea9d0a
**Branch**: shape/shelfmark
**Repository**: shelfmark

## Research Question

This change (`testing-pipeline-input-hardening`) is Phase 1 of `context/foundation/test-plan.md` §3: "Prove the SBOM/LLM pipeline doesn't silently drop data and malformed input can't crash it," covering:

- **Risk #1**: SBOM/LLM categorization pipeline silently drops libraries, stalls in the background, or reports success on an incomplete result.
- **Risk #7**: An uploaded SBOM or CSV with malformed/oversized/adversarial content crashes the backend or reaches the LLM prompt unsanitized.

The test-plan's "Context `/10x-research` must ground" column asks specifically for: the pipeline entry point, where batch/queue status is persisted, the shape of an LLM/GitHub timeout error (Risk #1); and current parser behavior on empty/malformed/huge input (Risk #7).

## Summary

The pipeline is asynchronous: `UploadHandler`/`ProjectUploadHandler` parse the upload, insert rows, and enqueue each library onto an **in-memory, non-persistent** worker queue (`backend/internal/catalog/queue.go`), then return immediately. Per-library status (`pending`/`categorized`/`needs_manual`/`failed`) is the only persisted state — there is no batch-level "success" flag, but also no code anywhere that reconciles the count of libraries inserted against the count that reach a terminal status. On an LLM or GitHub API error, `Queue.process` retries the single library up to 3 times (500ms backoff, 30s per-attempt timeout) and on exhaustion marks it `"failed"` while discarding the actual error (`_ = lastErr`, `queue.go:96`) — other libraries in the batch are unaffected, but the queue's in-memory state is lost entirely on process restart with no record that affected libraries were ever enqueued.

For input hardening: SBOM/CSV parsers correctly guard against nil/malformed structure and return clean 400s for invalid JSON/CSV, and per-row problems (bad purl, non-Maven) are collected as `SkippedRow` rather than failing the whole upload. However, **no file-size limit exists anywhere in the catalog upload path** (unlike the 10KB cap on `auth/handlers.go` login endpoints), and purl-derived strings (attacker-controlled via the uploaded file) flow into LLM prompts via plain `fmt.Sprintf` with no sanitization beyond Go's `%q` escaping — a confirmed prompt-injection surface. Third-party parser libraries (`cyclonedx-go`, `packageurl-go`) were not auditable for panic-safety from this repo.

Existing tests cover the happy path and one classic malformed-JSON case; no test exercises: an empty upload end-to-end, an oversized upload, a full upload→async-categorization→terminal-status reconciliation, an LLM/GitHub timeout specifically (only a generic finder error is simulated), or a purl shaped to attempt prompt injection.

## Detailed Findings

### Pipeline entry points and async execution model

- `UploadHandler` (`backend/internal/catalog/handlers.go:55-115`) — `POST /api/catalog/upload` (registered `backend/cmd/shelfmark/main.go:50`). Parses via `ParseCycloneDXSBOM` or `ParsePurlCSV`, calls `store.CreateLibraries`, re-reads via `store.ListLibraries`, and enqueues each via the queue (handlers.go:103-105). Returns immediately after enqueueing.
- `ProjectUploadHandler` (`backend/internal/catalog/project_handlers.go:26-111`) — `POST /api/projects/upload` (main.go:58). Second upload entry point for project-scoped SBOMs; follows the same enqueue pattern.
- `BatchProgressHandler` (`backend/internal/catalog/handlers.go:121-134`) — `GET /api/catalog/batches/{batchID}` (main.go:51). The only status-polling endpoint; returns a `GROUP BY status` count map, no aggregate "done"/"success" field.

### Queue / status persistence (Risk #1)

- `Queue` (`backend/internal/catalog/queue.go:29-98`) is an **in-memory** worker pool: `NewQueue(..., workers)` starts goroutines (queue.go:48-50) reading a buffered channel (`queueBufferSize = 1000`, queue.go:10). Its own doc comment states state does not persist across process restarts (queue.go:27-28) — libraries sitting in the channel at restart are silently lost with no record they were ever enqueued.
- Per-library status lives in the `libraries.status` Postgres column, updated via `Store.UpdateLibraryStatus` (~store.go:258-269). `BatchProgress` (~store.go:298-325) does a `GROUP BY status` count.
- No batch-level "success" status exists — `uploadResponse` (handlers.go:34-38) reports only `Created`/`Skipped` at upload time, which says nothing about later categorization outcomes. **No code anywhere compares the number of libraries created against the number that reach a terminal status** (checked `handlers.go`, `pipeline.go`, `queue.go`, `store.go`).
- `Enqueue` (queue.go:58-60) sends on the channel with no timeout/select; a full channel blocks the HTTP request goroutine (acknowledged in an adjacent comment, queue.go:56-57).
- If `UpdateLibraryStatus(ctx, lib.ID, "processing", ...)` itself fails (queue.go:76-78), the function returns early — the library is stuck at its prior status (e.g. `pending`) permanently, with no retry and no surfaced error.

### LLM/GitHub error handling and timeout shape (Risk #1)

- GitHub call: `GitHubClient.Topics` (`backend/internal/catalog/github.go:41-74`) returns an error on request failure, non-200, or decode failure (lines 59-61, 64-66, 69-71). Caller `ProcessLibrary` (`pipeline.go:49-52`) wraps and returns it, aborting only that library's current attempt.
- LLM call: `OpenRouterClient.complete` (`backend/internal/catalog/openrouter.go:77-115`) returns an error on request failure, non-2xx, decode failure, or empty choices; `FindRepo`/`Categorize` propagate it. `pipeline.go:63-66` wraps and returns, same per-library abort semantics.
- Retry logic, `Queue.process` (`queue.go:73-98`): up to `maxAttempts = 3` (queue.go:15) with a fixed 500ms backoff (queue.go:18) and a 30s per-attempt timeout (queue.go:23, `processAttemptTimeout`). On exhaustion: `_ = lastErr; _ = q.store.UpdateLibraryStatus(ctx, lib.ID, "failed", nil)` — the real failure reason is discarded, only the bare string `"failed"` persists. The rest of the batch is unaffected — failures are isolated per library, not batch-aborting.

### SBOM/CSV parsers and edge-case input behavior (Risk #7)

- Parsers: `ParseCycloneDXSBOM` (`sbom.go:24-59`), `ParseProjectSBOM` (`project_sbom.go:26-97`), `ParsePurlCSV` (`csv.go:13-49`), shared `ParsePurl` (`purl.go:13-33`).
- Empty file: SBOM path fails as malformed JSON (EOF) → 400 (`sbom.go:26-28`, surfaced via `handlers.go:80-83`). CSV path: `csv.NewReader(r).ReadAll()` on empty input returns zero records with no error (`csv.go:17-20`) — upload succeeds with `Created: 0` and no signal to the caller that the file was empty.
- Malformed JSON/CSV: both return a wrapped error → HTTP 400 (`sbom.go:26-28,30-32`; `csv.go:17-20`). Per-row issues (missing/unparseable/non-Maven purl) are non-fatal, collected into `SkippedRow`, upload still returns 200 (`sbom.go:44-53`, `csv.go:39-43`).
- **No file-size limit found anywhere in the catalog upload path.** Grep for `MaxBytesReader`/`ParseMultipartForm`/`LimitReader` across `backend/` finds it used only in `auth/handlers.go:41,107` (10KB cap on login endpoints) — not in `catalog/handlers.go` or `catalog/project_handlers.go`. `r.FormFile("file")` relies on Go's default 32MB in-memory multipart threshold, which only governs memory-vs-disk spillover, not a hard size cap. An oversized or adversarially nested JSON/CSV body is read to completion unbounded.
- Extra/unexpected fields are silently ignored by both the CycloneDX struct decode and the CSV parser (`FieldsPerRecord = -1`, only `record[0]` used — csv.go:15,31).
- No unchecked type assertions, nil derefs, or unguarded indexing found in this repo's own parser code (nil-checks present at `sbom.go:39-41`, `project_sbom.go:43-45,52-54,57-66`, `csv.go:28-30`). Third-party libraries (`cyclonedx-go`, `packageurl-go`) were not auditable from this repo, so panic-safety on adversarial input inside those libraries is unverified, not ruled out.

### Prompt-injection surface (Risk #7)

Confirmed direct path: uploaded file → `ParsePurl`/`ParseCycloneDXSBOM`/`ParsePurlCSV` → `Library.Purl` (attacker-controlled substring, only validated as `packageurl.FromString` + "must be type=maven") → `ProcessLibrary` → `OpenRouterClient.FindRepo`/`Categorize` → LLM prompt via `fmt.Sprintf`, with no sanitization beyond Go's `%q` (which escapes Go-string syntax, not prompt structure):
- `openrouter.go:121-126` (`FindRepo`): `group`/`artifact`/`version` (from `pipeline.go:35-41`) interpolated directly into the prompt text.
- `openrouter.go:153-161` (`Categorize`): `purl` (= `lib.Purl`, `pipeline.go:63`) interpolated directly, along with GitHub topics and taxonomy names.

### Existing test coverage

- `pipeline_test.go` — covers categorized/needs_manual/confidence branching and that a finder error leaves status unchanged at `"pending"` (lines ~219-243) — but doesn't test what the caller does with that error afterward.
- `queue_test.go` — `TestQueue_TransientFinderError_RetriesThenMarksFailed` (84-104) asserts exactly 3 retries then `"failed"`. No test simulates an LLM/GitHub *timeout* specifically (only a generic finder error), no test covers process-restart data loss or channel-full blocking.
- `handlers_test.go` — `TestUploadHandler_SBOMCreatesLibrariesAndReturnsSummary` (75-117) checks 2 created + 1 skipped from a 3-component fixture and exactly 2 enqueued/persisted — i.e. N-in/N-out *at the parse+create stage only*, via a `recordingEnqueuer` fake that never exercises the real async queue or categorization completion. `TestBatchProgressHandler_ReturnsCountsForKnownBatch` (178-211) asserts `pending == 2` right after upload only — no poll-to-terminal-state, no total-count reconciliation.
- `sbom_test.go` — valid-input, malformed-JSON, and no-BOM-structure (`{}`) cases. No empty-input or large-input test.
- `csv_test.go` — valid with/without header row only. No malformed-CSV, empty-file, or large-file test.
- `project_sbom_test.go` — valid dependency-graph case; malformed/empty variants not confirmed present.
- No test anywhere uses a purl shaped to probe prompt-injection behavior in the LLM prompt.

## Code References

- `backend/internal/catalog/handlers.go:55-115` — `UploadHandler`
- `backend/internal/catalog/handlers.go:121-134` — `BatchProgressHandler`
- `backend/internal/catalog/project_handlers.go:26-111` — `ProjectUploadHandler`
- `backend/cmd/shelfmark/main.go:50,51,58` — route registration
- `backend/internal/catalog/queue.go:10,15,18,23,27-28,29-98,48-50,56-60,73-98` — in-memory queue, retry/backoff/timeout constants, non-persistence doc comment
- `backend/internal/catalog/pipeline.go:35-41,49-52,63-66` — `ProcessLibrary`, purl parsing, GitHub/LLM call sites
- `backend/internal/catalog/github.go:41-74` — `GitHubClient.Topics`
- `backend/internal/catalog/openrouter.go:77-115,121-126,153-161` — `complete`, `FindRepo` prompt, `Categorize` prompt
- `backend/internal/catalog/sbom.go:24-59` — `ParseCycloneDXSBOM`
- `backend/internal/catalog/project_sbom.go:26-97` — `ParseProjectSBOM`
- `backend/internal/catalog/csv.go:13-49` — `ParsePurlCSV`
- `backend/internal/catalog/purl.go:13-33` — `ParsePurl`
- `backend/internal/auth/handlers.go:41,107` — only existing upload-size cap in the codebase (unrelated login endpoints, 10KB)
- `backend/internal/catalog/pipeline_test.go:219-243` — finder-error leaves status at `pending`
- `backend/internal/catalog/queue_test.go:84-104` — `TestQueue_TransientFinderError_RetriesThenMarksFailed`
- `backend/internal/catalog/handlers_test.go:75-117,178-211` — upload summary and batch-progress tests

## Architecture Insights

- Failure isolation is per-library, not per-batch: a GitHub/LLM error on one library never aborts or taints the rest of the batch, which is good for resilience but means there is structurally no place that currently *could* report "this batch had an incomplete result" — that would be new code, not a gap in existing logic.
- The in-memory queue is an explicit, documented tradeoff (queue.go:27-28) rather than an oversight — any test or fix for Risk #1's "stalls/drops" framing should treat process-restart data loss as a known, accepted limitation unless the plan decides otherwise, not a bug to silently patch.
- Input validation follows a consistent two-tier pattern: parse-level errors (malformed JSON/CSV) are fatal (400), row-level problems (bad purl) are collected and reported as skips (200) — any new validation (e.g. size limits) should fit this existing tier rather than introduce a third error shape.

## Historical Context (from prior changes)

- `context/foundation/lessons.md` — "Go nil slices marshal to JSON `null`, not `[]`" is about list-endpoint responses, not directly applicable to this pipeline's batch-status reporting, but the same discipline (make guarantees explicit, don't rely on absence-of-error-as-success) is the same failure shape as Risk #1's "reports success on incomplete result." Not a direct precedent, flagging the parallel only.
- No prior `context/changes/**/` or `context/archive/**/` artifact was found describing this pipeline's queue, LLM/GitHub integration, or SBOM/CSV parsers — this appears to be the first dedicated research pass on this subsystem.

## Related Research

- `context/foundation/test-plan.md` §2 (Risk Map, rows #1 and #7) and §3 (Phase 1) — the source document this research grounds.

## Open Questions

- Whether `cyclonedx-go` and `packageurl-go` (third-party, not vendored/readable in this repo) can panic on adversarial input was not verified — flagged as unaudited, not ruled out. A plan targeting Risk #7's "crashes the backend" framing should decide whether to fuzz-test against the real libraries or treat this as accepted third-party risk.
- No batch-level reconciliation code exists today (Risk #1). The plan will need to decide whether to add one (e.g. a derived "N created vs N terminal" check surfaced via `BatchProgressHandler`) or scope this phase to tests-only against current behavior.
- Whether a hard file-size limit should be added to `catalog/handlers.go`/`project_handlers.go` (mirroring the `auth/handlers.go` pattern) is a product/scope decision for `/10x-plan`, not resolved by this research.
