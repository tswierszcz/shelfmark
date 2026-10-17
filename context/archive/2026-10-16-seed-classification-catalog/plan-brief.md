# Admin seeds and categorizes the library catalog — Plan Brief

> Full plan: `context/changes/seed-classification-catalog/plan.md`

## What & Why

This is roadmap slice S-01 — the first slice with real product functionality. Without a seeded, categorized library catalog, nothing downstream (marking strategic libraries, the redundancy report) has any data to work with. It's also where the categorization mechanism — the most novel part of the product — gets proven out in isolation.

## Starting Point

F-01 (auth) and F-02 (Postgres + migrations) are done, but there's no catalog data model, no external API integrations (GitHub, deps.dev, OpenRouter), and no background processing anywhere in the codebase. This slice builds all three for the first time.

## Desired End State

An Admin uploads a CycloneDX SBOM or a CSV of purls at `/catalog`. The system seeds library rows, runs them through an automated categorization pipeline (GitHub topics validated/extended by an LLM), and shows the Admin a filterable catalog with each library's status and categories — correcting anything the pipeline couldn't confidently classify via a manual "add category" action.

## Key Decisions Made

| Decision | Choice | Why (1 sentence) |
| --- | --- | --- |
| SBOM format | CycloneDX | Official Maven plugin generates it directly; its dependency graph carries the direct/transitive distinction the PRD's open question asked about. |
| Purl → GitHub repo | deps.dev first, LLM fallback | deps.dev is a structured, reliable source; asking an LLM to guess a URL directly risks hallucinated repos. |
| LLM provider | OpenRouter, model `glm-5.3` | User's explicit choice, overriding the default Anthropic-direct recommendation. |
| Background job mechanism | In-memory worker pool, no persistence | User's explicit choice, overriding the default Postgres-backed queue — a restart mid-seeding drops in-flight work; re-upload is the recovery path. |
| Category taxonomy | Seeded baseline (14 Java/Maven categories) via migration | Unblocks implementation immediately; extendable later via FR-004. |
| CSV input | File upload only | Matches the SBOM upload UX; handles large lists without a giant textarea. |
| Retry policy | 2-3 attempts with backoff, then manual fallback | Matches the PRD's own cascade and avoids hammering GitHub/OpenRouter during real outages. |
| Worker concurrency | 5 concurrent | Stays well under GitHub's unauthenticated rate limit (60/hr) and bounds OpenRouter spend. |
| External API testing | Fake interfaces, no live calls in automated tests | Deterministic, no API keys needed in CI; matches F-02's Docker-gated-only (never live-third-party) test pattern. |

## Scope

**In scope:**
- `categories`/`libraries`/`library_categories` schema + seeded taxonomy.
- CycloneDX SBOM and CSV purl-list parsing and upload.
- deps.dev + GitHub + OpenRouter clients; the find-repo → fetch-topics → categorize cascade.
- In-memory retry-aware worker pool (5 concurrent).
- Admin-only `/catalog` page: upload, batch progress, filterable list, manual category add.

**Out of scope:**
- SPDX, Python ecosystem, persistent job queue, taxonomy-management UI, health metrics enrichment, resuming orphaned jobs after a restart.

## Architecture / Approach

`backend/internal/catalog/` holds everything: store (SQL behind a small Go API), parsers (purl/SBOM/CSV), three external clients (deps.dev, GitHub, OpenRouter) behind small interfaces, a pipeline function orchestrating the cascade per library, and an in-memory worker pool feeding that pipeline. The frontend gets one new Admin-only route (`/catalog`) and a role-aware extension to the existing route guard.

## Phases at a Glance

| Phase | What it delivers | Key risk |
| --- | --- | --- |
| 1. Data model | categories/libraries/library_categories schema + seed | None significant — follows F-02's established migration pattern |
| 2. Ingestion | SBOM/CSV upload → library rows | SBOM/CSV edge cases (missing purls, malformed rows) |
| 3. Categorization pipeline | deps.dev/GitHub/OpenRouter clients, cascade, worker pool | deps.dev response shape unverified until a live call is made; OpenRouter prompt reliability |
| 4. Admin UI | `/catalog` page, role-aware guard, remaining endpoints | None significant — follows F-01's frontend patterns |

**Prerequisites:** F-01, F-02 (both done).
**Estimated effort:** ~2-3 sessions across 4 phases — the largest slice so far, due to three new external integrations and the first background-processing code in the project.

## Open Risks & Assumptions

- deps.dev's exact response shape for the source-repo link is unverified against a live call — Phase 3's implementer confirms this with one real request before writing the parser (flagged in the plan's Critical Implementation Details).
- OpenRouter's `glm-5.3` model slug is used as given; if OpenRouter's actual naming differs, the `OPENROUTER_MODEL` env var absorbs that without a code change.
- A mid-seeding process restart silently drops in-flight/queued libraries (accepted tradeoff, not a defect) — re-uploading is the recovery path.

## Success Criteria (Summary)

- An Admin can seed the catalog from either an SBOM or a CSV and see libraries automatically categorized without manual work for the common case.
- Libraries the pipeline couldn't confidently classify are visibly flagged and correctable through the same UI, closing the PRD's manual-fallback loop.
- All of this is Admin-only, consistent with PRD Access Control.
