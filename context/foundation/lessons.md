# Lessons Learned

> Append-only register of recurring rules and patterns. Re-read at start by /10x-frame, /10x-research, /10x-plan, /10x-plan-review, /10x-implement, /10x-impl-review.

## Go nil slices marshal to JSON `null`, not `[]`

- **Context**: Found 2026-10-20 while manually verifying `/redundancy` (`web/app/routes/redundancy.tsx:57`, `backend` `GET /api/projects`) during the `redundancy-view-brand-tokens` close-out. Confirmed with `curl`: `GET /api/projects` for an account with zero projects returns the literal body `null` (content-length 5), not `[]`.
- **Problem**: Go's `encoding/json` marshals a nil slice as `null`. Any Go handler that returns a slice straight from a DB query (which is nil when the result set is empty, not an empty slice) ships `null` to the frontend. A frontend that does `setState(result)` without an `?? []` fallback then calls `.map()` on `null` and crashes — this is exactly what happened on `/redundancy` for any project-less architect/viewer account (`TypeError: Cannot read properties of null (reading 'map')`).
- **Rule**: Every Go handler that returns a list must guarantee a non-nil slice before marshaling (`if result == nil { result = []T{} }`, or initialize with `make([]T, 0)` instead of `var result []T`). Every frontend consumer of a list endpoint should still defensively fall back with `?? []` regardless, since the guarantee can regress.
- **Applies to**: All Go HTTP handlers that return JSON arrays (`backend/internal/**/*.go`), and all frontend `fetch`-and-`setState` call sites that consume a list endpoint (`web/app/lib/*.ts`, `web/app/routes/*.tsx`).
