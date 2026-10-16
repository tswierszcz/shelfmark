# Admin seeds and categorizes the library catalog Implementation Plan

## Overview

Give the Admin a way to seed Shelfmark's library catalog — by uploading a CycloneDX SBOM or a CSV of purls — and have each library automatically categorized by capability (GitHub topic tags, validated/extended by an LLM via OpenRouter), with a filterable catalog view and manual category correction for libraries the automated pipeline couldn't confidently classify.

## Current State Analysis

- No catalog data model exists. `backend/internal/db/migrations/` has one migration so far (F-02's `pgcrypto` extension) — this plan adds to the same embedded directory, following the same `000N_description.{up,down}.sql` naming.
- `backend/internal/auth/` establishes the role-gating pattern (`auth.RequireRole(auth.RoleAdmin)(handler)`) this slice reuses verbatim for every catalog endpoint — per PRD Access Control, catalog seeding/correction and catalog viewing are both Admin-only (FR-001–004 are all phrased as Admin capabilities).
- `backend/internal/db/pool.go` is the only existing DB access point (a shared `*pgxpool.Pool`); this plan's store layer takes that pool as a dependency, matching `db.HealthCheck(pool)`'s own shape.
- Frontend: `web/app/lib/require-auth.tsx`'s `RequireAuth` only checks "is logged in," not role — this plan extends it to optionally require a specific role, since the catalog page is Admin-only and F-01 didn't need that distinction yet (its one protected page, `/dashboard`, was role-agnostic).
- No external API clients (GitHub, deps.dev, OpenRouter) exist anywhere in the codebase — this is the first slice to make outbound HTTP calls to third parties.
- No background/async processing exists — this is the first slice to run work outside the request/response cycle.

### Key Discoveries:

- Per user decision, the background worker pool is **in-memory only, no persistence** — a process restart mid-seeding silently drops queued/in-flight libraries (they stay in their last-written DB status, e.g. `pending`, with no automatic resumption). This is an accepted tradeoff, not an oversight; re-uploading the same purls is the recovery path if that happens.
- The "find the library's GitHub repo" step and the "categorize from GitHub topics" step are two separate LLM-eligible points in the PRD's cascade: (1) deps.dev lookup for the source repo, LLM fallback only if deps.dev has no mapping; (2) once a repo is known, fetch its GitHub topics, then one LLM call validates/corrects/extends categories against the seeded taxonomy. A library whose repo can't be found by either method skips straight to `needs_manual` — there's nothing to pull topics from.

## Desired End State

An Admin uploads a CycloneDX SBOM or a CSV of purls at `/catalog`. The system creates a `libraries` row per purl, processes them through a bounded in-memory worker pool (5 concurrent) that looks up each library's GitHub repo (deps.dev, LLM fallback), pulls GitHub topics, and asks an LLM to confirm/correct/extend categories from the seeded taxonomy — retrying transient failures a couple of times before leaving a library as `needs_manual`. The Admin sees a filterable catalog list (by status, category, ecosystem) and can add a category to any library directly (the PRD-specified manual fallback path).

Verification: `go build ./...`, `go vet ./...`, `go test ./...` pass in `backend/`; `npm run typecheck` and `npm run build` pass in `web/`; manually uploading a small real SBOM/CSV visibly seeds and categorizes libraries end-to-end in the browser.

## What We're NOT Doing

- No SPDX support — CycloneDX only, per the SBOM format decision.
- No persistent job queue / job table — the worker pool is in-memory, per the explicit tradeoff decision above.
- No dedicated UI for the manual-fallback path beyond "add a category to a library" (FR-004's own UI) — per the PRD, a library with no automated category is simply visible as `needs_manual` in the list, and the Admin corrects it through the same add-category action FR-004 already provides. No separate "triage queue" screen.
- No Python ecosystem support — Java/Maven only, per PRD NFR (Python is nice-to-have, parked).
- No retry/resume of libraries orphaned by a mid-seeding restart — accepted per the in-memory worker pool decision.
- No taxonomy management UI (add/rename/delete taxonomy categories) — the taxonomy is seeded via migration; FR-004's "add own categories" means attaching an existing taxonomy category to a library, not creating new taxonomy entries. (If a genuinely new capability is needed, it's a follow-up migration, not an MVP UI.)
- No health metrics enrichment — PRD Secondary success criterion, already parked on the roadmap.

## Implementation Approach

Bottom-up: data model first (nothing else can exist without it), then ingestion (gets purls into rows), then the categorization pipeline (the novel, highest-risk part — external integrations + concurrency), then the UI that ties it together for the Admin. Each phase's automated tests avoid live third-party calls (per the testing-strategy decision) by testing against small interfaces with fake implementations.

## Critical Implementation Details

- **OpenRouter auth & shape**: OpenRouter exposes an OpenAI-compatible `POST https://openrouter.ai/api/v1/chat/completions` endpoint, authenticated with `Authorization: Bearer <OPENROUTER_API_KEY>`. The model string is read from an env var (`OPENROUTER_MODEL`, e.g. `"glm-5.3"` per the user's choice) rather than hardcoded, since exact OpenRouter model slugs change and the operator may need to adjust it without a code change.
- **deps.dev response shape is not yet verified against a live call in this plan.** Its `GET https://api.deps.dev/v3/systems/maven/packages/{group}:{artifact}/versions/{version}` response is expected to carry a `links` object that may include a `repo` URL when deps.dev has resolved the package's source repository; implementers should hit the live endpoint once with a known Maven coordinate (e.g. Jackson or Guava) during Phase 3 to confirm the exact field path before writing the parser, and treat "field not present" as a normal "not found" (fall through to the LLM fallback), not an error.
- **GitHub topics**: `GET https://api.github.com/repos/{owner}/{repo}` with header `Accept: application/vnd.github+json` returns a `topics` array directly in the base repo response (no separate endpoint call needed). An optional `GITHUB_TOKEN` env var, when set, is sent as `Authorization: Bearer <token>` to raise the rate limit from 60/hr (unauthenticated) to 5000/hr; its absence is not an error — unauthenticated calls still work, just at a much lower ceiling worth knowing about if seeding a large catalog is slow.

## Phase 1: Data model

### Overview

Add the `categories`, `libraries`, and `library_categories` tables, plus a migration that seeds the baseline taxonomy.

### Changes Required:

#### 1. Categories + taxonomy seed

**File**: `backend/internal/db/migrations/0002_catalog_schema.up.sql`, `backend/internal/db/migrations/0002_catalog_schema.down.sql`

**Intent**: Create the curated taxonomy table and seed it with a baseline set of Java/Maven capability categories, plus the libraries and join tables that reference it.

**Contract**: Up migration creates:
- `categories (id uuid primary key default gen_random_uuid(), name text unique not null)`, seeded with `INSERT INTO categories (name) VALUES (...)` for: `logging`, `http-client`, `serialization`, `orm-persistence`, `dependency-injection`, `testing`, `build-tooling`, `security-crypto`, `messaging`, `metrics-observability`, `web-framework`, `utility`, `json-processing`, `validation`.
- `libraries (id uuid primary key default gen_random_uuid(), purl text unique not null, name text not null, ecosystem text not null default 'maven', github_repo text, status text not null default 'pending' check (status in ('pending','processing','categorized','needs_manual','failed')), upload_batch_id uuid not null, created_at timestamptz not null default now(), updated_at timestamptz not null default now())`.
- `library_categories (library_id uuid not null references libraries(id) on delete cascade, category_id uuid not null references categories(id) on delete restrict, source text not null check (source in ('github','llm','manual')), primary key (library_id, category_id))`.
- An index on `libraries(upload_batch_id)` and `libraries(status)` to support the batch-progress and filtered-list queries Phase 2–4 need.

Down migration drops all three tables (and their index) and the seeded rows along with them.

#### 2. Store layer

**File**: `backend/internal/catalog/store.go`

**Intent**: Centralize the SQL this slice needs behind a small Go API, so later phases call Go functions rather than embedding SQL inline in HTTP handlers.

**Contract**: A `Store` struct wrapping `*pgxpool.Pool` (constructed via `NewStore(pool *pgxpool.Pool) *Store`, matching `db.HealthCheck`'s pattern of taking the pool directly). Methods: `CreateLibraries(ctx, batchID uuid.UUID, libs []NewLibrary) error` (bulk insert, `ON CONFLICT (purl) DO NOTHING` so re-uploading an existing purl is a safe no-op rather than an error), `ListTaxonomy(ctx) ([]Category, error)`, `ListLibraries(ctx, filter LibraryFilter) ([]Library, error)` (filter by status/category/ecosystem/batch, all optional), `UpdateLibraryStatus(ctx, id uuid.UUID, status string, githubRepo *string) error`, `AttachCategory(ctx, libraryID, categoryID uuid.UUID, source string) error` (upsert into `library_categories`), `BatchProgress(ctx, batchID uuid.UUID) (BatchProgress, error)` (counts grouped by status for one batch).

### Success Criteria:

#### Automated Verification:

- `go build ./...` succeeds in `backend/`
- `go vet ./...` succeeds in `backend/`
- Unit/integration tests pass: `go test ./backend/internal/catalog/...` — covering (Docker-gated, following F-02's `migrate_test.go` pattern of starting a throwaway Postgres container): migration applies cleanly and `categories` contains the seeded taxonomy; `CreateLibraries` is idempotent on a duplicate purl; `AttachCategory` upserts without erroring on a repeat call; `BatchProgress` returns correct counts for a mixed-status batch.

#### Manual Verification:

- After running the migration against a local Postgres (via `docker compose up -d`), `SELECT name FROM categories;` shows the seeded taxonomy.

---

## Phase 2: Ingestion

### Overview

Parse an uploaded CycloneDX SBOM or CSV purl list into `libraries` rows, grouped under one `upload_batch_id` per upload.

### Changes Required:

#### 1. Purl parsing

**File**: `backend/internal/catalog/purl.go`

**Intent**: Turn a purl string into the fields `libraries` needs (name, ecosystem), using the standard purl-parsing library rather than hand-rolled string splitting.

**Contract**: `ParsePurl(raw string) (NewLibrary, error)` using `github.com/package-url/packageurl-go`; rejects (returns an error, not a panic) any purl whose type isn't `maven` — per the Java/Maven-only NFR, a non-Maven purl in an uploaded list/SBOM is a per-row validation failure, not a fatal upload error (see the CSV/SBOM handlers below for how per-row failures are surfaced).

#### 2. CycloneDX SBOM parsing

**File**: `backend/internal/catalog/sbom.go`

**Intent**: Extract purls from an uploaded CycloneDX SBOM's components.

**Contract**: `ParseCycloneDXSBOM(r io.Reader) ([]NewLibrary, error)` using `github.com/CycloneDX/cyclonedx-go` to decode JSON (the format the official CycloneDX Maven plugin produces by default), iterating `bom.Components` and calling `ParsePurl` on each component's `PackageURL` field. A component with no purl or an unparseable one is skipped, not fatal to the whole upload — collect skipped-row reasons to return alongside the successfully parsed libraries so the upload handler can report "N parsed, M skipped" to the Admin.

#### 3. CSV purl list parsing

**File**: `backend/internal/catalog/csv.go`

**Intent**: Extract purls from an uploaded CSV file (one purl per line, optional `purl` header).

**Contract**: `ParsePurlCSV(r io.Reader) ([]NewLibrary, []SkippedRow, error)` using `encoding/csv`; if the first row's single column reads literally `purl` (case-insensitive), treat it as a header and skip it, otherwise treat every row as data. Same skip-not-fail behavior as the SBOM parser for unparseable rows.

#### 4. Upload endpoint

**File**: `backend/internal/catalog/handlers.go`

**Intent**: Accept a multipart file upload (either an SBOM or a CSV — distinguished by a required `kind` form field, not file-extension sniffing), parse it, and create the library rows.

**Contract**: `UploadHandler(store *Store, queue *Queue) http.HandlerFunc` — `POST /api/catalog/upload`, wrapped in `auth.RequireRole(auth.RoleAdmin)`. Reads multipart field `file` plus `kind` (`"sbom"` or `"csv"`), dispatches to the matching parser, generates a new `upload_batch_id` (`uuid.New()`), calls `store.CreateLibraries`, then enqueues every newly created library onto the in-memory `Queue` (Phase 3) for background processing. Responds with `{"batch_id": "...", "created": N, "skipped": [...]}`.

#### 5. Batch progress endpoint

**File**: `backend/internal/catalog/handlers.go`

**Intent**: Let the frontend poll upload progress after a batch is submitted.

**Contract**: `BatchProgressHandler(store *Store) http.HandlerFunc` — `GET /api/catalog/batches/{batchID}`, wrapped in `auth.RequireRole(auth.RoleAdmin)`, returns `store.BatchProgress` as JSON (counts per status).

#### 6. Wire into main

**File**: `backend/cmd/shelfmark/main.go`

**Intent**: Construct the catalog store/queue and register the two new routes.

**Contract**: After the existing DB connect/migrate block: `catalogStore := catalog.NewStore(pool)`, construct the Phase 3 queue (see below), register `mux.HandleFunc("POST /api/catalog/upload", ...)` and `mux.HandleFunc("GET /api/catalog/batches/{batchID}", ...)`.

### Success Criteria:

#### Automated Verification:

- `go build ./...` succeeds in `backend/`
- `go vet ./...` succeeds in `backend/`
- Unit tests pass: `go test ./backend/internal/catalog/...` — covering: `ParsePurl` accepts a valid Maven purl and rejects a non-Maven one; `ParseCycloneDXSBOM` against a small fixture SBOM (2-3 components, one with a missing purl) returns the right parsed/skipped split; `ParsePurlCSV` against a fixture CSV (with and without a header row) does the same; `UploadHandler` with a fixture SBOM creates the expected library rows and returns the right batch summary (using a real Docker-started Postgres per the established pattern, not a mock store, so the full insert path is exercised).

#### Manual Verification:

- Uploading a real small CycloneDX SBOM (e.g. generated by `mvn org.cyclonedx:cyclonedx-maven-plugin:makeBom` on a small sample Maven project) via `curl` creates the expected number of `libraries` rows, visible via `SELECT * FROM libraries;`.
- Uploading a CSV with a mix of valid and invalid purls returns a response showing both counts.

---

## Phase 3: Categorization pipeline

### Overview

Build the external API clients (deps.dev, GitHub, OpenRouter), the two-step cascade (find repo → fetch topics → LLM-validate categories), and the bounded in-memory worker pool that drives it.

### Changes Required:

#### 1. deps.dev client

**File**: `backend/internal/catalog/depsdev.go`

**Intent**: Resolve a Maven coordinate to its GitHub repo, when deps.dev has that mapping.

**Contract**: `type RepoFinder interface { FindRepo(ctx context.Context, group, artifact, version string) (repo string, found bool, err error) }`. `DepsDevClient` implements it by calling the deps.dev API (see Critical Implementation Details for the endpoint-verification note) and extracting a `github.com/owner/repo`-shaped URL from the response if present; `found=false` (not an error) when deps.dev has no mapping or the linked repo isn't on GitHub.

#### 2. LLM-based repo fallback

**File**: `backend/internal/catalog/openrouter.go`

**Intent**: When deps.dev can't resolve a repo, ask the LLM for the library's GitHub link as a fallback (per the purl→GitHub decision), and, separately, validate/extend GitHub-topic-derived categories (per the LLM-choice decision).

**Contract**: `type OpenRouterClient struct { apiKey, model string; httpClient *http.Client }`, constructed from `OPENROUTER_API_KEY` and `OPENROUTER_MODEL` env vars (fail-fast if the API key is empty, matching the `LoadAccounts`/`Connect` convention — the model var defaults to `"glm-5.3"` if unset, since that's the chosen default rather than a hard requirement). Two methods: `FindRepo(ctx, purl string) (repo string, found bool, err error)` (a focused prompt asking only for a GitHub URL or "unknown"); `Categorize(ctx, purl string, githubTopics []string, taxonomy []string) (categories []string, confident bool, err error)` (a prompt listing the taxonomy and GitHub topics, asking the model to return zero or more taxonomy category names it's confident apply, or an empty/unconfident result — parsed from a structured JSON response the prompt requests).

#### 3. GitHub topics client

**File**: `backend/internal/catalog/github.go`

**Intent**: Fetch a repo's topics once its owner/repo is known.

**Contract**: `type TopicsFetcher interface { Topics(ctx context.Context, repo string) ([]string, error) }`. `GitHubClient` implements it per the Critical Implementation Details note (one GET, optional `GITHUB_TOKEN`).

#### 4. Categorization pipeline

**File**: `backend/internal/catalog/pipeline.go`

**Intent**: Orchestrate the per-library cascade: find repo (deps.dev, then LLM fallback) → if found, fetch GitHub topics → LLM-categorize against the taxonomy → persist; if repo not found or LLM isn't confident, mark `needs_manual`.

**Contract**: `func ProcessLibrary(ctx context.Context, store *Store, repoFinders []RepoFinder, topics TopicsFetcher, llm *OpenRouterClient, lib Library) error` — tries each `RepoFinder` in order (deps.dev, then the OpenRouter-backed fallback) until one returns `found=true`; on success, calls `topics.Topics`, then `llm.Categorize` with the taxonomy loaded from `store.ListTaxonomy`; persists matched categories via `store.AttachCategory(..., source: "github")` when GitHub topics directly matched a taxonomy name, or `source: "llm"` for LLM-added ones, and sets `status: "categorized"`. If no repo is found by any finder, or the LLM returns no confident categories, sets `status: "needs_manual"` without erroring the whole pipeline — this is an expected outcome per the PRD cascade, not a failure.

#### 5. Retry-aware worker pool

**File**: `backend/internal/catalog/queue.go`

**Intent**: Run `ProcessLibrary` across a bounded number of libraries concurrently (5, per the concurrency decision), retrying transient errors (network/rate-limit) a couple of times with a short backoff before giving up to `needs_manual` — in-memory only, no persistence across restarts, per the explicit tradeoff decision.

**Contract**: `type Queue struct { ... }`, `NewQueue(store *Store, finders []RepoFinder, topics TopicsFetcher, llm *OpenRouterClient, workers int) *Queue` starting `workers` goroutines reading from an internal buffered channel; `func (q *Queue) Enqueue(lib Library)` (called by Phase 2's upload handler per created library). Each worker calls `ProcessLibrary`, retrying up to 2 additional times (3 attempts total) with a short fixed backoff (e.g. 500ms) on a transient error (network error or HTTP 429/5xx from any client); a non-transient error (e.g. a malformed response it can't parse) or exhausted retries sets `status: "failed"` via `store.UpdateLibraryStatus` — distinct from `needs_manual`, which means "the pipeline ran successfully but found nothing confident," versus `failed`, which means "the pipeline itself errored." Both states surface identically to the Admin as "needs attention" in Phase 4's UI, but the distinction stays in the data for future debugging.

### Success Criteria:

#### Automated Verification:

- `go build ./...` succeeds in `backend/`
- `go vet ./...` succeeds in `backend/`
- Unit tests pass: `go test ./backend/internal/catalog/...` — covering: `ProcessLibrary` with fake `RepoFinder`/`TopicsFetcher`/`OpenRouterClient` implementations for: repo found + confident categories → `categorized` with the right `library_categories` rows; repo not found by any finder → `needs_manual`; repo found but LLM not confident → `needs_manual`; a fake finder returning a transient error → the queue retries up to the configured count before marking `failed`.

#### Manual Verification:

- With real `OPENROUTER_API_KEY` (and optionally `GITHUB_TOKEN`) set, re-uploading the SBOM from Phase 2's manual test and waiting a few seconds shows libraries transitioning from `pending` → `categorized` (or `needs_manual`) via `SELECT purl, status FROM libraries;`, with populated `library_categories` rows for the categorized ones.
- Temporarily setting an invalid `OPENROUTER_API_KEY` and re-running confirms affected libraries end up `failed` rather than hanging indefinitely.

---

## Phase 4: Admin UI

### Overview

A `/catalog` page (Admin-only) to upload SBOMs/CSVs, watch batch progress, browse/filter the catalog, and attach a category to a library manually.

### Changes Required:

#### 1. Role-aware route guard

**File**: `web/app/lib/require-auth.tsx`

**Intent**: Extend the existing "must be logged in" guard to optionally require a specific role, since `/catalog` is Admin-only and F-01's `/dashboard` didn't need this distinction.

**Contract**: `RequireAuth({ children, role }: { children: ReactNode; role?: "admin" | "architect" | "viewer" })` — when `role` is provided and `me.role !== role`, redirect to `/dashboard` instead of rendering (an authenticated non-admin visiting `/catalog` lands somewhere valid, not a dead end). Existing callers that don't pass `role` keep today's "any authenticated user" behavior — no change to `/dashboard`'s usage.

#### 2. Catalog client

**File**: `web/app/lib/catalog.ts`

**Intent**: Centralize the catalog API calls, mirroring `web/app/lib/auth.ts`'s structure.

**Contract**: `uploadCatalogFile(file: File, kind: "sbom" | "csv"): Promise<UploadResult>` (multipart `POST /api/catalog/upload`), `getBatchProgress(batchId: string): Promise<BatchProgress>` (`GET /api/catalog/batches/:id`), `listLibraries(filter: LibraryFilter): Promise<Library[]>` (`GET /api/catalog/libraries` with query params), `listTaxonomy(): Promise<Category[]>` (`GET /api/catalog/categories`), `attachCategory(libraryId: string, categoryId: string): Promise<void>` (`POST /api/catalog/libraries/:id/categories`).

#### 3. Remaining backend list/categories endpoints

**File**: `backend/internal/catalog/handlers.go`

**Intent**: The two read endpoints and the manual-attach endpoint the frontend needs that weren't already built in Phase 2.

**Contract**: `ListLibrariesHandler(store *Store) http.HandlerFunc` — `GET /api/catalog/libraries` (query params `status`, `category`, `ecosystem`, all optional), wrapped in `RequireRole(RoleAdmin)`. `ListTaxonomyHandler(store *Store) http.HandlerFunc` — `GET /api/catalog/categories`, same role gate. `AttachCategoryHandler(store *Store) http.HandlerFunc` — `POST /api/catalog/libraries/{libraryID}/categories` with body `{"category_id": "..."}`, calling `store.AttachCategory(..., source: "manual")`; same role gate. Registered in `main.go` alongside Phase 2's routes.

#### 4. Catalog page

**File**: `web/app/routes/catalog.tsx`

**Intent**: The Admin-facing page tying everything together: upload form, batch progress (polled), filterable table, per-row "add category" action.

**Contract**: Wrapped in `<RequireAuth role="admin">`. A file input + kind selector (SBOM/CSV) submitting to `uploadCatalogFile`; on success, poll `getBatchProgress` every ~2s until all libraries in the batch are out of `pending`/`processing`. A table listing libraries (purl, status, categories, github repo) with status/category filter controls calling `listLibraries`; each row has an "add category" control (a `<select>` populated from `listTaxonomy`) calling `attachCategory` and refreshing that row. Registered as `route("catalog", "routes/catalog.tsx")` in `web/app/routes.ts`.

#### 5. Dashboard link

**File**: `web/app/routes/dashboard.tsx`

**Intent**: Give an Admin a way to reach `/catalog` without typing the URL.

**Contract**: Add a link to `/catalog`, shown only when `me.role === "admin"`.

### Success Criteria:

#### Automated Verification:

- `npm run typecheck` succeeds in `web/`
- `npm run build` succeeds in `web/`
- `go build ./...`, `go vet ./...`, `go test ./backend/internal/catalog/...` succeed in `backend/` (covering the three new handlers with fixture data over a Docker-started Postgres, same pattern as Phase 2).

#### Manual Verification:

- Logging in as Admin, visiting `/catalog`, uploading a real SBOM, and watching the progress indicator move from pending counts to categorized/needs-manual counts.
- The catalog table shows the uploaded libraries with their categories and status; filtering by `status=needs_manual` shows only the uncategorized ones.
- Adding a category to a `needs_manual` library via the UI updates its displayed categories without a page reload.
- Logging in as Architect or Viewer and visiting `/catalog` directly redirects to `/dashboard` rather than showing the page.

---

## Testing Strategy

### Unit Tests:

- Purl/SBOM/CSV parsing: valid and invalid/edge-case inputs (non-Maven purl, missing purl field, header-vs-no-header CSV).
- Pipeline cascade: every branch (repo found via deps.dev / via LLM fallback / not found at all; LLM confident / not confident; transient error retried then failed).
- Store methods against a real Docker-started Postgres (per F-02's established pattern) rather than mocks, since the SQL itself (upserts, filters, batch counts) is the thing worth verifying.

### Integration Tests:

- None beyond the Docker-gated store/handler tests above — no live third-party API calls in the automated suite, per the testing-strategy decision.

### Manual Testing Steps:

1. Generate or obtain a small real CycloneDX SBOM for a handful of well-known Maven libraries.
2. Set `OPENROUTER_API_KEY` (and optionally `OPENROUTER_MODEL`, `GITHUB_TOKEN`) alongside the existing F-01/F-02 env vars.
3. Upload the SBOM via the `/catalog` UI as Admin; watch it move through pending → categorized/needs_manual.
4. Spot-check a couple of categorized libraries' assigned categories against their actual GitHub topics for sanity.
5. Manually attach a category to a `needs_manual` library and confirm it now shows as categorized in the list.

## Performance Considerations

The worker pool is capped at 5 concurrent libraries specifically to stay well under GitHub's unauthenticated rate limit (60/hr) if `GITHUB_TOKEN` isn't set, and to keep OpenRouter spend bounded during a large catalog seed. No further tuning is in scope for this MVP slice.

## Migration Notes

Not applicable — this is the first catalog data; nothing to migrate from.

## References

- Roadmap item: `context/foundation/roadmap.md` § S-01
- PRD: `context/foundation/prd.md` § Catalog & categorization (FR-001–004), § Business Logic
- Tech stack hand-off: `context/foundation/tech-stack.md` (`has_ai: true`, `has_background_jobs: true`)
- Existing fail-fast config pattern: `backend/internal/auth/accounts.go`, `backend/internal/db/pool.go`
- Existing migration pattern: `backend/internal/db/migrate.go`, `backend/internal/db/migrations/0001_enable_pgcrypto.up.sql`
- Existing role-gating: `backend/internal/auth/middleware.go` (`RequireRole`)
- Existing frontend auth pattern: `web/app/lib/auth.ts`, `web/app/lib/require-auth.tsx`

## Progress

> Convention: `- [ ]` pending, `- [x]` done. Append ` — <commit sha>` when a step lands. Do not rename step titles. See `references/progress-format.md`.

### Phase 1: Data model

#### Automated

- [x] 1.1 go build ./... succeeds in backend/ — c57b2de
- [x] 1.2 go vet ./... succeeds in backend/ — c57b2de
- [x] 1.3 Unit/integration tests pass: go test ./backend/internal/catalog/... — c57b2de

#### Manual

- [x] 1.4 Migration seeds the taxonomy, visible via SELECT name FROM categories — c57b2de

### Phase 2: Ingestion

#### Automated

- [x] 2.1 go build ./... succeeds in backend/ — 85d38cb
- [x] 2.2 go vet ./... succeeds in backend/ — 85d38cb
- [x] 2.3 Unit tests pass: go test ./backend/internal/catalog/... — 85d38cb

#### Manual

- [x] 2.4 Uploading a real SBOM creates the expected library rows — 85d38cb
- [x] 2.5 Uploading a mixed-validity CSV reports correct parsed/skipped counts — 85d38cb

### Phase 3: Categorization pipeline

#### Automated

- [x] 3.1 go build ./... succeeds in backend/ — 692d47b
- [x] 3.2 go vet ./... succeeds in backend/ — 692d47b
- [x] 3.3 Unit tests pass: go test ./backend/internal/catalog/... — 692d47b

#### Manual

- [x] 3.4 Re-uploading the SBOM with real API keys set shows libraries transitioning to categorized/needs_manual with populated categories — 692d47b
- [x] 3.5 An invalid OpenRouter API key results in failed status rather than a hang — 692d47b

### Phase 4: Admin UI

#### Automated

- [x] 4.1 npm run typecheck succeeds in web/ — 02a5827
- [x] 4.2 npm run build succeeds in web/ — 02a5827
- [x] 4.3 go build ./..., go vet ./..., go test ./backend/internal/catalog/... succeed in backend/ — 02a5827

#### Manual

- [x] 4.4 Admin can upload via /catalog UI and see progress move to completion — 02a5827
- [x] 4.5 Catalog table shows libraries with categories/status; status filter works — 02a5827
- [x] 4.6 Adding a category via the UI updates the displayed categories without reload — 02a5827
- [x] 4.7 Architect/Viewer visiting /catalog directly redirects to /dashboard — 02a5827
