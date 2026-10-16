# Minimal auth & role scaffold Implementation Plan

## Overview

Add a minimal login mechanism (email + password) and role-gated API access for Shelfmark's three roles (Admin, Architect, Viewer), with a matching minimal frontend login flow — without requiring a database (F-02 is a separate, parallel foundation).

## Current State Analysis

- Backend is a single file, `backend/cmd/shelfmark/main.go`: a `net/http` server exposing `GET /healthz` and a static-file/SPA handler. `go.mod` declares no dependencies.
- Frontend is React Router v8 in SPA mode (`web/react-router.config.ts` has `ssr: false`) with one route (`web/app/routes.ts` → `routes/home.tsx`, a template demo page). No auth-aware code exists.
- No proxy is configured between the Vite dev server (`web/vite.config.ts`) and the Go backend, so `fetch("/api/...")` calls made during `npm run dev` would hit the Vite server, not the Go process.
- No persistence layer exists yet (F-02 is still `ready`, not done) — this plan must not assume a database.

### Key Discoveries:

- SPA mode means there is no server-side loader to gate a route before render — route protection must happen client-side (check auth state, redirect if absent) backed by a server that actually enforces the role check on every API call. The client-side guard is a UX nicety, not the security boundary.
- Same-origin deployment (Go serves the built SPA and will serve `/api/*`) means no CORS handling is needed in production; only the dev-mode Vite proxy needs it.

## Desired End State

An operator can configure three accounts (one per role) via environment variables. A user can log in with email+password at `/login`, receives a signed session cookie, and sees their role reflected in a minimal protected page that calls a role-gated API endpoint. Logging out clears the session. Any backend route wrapped in `RequireRole` returns 401/403 to an unauthenticated/wrong-role caller. This closes out F-01 and unblocks S-01/S-02/S-03's role-gated actions.

Verification: `go build ./...` and `go test ./...` pass in `backend/`; `npm run typecheck` and `npm run build` pass in `web/`; manual login/logout/role-check flow works in a browser against `npm run dev` with the Go backend running locally.

## What We're NOT Doing

- No database-backed accounts — accounts are exactly 3, one per role, from environment variables (F-02 will bring persistence for later slices).
- No self-registration, password reset, or "forgot password" flow.
- No per-user accounts within a role (no multi-admin, multi-architect, etc.) — out of scope per this slice's decisions; revisit if a future slice needs it.
- No CSRF token — same-site cookie (`SameSite=Lax`) plus same-origin API calls are sufficient for this MVP's threat model (no third-party embedding, no cross-site state-changing GET).
- No session revocation list — a stolen/forgotten cookie is bounded by the fixed TTL, not individually revocable short of rotating `SESSION_SECRET` (which invalidates all sessions).

## Implementation Approach

Backend-first: build the credential/session primitives, then the role-gating middleware and an example protected endpoint, then wire a minimal frontend on top. Each phase is independently testable before the next depends on it.

## Phase 1: Backend auth core

### Overview

Load the three role accounts from environment variables, verify passwords with bcrypt, issue and verify a signed stateless session cookie, and expose `POST /api/login` / `POST /api/logout`.

### Changes Required:

#### 1. Account configuration

**File**: `backend/internal/auth/accounts.go`

**Intent**: Load exactly one account per role (Admin, Architect, Viewer) from environment variables at startup, failing fast if any is missing or malformed.

**Contract**: `LoadAccounts() (Accounts, error)` reads `ADMIN_EMAIL`, `ADMIN_PASSWORD_HASH`, `ARCHITECT_EMAIL`, `ARCHITECT_PASSWORD_HASH`, `VIEWER_EMAIL`, `VIEWER_PASSWORD_HASH`. `Accounts` is a lookup by email → `{Role, PasswordHash}`. Role is a small string-based type with three constants: `RoleAdmin`, `RoleArchitect`, `RoleViewer`.

#### 2. Password hashing helper command

**File**: `backend/cmd/hashpw/main.go`

**Intent**: Give the operator a way to turn a plaintext password into the bcrypt hash that goes into `*_PASSWORD_HASH` env vars, since bcrypt hashes aren't hand-writable.

