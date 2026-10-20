---
change_id: app-nav-menu
title: Shared navigation menu across authenticated views
status: impl_reviewed
created: 2026-10-20
updated: 2026-10-20
archived_at: null
---

## Notes

Each authenticated route (`dashboard`, `catalog`, `organization`, `redundancy`) was a self-contained page with no way to move between them except editing the URL — `dashboard.tsx` had one ad hoc "Go to catalog" link visible only to admins, and only `redundancy.tsx`/`dashboard.tsx` even showed the `Logo`. User asked for an actual menu to navigate between pages.

Added `web/app/components/nav.tsx`: a single `Nav` component reading `useAuth()` for the current user, showing the brand `Logo` (linking to `/dashboard`), a role-filtered link list (`Dashboard` always; `Catalog` admin-only; `Organization`/`Redundancy` architect+viewer), the current user's email, and a `Log out` button. The active link is highlighted with the `accent` token. Mounted at the top of all four authenticated views, replacing the old bare `Logo`/logout markup in `dashboard.tsx` and `redundancy.tsx`.

`dashboard.tsx`'s `<main>` width was widened from `max-w-lg` to `max-w-4xl` to match the other three views now that it carries the same nav bar.
