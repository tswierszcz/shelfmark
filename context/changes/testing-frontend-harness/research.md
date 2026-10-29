---
date: 2026-10-29T21:30:55+01:00
researcher: Claude (Opus 5.5) for tswierszcz
git_commit: 89b62da
branch: shape/shelfmark
repository: shelfmark
topic: "Ground test-plan Phase 4 (risk #5): frontend routes mishandling error/empty/role-mismatch responses"
tags: [research, web, routes, require-auth, test-harness, vitest]
status: complete
last_updated: 2026-10-29
last_updated_by: Claude (Opus 5.5)
---

# Research: Ground test-plan Phase 4 — frontend functional test harness (risk #5)

**Date**: 2026-10-29T21:30:55+01:00
**Researcher**: Claude (Opus 5.5) for tswierszcz
**Git Commit**: 89b62da (working tree adds only this change folder and the test-plan §3 status cell)
**Branch**: shape/shelfmark
**Repository**: shelfmark

## Research Question

Ground rollout Phase 4 of `context/foundation/test-plan.md`, risk #5: "A frontend route mishandles an error/empty/role-mismatch response and crashes or misrenders, caught only by manual clicking." Verify the response guidance (prove each route renders data/empty/error/loading per role without crashing, regardless of response shape), find the real failure paths, locate existing tests, pick the cheapest useful layer and a runner that fits React Router 8 + Vite 8.

## Summary

- **Risk #5 is real, but its wording is partly wrong.** Of the 4 data-fetching routes read (`catalog`, `organization`, `redundancy`, `users`; `dashboard` has no fetch), none crashes on a *top-level* `null` list: every top-level list `setState` uses `?? []`. The crash path that remains is on **nested arrays**: 4 call sites dereference `library.categories` / `library.strategic` with no fallback. The backend guards those fields and has raw-JSON tests for 2 of the 3 endpoints, so today this is defense in depth, not a live bug.
- **The more likely user-visible failures are misrenders, not crashes:**
  1. The 4 list routes have no loading state. The empty-state row ("No libraries match…", "No users yet.") shows while the request is in flight, and it shows *together with* the error message when the fetch fails.
  2. `/catalog` "Add category" (`handleAttach`) has no `catch`. A failed attach gives the user no feedback and raises an unhandled promise rejection.
  3. On `/catalog`, the batch line says " — done" after a poll timeout or a poll error. An empty batch (`{}`) never settles, so it polls for 60 s. The empty-batch-status change fixed this on `/redundancy` only.
  4. `AuthProvider` treats a failed `/api/me` (500 or network error) as logged out. It redirects to `/login` and leaves the rejection unhandled.
- **Role mismatch** is handled by `RequireAuth`: it renders `null` and redirects to `/dashboard`. One Playwright test covers one cell of that matrix (architect → `/catalog`). In-page role gating is not tested at any layer: the upload/delete/mark controls render only when `isArchitect`.
- **Cheapest layer: component tests in a jsdom DOM, with `fetch` stubbed per test.** All data loading happens in `useEffect` (no route `loader`s, `ssr: false`), so a route's default export can be rendered with a stubbed `fetch` and a fake auth context. The fit is **Vitest 5.x** (peer `vite ^6.4 || ^7 || ^8`), **@testing-library/react 16.x** (peer React ^18 || ^19) and **jsdom**. Use React Router's `createRoutesStub` (or a `MemoryRouter`) for `useNavigate`/`useLocation`.
- **"Manual clicking is a permanent substitute" is refuted.** Every misrender listed above survives the current CI gates (typecheck + build) and the 3 existing E2E specs, which test only happy paths and one redirect.

## Detailed Findings

### 1. Data loading model (what a test must drive)

- `ssr: false` SPA (`web/react-router.config.ts:6`). No route exports a `loader`/`clientLoader`. Every route fetches in `useEffect` through `web/app/lib/*.ts` clients that call the global `fetch` (e.g. `web/app/lib/catalog.ts:82`, `web/app/lib/projects.ts:66`).
- Consequence: a test can stub `globalThis.fetch` (or use MSW) and render the component. No router data APIs need to be faked, but `useNavigate`, `useLocation` and `Link` (in `Nav`, `RequireAuth`) need a router context.
- Every lib client's fetcher throws on `!res.ok`. Several use `errorMessage()`, which reads `{error}` from the body with a fallback (`web/app/lib/projects.ts:36-43`). They cast `res.json()` without validation (`as Project[]`), so a `null` body reaches the route unchanged.

