# Test Plan

> Phased test rollout for this project. Strategy is frozen at the top
> (§1–§5); cookbook patterns at the bottom (§6) fill in as phases ship.
> Read before writing any new test.
>
> Refresh: re-run `/10x-test-plan --refresh` when stale (see §8).
>
> Last updated: 2026-10-29

## 1. Strategy

Tests follow three non-negotiable principles for this project:

1. **Cost × signal.** The cheapest test that gives a real signal for the
   risk wins. Do not promote to e2e because e2e "feels safer." Do not put a
   vision model on top of a deterministic visual diff that already catches
   the regression.
2. **User concerns are first-class evidence.** Risks anchored in "the team
   is worried about X, and the failure would surface somewhere in area Y"
   carry the same weight as PRD lines or hot-spot data.
3. **Risks are scenarios, not code locations.** This plan documents *what
   could fail* and *why we believe it's likely* — drawn from documents,
   interview, and codebase *signal* (churn, structure, test base). It does
   NOT claim to know which line owns the failure. That knowledge is
   produced by `/10x-research` during each rollout phase. If the plan and
   research disagree about where the failure lives, research is the
   ground truth.

Hot-spot scope used for likelihood weighting: `backend/internal/` (catalog,
auth, db), `backend/cmd/`, `web/app/` (routes, lib, components).

## 2. Risk Map

The top failure scenarios this project must protect against, ordered by
risk = impact × likelihood. Risks are failure scenarios in user / business
terms, not test names. The Source column cites the *evidence that surfaced
this risk* — never a specific file as "where the failure lives."

