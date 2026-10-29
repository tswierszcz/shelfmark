# Frontend Functional Test Harness Implementation Plan

## Overview

Stand up a frontend component/unit test runner in `web/` (Vitest + React Testing Library on jsdom, `fetch` stubbed with raw response bodies). Use it to lock test-plan risk #5: a route must render the right state (loading / empty / error / data) for each role without crashing, whatever shape the API response has. Fix the misrenders that research found, write regression tests for them, and run the tests in CI and in the agent's end-of-turn hook.

## Current State Analysis

- `web/` has no unit/component runner: `web/package.json` has no test script and no vitest/jest/testing-library dependency. The CI web job runs `npm run typecheck` and `npm run build` only (`.github/workflows/ci.yml:46-48`), and the end-of-turn hook runs only `tsc` for web (`.claude/hooks/end-of-turn.sh:98-110`).
- Routes fetch in `useEffect` through `web/app/lib/*.ts` clients that call the global `fetch`; there are no loaders and `ssr: false` (`web/react-router.config.ts:6`). `AuthProvider` loads the user from `/api/me` (`web/app/lib/auth.ts:65-83`). `RequireAuth` gates on `loading` / `me` / `role` (`web/app/lib/require-auth.tsx:15-46`).
- Misrenders found in research (`context/changes/testing-frontend-harness/research.md` §3–§4):
  - The 4 list routes have no loading state. The empty row renders while the request is in flight and also *together with* the error message (`users.tsx:114, 131-137`; same pattern in `catalog.tsx`, `organization.tsx`, `redundancy.tsx`).
  - On `/redundancy`, a failed report reload keeps the previous project's rows (`redundancy.tsx:111-117`).
  - `/catalog` `handleAttach` has no `catch` (`catalog.tsx:140-153`).
  - On `/catalog`, the batch line reads " — done" after a timeout or a poll error, and an empty batch polls for 60 s (`catalog.tsx:21-27, 98-113, 204`). `/redundancy` already has the fixed logic (`redundancy.tsx:24-65`).
  - Nested `categories` / `strategic` are dereferenced with no fallback at 4 sites (`redundancy.tsx:369-383`, `catalog.tsx:297`, `organization.tsx:220-226`).
  - A 500 or network error on `/api/me` is treated as logged out, with an unhandled rejection (`auth.ts:45-47, 68-78`).

## Desired End State

- `cd web && npm test` runs Vitest once (non-watch) and passes. The CI web job runs it. The end-of-turn hook runs it when web files changed and blocks the turn on failure.
- Every list route (`/catalog`, `/organization`, `/users`, the `/redundancy` report) renders exactly one of four states: loading row, empty row, error message (no empty row), or data rows. Component tests pin each state with a raw stubbed response.
- A `null` nested `categories` / `strategic` in a response renders the library without crashing.
- `/catalog` and `/redundancy` share one batch-status helper. `/catalog` names timeout, error and empty-batch outcomes the same way `/redundancy` does. A failed "Add category" shows an error message.
- `RequireAuth` behavior (loading, no session → `/login`, wrong role → `/dashboard`, allowed role) and the in-page architect-only controls are covered by tests. The `/api/me` 500 behavior is pinned by a characterization test.
- Test-plan §4, §5 and §6.4 describe the runner and how to add a frontend test.

### Key Discoveries:

- `vitest` 5.0.5 peers `vite ^6.4 || ^7 || ^8`; the installed Vite is 8.3.5. `@testing-library/react` 16.3.3 peers React ^18 || ^19 and needs `@testing-library/dom` ^10. `jsdom` is at 30.1.3 (registry, checked 2026-10-29).
- `reactRouter()` in `web/vite.config.ts:6` expects the React Router build pipeline. Vitest needs its own config without it.
- `web/tsconfig.json:2-7` includes `**/*`, so test files are type-checked by the existing `typecheck` gate.
- `describeBatch` / `isBatchSettled` in `redundancy.tsx:27-65` are pure functions and can be moved as-is.
- Wording for an empty batch, " — done: no new libraries to categorize", was approved in `context/archive/2026-10-27-empty-batch-status/plan.md:5`.

