<!-- IMPL-REVIEW-REPORT -->
# Implementation Review: Architect marks strategic libraries

- **Plan**: context/changes/mark-strategic-libraries/plan.md
- **Scope**: Full plan
- **Reviewed phases**: 1, 2, 3, 4
- **Date**: 2026-10-18
- **Verdict**: NEEDS ATTENTION
- **Findings**: 0 critical, 2 warnings, 1 observation

## Verdicts

| Dimension | Verdict |
|-----------|---------|
| Plan Adherence | PASS |
| Scope Discipline | PASS |
| Safety & Quality | WARNING |
| Architecture | PASS |
| Pattern Consistency | WARNING |
| Success Criteria | PASS |

## Findings

### F1 — Not-found detection via string-matching is fragile

- **Severity**: ⚠️ WARNING
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Safety & Quality
- **Location**: backend/internal/catalog/handlers.go:352 (store.go:343)
- **Detail**: `UnmarkStrategicHandler` maps the "not currently marked strategic" case to 404 by checking `strings.Contains(err.Error(), "no active strategic mark found")` against store.go's error text, instead of a typed/sentinel error. A future wording change in store.go would silently degrade this to a 500 with no compile-time signal. No existing "not-found vs other error" convention elsewhere in this codebase to match — this is a new, fragile coupling introduced by this feature, not a violation of an established pattern.
- **Fix**: Add a sentinel error (e.g. `var ErrStrategicMarkNotFound = errors.New("catalog: no active strategic mark found")`) in store.go, return it (wrapped) from `UnmarkStrategic`, and have the handler check with `errors.Is`.
  - Strength: Removes the string-matching coupling entirely; a future message rewording can't silently break the 404 mapping.
  - Tradeoff: One new exported error value to maintain.
  - Confidence: HIGH — standard Go idiom, no ambiguity in applying it.
  - Blind spot: None significant.
- **Decision**: FIXED — added `catalog.ErrStrategicMarkNotFound` sentinel, store.go wraps it, handlers.go uses `errors.Is`.

### F2 — FK violation on a bad library/category ID falls through to 500

- **Severity**: ⚠️ WARNING
- **Impact**: 🔎 MEDIUM — real tradeoff; pause to reason through it
- **Dimension**: Safety & Quality
- **Location**: backend/internal/catalog/handlers.go:320 (MarkStrategicHandler)
- **Detail**: Marking strategic with a libraryID or categoryID that doesn't exist hits the `strategic_marks` foreign keys and returns a generic 500 rather than 400/404. This path is untested. Low real-world likelihood (IDs come from the UI's own dropdowns), but an API client hitting the endpoint directly gets a misleading "internal server error" for what's really a bad request.
- **Fix A ⭐ Recommended**: Catch the Postgres FK-violation error code (23503) in `MarkStrategic`/Store and return a typed not-found error the handler maps to 400.
  - Strength: One round trip, matches how a DB-level constraint violation should surface as a client error.
  - Tradeoff: Requires inspecting pgx error codes, a style not yet used elsewhere in store.go.
  - Confidence: MEDIUM — mechanism is standard pgx/pgconn usage, but it's a new pattern for this file.
  - Blind spot: Haven't verified no other FK-backed insert in this codebase already does this that could be mirrored instead.
- **Fix B**: Pre-check library/category existence before the upsert.
  - Strength: Simpler to read, no error-code inspection needed.
  - Tradeoff: Extra round trip on every mark; still a TOCTOU race between check and insert (narrow, low-stakes here).
  - Confidence: MEDIUM — straightforward but slightly wasteful.
  - Blind spot: None significant.
- **Decision**: FIXED via Fix A — added `catalog.ErrInvalidReference`, store.go detects pgerrcode.ForeignKeyViolation, handler maps it to 400. Covered by a new test case in TestMarkAndUnmarkStrategicHandler_RoundTrip.

### F3 — Session "ok" ignored in Mark/UnmarkStrategicHandler

- **Severity**: ℹ️ OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Pattern Consistency
- **Location**: backend/internal/catalog/handlers.go:318, 346
- **Detail**: `session, _ := auth.SessionFromContext(...)` discards `ok`, unlike `auth.MeHandler` which explicitly checks it and returns 401. Safe today only because both routes sit behind `RequireRole(auth.RoleArchitect)` in main.go, which guarantees a session is present — but it's a silent landmine (empty `marked_by`) if these handlers are ever reused without that gate.
- **Fix**: Check `ok` explicitly and return 401 if false, matching MeHandler's style.
- **Decision**: FIXED — both Mark/UnmarkStrategicHandler now check `ok` and return 401, matching auth.MeHandler.

## Verification re-run

- `go test ./internal/db/...` — PASS
- `go test ./internal/catalog/... ./cmd/...` — PASS
- `go vet ./...` — PASS
- `go build ./...` — PASS
- `npm run typecheck` — PASS
- `npm run build` — PASS
- Manual verification items (3.3, 4.3, 4.4, 4.5) — previously confirmed live with the user during implementation (curl role-gating + browser mark/unmark/viewer/admin-redirect walkthrough).

## Drift check (Agent 1)

All 11 changed files MATCH their phase's plan contract exactly. No MISSING or EXTRA items. The down-migration's `DROP TABLE IF EXISTS` (vs. plan's plain `DROP TABLE`) mirrors the existing `0002_catalog_schema.down.sql` style and is accepted as intentional style-matching, not drift.