### 2. Auth & role gating

- `RequireAuth` (`web/app/lib/require-auth.tsx:15-46`) shows "Loading…" while `loading`. When there is no `me` it redirects to `/login`, and on a role mismatch it redirects to `/dashboard`. In both cases it renders `null`, so the protected content is never mounted.
- `AuthProvider` (`web/app/lib/auth.ts:65-83`): `me()` throws on any non-401 failure (`auth.ts:45-47`). The `.then` is skipped, `.finally` sets `loading=false`, and `currentUser` stays `null`, so the user is sent to `/login` and the rejection has no `.catch`. **Observed behavior; whether a 500 on `/api/me` *should* read as logged out is a product choice (open question 1).**
- Route role sets (default exports): `/catalog` and `/users` → `"admin"` (`catalog.tsx:348`, `users.tsx:148`); `/organization` and `/redundancy` → `["architect","viewer"]` (`organization.tsx:314`, `redundancy.tsx:426`); `/dashboard` → any signed-in user (`dashboard.tsx:22`). `Nav`'s `LINKS` table (`web/app/components/nav.tsx:8-14`) repeats the same sets separately, so the two can drift.
- In-page gating: on `/redundancy` the upload form and the Delete button render only when `me.role === "architect"` (`redundancy.tsx:69, 235, 324`); on `/organization`, Mark/Unmark do the same (`organization.tsx:18, 243, 257`). A viewer must not see them. The backend enforces the role anyway (route × role matrix, test-plan §6.2), so a regression here is a UI-only defect.

### 3. Empty / null response handling per route

Top-level lists: all fall back with `?? []`. These are `redundancy.tsx:98, 115`, `catalog.tsx:61, 70`, `organization.tsx:45, 54` and `users.tsx:22`.

Nested arrays dereferenced with no fallback (all 4 crash with `TypeError` if the field is `null`):

| Site | Expression |
|---|---|
| `web/app/routes/redundancy.tsx:369, 379, 383` | `library.categories.some(...)`, `.length`, `.map` |
| `web/app/routes/catalog.tsx:297` | `library.categories.length` / `.join` |
| `web/app/routes/organization.tsx:220, 224` | `library.categories.length` / `.map` |
| `web/app/routes/organization.tsx:226` | `library.strategic.find(...)` |

A render-time `TypeError` reaches the root `ErrorBoundary` (`web/app/root.tsx:53-80`), which replaces the whole page with "Oops! / An unexpected error occurred." (the message appears only in DEV).

Backend status of those fields (the reason the likelihood is lower than the test plan implies):
- `/api/catalog/libraries` and `/api/organization/libraries`: `categories`/`strategic` are nil-guarded (`backend/internal/catalog/handlers.go:213, 279, 283`) and asserted `[]` in raw JSON by `assertLibraryListContract` (`backend/internal/catalog/json_contract_test.go:103-157`).
- `/api/projects/{id}/report`: each library starts with `Categories: []projectReportCategory{}` (`backend/internal/catalog/project_handlers.go:201`) and the outer slice uses `make` (`:219`). No raw-JSON never-null test was found for this endpoint: `assertNoNullArrays` is used only in `json_contract_test.go`, whose 5 tests cover libraries, organization libraries, batch progress, upload and project upload.
- The lesson's frontend half ("every frontend consumer … should still defensively fall back", `context/foundation/lessons.md`) is followed at the top level and not followed at these nested sites.

### 4. Loading and error rendering (misrender, not crash)

