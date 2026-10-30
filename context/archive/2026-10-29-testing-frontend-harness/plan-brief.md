# Frontend Functional Test Harness — Plan Brief

> Full plan: `context/changes/testing-frontend-harness/plan.md`
> Research: `context/changes/testing-frontend-harness/research.md`

## What & Why

Test-plan Phase 4, risk #5: a frontend route can mishandle an error, empty or role-mismatch response and crash or misrender, and today only manual clicking would catch it. `web/` has no unit/component runner. This plan adds one, fixes the misrenders research found, and locks them with tests that run in CI and in the agent's end-of-turn hook.

## Starting Point

Routes fetch in `useEffect` through `web/app/lib/*.ts` with the global `fetch`; CI checks only typecheck + build. Research found these defects: the empty row shows during loading and next to errors on 4 list routes; `/catalog` says "done" after a poll timeout or error and polls an empty batch for 60 s (`/redundancy` was fixed, `/catalog` was not); "Add category" fails silently; and 4 sites crash on a `null` nested `categories` / `strategic`.

## Desired End State

`cd web && npm test` runs a Vitest suite that is a CI gate and an end-of-turn hook check. Each list route shows exactly one of loading / empty / error / data. Nested `null` no longer crashes. `/catalog` and `/redundancy` share one batch-status helper. A failed attach shows an error. Role gating (redirects and architect-only controls) is pinned by tests. Test-plan §6.4 explains how to add the next frontend test.

## Key Decisions Made

| Decision | Choice | Why (1 sentence) | Source |
| --- | --- | --- | --- |
| Test layer | Component tests (jsdom), not E2E | All failure paths are reachable by stubbing `fetch`; E2E would need a binary + DB + `page.route()` per case | Research |
| Runner | Vitest 5 + @testing-library/react 16 + jsdom | Vitest 5 peers Vite 8; RTL 16 peers React 19 | Research |
| Mocking | Stub `fetch` with raw bodies; never `vi.mock` app/lib | Only raw `null` / 500 / `{error}` proves "regardless of response shape" | Research |
| Scope | Tests **and** minimal fixes | Tests alone would pin known bugs as expected behavior | Plan (user) |
| `/api/me` 500 | Keep "treated as logged out", pin it with a characterization test | Product question left open; the change becomes an explicit test edit | Plan (user) |
| List states | Add a real loading state + never show empty with error | Makes the 4 states distinct and testable, as risk #5 asks | Plan (user) |
| Batch logic | Extract `web/app/lib/batch.ts`, used by both routes | Duplication caused the `/catalog` divergence; pure functions are the cheapest tests | Plan (user) |
| Hook | Run `vitest run` at end of turn when web changed | Matches Go tests in the hook; test-plan §5 says "required after Phase 4" | Plan (user) |

## Scope

**In scope:**
- Vitest config, `npm test`, `renderRoute` + `stubFetch` helpers, CI step
- `RequireAuth` / `AuthProvider` tests, including a `/api/me` 500 characterization
- `lib/batch.ts` with unit tests; `/catalog` batch outcomes and attach error
- Loading / empty / error / data on `/catalog`, `/organization`, `/users`, `/redundancy` report; nested `?? []`
- End-of-turn hook check + hooktest case; test-plan §4/§5/§6.4

**Out of scope:**
- Changing `/api/me` failure behavior; filter-change response race; backend changes
- MSW, new E2E specs, visual/snapshot tests
- Consolidating `errorMessage` helpers or the duplicated role tables

## Architecture / Approach

One helper renders a route's default export inside the real `AuthProvider` and a `createRoutesStub` router (with stub `/login` and `/dashboard` targets). `fetch` is answered per URL from a table of raw responses; `/api/me` sets the role. Pure logic (batch status) moves to `app/lib` and gets table-driven unit tests. For each fix: write the failing test first, then change the route.

## Phases at a Glance

| Phase | What it delivers | Key risk |
| --- | --- | --- |
| 1. Harness + auth gate | Runner, helpers, `RequireAuth` tests, CI step | `reactRouter()` plugin or router stub not working under Vitest |
| 2. Shared batch logic + /catalog fixes | `lib/batch.ts`, correct `/catalog` batch line, attach error | Polling tests hanging without fake timers |
| 3. List states in 4 routes | Distinct loading/empty/error/data, nested `?? []`, role-gated control tests | Loading flag flashing the empty row for a frame |
| 4. Hook + docs | Vitest in end-of-turn hook, hooktest case, test-plan cookbook | Hook fixture unable to run vitest (must report skip, not pass silently) |

**Prerequisites:** Node 24 + `npm install` in `web/`; Docker Postgres only for the Phase 3 E2E regression run.
**Estimated effort:** ~1 day across 4 phases (Phase 3 is the largest).
