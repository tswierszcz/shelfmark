---
date: 2026-10-26T20:58:49+01:00
researcher: Claude (Opus 5.5)
git_commit: a7d240d
branch: shape/shelfmark
repository: shelfmark
topic: "Ground test-plan Phase 2: role gating (#3), list endpoints never null (#2), strategic-marks cap (#6)"
tags: [research, testing, auth, require-role, catalog, strategic-marks, json-null]
status: complete
last_updated: 2026-10-26
last_updated_by: Claude (Opus 5.5)
---

# Research: Ground test-plan Phase 2 — access control & data-integrity regressions

**Date**: 2026-10-26T20:58:49+01:00
**Researcher**: Claude (Opus 5.5)
**Git Commit**: a7d240d
**Branch**: shape/shelfmark
**Repository**: shelfmark

## Research Question

Ground rollout Phase 2 of `context/foundation/test-plan.md` (risks #3, #2, #6): find the real failure paths in code, verify or correct the risk response guidance, locate existing tests, identify the cheapest useful test layer, and flag speculative risks or misleading evidence.

## Summary

- **#3 role gating — risk confirmed, guidance needs one correction.** `auth.RequireRole` is the only authorization gate for the 14 protected routes (`backend/cmd/shelfmark/main.go:63-76`); handlers use the session only for attribution, never to narrow access. The middleware itself is unit-tested with a stub handler, but **no test exercises the real route→role wiring in `main.go`**: the mux is built inline in `main()` (`main.go:59-77`), after DB connect/migrate/seed and `NewOpenRouterClient()`, so it is unreachable from tests. The only route-specific gating test (`TestMarkStrategicHandler_RoleGating`, `catalog/handlers_test.go:647`) re-wraps `RequireRole(RoleArchitect)` itself and therefore cannot catch a wiring regression in `main.go`. Cheapest real protection: extract the route registration into a function in package `main` and add a table-driven test over routes × {no session, admin, architect, viewer} that asserts denials (401/403). Denial cases never reach the handler, so nil dependencies suffice.
- **#2 list endpoints never `null` — risk confirmed with a live bug.** Of 9 endpoints whose body is or contains a JSON array/map, the two upload endpoints currently return `"skipped": null` when nothing was skipped: `ParsePurlCSV` (`catalog/csv.go:22-25`), `ParseCycloneDXSBOM` (`catalog/sbom.go:33-40`) and `ParseProjectSBOM` (`catalog/project_sbom.go:36-39`) declare `var skipped []SkippedRow` and the handlers pass it through. An existing test (`project_handlers_test.go:161`, `len(resp.Skipped) != 0`) exercises exactly this case and passes, because Go decodes `null` and `[]` identically. Only 4 of the 9 endpoints have a raw-body `[]` assertion.
- **#6 strategic cap — sequential cap confirmed tested; concurrency gap found.** `MarkStrategic` (`catalog/store.go:367-414`) counts active marks per (category, ecosystem) excluding the library itself and rejects at `count >= 2`, inside a transaction opened with `pool.Begin` at the default READ COMMITTED isolation. No `FOR UPDATE`, advisory lock or serializable isolation exists anywhere under `backend/internal` (grep, 0 hits), and the schema has no DB-level cap (`migrations/0003_strategic_marks.up.sql`). Two concurrent marks of different libraries in the same pair can both read `count = 1` and both insert, giving 3 active marks. The "2 OK, 3rd rejected" sequential boundary is already tested (`store_test.go:319`); the HTTP 409 mapping and the concurrent case are not.
- **PRD vs code divergence (product decision, not a test question):** PRD §Access Control grants Admin "uploads project SBOMs" (`prd.md:97`), but `POST /api/projects/upload` allows only Architect (`main.go:74`). Admin also cannot list projects/reports or organization libraries (`main.go:71,75-76`).

## Detailed Findings

### Risk #3 — role gating

**Route table** (`backend/cmd/shelfmark/main.go:60-77`):

| Line | Route | Allowed roles |
|---|---|---|
| 60 | GET /healthz | public |
| 61 | POST /api/login | public |
| 62 | POST /api/logout | public |
| 63 | GET /api/me | admin, architect, viewer |
| 64 | POST /api/users | admin |
| 65 | GET /api/users | admin |
| 66 | POST /api/catalog/upload | admin |
| 67 | GET /api/catalog/batches/{batchID} | admin |
| 68 | GET /api/catalog/libraries | admin |
| 69 | GET /api/catalog/categories | admin, architect, viewer |
| 70 | POST /api/catalog/libraries/{libraryID}/categories | admin |
| 71 | GET /api/organization/libraries | architect, viewer |
| 72 | POST /api/organization/libraries/{libraryID}/categories/{categoryID}/strategic | architect |
| 73 | DELETE (same path) | architect |
| 74 | POST /api/projects/upload | architect |
| 75 | GET /api/projects | architect, viewer |
| 76 | GET /api/projects/{projectID}/report | architect, viewer |
| 77 | GET / (SPA catch-all) | public; an unknown `GET /api/...` falls through to `index.html` with 200 |

**Middleware behaviour** (`backend/internal/auth/middleware.go:29-64`): any `VerifySession` error → 401 with empty body (`:45`); this covers missing cookie, malformed/expired cookie, bad signature (logged warn) and missing `SESSION_SECRET` (logged error). Valid session with a role outside `allowed` → 403 with empty body (`:55-57`). A wrong HTTP method on a registered path is answered 405 by the Go mux before `RequireRole` runs.

**No second gate:** handlers reading `SessionFromContext` use it only for nil-guard or attribution — `auth/handlers.go:90` (Me), `catalog/handlers.go:363,403` (marked-by), `catalog/project_handlers.go:32` (created_by). `ProjectReportHandler` checks existence only, not ownership (`project_handlers.go:161-176`); `ListProjects` returns every project (`project.go:164`). This matches the PRD's single-organization, role-only model (`prd.md:91-101`) — not an IDOR finding.

**Testability gap:** no function returns the router; registration is inline in `main()` (`main.go:59-77`) behind `db.Connect`, `db.Migrate`, `SeedFromEnvIfEmpty` and `NewOpenRouterClient()` (fails without `OPENROUTER_API_KEY`, `main.go:44-47`). Handlers take `*catalog.Store`, `*auth.UsersStore` and `catalog.Enqueuer` (interface); constructors only capture them, so denial-path requests (rejected by `RequireRole` before the handler runs) work with nil dependencies. `db.HealthCheck(nil)` would panic if invoked, but `/healthz` is public and outside the denial matrix.

**Existing tests (401/403):**
- `auth/middleware_test.go`: `TestRequireRole_NoCookieReturns401` (:69), `_InvalidCookieReturns401` (:86), `_ExpiredCookieReturns401` (:104), `_ValidSessionWrongRoleReturns403` (:130, viewer vs `RequireRole(admin)`), `TestRequireRole_LogsSessionFailures` (:228) — all against a stub handler, not real routes.
- `catalog/handlers_test.go`: `TestMarkStrategicHandler_RoleGating` (:647) — viewer → 403, no session → 401, but with its own `RequireRole` wrapper (:655).
- Gap: every cell of the 14 protected routes × {no session, admin, architect, viewer} is untested against the real wiring.

**Session helpers for tests:** `issueTestSession` (`auth/middleware_test.go:23`), `issueAndExtractCookie` (`auth/session_test.go:19`), `sessionCookie` (`catalog/handlers_test.go:446`) — all unexported test helpers; each requires `SESSION_SECRET` set first. A test in package `main` would need its own small helper over the exported `auth.IssueSession` (`auth/session.go:78`).

**Guidance verdict:** "Testing only one role" anti-pattern is correct. Correction: the protection must run against the **real route registration** (`main.go`), otherwise a test that wraps handlers itself proves only the middleware, which is already proven. Cheapest layer: integration at the HTTP-router level in package `main`, denial cases only, no database.

### Risk #2 — list endpoints never `null`

| Endpoint | List fields | Non-nil guarantee | Raw-body test |
|---|---|---|---|
| GET /api/users | top level | store `[]UserSummary{}` (`auth/users_store.go:81`) | Y — `auth/handlers_test.go:225` |
| GET /api/catalog/categories | top level | store `[]Category{}` (`catalog/store.go:137`) | Y — `catalog/handlers_test.go:275` |
| GET /api/projects | top level | store `[]Project{}` (`catalog/project.go:171`) | Y — `project_handlers_test.go:592` |
| GET /api/projects/{id}/report | top level, `categories` | handler `make` (`project_handlers.go:212`) + `[]projectReportCategory{}` (`:198`); store `[]` (`project.go:215`) | top level only — `project_handlers_test.go:612` |
| GET /api/catalog/libraries | top level, `categories`, `strategic` | handler `make` (`handlers.go:210`) + nil-check (`:213-215`) + literal (`:216`); store returns nil (`store.go:205`, `var libraries []Library`) | N (`handlers_test.go:361` decodes + `len`) |
| GET /api/organization/libraries | top level, `categories`, `strategic` | handler `make` (`handlers.go:276`) + nil-checks (`:279-285`) | N (`handlers_test.go:526` decodes + `len`) |
| GET /api/catalog/batches/{id} | map | store `BatchProgress{}` (`store.go:346`) | N (`handlers_test.go:186` decodes + `len`) |
| POST /api/catalog/upload | `skipped` | **none — `null` today** (`csv.go:22-25`, `sbom.go:33-40`) | N |
| POST /api/projects/upload | `skipped` | **none — `null` today** (`project_sbom.go:36-39`) | N — `project_handlers_test.go:161` passes on `null` |

- Store-level guarantees are inherited by any new handler that reuses those store functions; handler-level guarantees (`ListLibraries`, `CategoryNamesByLibrary`, `StrategicMarksByLibrary` consumers) must be repeated by any new handler built on them — this is where the "can regress" warning in `lessons.md` applies.
- The `skipped` bug is latent in the UI: the frontend types `skipped: SkippedRow[]` (`web/app/lib/catalog.ts:31`, `projects.ts:33`) but does not read it today.
- `SkippedRow` has no json tags (`sbom.go:13-16`), so items serialize as `Index`/`Reason` — a naming inconsistency, not a null bug.
- Legitimate `null`s that a generic check must allow: `github_repo` (`*string`, `store.go:54`) on library and report items. `error_detail` is `omitempty`, absent rather than `null`.
- `null` vs `[]` is invisible to tests that decode into Go structs; only raw-body or `map[string]any`/`[]any` inspection can tell them apart.

**Guidance verdict:** "testing only the 4 already-fixed functions" anti-pattern is confirmed — exactly those 4 are covered and the upload `skipped` regression slipped through. Cheapest layer: handler-level integration with an empty Postgres, asserting raw JSON shape (array fields never `null`, allowlist for nullable scalars); plus fixing the 3 parsers (return `[]SkippedRow{}`).

### Risk #6 — strategic-marks cap

- PRD FR-007 (`prd.md:73`): "Normally one strategic library per category per ecosystem, but up to two are allowed." Scope per (category, ecosystem) is explicit; active-only counting, re-mark and unmark semantics come from the archived plan (`context/archive/2026-10-20-org-data-fixes-and-user-management/plan.md:91`), not the PRD.
- Enforcement (`catalog/store.go:367-414`): `pool.Begin` (`:368`, default isolation); ecosystem lookup (`:374`, no row → `ErrInvalidReference`); count (`:383-387`) `COUNT(DISTINCT sm.library_id) … WHERE sm.active AND sm.category_id = $1 AND l.ecosystem = $2 AND sm.library_id != $3`; reject `count >= 2` (`:392-393`) with `ErrStrategicCapReached` (`:31`); upsert `ON CONFLICT (library_id, category_id) DO UPDATE SET active = true …` (`:397-400`).
- Schema (`db/migrations/0003_strategic_marks.up.sql`): `PRIMARY KEY (library_id, category_id)`, `active boolean`, partial index on active; no ecosystem column, no constraint or trigger for the cap.
- Concurrency: grep for `FOR UPDATE|advisory|BeginTx|IsoLevel|Serializable` under `backend/internal` returns 0 hits. With one active mark, two concurrent `MarkStrategic` calls for different libraries in the same pair both read 1, insert different primary keys (no conflict), and commit → 3 active marks. The archived plan's claim that the transaction prevents concurrent bypass (`…/plan.md:47`) does not hold at READ COMMITTED.
- Edge behaviour (sequential): re-marking an active library at the cap succeeds (excluded by `!= $3`); re-activating an inactive mark when 2 others are active is rejected; same library in another category, or another ecosystem in the same category, counts separately; unmark (`store.go:422-437`, `active = false`) frees a slot.
- Handler mapping: `ErrStrategicCapReached` → 409 "already 2 strategic libraries marked for this category and ecosystem" (`handlers.go:374-376`); `ErrInvalidReference` → 400 (`:370-371`); other → 500 (`:378`); unmark not found → 404 (`:410-411`).
- Existing tests: `TestMarkStrategic_EnforcesTwoPerCategoryEcosystemCap` (`store_test.go:319`) — two maven marks OK, third `ErrStrategicCapReached`, re-mark at cap OK; plus `_UpsertsWithoutErrorOnRepeat` (:269), unmark tests (:371, :399), strategic filters (:445, :507), handler round-trip (`handlers_test.go:559`). No test asserts HTTP 409; none covers different-ecosystem-at-cap, unmark-frees-slot, re-activation-at-cap, or concurrency.

**Guidance verdict:** the "2 OK, 3rd rejected" anti-pattern warning is already satisfied for sequential calls. Correction: the real remaining gap is **concurrent** marking (fails today) plus the 409 contract and the remaining edge cases. Cheapest layer: store-level integration with real Postgres; concurrency via N goroutines behind a barrier (probabilistic) or two manually interleaved transactions (deterministic but duplicates the SQL). A fix (advisory transaction lock per (category, ecosystem), `FOR UPDATE` on the category row, or SERIALIZABLE + retry) is a code change the plan must decide on.

## Code References

- `backend/cmd/shelfmark/main.go:59-77` — inline route registration (all role sets)
- `backend/internal/auth/middleware.go:29-64` — `RequireRole` (401 `:45`, 403 `:55-57`)
- `backend/internal/auth/session.go:78` — exported `IssueSession`
- `backend/internal/catalog/handlers_test.go:647` — only route-specific gating test (self-wrapped)
- `backend/internal/catalog/csv.go:22-25`, `sbom.go:33-40`, `project_sbom.go:36-39` — nil `skipped`
- `backend/internal/catalog/project_handlers_test.go:161` — test that passes on `"skipped": null`
- `backend/internal/catalog/store.go:205` — `ListLibraries` returns nil on no rows
- `backend/internal/catalog/store.go:367-414` — `MarkStrategic` check-then-insert at READ COMMITTED
- `backend/internal/db/migrations/0003_strategic_marks.up.sql` — no DB-level cap
- `context/foundation/prd.md:73` (FR-007), `:91-101` (Access Control)

## Architecture Insights

- Authorization is a single declarative layer (role sets at registration); its correctness lives in `main.go`, which currently has no tests. Making the route table testable is the key enabler for #3 and also gives #2 a single place to iterate list endpoints.
- Non-nil list guarantees are split between store, handler and parser layers; the layer that owns the guarantee determines whether a new endpoint inherits it.
- Invariants spanning rows (the cap) are enforced in application code only; at READ COMMITTED such check-then-insert invariants need an explicit lock or a DB constraint.

## Historical Context (from prior changes)

- `context/archive/2026-10-20-projects-empty-list-null-fix/` — fixed 4 list functions and added the 4 raw-body tests; `context/foundation/lessons.md` records the rule and warns the guarantee can regress. Supported: the regression happened in the upload `skipped` field.
- `context/archive/2026-10-20-org-data-fixes-and-user-management/plan.md:47,91` — introduced the per-(category, ecosystem) count excluding the marked library (supported, `store.go:383-387`); claimed the transaction prevents concurrent bypass (contradicted at READ COMMITTED, see Risk #6).

## Related Research

- `context/archive/2026-10-21-testing-pipeline-input-hardening/research.md` — Phase 1 of the same test rollout.

## Open Questions

1. **PRD vs code (product decision):** should Admin be allowed to upload project SBOMs (`prd.md:97` says yes; `main.go:74` says Architect only) and to read projects/reports/organization libraries? Tests will lock in whichever is chosen.
2. **Concurrency fix scope:** fix the cap race in this change (test + lock), or record the race as a known gap and only test sequential behaviour?
3. **Unknown `/api/*` paths return the SPA with 200** (`main.go:77`) — out of this phase's risks; worth a follow-up (404 for `/api/*`).
