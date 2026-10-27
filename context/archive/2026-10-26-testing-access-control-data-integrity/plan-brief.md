# Access control and data-integrity regression tests — Plan Brief

> Full plan: `context/changes/testing-access-control-data-integrity/plan.md`
> Research: `context/changes/testing-access-control-data-integrity/research.md`

## What & Why

Rollout Phase 2 of the test plan: make sure a disallowed role can never reach a protected endpoint (#3), list endpoints never send `null` instead of `[]` (#2), and the "max 2 strategic libraries per (category, ecosystem)" rule holds (#6). Research found two of these already broken: uploads send `"skipped": null`, and concurrent marks can create a third strategic library.

## Starting Point

Routes and their role sets live inline in `main()`, unreachable by tests; role gating is only unit-tested on a stub handler. 4 of 9 list endpoints have raw-JSON tests. The cap is a check-then-insert at READ COMMITTED without a lock; only its sequential boundary is tested.

## Desired End State

A test compares every route's roles with an independent expected table and proves every disallowed (route, role) cell returns 401/403 through the real router. Every list endpoint is asserted on its raw JSON. The cap holds under concurrent requests and returns 409. The PRD matches the enforced roles.

## Key Decisions Made

| Decision | Choice | Why (1 sentence) | Source |
| --- | --- | --- | --- |
| Admin and project SBOMs | Keep code (Architect-only upload; Architect + Viewer read); correct the PRD | No behaviour change right before production | Research → user |
| Cap race | Fix now: failing concurrent test, then `pg_advisory_xact_lock` on (category, ecosystem) | One SQL line closes a real bypass; test-first | Research → user |
| Nil `skipped` | Fix in the 3 parsers (+ `ListLibraries` returns `[]`) | Guarantee at the source is inherited by new handlers | Research → user |
| How to test wiring | Route table as data (`routes()` + `newMux`), test against an independent oracle table, denial cells only | Tests the real registration without needing DB deps | Plan |
| How to test `null` | Raw JSON decoded into `any`, nullable allowlist (`github_repo`) | Go structs cannot tell `null` from `[]` | Research |
| Concurrency test shape | Deterministic interleaving via an `afterCapCheck` test seam (nil in prod) + a goroutine load test | Red before the lock is proven, not lucky | Plan (+ plan-review F1) |
| Role table oracle | `expectedRoles` is a regression lock, reviewed by the user against the PRD (Manual 1.5) | PRD has no per-route roles, so a test alone cannot prove correctness | Plan-review F2 |

## Scope

**In scope:** route registration refactor + role matrix test; 5 raw-JSON tests + nil fixes; cap 409/edge/concurrency tests + advisory lock; PRD §Access Control; test-plan §6.2/§6.5.

**Out of scope:** role-set changes; allowed-role router tests; ownership/IDOR checks; 404 for unknown `/api/*`; DB-level cap; `SkippedRow` json tags; frontend.

## Architecture / Approach

`routes(deps) []route` becomes the single source of route wiring; `newMux` registers it and `main` uses it. Tests drive `newMux` with nil dependencies because `RequireRole` rejects before any handler runs. The cap lock is transaction-scoped and keyed by the pair the rule is defined on.

## Phases at a Glance

| Phase | What it delivers | Key risk |
| --- | --- | --- |
| 1. Route wiring + role matrix (#3) | Testable routes, oracle + 401/403 matrix | Refactor must not change any route |
| 2. Lists never `null` (#2) | 5 raw-JSON tests, parser/store fixes | Nested fields missed by helper |
| 3. Strategic cap (#6) | 409 + edge tests, deterministic interleaving test, advisory lock | Seam timeout vs lock wait |
| 4. Docs | PRD aligned, cookbook patterns | — |

**Prerequisites:** Docker for integration tests; hooks from `.claude/hooks` active.
**Estimated effort:** ~1 session (well-scoped, suitable for `/10x-goal-implement`).

## Open Risks & Assumptions

- A one-line test seam (`afterCapCheck`) lives in production code (nil, no-op).
- Unknown `/api/*` paths still return the SPA with 200 (recorded follow-up).

## Success Criteria (Summary)

- Changing any route's role set or adding an unlisted route fails `go test`.
- No API response contains `null` where a list is expected.
- Concurrent marking never yields a third strategic library; the third gets 409.
