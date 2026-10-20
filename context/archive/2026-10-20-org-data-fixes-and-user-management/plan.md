# Org data fixes and user management — Implementation Plan

## Overview

Fix two data-correctness gaps found in a Milestone 1 completeness audit (FR-006's "Direct" column is a dead stub; FR-007's "up to two strategic libraries" cap is unenforced), and add a simple admin page for creating users — migrating login accounts from `.env` variables to a `users` database table so new accounts take effect without a restart.

## Current State Analysis

- **FR-006**: `libraries.is_direct` (added by migration `0003_strategic_marks.up.sql`) defaults to `true` and is never updated after insert. `Store.ListLibraries` (`backend/internal/catalog/store.go:148`) selects this stale column directly; `/organization`'s "Direct" column (`web/app/routes/organization.tsx:217`) always reads "Yes". The *correct* direct/transitive signal already exists per-project in `project_libraries.is_direct`, written correctly by `ReplaceProjectLibraries` (`project.go:105-135`) from the real SBOM dependency graph (`project_sbom.go`).
- **FR-007**: `strategic_marks` has a unique constraint on `(library_id, category_id)` but nothing counts how many *distinct libraries* hold an active mark for a given `(category_id, ecosystem)` pair. `MarkStrategicHandler` (`handlers.go:308-336`) and `Store.MarkStrategic` (`store.go:324-341`) accept unlimited marks.
- **Auth model**: `auth.LoadAccounts()` (`backend/internal/auth/accounts.go`) reads exactly 3 accounts from `ADMIN_EMAIL`/`ADMIN_PASSWORD_HASH` (+ Architect, Viewer) **once, at startup**, into a static `map[string]Account`. `LoginHandler(accounts)` (`handlers.go:32-60`) closes over that map — there is no reload path, so a 4th account could never be added without a code change and restart. Sessions themselves are stateless signed cookies (`session.go`) with no per-request DB check, independent of how accounts are stored.
- `backend/internal/db/migrate.go` runs embedded `golang-migrate` migrations (`backend/internal/db/migrations/0001`–`0004`) in numeric order; new migrations follow the same `uuid PRIMARY KEY DEFAULT gen_random_uuid()` convention (`0002_catalog_schema.up.sql:1-2`).
- `LibrariesByPurls` (`project.go:71-100`) also selects `libraries.is_direct`, but its only caller (`project_handlers.go:93-95`) never reads the returned field — it uses its own SBOM-derived `isDirect` map instead. This query must be updated too once the column is dropped, even though nothing reads the value.

## Desired End State

- `/organization`'s "Direct" column reflects whether a library has ever appeared as a direct dependency in any uploaded project — computed live, never stale.
- Marking a 3rd distinct library as strategic in the same `(category, ecosystem)` is rejected with a clear error; unmarking or re-marking an already-marked library still works.
- An admin can open `/users`, add a new user (email, password, role) through a plain form, and log in as that user immediately — no restart, no `.env` edit, no `hashpw` CLI step.
- A fresh deployment (empty `users` table) still bootstraps its 3 accounts from `ADMIN_EMAIL`/`ARCHITECT_EMAIL`/`VIEWER_EMAIL` + their password hashes, exactly once; after that, `.env` account variables are no longer required to start the app.

**Verification**: `cd web && npm run typecheck && npm run build` succeed; `cd backend && go build ./... && go vet ./...` succeed; hardcoded-value grep scan returns 0 on new/changed frontend files; manual pass covering bootstrap-from-empty-DB, add-user-then-login-without-restart, the FR-007 cap, and the FR-006 Direct column.

### Key Discoveries:

- `project_libraries.is_direct` is already the source of truth per project — FR-006's fix is a read-side query change, not a new write path.
- `strategic_marks` has no `ecosystem` column of its own; ecosystem must be read via a join to `libraries.ecosystem` for the FR-007 cap check.
- `golang-migrate`'s embedded FS (`db.Migrate`) means adding a migration is just two new files — no registration step elsewhere.
- `auth.LoadAccounts()` can be reused unchanged as the one-time seed source; it already does exactly the fail-fast parsing a bootstrap step needs.

## What We're NOT Doing

