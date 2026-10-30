# Landing Page Polish Implementation Plan

## Overview

Three small fixes on the public landing page (`/`, `web/app/routes/home.tsx`), which is what production serves at shelfmark.online: switch the contact address to `hello@shelfmark.online`, drop the "In development" badge, and send a signed-in user straight to `/dashboard`. A Vitest test locks the redirect and the landing content in place.

## Current State Analysis

- `home.tsx` was ported from the original shelfmark.online page on 2026-10-20 (`context/archive/2026-10-20-home-landing-page/`). shelfmark.online now resolves to the production Fly app (same IP and build as `shelfmark-tsw.fly.dev`).
- The contact address `hello@shelfmark.dev` appears three times: the "Get in touch" `mailto:` (`web/app/routes/home.tsx:94`) and the footer link (`home.tsx:105-106`). No other file references it.
- The "In development" badge is the block at `home.tsx:47-51` (pulsing accent dot + label).
- `/` does not read the session. Every visitor sees the landing and the header "Zaloguj" link to `/login` (`home.tsx:41-43`), including signed-in users.
- `AuthProvider` wraps the whole app and loads `/api/me` once. `useAuth()` exposes `{ me, loading }`. A failed `/api/me` (500/network) is logged and reads as "no session" (`web/app/lib/auth.ts:69-80`).

## Desired End State

- The landing shows `hello@shelfmark.online` in both the "Get in touch" button and the footer, and no "In development" badge.
- An anonymous visitor sees the landing immediately, with no loading screen, and keeps the "Zaloguj" button.
- A signed-in user of any role who opens `/` is replaced onto `/dashboard` as soon as `/api/me` answers. A split-second glimpse of the landing before that is accepted.
- If `/api/me` fails, the visitor stays on the landing.
- `web/app/routes/home.test.tsx` covers these cases and fails if the redirect, the address or the badge regresses.

### Key Discoveries:

- Redirect pattern to follow: `useAuth()` + `useEffect` + `navigate(...)` in `web/app/lib/require-auth.tsx:15-31`.
- Test harness: `renderRoute(Component, { path, role, fetch })` already stubs `/login` and `/dashboard` with identifiable headings (`web/app/test/render-route.tsx:20-44`). The pattern to mirror is in `web/app/lib/require-auth.test.tsx`. Unstubbed fetches and unexpected `console.error` calls fail a test (`web/app/test/setup.ts`).

## What We're NOT Doing

- Not changing the "Zaloguj" button. It is present and works; a stale cached page only made it look missing.
- No changes to `/login` behaviour for signed-in users, no new marketing copy, no visual restyle.
- Not touching the dashboard. Stats are a separate, later change.
- No backend changes.

## Implementation Approach

Edit `home.tsx` in place: replace the address strings, delete the badge block, and add a session-aware redirect that never blocks rendering. The landing renders unconditionally. An effect navigates to `/dashboard` with `replace` once `loading` is false and `me` is set, so Back does not return to the landing.

## Phase 1: Landing polish + test

### Overview

Apply the three landing fixes and add a route test for them.

### Changes Required:

#### 1. Landing route

**File**: `web/app/routes/home.tsx`

**Intent**: Use the shelfmark.online contact address, remove the "In development" badge, and redirect signed-in users to the dashboard without delaying the page for anonymous visitors.

**Contract**:
- Both `mailto:` hrefs use `hello@shelfmark.online` (the "Get in touch" one keeps `?subject=Shelfmark`). The footer link text reads `hello@shelfmark.online`.
- The badge `<div>` (dot + "In development") is removed. Nothing replaces it, and the `h1` keeps its spacing.
- `Home` reads `useAuth()`. When `!loading && me`, it calls `navigate("/dashboard", { replace: true })` from an effect. The landing JSX always renders and has no loading branch.
- Only tokens from `app.css` are used, with no new palette or hex values (CLAUDE.md route-file rule).

#### 2. Landing route test

**File**: `web/app/routes/home.test.tsx` (new)

**Intent**: Lock in the redirect behaviour and the landing content per role and `/api/me` outcome.

**Contract**: Uses `renderRoute(Home, { path: "/", role, fetch })` and covers:
- anonymous (`role: null`): the landing heading renders, the "Zaloguj" link points to `/login`, both contact links use `hello@shelfmark.online`, and no "In development" text appears
- `/api/me` pending: the landing content is already visible (no "Loading…")
- each of `admin`, `architect`, `viewer`: ends on the "Dashboard page" heading
- `/api/me` 500: stays on the landing, with the expected `console.error` allowed through `allowConsoleError`

### Success Criteria:

#### Automated Verification:

- Frontend tests pass: `cd web && npm test`
- Type checking passes: `cd web && npm run typecheck`
- Build passes: `cd web && npm run build`
- `grep -rn "shelfmark.dev\|In development" web/app` returns no matches
- Removing the redirect effect from `home.tsx` makes `npm test` fail, and restoring it makes it pass

#### Manual Verification:

- On a local build, opening `/` while logged out shows the landing with no badge and with `hello@shelfmark.online` in the button and footer
- After logging in, opening `/` lands on `/dashboard`, and browser Back does not return to the landing

**Implementation Note**: After completing this phase and all automated verification passes, pause here for manual confirmation from the human that the manual testing was successful.

---

## Testing Strategy

### Unit Tests:

- `home.test.tsx` as above: anonymous, pending, three signed-in roles, and `/api/me` 500.

### Manual Testing Steps:

1. `cd web && npm run build`, run the binary, open `/` in a private window and check the badge and both addresses.
2. Log in as any seed account, open `/`, confirm it ends on `/dashboard`, press Back.

## References

- Original port: `context/archive/2026-10-20-home-landing-page/`
- Redirect pattern: `web/app/lib/require-auth.tsx:15-31`
- Test pattern: `web/app/lib/require-auth.test.tsx`

## Progress

> Convention: `- [ ]` pending, `- [x]` done. Append ` — <commit sha>` when a step lands. Do not rename step titles. See `references/progress-format.md`.

### Phase 1: Landing polish + test

#### Automated

- [x] 1.1 Frontend tests pass: `cd web && npm test` — f1559bc
- [x] 1.2 Type checking passes: `cd web && npm run typecheck` — f1559bc
- [x] 1.3 Build passes: `cd web && npm run build` — f1559bc
- [x] 1.4 `grep -rn "shelfmark.dev\|In development" web/app` returns no matches — f1559bc
- [x] 1.5 Removing the redirect effect from `home.tsx` makes `npm test` fail, and restoring it makes it pass — f1559bc

#### Manual

- [x] 1.6 On a local build, opening `/` while logged out shows the landing with no badge and with `hello@shelfmark.online` in the button and footer — f1559bc
- [x] 1.7 After logging in, opening `/` lands on `/dashboard`, and browser Back does not return to the landing — f1559bc
