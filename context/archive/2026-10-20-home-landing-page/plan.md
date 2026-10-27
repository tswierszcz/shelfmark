# Home landing page — Implementation Plan

## Overview

Replace the React Router starter `<Welcome />` on `/` with the real Shelfmark marketing content from `https://shelfmark.online/`, styled with the app's existing tokens, plus a login button.

## Desired End State

`/` shows the live site's header (logo + badge + headline + lede), three capability points, the "Built on the SBOMs…" contact section with its mailto CTA, and a footer — all using `app.css` tokens, no literal hex/rgb. A "Zaloguj" link in the header navigates to `/login`.

**Verification**: `cd web && npm run typecheck && npm run build` succeed; hardcoded-value grep scan returns 0 hits on `home.tsx`; `/` renders in the browser matching the source site's layout/copy.

## What We're NOT Doing

- Not changing `/login`, `/dashboard`, or any other route.
- Not adding a new design token — the live site's palette is already identical to `web/app/app.css`.
- Not keeping the "Get in touch" mailto CTA's literal `rgba()` shadow values — ported to `color-mix()` against the existing `--color-accent` token instead.

## Implementation

### 1. `web/app/components/button.tsx`
Export `BUTTON_VARIANT_CLASSES` (the existing variant class strings, now named and exported) so a router `Link` can reuse the "secondary" button look without a `<button>` element.

### 2. `web/app/routes/home.tsx`
Full rewrite. Header: `Logo` + `Link to="/login"` styled with `BUTTON_VARIANT_CLASSES.secondary`; "In development" badge; `h1`/lede from the source copy. Main: three-point grid, contact paragraph + mailto CTA. Footer: brand + contact email. All literal colors mapped to tokens (`bg`, `bg-soft`, `ink`, `ink-dim`, `ink-faint`, `accent`, `rule`); two shadow accents use `color-mix(in srgb, var(--color-accent) N%, transparent)`.

### 3. Remove `web/app/welcome/`
Delete the unused starter scaffold (`welcome.tsx`, `logo-dark.svg`, `logo-light.svg`) — nothing references it after the rewrite.

## Success Criteria

### Automated
- `cd web && npm run typecheck`
- `cd web && npm run build`
- Hardcoded-value grep scan on `home.tsx` returns 0.

### Manual
- `/` visually matches `shelfmark.online`'s layout and copy, dark brand theme, no white flash.
- "Zaloguj" button navigates to `/login`.

## Progress

- [x] `button.tsx`: exported `BUTTON_VARIANT_CLASSES`.
- [x] `home.tsx`: rewritten with source content + login link.
- [x] `web/app/welcome/` removed.
- [x] Typecheck passes.
- [x] Build passes.
- [x] Grep scan returns 0 hits on `home.tsx`.
