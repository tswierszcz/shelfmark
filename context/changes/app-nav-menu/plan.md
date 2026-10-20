# Shared navigation menu — Implementation Plan

## Overview

Add one `Nav` component and mount it on every authenticated route so users can move between `/dashboard`, `/catalog`, `/organization`, `/redundancy` without editing the URL.

## Desired End State

Visiting any authenticated route shows a top nav bar: logo (→ dashboard), role-appropriate links with the current route highlighted, user email, and a log-out button.

**Verification**: `cd web && npm run typecheck && npm run build` succeed; hardcoded-value grep scan returns 0 on all touched files; manually click through the nav as each role (admin/architect/viewer) and confirm only the links that role can access appear, and that the active link is highlighted.

## What We're NOT Doing

- Not adding a mobile hamburger / collapsed menu — the bar wraps via `flex-wrap`, which is enough at this screen count.
- Not touching `/`, `/login`, or `/dev/kitchen-sink-redundancy` — no auth, no nav needed there.
- Not changing route access rules (`RequireAuth role=...`) — `Nav`'s link list mirrors the existing rules, it doesn't change them.

## Implementation

### 1. `web/app/components/nav.tsx` (new)
`Nav` component: `Logo` linking to `/dashboard`; a `LINKS` table of `{ to, label, roles }` filtered against `me.role` from `useAuth()`; active link (`useLocation().pathname === to`) gets `bg-accent text-bg`, others `text-ink-dim hover:text-ink`; right side shows `me.email` and a `Button variant="secondary"` that calls `logout()` then navigates to `/login`.

### 2. `web/app/routes/dashboard.tsx`
Replace the inline `Logo` + conditional "Go to catalog" link + logout `Button` with `<Nav />`. Widen `<main>` to `max-w-4xl`.

### 3. `web/app/routes/redundancy.tsx`
Replace `<Logo className="mb-6" />` with `<Nav />`.

### 4. `web/app/routes/catalog.tsx` / `web/app/routes/organization.tsx`
Add `<Nav />` as the first child of `<main>`, above the existing `<h1>`.

## Success Criteria

### Automated
- `cd web && npm run typecheck`
- `cd web && npm run build`
- Hardcoded-value grep scan returns 0 on `nav.tsx`, `dashboard.tsx`, `catalog.tsx`, `organization.tsx`, `redundancy.tsx`.

### Manual
- Log in as each role; confirm the nav shows only the links that role can reach, the current page is highlighted, and `Log out` returns to `/login`.

## Progress

- [x] `nav.tsx` created.
- [x] `dashboard.tsx`, `redundancy.tsx`, `catalog.tsx`, `organization.tsx` updated.
- [x] Typecheck passes.
- [x] Build passes.
- [x] Grep scan returns 0 on all touched files.
