# Minimal auth & role scaffold — Plan Brief

> Full plan: `context/changes/minimal-auth-role-scaffold/plan.md`

## What & Why

Shelfmark needs a minimal login mechanism and role-gated access before any other slice can restrict actions to Admin/Architect/Viewer. This is roadmap foundation F-01: every downstream slice (seeding the catalog, marking strategic libraries, the redundancy report) gates at least one action by role, so the gate is built once here rather than retrofitted into three working slices later.

## Starting Point

Today the backend is a single `net/http` file serving `/healthz` and the built SPA; the frontend is a React Router v8 SPA (no server-side loaders) with one template demo route. There is no login, no session, no role concept, and no database (F-02, the data layer, is a separate parallel foundation — this plan doesn't depend on it).

## Desired End State

An operator sets six environment variables (email + bcrypt hash per role) and a signing secret. A user logs in at `/login`, lands on a `/dashboard` that shows their email and role via a role-gated API call, and can log out. Any backend route wrapped in the new `RequireRole` helper enforces the check server-side — the real security boundary — while the frontend guard just redirects unauthenticated visitors for UX.

## Key Decisions Made

| Decision | Choice | Why (1 sentence) |
| --- | --- | --- |
| Account provisioning | Environment variables, one account per role (3 total) | Matches the Fly/Docker deploy target and the PRD's literal "three roles" framing — no config file to mount. |
| Password hashing | bcrypt via `golang.org/x/crypto` | The one widely-trusted non-stdlib Go crypto package — avoids homemade password-hashing crypto. |
| Session mechanism | Signed stateless cookie (HMAC, `crypto/hmac`) | No persistence layer exists yet (F-02 isn't built); a stateless session needs no server-side store. |
| Role-gating pattern | `RequireRole(role...)(handler)` decorator | Composable per-route, idiomatic for Go 1.22+ `net/http.ServeMux`, easy for S-01/S-02/S-03 to reuse. |
| Frontend scope | Build login page + route guard now | F-01 explicitly unlocks role-gated views for every later slice — building the guard once avoids each slice reinventing it. |
| Session expiry | Fixed 24h TTL + explicit `/api/logout` | Bounds how long a stolen/forgotten cookie stays valid and gives the frontend a real logout action. |

## Scope

**In scope:**
- Backend: env-var account loading, bcrypt verification, signed session cookie, login/logout endpoints, `RequireRole` middleware, one example protected endpoint (`/api/me`).
- Frontend: login page, auth client/context, route guard, one protected example page (`/dashboard`), dev proxy to the Go backend.

**Out of scope:**
- Database-backed or multi-account-per-role accounts (no persistence layer yet).
- Self-registration, password reset.
- CSRF tokens, session revocation lists (bounded instead by TTL + secret rotation).

## Architecture / Approach

Stateless, same-origin design: the Go backend issues an HMAC-signed cookie on login; every protected route re-verifies that signature on each request (no server-side session store). In production the Go binary serves both the SPA and `/api/*` from one origin, so there's no CORS surface — only the dev-mode Vite server needs a proxy to reach the Go backend during `npm run dev`.

## Phases at a Glance

| Phase | What it delivers | Key risk |
| --- | --- | --- |
| 1. Backend auth core | Accounts, bcrypt, signed session cookie, login/logout endpoints | Getting the HMAC signing/verification right (timing-safe compare, expiry check) |
| 2. Backend role gating | `RequireRole` middleware + `/api/me` example | None significant — straightforward once Phase 1's session verify exists |
| 3. Frontend auth | Login page, auth client, route guard, `/dashboard` example | SPA mode means no server-side loader protection — must rely on the backend's own enforcement, not just the client guard |

**Prerequisites:** None (F-01 has no roadmap prerequisites).
**Estimated effort:** ~1 session across 3 phases — small, well-bounded scaffold.

## Open Risks & Assumptions

- Assumes exactly one account per role is sufficient for the MVP; if a team needs multiple architects, the env-var scheme will need to grow (explicitly deferred, per the Multi-account decision).
- Assumes same-origin deployment (Go serving both SPA and API) continues to hold in production, so no CORS layer is ever needed.

## Success Criteria (Summary)

- All three role accounts can log in and are shown their correct role on `/dashboard`.
- An unauthenticated or wrong-role request to a `RequireRole`-wrapped endpoint is rejected (401/403) by the backend itself, not just hidden by the frontend.
- Logging out ends the session; a fresh page load afterward is treated as logged out.