- Not adding edit/delete/reset-password for existing users — only **adding** a new user, per explicit scope.
- Not revoking or invalidating existing session cookies when a user is added (not applicable — adding a user doesn't affect anyone else's session) or when a role changes (out of scope — no edit feature exists yet to trigger it).
- Not adding email-format validation on the new user form (explicit instruction) or a "confirm password" field.
- Not changing `RequireAuth` role rules on any existing route.
- Not adding rate limiting or lockout on repeated failed logins — unrelated to this change.
- Not touching the `hashpw` CLI command — it stays as a standalone dev utility; the new form hashes passwords itself.

## Implementation Approach

Fix the two read-path data bugs first (Phase 1, fully isolated, no schema dependency on the auth work). Then build the users table bottom-up: schema + store + bootstrap (Phase 2a), then the HTTP handlers and wiring that replace the static accounts map (Phase 2b) — kept as one phase since login breaks if split across a commit boundary. Then the frontend page (Phase 3). Close with cross-cutting verification (Phase 4).

## Critical Implementation Details

- **Startup ordering changes.** `main.go` currently calls `auth.LoadAccounts()` *before* `db.Connect`/`db.Migrate`. The new bootstrap (`UsersStore.SeedFromEnvIfEmpty`) needs the pool and the `users` table to exist first, so the accounts step moves to *after* migration — and it no longer fails the whole process just because env vars are absent (it only needs them if the table is empty).
- **Transaction required for the FR-007 cap.** The cap check (count) and the upsert must run in the same transaction `Store.MarkStrategic` opens, otherwise two concurrent marks could both pass the count check and together exceed 2.

## Phase 1: Fix FR-006 (dead Direct column) and FR-007 (unenforced strategic cap)

### Overview

Two independent read/write-path fixes in the existing `catalog` package; no new tables.

### Changes Required:

#### 1. Drop the stale `libraries.is_direct` column

**File**: `backend/internal/db/migrations/0005_drop_libraries_is_direct.up.sql` (new)

**Intent**: Remove the column nothing maintains, so it can't silently drift again.

**Contract**: `ALTER TABLE libraries DROP COLUMN is_direct;`

**File**: `backend/internal/db/migrations/0005_drop_libraries_is_direct.down.sql` (new)