## What We're NOT Doing

- No change to `/api/me` failure handling: a 500 still reads as logged out. It is pinned by a `..._characterization` test so changing it later is an explicit test edit.
- No fix for the filter-change response race on `/catalog` / `/organization` (research §4, speculative, not reproduced).
- No backend changes, no new raw-JSON test for `/api/projects/{id}/report`.
- No MSW or other network-mock library: a small in-repo `fetch` stub covers the cases.
- No new E2E specs and no `page.route()` interception.
- No visual or snapshot tests (test-plan §7).
- No consolidation of the duplicated `errorMessage` helpers in `web/app/lib/*.ts`, and no single source for the role tables (`nav.tsx` `LINKS` vs route `RequireAuth role`).
- `/home`, `/login`, `/dashboard` content and `dev.kitchen-sink-redundancy` get no route tests beyond what `RequireAuth` tests cover.

## Implementation Approach

Build the harness first and prove it on the smallest surface (`RequireAuth` and auth), so later phases only add tests and fixes. Then pull the batch logic into a pure module, because pure functions are the cheapest tests and the extraction removes the duplication that caused the `/catalog` divergence. Then fix list states route by route: write a test that fails on the current misrender, then fix it. Wire the hook last, once the suite is stable and fast.

Tests stub `globalThis.fetch` with raw `Response` bodies (`null`, `{"error": …}`, status 500, a never-resolving promise for loading). They never mock `web/app/lib/*` modules, because the point is to prove behavior "regardless of response shape" (research §7).

## Critical Implementation Details

- **Timing & lifecycle** — Batch polling uses `setTimeout` with 2 s intervals and a 60 s cap. Route tests that cover polling must use Vitest fake timers (`vi.useFakeTimers({ shouldAdvanceTime: true })` or explicit `advanceTimersByTimeAsync`), or they will hang or take a minute. Prefer testing outcomes through the pure `describeBatch`, and keep route-level polling tests to the single `/catalog` wiring check.
- **State sequencing** — "Loading" must be true from the first render, not set inside the effect, or the empty row flashes for one frame and a test with an unresolved fetch sees it. Initialize the loading flag to `true`, and clear it in `finally`.

## Phase 1: Harness + auth gate

### Overview

Install the runner, add a route-rendering helper, prove it on `RequireAuth`, and add the suite to CI.

### Changes Required:

#### 1. Dependencies and scripts

**File**: `web/package.json`, `web/package-lock.json`

**Intent**: Add the test toolchain as dev dependencies and a non-watch test script used by CI and the hook.

**Contract**: devDependencies `vitest` ^5.0.5, `@testing-library/react` ^16.3.3, `@testing-library/dom` ^10, `jsdom` ^30; scripts `"test": "vitest run"`. Installed with `npm install` so the lockfile stays consistent for `npm ci`.

#### 2. Vitest config

**File**: `web/vitest.config.ts` (new)

**Intent**: Run tests in jsdom without the React Router Vite plugin, reusing the Tailwind-free parts of the app config.