- None of the 4 list routes has a loading flag for its list. The initial state is `[]` (`catalog.tsx:43`, `organization.tsx:25`, `users.tsx:15`, `redundancy.tsx:85`), so the empty-state row renders until the first response arrives (`catalog.tsx:331-337`, `organization.tsx:297-303`, `users.tsx:131-137`, `redundancy.tsx:408-414`).
- When the list fetch fails, the error `<p>` and the empty-state row render together. Example: `users.tsx:114` shows "Failed to load users." while `users.tsx:131-137` still shows "No users yet." The same holds for catalog and organization. On `/redundancy` the report row appears only after a project is selected, and a failed report reload keeps the *previous* project's rows: `refreshReport` does not clear `report` on error (`redundancy.tsx:111-117`).
- `/catalog` `handleAttach` (`catalog.tsx:140-153`): `try/finally` with no `catch`. A failed `attachCategory` (which throws, `catalog.ts:114-116`) gives the user no message and leaves an unhandled rejection.
- `/catalog` batch line (`catalog.tsx:204`): `polling ? " — processing…" : " — done"`. That reads " — done" after a timeout (`:98-103`) and after a poll error (`:107-113`). `isBatchSettled` returns `false` for `{}` (`catalog.tsx:21-27`), so an all-known-purls upload polls for 60 s and then says "done". `/redundancy` already has the fixed version (`redundancy.tsx:24-65`, `describeBatch`). This divergence is a test target, and it may be a bug to fix in a separate change (open question 2).
- Filter race (inference, not reproduced): `catalog` and `organization` refetch on every filter change with no request token or abort (`catalog.tsx:75-78`, `organization.tsx:59-62`). A slower earlier response can overwrite a newer one. `/redundancy` polling does use a token (`redundancy.tsx:92, 139, 147`). The race is noted as speculative for this phase.

### 5. Existing tests

- Frontend unit/component: **none**. `web/package.json` has no test script and no vitest/jest/testing-library dependency, and none is installed under `web/node_modules`.
- E2E (Playwright 1.64.1, `web/playwright.config.ts`): 3 spec files plus 1 setup file under `web/tests/e2e/`. They are `seed.spec.ts` (architect → `/catalog` redirects to `/dashboard`, one role-mismatch cell), `sbom-upload-report.spec.ts`, `project-delete.spec.ts` and `auth.setup.ts`. They run against the real binary and Postgres, with only the external APIs mocked. They cannot force a 500 or a `null` body from `/api/*` without `page.route()` interception, which is not used today (`web/tests/e2e/`, grep for `page.route`: no hits in specs; the 1 hit is a comment in `mocks/external-apis-mock.mjs:3`).
- CI web gate: `npm run typecheck && npm run build` (AGENTS.md). Neither renders a route.

### 6. Runner fit (React Router 8 + Vite 8 + React 19)

Registry versions checked on 2026-10-29 with `npm view`:
- `vitest` 5.0.5, peer `vite: ^6.4.0 || ^7.0.0 || ^8.0.0`. Installed Vite is 8.3.5 (`web/node_modules/vite/package.json`). Vitest reuses `vite.config.ts`, but the `reactRouter()` plugin should be **excluded in test mode** (it expects the RR build/dev pipeline). The usual pattern is a separate `vitest.config.ts`, or `plugins: process.env.VITEST ? [] : [...]`. Plan should confirm this with a smoke test.
- `@testing-library/react` 16.3.3, peer react/react-dom ^18 || ^19, plus `@testing-library/dom` ^10.
- `jsdom` 30.1.3 (alternative: happy-dom).
- Router context: `createRoutesStub` from `react-router` (RR 7+/8 testing helper) or `MemoryRouter`. Either provides `useNavigate`/`useLocation`, and redirects can be asserted by rendering a stub `/dashboard` / `/login` route.
- Auth context: `AuthProvider` fetches `/api/me` itself, so stubbing `fetch` for `/api/me` is enough to drive every role. No module mocking of `auth.ts` is required.
- `typecheck` runs `tsc` over `**/*` (`web/tsconfig.json:2-7`), so test files are type-checked by the existing gate once `vitest/globals` or explicit imports are typed.

### 7. Response guidance — verdict