**Contract**: `ALTER TABLE libraries ADD COLUMN is_direct boolean NOT NULL DEFAULT true;` (mirrors `0003`'s down, restores the prior shape on rollback).

#### 2. Compute `is_direct` live in `ListLibraries`

**File**: `backend/internal/catalog/store.go`

**Intent**: `/organization` (and anywhere else `ListLibraries` feeds) should show whether a library has ever been uploaded as a direct dependency in any project, not a frozen default.

**Contract**: In the `SELECT` at `store.go:148`, replace the `l.is_direct` column reference with `EXISTS (SELECT 1 FROM project_libraries pl WHERE pl.library_id = l.id AND pl.is_direct) AS is_direct`. The `Scan` call and the `Library` struct/JSON shape are unchanged — only the SQL expression behind `is_direct` changes.

#### 3. Stop selecting the dropped column in `LibrariesByPurls`

**File**: `backend/internal/catalog/project.go`

**Intent**: This query's `is_direct` result is never read by its caller (`project_handlers.go:93-95`, which uses its own SBOM-derived map) — once the column is dropped, this query must stop selecting it or it won't compile against the schema.

**Contract**: Remove `is_direct` from the `SELECT` list at `project.go:77` and from the corresponding `Scan` call at `project.go:90`; `Library.IsDirect` is simply left at its zero value (`false`) for results returned by this specific function, which is correct since nothing reads it.

#### 4. Enforce the 2-strategic-per-category/ecosystem cap

**File**: `backend/internal/catalog/store.go`

**Intent**: FR-007 says "up to two" strategic libraries are allowed per category/ecosystem; a 3rd distinct library must be rejected, while re-marking an already-marked library (upsert) must keep working.

**Contract**: Wrap `MarkStrategic`'s single `Exec` in a transaction. Before the upsert, run `SELECT ecosystem FROM libraries WHERE id = $1` for `libraryID` (not-found → `ErrInvalidReference`, matching the existing foreign-key-violation mapping), then `SELECT COUNT(DISTINCT sm.library_id) FROM strategic_marks sm JOIN libraries l ON l.id = sm.library_id WHERE sm.active AND sm.category_id = $1 AND l.ecosystem = $2 AND sm.library_id != $3` (excluding the library being marked, so re-marking/updating its own justification never trips the cap). If the count is `>= 2`, return a new `ErrStrategicCapReached` sentinel without running the upsert; otherwise proceed with the existing `INSERT ... ON CONFLICT` and commit.

#### 5. Surface the cap error as a 409

**File**: `backend/internal/catalog/handlers.go`

**Intent**: `MarkStrategicHandler` needs to translate the new sentinel into an HTTP response the frontend can show.

**Contract**: In the `errors.Is` chain at `handlers.go:326-330`, add a branch for `errors.Is(err, ErrStrategicCapReached)` → `writeCatalogError(w, http.StatusConflict, "already 2 strategic libraries marked for this category and ecosystem")`.

#### 6. Test coverage for the new cap

**File**: `backend/internal/catalog/store_test.go`

**Intent**: `MarkStrategic` already has tests (including upsert-on-repeat), but none exercise the new cap — a regression here would only be caught by manual testing otherwise.

**Contract**: Add a test case that marks 2 distinct libraries strategic in the same category+ecosystem (both succeed), then attempts a 3rd distinct library and asserts `errors.Is(err, catalog.ErrStrategicCapReached)`. Add a second assertion that re-marking (upserting) one of the original two still succeeds despite the cap.

### Success Criteria:

#### Automated Verification:

- Backend builds: `cd backend && go build ./...`
- Backend vets clean: `cd backend && go vet ./...`
- Backend tests pass: `cd backend && go test ./...`

#### Manual Verification:

- After uploading a project SBOM, the library's row on `/organization` shows "Yes" in Direct if and only if it was direct in at least one uploaded project.
- Marking a 3rd distinct library strategic in the same category+ecosystem is rejected with a visible error; a 2nd mark still succeeds; re-marking (editing justification on) one of the existing two still succeeds.

---

## Phase 2: Users table, bootstrap seeding, and admin CRUD API

### Overview

Replace the static `.env`-backed accounts map with a `users` database table, seeded once from the existing env vars, with new endpoints for an admin to list and create users.

### Changes Required:

#### 1. `users` table

**File**: `backend/internal/db/migrations/0006_users.up.sql` (new)

**Intent**: Persistent, growable account storage, replacing the fixed 3-account env-var map.

**Contract**: `CREATE TABLE users (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), email text NOT NULL UNIQUE, password_hash text NOT NULL, role text NOT NULL CHECK (role IN ('admin', 'architect', 'viewer')), created_at timestamptz NOT NULL DEFAULT now());`

**File**: `backend/internal/db/migrations/0006_users.down.sql` (new)

**Contract**: `DROP TABLE users;`

#### 2. Users store

**File**: `backend/internal/auth/users_store.go` (new)

**Intent**: Give `auth` a persistence layer for accounts, parallel to `catalog.Store` but scoped to this package.

**Contract**: `NewUsersStore(pool *pgxpool.Pool) *UsersStore`. Methods: `GetByEmail(ctx, email string) (Account, bool, error)` (returns `found=false` on no rows, no error); `Create(ctx, email, passwordHash string, role Role) error` (unique-violation on `email` → `ErrEmailTaken` sentinel, following `store.go`'s `pgerrcode` mapping convention); `List(ctx) ([]UserSummary, error)` ordered by `email`, where `UserSummary{Email string; Role Role}` (no password field — never selected); `Count(ctx) (int, error)`.

#### 3. One-time env-var seeding

**File**: `backend/internal/auth/users_store.go`

**Intent**: A fresh database bootstraps its 3 accounts from the existing env vars exactly once; every later boot skips this entirely, so the env vars become optional once the table has rows.

**Contract**: `(s *UsersStore) SeedFromEnvIfEmpty(ctx context.Context) error` — calls `Count`; if `> 0`, return `nil` immediately. If `0`, call the existing `LoadAccounts()` (unchanged) and `Create` each of its 3 accounts; propagate any `LoadAccounts` error (missing env var) as a fatal startup error, since an empty table with no seed source means nobody could ever log in.

#### 4. Login reads the database instead of a static map

**File**: `backend/internal/auth/handlers.go`

**Intent**: `LoginHandler` must see newly-created users without a restart.

**Contract**: Change `LoginHandler`'s parameter from `accounts Accounts` to `store *UsersStore`; replace the map lookup at `handlers.go:40` with `account, found, err := store.GetByEmail(r.Context(), req.Email)` (treat `err != nil` as a 500, `!found` the same as the existing "not ok" branch → `writeInvalidCredentials`). `Accounts`/`LoadAccounts` stay as-is (still used by `SeedFromEnvIfEmpty`), but are no longer called directly from `main.go`.

#### 5. Create/list user endpoints

**File**: `backend/internal/auth/handlers.go`

**Intent**: Give the admin page something to call.

**Contract**: `CreateUserHandler(store *UsersStore) http.HandlerFunc` for `POST /api/users` — decodes `{email, password, role}`; rejects (400) if `email` or `password` is empty, `len(password) < 8`, or `role` is not one of `admin`/`architect`/`viewer`; hashes with `bcrypt.GenerateFromPassword` (same cost as `cmd/hashpw`); calls `store.Create`; maps `ErrEmailTaken` → 409; on success, `204 No Content`. `ListUsersHandler(store *UsersStore) http.HandlerFunc` for `GET /api/users` — returns `store.List` as a JSON array of `{email, role}`.

#### 6. Wire it all up, in the right order

**File**: `backend/cmd/shelfmark/main.go`

**Intent**: Connect to the database and migrate *before* touching accounts, since seeding now depends on both.

**Contract**: Remove the early `auth.LoadAccounts()` call (and its `log.Fatalf`) from before `db.Connect`. After `db.Migrate` succeeds, add `usersStore := auth.NewUsersStore(pool)` then `if err := usersStore.SeedFromEnvIfEmpty(context.Background()); err != nil { pool.Close(); log.Fatalf("auth: failed to seed accounts: %v", err) }`. Change `auth.LoginHandler(accounts)` to `auth.LoginHandler(usersStore)`. Add two routes: `mux.HandleFunc("POST /api/users", auth.RequireRole(auth.RoleAdmin)(auth.CreateUserHandler(usersStore)))` and `mux.HandleFunc("GET /api/users", auth.RequireRole(auth.RoleAdmin)(auth.ListUsersHandler(usersStore)))`.

#### 7. Note the architecture change in the PRD

**File**: `context/foundation/prd.md`

**Intent**: The Access Control section currently says accounts are "provisioned via instance configuration" — that's no longer accurate once this ships.

**Contract**: Append one sentence to the `## Access Control` section noting that accounts are now managed through an in-app admin page backed by a database table, with the original `.env` variables retained only as the one-time bootstrap source for a fresh deployment.

#### 8. Update the existing login test fixtures

**File**: `backend/internal/auth/handlers_test.go`

**Intent**: This file calls `LoginHandler(accounts)` with a static `Accounts` map at 3 call sites — it won't compile once `LoginHandler` takes `*UsersStore` instead.

**Contract**: At each of the 3 call sites, replace the `Accounts`-map fixture with a `*UsersStore` backed by the test database connection already used elsewhere in this package's tests (mirror the `*Store`-against-a-test-database setup pattern used in `catalog/store_test.go`), pre-populated via `Create` with the same test accounts the old fixture declared.

### Success Criteria:

#### Automated Verification:

- Backend builds: `cd backend && go build ./...`
- Backend vets clean: `cd backend && go vet ./...`
- Backend tests pass: `cd backend && go test ./...`

#### Manual Verification:

- Starting the stack against a freshly-migrated, empty database still logs in successfully as the 3 `.env`-configured accounts (bootstrap ran).
- Restarting the stack a second time (table now non-empty) succeeds even with the 3 env vars unset.
- `POST /api/users` as admin with a duplicate email returns 409; with a 7-character password returns 400.

**Implementation Note**: Pause here for manual confirmation before proceeding to Phase 3 — this phase changes how login works for every role.

---

## Phase 3: Admin "Users" page

### Overview

A simple, admin-only page to add a user and see the existing list.

### Changes Required:

#### 1. API client

**File**: `web/app/lib/users.ts` (new)

**Intent**: Follow the existing `lib/organization.ts` client pattern (typed fetch wrappers, shared `errorMessage` helper shape).

**Contract**: `export type User = { email: string; role: "admin" | "architect" | "viewer" }`; `listUsers(): Promise<User[]>` (`GET /api/users`); `createUser(email: string, password: string, role: User["role"]): Promise<void>` (`POST /api/users`, JSON body `{email, password, role}`), both throwing `Error(await errorMessage(res, "..."))` on a non-ok response.

#### 2. Route

**File**: `web/app/routes/users.tsx` (new)

**Intent**: Add-user form (email, password, role select) above a read-only table of existing users, following `catalog.tsx`'s `RequireAuth role="admin"` + `<Nav />` + token-only styling convention.

**Contract**: Wrapped in `RequireAuth role="admin"`. On submit: call `createUser`, clear the form, re-fetch `listUsers()` into local state, show `text-destructive` on error (same pattern as `catalog.tsx`'s upload error). List renders as a `<table>` with `Email` / `Role` columns, matching the existing table styling in `catalog.tsx`/`organization.tsx`.

#### 3. Register the route

**File**: `web/app/routes.ts`

**Intent**: Make `/users` reachable.

**Contract**: Add `route("users", "routes/users.tsx")` to the exported array.

#### 4. Nav entry

**File**: `web/app/components/nav.tsx`

**Intent**: Admins need a way to reach the new page without typing the URL.

**Contract**: Add `{ to: "/users", label: "Users", roles: ["admin"] }` to the `LINKS` array (position after `"Catalog"`, since both are admin-only).

### Success Criteria:

#### Automated Verification:

- Type checking passes: `cd web && npm run typecheck`
- Build succeeds: `cd web && npm run build`
- Hardcoded-value grep scan returns 0 on `users.tsx`, `nav.tsx`, `routes.ts`: `grep -ncE '#[0-9a-fA-F]{3,8}\b|rgba?\(|\b(bg|text|border|ring|outline|from|via|to|fill|stroke|shadow|divide)-(slate|gray|zinc|neutral|stone|red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose|white|black)\b' web/app/routes/users.tsx web/app/components/nav.tsx web/app/routes.ts`

#### Manual Verification:

- As admin, `/users` shows the "Users" link in nav; adding a new user (email/password/role) succeeds and the user appears in the list immediately.
- Logging out and logging back in as the newly-created user (no restart in between) succeeds, and the nav shows links matching that user's role.
- A non-admin role never sees the "Users" link, and hitting `/users` directly as a non-admin redirects away (existing `RequireAuth` behavior).

---

## Phase 4: End-to-end verification

### Overview

Prove the whole change works together, not just phase-by-phase.

### Changes Required:

No file changes — this phase is verification only.

### Success Criteria:

#### Automated Verification:

- Full backend build + vet + tests: `cd backend && go build ./... && go vet ./... && go test ./...`
- Full frontend typecheck + build: `cd web && npm run typecheck && npm run build`

#### Manual Verification:

- Reset the database volume, start the stack fresh: bootstrap seeds the 3 `.env` accounts, all 3 can log in.
- As admin, add a 4th user (architect role) via `/users`; log in as that user immediately, confirm nav + role-gated pages match an architect's access.
- As that architect, upload a project SBOM, then as architect or viewer confirm `/organization`'s Direct column for libraries in that project reads correctly (Yes for direct, No for transitive-only, assuming the SBOM carries the distinction).
- Attempt to mark a 3rd distinct library strategic in a category/ecosystem that already has 2 — confirm rejection with a visible error.

## Testing Strategy

### Manual Testing Steps:

1. `docker compose down -v && docker compose up -d db` (fresh, empty database), start the backend — confirm bootstrap seeding logs/succeeds and all 3 `.env` accounts can log in.
2. Restart the backend again with the 3 account env vars unset — confirm it still starts and those accounts still work (seed already persisted).
3. As admin, open `/users`, add a new architect account, log in as them in a different browser/session without restarting anything.
4. As that architect, mark two different libraries strategic in the same category+ecosystem (should succeed), then attempt a third (should be rejected with a clear message).
5. Upload a project SBOM containing both direct and transitive libraries; confirm `/organization`'s Direct column matches reality for those libraries.

## Performance Considerations

None beyond what's already in place — `users` and `strategic_marks` are tiny tables at this project's target scale (PRD: small/low qps); the new `EXISTS` subquery in `ListLibraries` and the cap-check query in `MarkStrategic` both run against indexed foreign keys.

## Migration Notes

Migration `0005` is destructive (drops a column) but trivially reversible via its `.down.sql` since the column's data was never meaningful. Migration `0006` is purely additive. The `.env`-to-DB account seed is one-way: once `SeedFromEnvIfEmpty` has run against a non-empty table, there is no code path that re-reads `.env` accounts — recovering from a bad seed means fixing the `users` row directly (e.g. via `psql`) rather than re-running a migration.

## References

- Related research: Milestone 1 completeness audit (this conversation; no `research.md` file written for this change).
- Prior art for the API client / route pattern: `web/app/lib/organization.ts`, `web/app/routes/catalog.tsx`.

## Progress

> Convention: `- [ ]` pending, `- [x]` done. Append ` — <commit sha>` when a step lands. Do not rename step titles. See `references/progress-format.md`.

### Phase 1: Fix FR-006 (dead Direct column) and FR-007 (unenforced strategic cap)

#### Automated

- [x] 1.1 Backend builds: `cd backend && go build ./...` — 74daf0f
- [x] 1.2 Backend vets clean: `cd backend && go vet ./...` — 74daf0f
- [x] 1.3 Backend tests pass: `cd backend && go test ./...` — 74daf0f

#### Manual

- [x] 1.4 `/organization` Direct column reflects real project-upload data — 74daf0f
- [x] 1.5 3rd distinct strategic mark in the same category+ecosystem is rejected; 2nd mark and re-marking still succeed — 74daf0f

### Phase 2: Users table, bootstrap seeding, and admin CRUD API

#### Automated

- [x] 2.1 Backend builds: `cd backend && go build ./...` — 65137a8
- [x] 2.2 Backend vets clean: `cd backend && go vet ./...` — 65137a8
- [x] 2.3 Backend tests pass: `cd backend && go test ./...` — 65137a8

#### Manual

- [x] 2.4 Fresh empty database bootstraps 3 `.env` accounts; all 3 can log in — 65137a8
- [x] 2.5 Second restart with env vars unset still works (seed already persisted) — 65137a8
- [x] 2.6 `POST /api/users` rejects duplicate email (409) and short password (400) — 65137a8

### Phase 3: Admin "Users" page

#### Automated

- [x] 3.1 Type checking passes: `cd web && npm run typecheck` — 63a24e8
- [x] 3.2 Build succeeds: `cd web && npm run build` — 63a24e8
- [x] 3.3 Hardcoded-value grep scan returns 0 on `users.tsx`, `nav.tsx`, `routes.ts` — 63a24e8

#### Manual

- [x] 3.4 Admin sees "Users" nav link, adds a user, sees it in the list immediately — 63a24e8
- [x] 3.5 New user logs in immediately (no restart), nav matches their role — 63a24e8
- [x] 3.6 Non-admin never sees the "Users" link and can't reach `/users` — 63a24e8

### Phase 4: End-to-end verification

#### Automated

- [x] 4.1 Full backend build + vet + tests — 661ebc5
- [x] 4.2 Full frontend typecheck + build — 661ebc5

#### Manual

- [x] 4.3 Fresh-DB bootstrap, all 3 seeded accounts log in — 661ebc5
- [x] 4.4 Admin-added 4th user logs in immediately with correct role-gated access — 661ebc5
- [x] 4.5 Direct column correct after a real project SBOM upload — 661ebc5
- [x] 4.6 3rd strategic mark in a full category/ecosystem is rejected — 661ebc5
