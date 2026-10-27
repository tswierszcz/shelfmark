# Surface failed and uncategorized libraries — Plan Brief

> Full plan: `context/changes/batch-status-uncategorized-libraries/plan.md`

## What & Why

`/redundancy` says "Batch <id> — done" and "No libraries recorded for this project." when categorization produced no categories, although the libraries were stored (test-plan risk #1: reports success on an incomplete result). Users must see which libraries need review or failed.

## Starting Point

Batch progress is admin-only, so the architect's poll 403s and the UI falls through to "done". The report query inner-joins categories, hiding every uncategorized library. The pipeline can mark a library `categorized` with zero categories attached.

## Desired End State

The batch line says `— done` only when every library is categorized, otherwise `— done: N need review, M failed`; poll errors and timeouts say so. The report lists every project library; uncategorized ones carry a `Needs review` / `Failed` / `Processing` badge.

## Key Decisions Made

| Decision | Choice | Why (1 sentence) |
| --- | --- | --- |
| Batch-progress access | Add architect to existing route | One-line change; batch IDs are random and expose only counts |
| Batch line wording | `— done: N need review, M failed`; `— status unavailable`; `— still processing` | States what is incomplete; keeps `— done` for the E2E happy path |
| Report wording | Status badge per uncategorized library | Tells the user why there is no category |
| Zero attached categories | `needs_manual`, not `categorized` | A categorized library with no category is a status lie |

## Scope

**In scope:** route roles, `ProjectReport` LEFT JOIN, pipeline guard, Go integration tests, `/redundancy` batch line + badges, `Badge` neutral tone, kitchen-sink states.

**Out of scope:** new routes, re-categorize actions, LLM prompt changes, schema migrations, `/catalog` page.

## Architecture / Approach

Backend first (truthful API with integration tests), then the UI renders it. The existing E2E smoke test is the regression gate for the happy path.

## Phases at a Glance

| Phase | What it delivers | Key risk |
| --- | --- | --- |
| 1. Backend truthfulness | Architect batch progress, full report, needs_manual guard, tests | Nullable category columns leaking `null` lists |
| 2. /redundancy UI | Honest batch line, status badges, kitchen sink, E2E green | Changing the `— done` string breaks E2E |

**Prerequisites:** Docker (Go integration tests spin Postgres); `docker compose up -d db` + `.env` E2E vars for Playwright.
**Estimated effort:** ~1 session, 2 phases.

## Open Risks & Assumptions

- Architects can read status counts of admin catalog batches if they know the batch ID (accepted).

## Success Criteria (Summary)

- An upload the categorizer can't place no longer reads as a clean "done".
- Every uploaded library appears in the project report.
- Go integration tests and the E2E smoke test pass.