**Contract**: `environment: "jsdom"`, `include: ["app/**/*.test.{ts,tsx}"]` (keeps Playwright's `tests/e2e/**` out), `setupFiles: ["app/test/setup.ts"]`, `restoreMocks: true`; no `reactRouter()` plugin. The `~/*` path alias resolves as in `tsconfig.json`.

#### 3. Test support

**File**: `web/app/test/setup.ts`, `web/app/test/render-route.tsx`, `web/app/test/fetch-stub.ts` (new)

**Intent**: One helper renders a route's default export inside a real `AuthProvider` and a router stub, with `fetch` answered per URL from a table. `/api/me` decides the role.

**Contract**:
- `stubFetch(routes: Record<string, StubResponse>)`: matches `"METHOD /path"` or `"/path"` (pathname, query ignored unless the key has one). A `StubResponse` is `{ status?, body? }` (body serialized as raw JSON, so `null` is sent as `null`), `"pending"` (a never-resolving promise for loading), or `"network-error"` (rejects). An unmatched request is recorded and answered with status 599, never thrown: a throw would land in the route's own `catch` and could make an error-state test pass by accident. `setup.ts` fails the test in `afterEach` if any unmatched URL was recorded, listing them.
- `renderRoute(Component, { path, role | null, fetch })`: stubs `/api/me` (200 with the role, or 401 for `null`) plus the given routes. It renders through `createRoutesStub` from `react-router` with the target path plus stub `/login` and `/dashboard` routes that render identifiable headings, so redirects are assertable.
- `setup.ts` runs RTL `cleanup` after each test, restores `fetch`, and fails the test when `console.error` was called or an `unhandledrejection` occurred during it (this catches act() warnings and swallowed promise rejections). A test that expects one opts out explicitly.

#### 4. Auth gate tests

**File**: `web/app/lib/require-auth.test.tsx` (new)

**Intent**: Pin the role gate on the frontend, the UI half of test-plan risk #3/#5.

**Contract**: cases: `/api/me` pending → "Loading…" and no protected content; 401 → lands on `/login`; architect on an `role="admin"` page → lands on `/dashboard`, protected content never rendered; admin on the same page → content rendered; role array (`["architect","viewer"]`) admits viewer. Plus `AuthProvider_ApiMe500_TreatedAsLoggedOut_Characterization`: `/api/me` 500 → lands on `/login`, with a comment pointing at research.md open question 1.

#### 5. CI

**File**: `.github/workflows/ci.yml`

**Intent**: Make the frontend suite a required gate.

**Contract**: web job gains `- run: npm test` after `npm run typecheck`; the header comment lists it.

### Success Criteria:

#### Automated Verification:

- Frontend tests pass: `cd web && npm test`
- Type checking passes: `cd web && npm run typecheck`
- Build passes: `cd web && npm run build`
- Breaking the wrong-role redirect in `require-auth.tsx` makes `npm test` fail, and restoring it makes it pass
- A throwaway test that calls `console.error` fails under the `setup.ts` guard (then removed)

#### Manual Verification:

- `npm test` finishes in under 30 s locally

**Implementation Note**: After completing this phase and all automated verification passes, pause here for manual confirmation from the human that the manual testing was successful before proceeding to the next phase.

---

## Phase 2: Shared batch logic + /catalog fixes

### Overview

Move batch-status logic into a pure module used by both upload routes, and give `/catalog` the fixed outcomes plus an attach error.

### Changes Required:

#### 1. Batch module

**File**: `web/app/lib/batch.ts` (new), `web/app/lib/batch.test.ts` (new)

**Intent**: Single source for "is this batch settled" and "what does the batch line say".

**Contract**: exports `SETTLED_STATUSES`, `POLL_INTERVAL_MS`, `POLL_MAX_DURATION_MS`, `type PollOutcome = "settled" | "timeout" | "error"`, `isBatchSettled(progress: BatchProgress): boolean` (empty map is settled), and `describeBatch(polling, outcome, progress): { text: string; problem: boolean }`, all moved unchanged from `redundancy.tsx:20-65`. Table-driven tests cover: polling, error, timeout, `{}`, all categorized, needs_manual singular/plural, failed, both.

#### 2. /redundancy uses the module

**File**: `web/app/routes/redundancy.tsx`

**Intent**: Replace the local copies with imports; behavior unchanged.

**Contract**: the local constants, `isBatchSettled`, `PollOutcome` and `describeBatch` are removed and imported from `../lib/batch`.

#### 3. /catalog batch line and polling

**File**: `web/app/routes/catalog.tsx`

**Intent**: `/catalog` tracks the poll outcome like `/redundancy` does, so timeout, error and an empty batch read correctly and problems use the destructive tone.

**Contract**: `/catalog` keeps a `pollOutcome` state set to `settled` / `timeout` / `error` where polling stops. The batch `<p>` renders `describeBatch(...)` with `text-destructive` when `problem`. It uses the shared `isBatchSettled`, so `{}` settles on the first poll. The existing refresh-on-settle behavior stays.

#### 4. /catalog attach error

**File**: `web/app/routes/catalog.tsx`

**Intent**: A failed "Add category" tells the user instead of failing silently.

**Contract**: `handleAttach` catches the error and sets an `attachError` message (the thrown message or "Failed to add category."), rendered as `text-destructive` in the Libraries section. It is cleared on the next attach attempt.

#### 5. /catalog tests

**File**: `web/app/routes/catalog.test.tsx` (new)

**Intent**: Lock the wiring of the fixes on the route.

**Contract**: cases: upload → batch progress `{}` → batch line " — done: no new libraries to categorize" without waiting 60 s; batch progress request 500 → " — status unavailable" in destructive tone; attach `POST` 500 with `{"error":"boom"}` → "boom" shown. Fake timers per Critical Implementation Details.

### Success Criteria:

#### Automated Verification:

- Frontend tests pass: `cd web && npm test`
- Type checking passes: `cd web && npm run typecheck`
- Build passes: `cd web && npm run build`
- `grep -n "function describeBatch\|function isBatchSettled" web/app/routes/*.tsx` returns no matches

#### Manual Verification:

- On a local build, as admin, uploading `docs/samples/smoke-catalog.csv` twice: the second upload's batch line settles at once with "— done: no new libraries to categorize"
- `/redundancy` batch line still behaves as before for a fresh and a repeated SBOM upload (architect)

**Implementation Note**: After completing this phase and all automated verification passes, pause here for manual confirmation from the human that the manual testing was successful before proceeding to the next phase.

---

## Phase 3: List states in 4 routes

### Overview

Give every list one of four distinct states, stop nested `null` from crashing, and test each route per role.

### Changes Required:

#### 1. Loading / empty / error / data on list routes

**File**: `web/app/routes/catalog.tsx`, `web/app/routes/organization.tsx`, `web/app/routes/users.tsx`

**Intent**: The empty row means "loaded and empty", never "still loading" or "failed".

**Contract**: each route keeps a `loading` flag, initialized `true` and cleared in `finally` of its refresh. The table body renders a "Loading…" row (`text-ink-faint`) only when `loading && list.length === 0`, and the existing empty row only when `!loading && !error && list.length === 0`. No empty row is shown when an error is set. The error `<p>` is unchanged. Rows already shown stay visible, with no "Loading…" row, during a filter refetch. A failed refetch clears the list rather than leaving stale rows next to the error.

#### 2. /redundancy report states

**File**: `web/app/routes/redundancy.tsx`

**Intent**: Same four states for the report table, and no stale rows from a previous project.

**Contract**: `refreshReport` sets a `reportLoading` flag and clears `report` before the request and on error. The table shows a "Loading…" row while loading, "No libraries recorded for this project." only on a successful empty response, and no empty row on error. The project picker's `projectsError` behavior is unchanged.

#### 3. Nested array fallbacks

**File**: `web/app/routes/redundancy.tsx`, `web/app/routes/catalog.tsx`, `web/app/routes/organization.tsx`

**Intent**: Apply the frontend half of the nil-slice lesson to nested arrays.

**Contract**: every read of `library.categories` / `library.strategic` at the 4 research sites goes through `?? []` (a local `const categories = library.categories ?? []` per row is fine). The types in `web/app/lib/*.ts` stay non-nullable; the fallback is defensive.

#### 4. Route tests

**File**: `web/app/routes/users.test.tsx`, `web/app/routes/organization.test.tsx`, `web/app/routes/redundancy.test.tsx` (new); `web/app/routes/catalog.test.tsx` (extend)

**Intent**: Prove risk #5 per route with raw responses.

**Contract**: for each list route: pending → "Loading…" and no empty row; `[]` → empty row; 500 with `{"error":…}` → error message and **no** empty row; top-level `null` body → empty row, no crash; one item with `categories: null` (and `strategic: null` where present) → item row renders, no crash; data → rows. Role cases: `/organization` and `/redundancy` as viewer → no "Mark strategic" / upload form / "Delete project"; as architect → present. `/redundancy`: report 500 after a successful project → old rows gone, error shown.

### Success Criteria:

#### Automated Verification:

- Frontend tests pass: `cd web && npm test`
- Type checking passes: `cd web && npm run typecheck`
- Build passes: `cd web && npm run build`
- Removing one `?? []` fallback or the `!error` empty-row guard makes `npm test` fail, and restoring it makes it pass
- E2E still green: `cd web && npx playwright test`

#### Manual Verification:

- With the backend stopped (`/api/*` failing), `/users` as admin shows "Failed to load users." without "No users yet."
- On a slow network (DevTools throttling), each list shows "Loading…" before data, with no flash of the empty row

**Implementation Note**: After completing this phase and all automated verification passes, pause here for manual confirmation from the human that the manual testing was successful before proceeding to the next phase.

---

## Phase 4: Hook + docs

### Overview

Run the suite at end of turn when web changed, prove the hook, and record the harness in the test plan.

### Changes Required:

#### 1. End-of-turn hook

**File**: `.claude/hooks/end-of-turn.sh`

**Intent**: The agent sees frontend test failures before ending a turn, like Go tests for changed packages.

**Contract**: in the existing `WEB_CHANGED` block, after `tsc` passes, run `npx --no-install vitest run` when `web/vitest.config.ts` exists. Failure is added to the report under the heading `Web tests fail ($ROOT/web):` and blocks (exit 2). If vitest is not installed, the hook reports a skip (systemMessage) rather than passing silently, consistent with the existing "skipped" handling. The header comment lists the new check.

#### 2. Hook test

**File**: `backend/hooktest/agent_hooks_test.go`

**Intent**: Prove the new gate blocks on a failing frontend test and does not fire silently.

**Contract**: the fixture never runs real tools. `npxStub` (`agent_hooks_test.go:22-36`) ends in `exit 0`, so without a change a vitest call would pass silently. Extend `npxStub` with a `*vitest*` branch: it prints `FAIL <file>` and exits 1 for every `app/**/*.test.ts(x)` containing `BROKEN_TEST`, and `STUB_VITEST=missing` fails it the way a missing local binary does. `newFixture` writes `web/vitest.config.ts` so the hook's gate is open. New cases in `TestStop_SweepsChangesWithOneRetry` (or a sibling test): a test file with `BROKEN_TEST` → block containing "Web tests fail"; `STUB_VITEST=missing` → pass with a systemMessage naming the skipped web tests, following the `catalog skip is reported, not silent` case.

#### 3. Test plan

**File**: `context/foundation/test-plan.md`

**Intent**: Record the runner and the cookbook for the next frontend test.

**Contract**: §4 Stack row "unit + integration (frontend)" → Vitest + @testing-library/react + jsdom with versions and `checked: 2026-10-29`. §5 "frontend unit/component" gate → wired (CI + end-of-turn hook). §6.4 replaces "TBD" with: use `renderRoute` + `stubFetch` with raw bodies, never `vi.mock` of `app/lib`; cover loading/empty/error/data plus a `null` nested array; use fake timers for polling; prove the test by breaking the guarded line. A §6.5 Phase 4 note. §2 risk #5: reword the Source/likelihood evidence per research open question 3 (nested `null` is backend-guarded defense in depth; the live part is error/empty/loading misrenders and silent action failures). This is the post-research backport §2 allows, with no file anchors added. §3 Phase 4 status is updated by the lifecycle (`implementing` / `complete`), not by this phase.

### Success Criteria:

#### Automated Verification:

- Hook tests pass: `cd backend && go test ./hooktest/...`
- Frontend tests pass: `cd web && npm test`
- `grep -n "TBD — see §3 Phase 4" context/foundation/test-plan.md` returns no matches

#### Manual Verification:

- Editing a route so a test fails and ending a turn makes the hook block with "Web tests fail"; reverting lets the turn end

**Implementation Note**: After completing this phase and all automated verification passes, pause here for manual confirmation from the human that the manual testing was successful before proceeding to the next phase.

---

## Testing Strategy

### Unit Tests:

- `lib/batch.ts`: every outcome of `describeBatch` and `isBatchSettled` (table-driven, one case per row, explicit `want`).

### Component Tests:

- `RequireAuth` / `AuthProvider`: loading, 401, wrong role, allowed role, role array, `/api/me` 500 characterization.
- Each list route: pending, `[]`, error with body, top-level `null`, nested `null`, data; role-gated controls for viewer vs architect.
- `/catalog`: empty batch, poll error, attach error.

### Manual Testing Steps:

1. Repeat a catalog CSV upload and see the batch settle at once with the "no new libraries" wording.
2. Stop the backend and open `/users`: error only, no empty row.
3. Throttle the network and open `/organization`: "Loading…" before rows.
4. Break a route test, end a turn: the hook blocks.

## Performance Considerations

The suite should stay under ~30 s so the end-of-turn hook stays fast. Fake timers keep polling tests instant.

## Migration Notes

Not applicable: no schema or data changes.

## References

- Research: `context/changes/testing-frontend-harness/research.md`
- Test plan risk #5 and Phase 4: `context/foundation/test-plan.md` §2, §3
- Fixed batch logic to extract: `web/app/routes/redundancy.tsx:20-65`
- Empty-batch wording decision: `context/archive/2026-10-27-empty-batch-status/plan.md:5`
- Nil-slice lesson: `context/foundation/lessons.md`
- Hook pattern: `.claude/hooks/end-of-turn.sh:98-110`, `backend/hooktest/agent_hooks_test.go:227-266`

## Progress

> Convention: `- [ ]` pending, `- [x]` done. Append ` — <commit sha>` when a step lands. Do not rename step titles. See `references/progress-format.md`.

### Phase 1: Harness + auth gate

#### Automated

- [x] 1.1 Frontend tests pass: `cd web && npm test` — 6d539d1
- [x] 1.2 Type checking passes: `cd web && npm run typecheck` — 6d539d1
- [x] 1.3 Build passes: `cd web && npm run build` — 6d539d1
- [x] 1.4 Breaking the wrong-role redirect in `require-auth.tsx` makes `npm test` fail, and restoring it makes it pass — 6d539d1
- [x] 1.5 A throwaway test that calls `console.error` fails under the `setup.ts` guard (then removed) — 6d539d1

#### Manual

- [x] 1.6 `npm test` finishes in under 30 s locally — 6d539d1

### Phase 2: Shared batch logic + /catalog fixes

#### Automated

- [x] 2.1 Frontend tests pass: `cd web && npm test`
- [x] 2.2 Type checking passes: `cd web && npm run typecheck`
- [x] 2.3 Build passes: `cd web && npm run build`
- [x] 2.4 `grep -n "function describeBatch\|function isBatchSettled" web/app/routes/*.tsx` returns no matches

#### Manual

- [x] 2.5 On a local build, as admin, uploading `docs/samples/smoke-catalog.csv` twice: the second upload's batch line settles at once with "— done: no new libraries to categorize"
- [x] 2.6 `/redundancy` batch line still behaves as before for a fresh and a repeated SBOM upload (architect)

### Phase 3: List states in 4 routes

#### Automated

- [ ] 3.1 Frontend tests pass: `cd web && npm test`
- [ ] 3.2 Type checking passes: `cd web && npm run typecheck`
- [ ] 3.3 Build passes: `cd web && npm run build`
- [ ] 3.4 Removing one `?? []` fallback or the `!error` empty-row guard makes `npm test` fail, and restoring it makes it pass
- [ ] 3.5 E2E still green: `cd web && npx playwright test`

#### Manual

- [ ] 3.6 With the backend stopped (`/api/*` failing), `/users` as admin shows "Failed to load users." without "No users yet."
- [ ] 3.7 On a slow network (DevTools throttling), each list shows "Loading…" before data, with no flash of the empty row

### Phase 4: Hook + docs

#### Automated

- [ ] 4.1 Hook tests pass: `cd backend && go test ./hooktest/...`
- [ ] 4.2 Frontend tests pass: `cd web && npm test`
- [ ] 4.3 `grep -n "TBD — see §3 Phase 4" context/foundation/test-plan.md` returns no matches

#### Manual

- [ ] 4.4 Editing a route so a test fails and ending a turn makes the hook block with "Web tests fail"; reverting lets the turn end