| Guidance item | Verdict |
|---|---|
| Prove: correct state (data/empty/error/loading) per role without crashing, regardless of response shape | **Keep, sharpen.** "Loading" as a distinct state does not exist in 4 list routes today. A test asserting it would test a feature not yet built. Assert instead what is observable: no crash, and empty-state vs error not shown together (this one fails today; see §4). |
| Challenge "manual clicking is a permanent substitute" | **Confirmed wrong.** None of the §4 misrenders is caught by typecheck, build or the existing E2E specs. |
| Context: how `require-auth.tsx` and fetch call sites handle error/null/slow | Grounded in §2–§4. |
| Cheapest layer: unit/component | **Confirmed.** Component tests with stubbed `fetch` reach every §3/§4 path. E2E would need `page.route()` per case, and a full binary + DB. |
| Anti-pattern: happy path only | **Keep.** Add: do not stub at the lib-function level (`vi.mock("../lib/catalog")`). Stubbing `fetch` with raw bodies (`null`, `{error}`, 500) is what proves "regardless of response shape". |

## Code References

- `web/app/lib/require-auth.tsx:15-46` - loading / no-session / wrong-role branches
- `web/app/lib/auth.ts:38-50, 65-83` - `me()` throws on non-401; provider has no catch
- `web/app/routes/redundancy.tsx:111-117` - report error keeps stale rows
- `web/app/routes/redundancy.tsx:369-383` - nested `categories` deref
- `web/app/routes/catalog.tsx:21-27, 98-113, 204` - empty-batch / timeout / error read as "done"
- `web/app/routes/catalog.tsx:140-153` - attach without catch
- `web/app/routes/catalog.tsx:297` - nested `categories` deref
- `web/app/routes/organization.tsx:220-226` - nested `categories` / `strategic` deref
- `web/app/routes/users.tsx:114, 131-137` - error and empty-state rendered together
- `web/app/root.tsx:53-80` - root ErrorBoundary (whole-page fallback)
- `web/app/components/nav.tsx:8-14` - duplicated role table
- `backend/internal/catalog/json_contract_test.go:103-157` - nested never-null asserted for both library lists
- `backend/internal/catalog/project_handlers.go:201, 219` - report nil-guards (no raw-JSON test)

## Architecture Insights

- Fetch-in-effect with lib clients is uniform across routes, so one test helper (render route + stub fetch by URL + set role via `/api/me`) covers every route.
- Error handling is per-route copy-paste (`errorMessage` is duplicated in 4 lib files; batch polling in 2 routes). The catalog/redundancy batch divergence is a direct result.
- Role sets live in 3 places: backend `routes()`, route-file `RequireAuth role=…`, and `Nav` `LINKS`. Only the backend copy has a regression lock.

## Historical Context (from prior changes)

- `context/foundation/lessons.md` - nil-slice → `null` lesson; the frontend half (defensive `?? []`) is applied at the top level only.
- `context/archive/2026-10-27-empty-batch-status/plan.md:5, 17-21` - fixed the empty-batch settle and wording on `/redundancy` only. Its "What We're NOT Doing" does not mention `/catalog`, so the divergence looks unplanned rather than deliberate.
- `context/archive/2026-10-26-testing-access-control-data-integrity/` - backend route × role matrix and raw-JSON never-null tests (test-plan §6.2), the backend counterpart of this phase.
- `context/foundation/test-stack.md` - Playwright setup; seed spec covers one UI role-gate cell.

## Related Research

- Not applicable. No earlier frontend-testing research exists in `context/changes/` or `context/archive/`.

## Open Questions

1. Should a 500/network failure on `/api/me` show an error state instead of redirecting to `/login`? This is a product choice. The test should pin whichever behavior is chosen.
2. Should this phase *fix* the misrenders it exposes (catalog batch line, attach without catch, error + empty shown together, nested `?? []`), or only add tests that characterize them? The test-plan Phase 4 goal says "stand up a runner and cover…", which suggests a minimal fix plus regression tests for the clear bugs, and `..._characterization` tests for the product questions (pattern from test-plan §6.3).
3. Does test-plan §2 need a backport? Research suggests rewording risk #5's likelihood evidence: nested null is defense in depth (backend-guarded), while error/empty/loading misrenders and silent action failures are the live part.
4. CI: add `npm test` to the web gate in `.github/workflows/ci.yml` and to the end-of-turn hook? Test-plan §5 says frontend unit/component is "required after Phase 4".
