# Testing pipeline input hardening — Plan Brief

> Full plan: `context/changes/testing-pipeline-input-hardening/plan.md`
> Research: `context/changes/testing-pipeline-input-hardening/research.md`

## What & Why

Phase 1 of the project's test rollout (`context/foundation/test-plan.md` §3): prove the SBOM/LLM categorization pipeline doesn't silently drop or stall libraries (Risk #1), and that malformed/oversized/adversarial uploads can't crash the backend or reach the LLM unsanitized (Risk #7). Scope is tests plus targeted fixes for the concrete gaps research confirmed — not a pipeline redesign.

## Starting Point

Today, `backend/internal/catalog` has no file-size cap on either upload endpoint, purl-derived strings flow unsanitized into LLM prompts, a failed library's actual error is discarded (only the bare status `"failed"` persists), and nothing anywhere reconciles the number of libraries created against the number that reach a terminal status. The frontend already derives its own batch-settled check from the existing `BatchProgressHandler` response shape, in two duplicated places.

## Desired End State

Both upload endpoints reject oversized files with a clean 400. A crafted adversarial purl can no longer inject raw structure into an LLM prompt. A library that exhausts its retries has its real failure reason persisted, not discarded. A backend test proves, against the real async queue and a real Postgres instance, that a settled batch's status counts always sum to its created-library count with none left non-terminal — without touching the API contract the frontend depends on.

## Key Decisions Made

| Decision | Choice | Why (1 sentence) | Source |
| --- | --- | --- | --- |
| Overall scope | Tests + targeted fixes, not tests-only or full remediation | Matches the phase name's "hardening" half and the test-plan's cost×signal principle — small, bounded fixes with real signal | Plan (interview) |
| Upload size limit | Hard cap via `http.MaxBytesReader` (10MB), mirroring `auth/handlers.go`'s existing pattern | Cheap, idiomatic, closes an unbounded-read surface | Plan (interview) |
| Batch reconciliation | Backend-only test invariant; no `BatchProgressHandler` API shape change | The flat `Record<string,number>` response is already consumed by 2 frontend routes with no frontend test harness yet (that's test-plan Phase 4) — changing it now would cross a phase boundary | Plan (interview, revised after research found the frontend consumers) |
| Prompt injection | Lightweight `sanitizeForPrompt` (strip control chars, cap length) + adversarial unit test | Narrow, bounded fix matching the purl's already-constrained grammar; doesn't attempt full elimination | Plan (interview) |
| Failure-reason visibility | Persist `lastErr` via a new nullable `libraries.error_detail` column | Small, isolated fix closing a concrete observability gap research flagged (`_ = lastErr`, queue.go:96) | Plan (interview) |
| Async test strategy | Reuse the existing real-queue + real-Postgres pattern (`startTestPostgres`, `pollUntilStatus`) | Matches the codebase's established convention in `queue_test.go`; no new test infra | Plan (interview) |

## Scope

**In scope:**
- A hard upload-size cap on `UploadHandler` and `ProjectUploadHandler`.
- A bounded sanitizer for purl-derived strings before they reach LLM prompts.
- Persisting the queue's discarded failure reason on a new `error_detail` column.
- A real-queue/real-DB test proving batch-level count reconciliation and failure-reason persistence.

**Out of scope:**
- Any change to `BatchProgressHandler`'s JSON shape or any frontend file.
- A durable/persistent queue surviving process restarts.
- Full prompt-injection elimination (structured-output enforcement, a guard model).
- Frontend tests or a frontend test runner (test-plan Phase 4).
- Third-party parser library (`cyclonedx-go`, `packageurl-go`) panic-safety auditing.
- Risks #2–#6 from the test-plan risk map (later rollout phases).

## Architecture / Approach

Three independent phases in `backend/internal/catalog`, each following existing package conventions (stdlib `testing`, fakes for `RepoFinder`/`TopicsFetcher`/`Categorizer`, the existing `startTestPostgres`/`pollUntilStatus` real-DB pattern). Phases 1–2 are pure additions with no schema change; Phase 3 is the only one touching the DB schema and the `UpdateLibraryStatus` signature.

## Phases at a Glance

| Phase | What it delivers | Key risk |
| --- | --- | --- |
| 1. Upload size hardening | `http.MaxBytesReader` cap on both upload handlers + tests | A wrong threshold could reject a legitimate large SBOM — set generously (10MB) |
| 2. LLM prompt-injection mitigation | `sanitizeForPrompt` helper applied at both LLM call sites + adversarial unit test | Sanitization reduces, doesn't eliminate, the injection surface |
| 3. Pipeline failure visibility & reconciliation | `error_detail` migration + queue fix + real-queue/DB reconciliation test | Touches `UpdateLibraryStatus`'s signature — all call sites must be updated together |

**Prerequisites:** Docker available locally for the real-Postgres test helper (`startTestPostgres`) used by Phase 3's new test and already required by existing `queue_test.go`/`store_test.go` tests.
**Estimated effort:** ~3 sessions, one per phase — each is a small, independently committable diff.

## Open Risks & Assumptions

- The 10MB upload cap is a judgment call, not a measured requirement — revisit if a real SBOM turns out to exceed it.
- `sanitizeForPrompt`'s control-character stripping reduces but doesn't eliminate prompt-injection risk; plain-text instruction-like content within otherwise-valid purl characters is not caught by this phase.
- Third-party parser panic-safety (`cyclonedx-go`, `packageurl-go`) remains unverified — flagged in research, not addressed here.

## Success Criteria (Summary)

- An oversized SBOM/CSV upload is rejected with a clear error instead of being read unbounded.
- An adversarial purl can no longer inject raw newline/structure into an LLM prompt.
- A failed library's actual error reason is visible in the database, not discarded.
- A real end-to-end test proves a settled batch's counts always reconcile against what was uploaded.
