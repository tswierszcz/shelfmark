---
change_id: home-landing-page
title: Real home page content, sourced from shelfmark.online, with a login CTA
status: archived
created: 2026-10-20
updated: 2026-10-20
archived_at: 2026-10-20T11:27:29Z
---

## Notes

`web/app/routes/home.tsx` was still the unmodified React Router starter template (`<Welcome />`), explicitly out of scope in `redundancy-view-brand-tokens` and `views-brand-tokens-rollout`. User asked to replace it with the actual marketing content from the live `shelfmark.online` site, plus a "Zaloguj" (log in) button that routes to `/login`.

Fetched the live page's HTML/CSS directly (`curl https://shelfmark.online/`) and ported its header/points/contact/footer sections into `home.tsx` using the design tokens and `Logo` component already in the repo (`web/app/app.css`, `web/app/components/logo.tsx`) — the live site's own CSS variables are identical to the app's tokens, so no new values were introduced. Two `box-shadow` accents from the source used raw `rgba(201,162,39,…)`; ported those as `color-mix(in srgb, var(--color-accent) …%, transparent)` instead, to keep the file free of literal hex/rgb per `CLAUDE.md`'s route-file rule.

Added a secondary-style `Link to="/login"` in the header next to the logo. No existing `Button` variant supports rendering as a router `<Link>`, so `button.tsx` now exports its `BUTTON_VARIANT_CLASSES` map so other files can apply the same visual style to a non-`<button>` element without duplicating the class strings.

Removed the now-unused `web/app/welcome/` scaffold (`welcome.tsx` + both starter logo SVGs) since nothing references it anymore.
