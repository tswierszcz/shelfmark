# Landing Page Polish — Plan Brief

> Full plan: `context/changes/landing-polish/plan.md`

## What & Why

The public landing (`/`, served at shelfmark.online) still carries leftovers from the temporary site: a contact address on a different domain (`hello@shelfmark.dev`) and an "In development" badge. Signed-in users who open `/` also see marketing copy instead of their app. This change fixes all three.

## Starting Point

`web/app/routes/home.tsx` holds the landing ported from the original shelfmark.online page on 2026-10-20. It never reads the session, so every visitor sees the same page and the "Zaloguj" button.

## Desired End State

Anonymous visitors see the landing immediately, with no badge and with `hello@shelfmark.online` in the "Get in touch" button and the footer. A signed-in user of any role who opens `/` lands on `/dashboard`, and Back does not bounce them to the landing.

## Key Decisions Made

| Decision | Choice | Why (1 sentence) | Source |
| --- | --- | --- | --- |
| Contact address | `hello@shelfmark.online` | Matches the domain the site is served from. | change.md |
| "In development" badge | Remove | The page should read as the product, not a placeholder. | change.md |
| Signed-in user on `/` | Redirect to `/dashboard` (`replace`) | Signed-in users have no reason to see marketing copy. | change.md |
| While `/api/me` loads | Render the landing at once and redirect when the session is known | Anonymous visitors, the main audience, never wait. A signed-in user may glimpse the landing briefly. | Plan |
| `/api/me` fails | Stay on the landing | Same "no session" reading as the rest of the app (`auth.ts`). | Plan |
| "Zaloguj" button | Unchanged | It already exists; a stale cache only hid it. | Plan |

## Scope

**In scope:**
- Contact address, badge removal and the signed-in redirect in `home.tsx`
- A new `home.test.tsx` route test

**Out of scope:**
- Dashboard statistics (a separate, later change)
- `/login` behaviour for signed-in users, copy or visual changes, backend work

## Architecture / Approach

`Home` reads `useAuth()` and, from an effect, calls `navigate("/dashboard", { replace: true })` once `loading` is false and `me` is set. This mirrors `RequireAuth`, but the landing always renders and has no loading gate. The test uses the existing `renderRoute` harness, which already stubs `/dashboard` and `/login`.

## Phases at a Glance

| Phase | What it delivers | Key risk |
| --- | --- | --- |
| 1. Landing polish + test | Address, no badge, redirect, `home.test.tsx` | Redirect firing for anonymous or errored sessions (covered by tests) |

**Prerequisites:** none.
**Estimated effort:** one short session.

## Open Risks & Assumptions

- A signed-in user may see the landing for one `/api/me` round-trip before the redirect. This was accepted deliberately.

## Success Criteria (Summary)

- shelfmark.online shows no badge and only the `hello@shelfmark.online` address.
- A signed-in user opening `/` ends on `/dashboard`.
- `npm test`, `typecheck` and `build` pass, and removing the redirect makes the tests fail.