**Contract**: A small CLI: reads a password from stdin (no echo requirement needed for MVP — plain stdin read is fine), prints the bcrypt hash (`bcrypt.GenerateFromPassword`, default cost) to stdout. `go run ./cmd/hashpw` with no args prompts "Password: " and reads a line.

#### 3. Dependency addition

**File**: `backend/go.mod`, `backend/go.sum`

**Intent**: Add `golang.org/x/crypto` for `bcrypt`.

**Contract**: `go get golang.org/x/crypto` run from `backend/`, pinning the latest compatible version.

#### 4. Session cookie issuance & verification

**File**: `backend/internal/auth/session.go`

**Intent**: Issue a signed, stateless session token encoding the user's email, role, and expiry; verify and decode it on incoming requests.

**Contract**: `IssueSession(w http.ResponseWriter, email string, role Role)` sets a cookie named `shelfmark_session` with `HttpOnly`, `SameSite=Lax`, `Secure` when the request is HTTPS, `Max-Age` of 24h. `VerifySession(r *http.Request) (Session, error)` reads the cookie, checks the HMAC signature (`crypto/hmac` + `crypto/sha256`) against `SESSION_SECRET`, checks expiry, and returns `{Email, Role}` or an error. `ClearSession(w http.ResponseWriter)` overwrites the cookie with `Max-Age=-1`.

The signed payload is `base64url(email|role|expiryUnix) + "." + base64url(hmac)`, verified with `hmac.Equal` to avoid timing leaks. `SESSION_SECRET` is a required env var (server fails to start if absent — a missing signing key is a deployment error, not a runtime fallback).

#### 5. Login & logout HTTP handlers

**File**: `backend/internal/auth/handlers.go`

**Intent**: Handle the login form submission (verify credentials, issue session) and logout (clear session).

**Contract**: `LoginHandler(accounts Accounts) http.HandlerFunc` — `POST /api/login` accepts JSON `{"email": "...", "password": "..."}`; looks up the account by email, verifies with `bcrypt.CompareHashAndPassword`, calls `IssueSession` on success, returns `{"email": ..., "role": ...}` as JSON; returns 401 with a generic "invalid credentials" message on any failure (unknown email or wrong password — do not distinguish the two in the response). `LogoutHandler() http.HandlerFunc` — `POST /api/logout` calls `ClearSession`, returns 204.

#### 6. Wire into main

**File**: `backend/cmd/shelfmark/main.go`

**Intent**: Load accounts at startup (exit with a clear error if misconfigured), register the new routes.

**Contract**: Add `mux.HandleFunc("POST /api/login", auth.LoginHandler(accounts))` and `mux.HandleFunc("POST /api/logout", auth.LogoutHandler())` before the existing `GET /` SPA handler registration (so `/api/*` is matched first — Go 1.22+ `ServeMux` already prioritizes the more specific pattern, but the registration order is kept clear for readability).

### Success Criteria:

#### Automated Verification:

- `go build ./...` succeeds in `backend/`
- `go vet ./...` succeeds in `backend/`
- Unit tests pass: `go test ./backend/internal/auth/...` — covering: correct credentials issue a valid session; wrong password is rejected; unknown email is rejected; a tampered cookie signature is rejected; an expired session is rejected; `IssueSession` → `VerifySession` round-trips the same email/role.

#### Manual Verification:

