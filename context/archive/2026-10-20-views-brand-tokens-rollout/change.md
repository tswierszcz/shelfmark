---
change_id: views-brand-tokens-rollout
title: Roll out Shelfmark brand design tokens to the remaining views
status: archived
created: 2026-10-20
updated: 2026-10-20
archived_at: 2026-10-20T11:21:14Z
---

## Notes

Follow-up to `redundancy-view-brand-tokens`, which introduced the token set (`web/app/app.css`) and the three shared components (`Logo`, `Button`, `Badge`) and applied them only to `redundancy.tsx`. That change's notes explicitly called out that `catalog.tsx`, `organization.tsx`, `login.tsx`, and `dashboard.tsx` would each "get their own future change inheriting these same tokens."

This change applies the existing tokens/components to all four remaining routes in one pass (user decision: single change covering all four views, not one change per view, to move faster). No new tokens or components are introduced — this is pure consumption of what already exists:

- `login.tsx` — form card, inputs, error text, submit button
- `dashboard.tsx` — heading, logout button
- `catalog.tsx` — upload form, filters, table, status chips
- `organization.tsx` — filters, table, "Strategic" badge, mark/unmark forms

`home.tsx` (unmodified React Router starter template) stays out of scope, same as the prior change.