| # | Risk (failure scenario) | Impact | Likelihood | Source (evidence — not anchor) |
|---|---|---|---|---|
| 1 | SBOM/LLM categorization pipeline silently drops libraries, stalls in the background, or reports success on an incomplete result | High | Medium-High | interview Q1; `lessons.md` (prior silent-null precedent); hot-spot dir `backend/internal/catalog/` (15 commits/30d); PRD §Business Logic (cascading fallback); tech-stack.md hint `has_background_jobs: true` |
| 2 | The nil-slice → JSON `null` bug class (already fixed once) recurs in a new or changed list endpoint | Medium-High | High | `lessons.md` (explicit "the guarantee can regress" warning); interview Q2; hot-spot dir `backend/internal/catalog/` (highest churn) |
| 3 | A request outside the allowed role reaches a protected endpoint (Viewer mutates, or a role reaches another role's action) | High | Medium | PRD §Access Control (three-role contract); hot-spot dir `backend/internal/auth/` (recent migration from `.env` accounts to a DB table); mandatory abuse-surface lens (product has auth) |
| 4 | The redundancy report's cross-project / per-ecosystem aggregation produces a wrong strategic/retire recommendation | High | Medium | PRD US-01, FR-005, FR-008; interview Q4 ("want everything covered"); archive `project-redundancy-report` (recently added per-project direct/transitive logic) |
| 5 | A frontend route (`web/app/routes/*`) mishandles an error/empty/role-mismatch response and crashes or misrenders, caught only by manual clicking | Medium-High | High | Direct scan: 0 frontend test files, no vitest/jest/playwright in `web/package.json`; interview Q4; Phase 4 research: a nested `null` crash is defense in depth (the backend guards and tests those fields), the live failures were loading/empty/error misrenders and silently failing actions |
| 6 | The "max 2 strategic libraries per category/ecosystem" cap (FR-007) stops being enforced | Medium | Medium | PRD FR-007; archive `org-data-fixes-and-user-management` (this exact bug occurred once and was fixed) |
| 7 | An uploaded SBOM or CSV with malformed/oversized/adversarial content crashes the backend or reaches the LLM prompt unsanitized | Medium-High | Medium | PRD §Guardrails (explicit input boundary); FR-001/FR-002 (accept arbitrary uploads); mandatory abuse-surface lens (product accepts arbitrary user input) |

High × High focus: none strictly High/High, but #1, #3, #4 are High-impact and
ranked first for that reason. No High-impact × Low-likelihood scenario was
found (e.g. no cloud-provider-outage class risk surfaced) — nothing pushed
to observability-only.

### Risk Response Guidance

| Risk | What would prove protection | Must challenge | Context `/10x-research` must ground | Likely cheapest layer | Anti-pattern to avoid |
|------|-----------------------------|----------------|--------------------------------------|-----------------------|-----------------------|
| #1 | After processing an SBOM/CSV with N purls, exactly N libraries land in the catalog with a status that explicitly reflects the outcome (not just HTTP 200) | "200 OK means every library was categorized" | Pipeline entry point, where batch/queue status is persisted, the shape of an LLM/GitHub timeout error | integration (pipeline + store) | Assertion copied from the pipeline's current logic instead of from the expected purl count |
| #2 | Every list-returning endpoint — and every array field inside a response (e.g. upload `skipped`, nested categories/strategic) — ships `[]`, never `null`, on an empty result | "The existing fix covers all future endpoints" (Phase 2 research: it did not — upload `skipped` regressed) | Full list of endpoints whose body is or contains an array/map, and which layer (store, handler, parser) owns each non-nil guarantee | integration at handler level, asserting the raw JSON body | Testing only the 4 already-fixed functions; decoding into Go structs and checking `len` (cannot tell `null` from `[]`) |
| #3 | A request from a disallowed role gets 403 (401 without a valid session) on every protected endpoint, exercised through the real route registration | "Middleware tested once means tested everywhere" | Full route list with their `RequireRole(...)` sets, behavior on missing/expired session, and a way to build the real router in a test | integration at router level (routes × roles, denials only) | Testing only one role; wrapping handlers with `RequireRole` inside the test (proves the middleware, not the wiring) |
| #4 | For a project with N direct dependencies in the same category, the report flags exactly the non-strategic ones as "candidate to retire," regardless of upload order across projects | "One project is enough aggregation coverage" | How `is_direct`/strategic marks are joined cross-project, behavior with zero strategic marks in a category | integration (multi-project fixture) | Testing only one project/one category, missing cross-project aggregation bugs |
| #5 | A route renders the correct state (data/empty/error/loading) for a given role without crashing, regardless of the API response shape | "Manual clicking from the plan's Verification step is a permanent substitute" | How `require-auth.tsx` and the fetch call sites handle error/null/slow responses | unit/component (requires new setup — see §4) | Testing only happy-path rendering, skipping fetch errors and role mismatches |
| #6 | A third active strategic mark in the same (category, ecosystem) is rejected with an explicit error (HTTP 409), also when marks arrive concurrently | "The unique constraint on (library_id, category_id) is enough of a cap"; "a transaction makes check-then-insert safe" (not at READ COMMITTED) | Exact counting logic for active marks per (category, ecosystem), transaction isolation/locking around it | integration (store, real Postgres; concurrent goroutines) | Testing only adding 1 mark, never the "2 OK, 3 rejected" boundary; testing only sequential calls |
| #7 | A malformed/oversized/adversarial SBOM or CSV returns a readable validation error, not a 500 and not a partial DB write | "The parser already validates enough because the PRD doesn't mention limits" | Current parser behavior on empty/malformed/huge input | unit (parser) | Testing only the golden-path reference file, no fuzz/malformed input |

## 3. Phased Rollout

Each row is a discrete rollout phase that will open its own change folder
via `/10x-new`. Status moves left-to-right through the values below; the
orchestrator updates Status as artifacts appear on disk.

| # | Phase name | Goal (one line) | Risks covered | Test types | Status | Change folder |
|---|---|---|---|---|---|---|
| 1 | Critical-path pipeline & input hardening | Prove the SBOM/LLM pipeline doesn't silently drop data and malformed input can't crash it | #1, #7 | unit + integration | complete | `context/archive/2026-10-21-testing-pipeline-input-hardening/` |
| 2 | Access control & data-integrity regressions | Lock role gating and the strategic-marks cap against regression | #2, #3, #6 | unit + integration | complete | `context/archive/2026-10-26-testing-access-control-data-integrity/` |
| 3 | Redundancy report correctness | Prove cross-project aggregation yields the right retire/strategic call | #4 | integration | complete | `context/changes/refactor-opportunities/` |
| 4 | Frontend functional test harness | Stand up a frontend test runner and cover error/empty/role-mismatch rendering on the highest-churn routes | #5 | unit/component (new setup) | planned | `context/changes/testing-frontend-harness/` |

**Status vocabulary** (fixed — parser literals): `not started` → `change opened` → `researched` → `planned` → `implementing` → `complete`.

## 4. Stack

| Layer | Tool | Version | Notes |
|---|---|---|---|
| unit + integration (backend) | Go stdlib `testing` | go.mod-pinned | Already in use — 14 test files across `catalog`/`auth`/`db` |
| unit/component (frontend) | Vitest + @testing-library/react + jsdom | vitest 5.0.5, @testing-library/react 16.3.3, jsdom 30.1.3 (checked: 2026-10-29) | `cd web && npm test`; config `web/vitest.config.ts` (no `reactRouter()` plugin); helpers in `web/app/test/` — see §6.4 |
| API mocking | Go: none; frontend: in-repo `stubFetch` | — | Go tests hit a real Postgres per test; frontend tests stub global `fetch` with raw bodies (`web/app/test/fetch-stub.ts`) |
| e2e | none yet | — | Not scoped by this rollout; revisit via `/10x-e2e-setup` if a browser-level risk emerges later |
| accessibility | none yet | — | Not scoped by this rollout |
| (optional) AI-native | none | n/a | No AI-native test tooling in use or recommended at this time |

If a row reads "none yet", that gap is addressed by the named rollout phase (§3 Phase 4 for the frontend runner) or intentionally out of scope for this rollout.

**Stack grounding tools (current session):**
- Docs: none — no Context7 or framework-docs MCP exposed in this session; checked: 2026-10-21
- Search: generic WebSearch tool available but not used (no version-specific question required grounding); checked: 2026-10-21
- Runtime/browser: none — no Playwright/browser MCP exposed in this session; checked: 2026-10-21
- Provider/platform: none — no GitHub/Cloudflare/Supabase MCP exposed in this session; checked: 2026-10-21

## 5. Quality Gates

| Gate | Where | Required? | Catches |
|---|---|---|---|
| typecheck (`tsc` via `npm run typecheck`) + `go build` | local | required | syntactic / type drift |
| Go unit + integration (`go test ./...`) | local | required after §3 Phase 1 | backend logic regressions |
| frontend unit/component (`npm test`) | local (end-of-turn hook when web changed) + CI web job | required — wired 2026-10-29 | route state and frontend logic regressions |
| post-edit hook | local (agent loop) | wired 2026-10-26 — `.claude/hooks/` (per edit: gofmt + go vet / tsc; end of turn: vet, build, tests of changed packages except `internal/catalog`, web tsc); proven by `backend/hooktest` | regressions at edit time |
| e2e on critical flows | CI on PR | not scoped by this rollout | broken critical user paths |

## 6. Cookbook Patterns

How to add new tests in this project. Each sub-section is filled in once
the relevant rollout phase ships; before that, the sub-section reads
"TBD — see §3 Phase N."

### 6.1 Adding a Go unit test

- Pure functions: table-driven with an explicit `want` per case, one `t.Run` per row — never branch assertions on `tt.name` (see `backend/internal/catalog/openrouter_test.go`, `purl_test.go`).
- Prove the test guards something: break the production line, watch the test go red, restore.

### 6.2 Adding a Go integration test (store/handlers)

- Real Postgres per test via `startTestPostgres(t)` (`backend/internal/catalog/store_test.go`): container runs on `--tmpfs` and is removed with `docker rm -f -v`, so no Docker volumes leak. Copy that cleanup if you add a new helper.
- Async queue: drive a real `*Queue` with fake `RepoFinder`/`TopicsFetcher`/`Categorizer` and poll with `pollUntilStatus`; assert terminal state from the store, not from the fakes (see `TestQueue_BatchSettles_CountsReconcileAndFailureReasonPersists`).
- Handlers: `httptest.NewRecorder` + the handler func directly; assert status, body shape, and that rejected requests wrote no rows.
- Role gating: routes are data (`routes()` / `newMux` in `backend/cmd/shelfmark/routes.go`). `routes_test.go` compares every route's role set with an independent `expectedRoles` table (a regression lock, reviewed by a human against the PRD) and drives the real mux for every disallowed cell: no session → 401, wrong role → 403. A new route without a table entry fails the test.
- Never-`null` lists: assert the raw JSON body with `assertNoNullArrays` (`backend/internal/catalog/json_contract_test.go`), allowlisting nullable scalars such as `github_repo`; never decode into Go structs and check `len` (`null` and `[]` decode the same).
- Row-spanning invariants under concurrency: prove the race deterministically with a nil-in-production test seam between check and write (`afterCapCheck`, `TestMarkStrategic_InterleavedMarksRespectCap`), keep a barrier-released goroutine load test (`TestMarkStrategic_ConcurrentMarksNeverExceedCap`), and fix with a transaction-scoped advisory lock keyed on the invariant's scope.

### 6.3 Adding a multi-fixture integration test (cross-project)

- Fixture: two projects uploaded through `doProjectUpload` with CycloneDX SBOMs whose dependency graphs make at least one library direct in one project and transitive in the other; resolve shared libraries with `LibrariesByPurls`, not by batch (see `setUpMultiProjectReportFixture` in `backend/internal/catalog/project_report_test.go`).
- State: categories through `AttachCategory(..., "manual")`, strategic marks through `MarkStrategic` / `UnmarkStrategic` (never raw SQL), so the cap and the report read the same rows production writes.
- Run the whole scenario in both upload orders (A→B, B→A) on one container, with `resetCatalogTables` between them, and compare the two reports keyed by purl (`is_direct`, per-category `strategic`/`retire_candidate`) — never as raw JSON, since `id` and `upload_batch_id` differ between runs.
- Assert the exact set of `purl | category` retire candidates per project, not a count, so a wrong library being flagged fails the test; cover a category reviewed only through a library outside the project and a category with no marks.
- Pin open product questions as `..._Characterization` tests with a comment pointing at the decision (e.g. K1 / FR-007 ecosystem scope), so changing the behavior is an explicit test edit.

### 6.4 Adding a frontend component/unit test

- Put the test next to the source as `*.test.ts(x)` under `web/app/`; run `cd web && npm test` (or `npx vitest run <file>`).
- Pure logic (e.g. `web/app/lib/batch.ts`): table-driven with an explicit `want` per row, like the Go tests.
- Routes: `renderRoute(Route, { path, role, fetch })` from `web/app/test/render-route.tsx` renders the route's default export in the real `AuthProvider` and a router stub; `role` answers `/api/me` (`null` = 401), and `/login` / `/dashboard` render headings so redirects are assertable.
- Stub `fetch` with raw bodies (`{ status, body }`, `body: null`, `"pending"`, `"network-error"`); never `vi.mock` a module in `app/lib`, because only raw `null` / 500 / `{error}` proves "regardless of response shape". Every request a route makes must be stubbed: an unmatched URL fails the test.
- Cover per list: pending (loading), `[]` (empty), 500 (error and no empty row), top-level `null`, an item with a nested `null` array, data; plus the controls each role must or must not see.
- `setup.ts` fails a test on any `console.error` (act() warnings, render errors); a test that expects one calls `allowConsoleError()` and says why. Vitest itself fails the run on an unhandled rejection.
- Polling: assert outcomes through the pure helper; route-level tests should settle on the first poll or use fake timers, never wait out real intervals.
- Prove the test guards something: break the production line, watch it go red, restore.

### 6.5 Per-rollout-phase notes

(Appended by `/10x-implement` after each phase lands.)

- **Phase 1 — pipeline & input hardening (complete 2026-10-26):** 10MB upload cap on both upload endpoints; `sanitizeForPrompt` on purl fields and GitHub topics before LLM prompts; `libraries.error_detail` (migration 0007) persists the exhausted-retry reason, admin-only in the API; reconciliation test proves settled batch counts sum to the created count. Manual check surfaced that a bad `OPENROUTER_API_KEY` fails every library (401) — now visible in `error_detail`, but not logged server-side.
- **Phase 4 — frontend functional test harness (2026-10-29):** Vitest + Testing Library harness with `renderRoute`/`stubFetch`; `RequireAuth` role-gate tests and a characterization of `/api/me` 500 read as logged out (now logged instead of an unhandled rejection); shared `lib/batch.ts` fixed `/catalog` reading timeout/error/empty batches as "done"; "Add category" errors are shown; 4 list routes have distinct loading/empty/error/data states and nested `?? []`. Wired into CI and the end-of-turn hook.
- **Phase 2 — access control & data integrity (2026-10-26):** route × role denial matrix through the real router (35 cells); raw-JSON never-`null` tests for 5 endpoints — fixed live `"skipped": null` in both upload responses; strategic cap covered for 409, edges and concurrency — fixed a real race (concurrent marks reached 3–5 active) with `pg_advisory_xact_lock` per (category, ecosystem).

## 7. What We Deliberately Don't Test

Exclusions agreed during the rollout (Phase 2 interview, Q5).

- **Brand/UI tokens (visual detail)** — the kitchen-sink route plus manual look already proves tokens render; no snapshot/visual-regression budget. Re-evaluate if a design-system package starts shipping independently of this app. (Source: Phase 2 interview Q5.)
- **Home/landing marketing content** — static content, small blast radius, no effect on the core value loop. (Source: PRD "Non-Goals"-adjacent framing; Phase 2 interview context.)
- **Health-metrics enrichment** — PRD marks this "where possible" / secondary; no regression-test budget until it becomes a must-have. (Source: PRD §Success Criteria, Secondary.)

## 8. Freshness Ledger

- Strategy (§1–§5) last reviewed: 2026-10-21
- Stack versions last verified: 2026-10-21 (frontend runner: 2026-10-29)
- AI-native tool references last verified: n/a (none in use)

Refresh (`/10x-test-plan --refresh`) when:

- a new top-3 risk surfaces from the roadmap or archive,
- a recommended tool's `checked:` date is older than three months,
- the project's tech stack changes (new framework, new test runner),
- §7 negative-space no longer matches what the team believes.