- `curl -i -X POST localhost:8080/api/login -d '{"email":"...","password":"..."}' -H 'Content-Type: application/json'` returns `Set-Cookie: shelfmark_session=...` and a 200 with the account's email/role.
- Re-using that cookie on a follow-up request is accepted by `VerifySession` (exercised in Phase 2's `/api/me`).
- `POST /api/logout` with the cookie clears it (subsequent request without a fresh login is unauthenticated).

---

## Phase 2: Backend role gating

### Overview

Add a reusable `RequireRole` wrapper and one example protected endpoint (`GET /api/me`) that demonstrates the pattern later slices (S-01, S-02, S-03) will reuse for their own admin-only / architect-only routes.

### Changes Required:

#### 1. Role-gating middleware

**File**: `backend/internal/auth/middleware.go`

**Intent**: Wrap an `http.HandlerFunc` so it only runs when the caller's session has one of the allowed roles; otherwise respond with 401 (no/invalid session) or 403 (valid session, wrong role).

**Contract**: `RequireRole(allowed ...Role) func(http.HandlerFunc) http.HandlerFunc` — a decorator: `RequireRole(RoleAdmin)(someHandler)`. On success, it attaches the verified `Session` to the request context (`context.WithValue` with an unexported key type) so the wrapped handler can read `SessionFromContext(r.Context())` without re-verifying.

#### 2. Example protected endpoint

**File**: `backend/internal/auth/handlers.go`

**Intent**: Prove the middleware works end-to-end and give the frontend something to call right after login to learn the current user's role.

**Contract**: `MeHandler() http.HandlerFunc` — `GET /api/me`, wrapped as `RequireRole(RoleAdmin, RoleArchitect, RoleViewer)(MeHandler())` (i.e. any authenticated role), returns `{"email": ..., "role": ...}` from the context session as JSON.

#### 3. Wire into main

**File**: `backend/cmd/shelfmark/main.go`

**Intent**: Register the gated route.

**Contract**: `mux.HandleFunc("GET /api/me", auth.RequireRole(auth.RoleAdmin, auth.RoleArchitect, auth.RoleViewer)(auth.MeHandler()))`.

### Success Criteria:

#### Automated Verification:

- `go build ./...` succeeds in `backend/`
- Unit tests pass: `go test ./backend/internal/auth/...` — covering: `RequireRole` passes through for an allowed role; returns 401 with no cookie; returns 401 with an invalid/expired cookie; returns 403 for a valid session whose role isn't in the allowed set.

#### Manual Verification:

- `curl` with a valid Admin session cookie against `GET /api/me` returns 200 with that admin's email/role.
- `curl` with no cookie against `GET /api/me` returns 401.

---

## Phase 3: Frontend auth

### Overview

Add a login page, a small auth client, a client-side route guard, and a minimal protected page that calls `/api/me` — establishing the pattern future slices' admin/architect-only views will reuse.

### Changes Required:

#### 1. Dev proxy to the Go backend

**File**: `web/vite.config.ts`

**Intent**: So `fetch("/api/...")` calls work identically under `npm run dev` (Vite on its own port) and the production build (served by the Go binary on one origin).

**Contract**: Add `server: { proxy: { "/api": "http://localhost:8080" } }` to the `defineConfig` call.

#### 2. Auth client

**File**: `web/app/lib/auth.ts`

**Intent**: Centralize the three API calls (`login`, `logout`, `me`) and the client-side auth state so both the login page and the route guard share one source of truth.

**Contract**: `login(email: string, password: string): Promise<Me>` (`POST /api/login`, throws on non-2xx), `logout(): Promise<void>` (`POST /api/logout`), `me(): Promise<Me | null>` (`GET /api/me`, returns `null` on 401 instead of throwing). `Me = { email: string; role: "admin" | "architect" | "viewer" }`. An `AuthProvider` React context component calls `me()` once on mount, exposes `{ me, login, logout, loading }` via a `useAuth()` hook.

#### 3. Login page

**File**: `web/app/routes/login.tsx`

**Intent**: A form for email + password that calls `useAuth().login` and redirects to `/dashboard` on success, showing the generic error message on failure.

**Contract**: Registered as a new route in `web/app/routes.ts`: `route("login", "routes/login.tsx")`.

#### 4. Route guard + protected example page

**File**: `web/app/lib/require-auth.tsx`, `web/app/routes/dashboard.tsx`

**Intent**: A reusable guard component that redirects to `/login` when `useAuth().me` is absent after loading, and one example protected page that renders the logged-in user's email/role plus a logout button — the page S-01/S-02/S-03 will model their own protected views on.

**Contract**: `RequireAuth({ children }: { children: React.ReactNode })` reads `useAuth()`; while `loading`, renders nothing/a spinner; once loaded, renders `children` if `me` is set, otherwise redirects (`useNavigate` to `/login`). `dashboard.tsx` wraps its content in `<RequireAuth>`. Registered as `route("dashboard", "routes/dashboard.tsx")` in `routes.ts`.

#### 5. Wire the provider

**File**: `web/app/root.tsx`

**Intent**: Make `useAuth()` available to every route.

**Contract**: Wrap the `<Outlet />` in `App()` with `<AuthProvider>`.

### Success Criteria:

#### Automated Verification:

- `npm run typecheck` succeeds in `web/`
- `npm run build` succeeds in `web/`

#### Manual Verification:

- With the Go backend running locally and `npm run dev` in `web/`: visiting `/dashboard` while logged out redirects to `/login`.
- Logging in with each of the three role accounts at `/login` redirects to `/dashboard` and shows the correct email/role.
- Clicking logout on `/dashboard` returns the user to a logged-out state (next visit to `/dashboard` redirects to `/login` again).
- An invalid login attempt shows an error message and does not navigate away from `/login`.

---

## Testing Strategy

### Unit Tests:

- Password verification: correct password accepted, wrong password rejected, unknown email rejected.
- Session roundtrip: issue → verify returns the same email/role; tampered signature rejected; expired token rejected.
- `RequireRole`: allowed role passes, missing session → 401, wrong role → 403.

### Integration Tests:

- None added in this phase — `go test` unit coverage plus the manual curl/browser flows above are sufficient at this scale; S-01 onward can add HTTP-level integration tests once there's a persistence layer worth testing against.

### Manual Testing Steps:

1. Generate three bcrypt hashes with `go run ./backend/cmd/hashpw` and export all six `*_EMAIL`/`*_PASSWORD_HASH` vars plus `SESSION_SECRET` locally.
2. Run the Go backend (`go run ./backend/cmd/shelfmark`) and the frontend dev server (`npm run dev` in `web/`).
3. Log in as each role at `/login`, confirm `/dashboard` shows the right email/role, confirm logout works.
4. Confirm `curl` against `/api/me` without a cookie returns 401, and with a valid cookie returns the account's data.

## Performance Considerations

None — three accounts, one bcrypt comparison per login, one HMAC verify per request. No scale concerns at this stage.

## Migration Notes

Not applicable — no existing accounts or sessions to migrate from.

## References

- Roadmap item: `context/foundation/roadmap.md` § F-01
- PRD: `context/foundation/prd.md` § Access Control
- Current backend entrypoint: `backend/cmd/shelfmark/main.go`
- Current frontend routing: `web/app/routes.ts`, `web/app/root.tsx`
- Dev server config: `web/vite.config.ts`

## Progress

> Convention: `- [ ]` pending, `- [x]` done. Append ` — <commit sha>` when a step lands. Do not rename step titles. See `references/progress-format.md`.

### Phase 1: Backend auth core

#### Automated

- [x] 1.1 go build ./... succeeds in backend/ — d5d3636
- [x] 1.2 go vet ./... succeeds in backend/ — d5d3636
- [x] 1.3 Unit tests pass: go test ./backend/internal/auth/... — d5d3636

#### Manual

- [x] 1.4 POST /api/login with valid credentials returns Set-Cookie and 200 with email/role — d5d3636
- [x] 1.5 Reusing the session cookie is accepted on a follow-up request — d5d3636
- [x] 1.6 POST /api/logout clears the session — d5d3636

### Phase 2: Backend role gating

#### Automated

- [x] 2.1 go build ./... succeeds in backend/ — cd6dae2
- [x] 2.2 Unit tests pass: go test ./backend/internal/auth/... — cd6dae2

#### Manual

- [x] 2.3 GET /api/me with a valid Admin session returns 200 with email/role — cd6dae2
- [x] 2.4 GET /api/me with no cookie returns 401 — cd6dae2

### Phase 3: Frontend auth

#### Automated

- [x] 3.1 npm run typecheck succeeds in web/
- [x] 3.2 npm run build succeeds in web/

#### Manual

- [x] 3.3 Visiting /dashboard while logged out redirects to /login
- [x] 3.4 Logging in with each of the three role accounts shows the correct email/role on /dashboard
- [x] 3.5 Logout returns the user to a logged-out state
- [x] 3.6 An invalid login attempt shows an error and stays on /login
