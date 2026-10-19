---
date: 2026-10-19T18:04:48+02:00
researcher: Claude Sonnet 5
git_commit: d5366a18a12d8224e8ba94e30a46c451eed39224
branch: shape/shelfmark
repository: shelfmark
topic: "UI audit: introduce Shelfmark brand design tokens, applied to the redundancy report view"
tags: [research, codebase, ui, design-tokens, redundancy-report, tailwind]
status: complete
last_updated: 2026-10-19
last_updated_by: Claude Sonnet 5
---

# Research: UI audit for redundancy-view-brand-tokens

**Date**: 2026-10-19T18:04:48+02:00
**Researcher**: Claude Sonnet 5
**Git Commit**: d5366a18a12d8224e8ba94e30a46c451eed39224
**Branch**: shape/shelfmark
**Repository**: shelfmark

## Research Question

`/10x-ui` audit for the `redundancy-view-brand-tokens` change: where do design-token values and shared components currently live (or not) in this React Router + Tailwind v4 app, and what in `web/app/routes/redundancy.tsx` needs to change to read from a Shelfmark-brand token source instead of literal Tailwind gray/red/amber classes?

## Summary

The app has **no design system** (source → views direction): `web/app/app.css` defines only `--font-sans` inside `@theme` (`web/app/app.css:3-6`); no `--color-*` tokens exist, there is no components directory under `web/app`, and `package.json` lists no UI/component dependency (no shadcn, radix, clsx, or cva) — only `react`, `react-dom`, and `react-router` (`web/package.json:8-13`). Every visual decision in `redundancy.tsx` is a literal Tailwind palette class.

In the view → source direction, `redundancy.tsx` contains 15 literal-palette/arbitrary-value hits from the hardcoded-value scan (gray borders/backgrounds, red error text, amber/red status badges), none of which trace back to any token, because none exist yet. The brand reference (shelfmark.online, fetched live) defines a full dark, editorial token set (`--bg`, `--bg-soft`, `--ink`, `--ink-dim`, `--ink-faint`, `--accent`, `--rule`) plus Fraunces/Inter typography and an inline bookshelf-spine SVG mark, none of which are loaded or referenced anywhere in `web/app` today — `web/app/root.tsx:9-16` loads only Inter.

## Detailed Findings

### Token source (does not exist yet)

- `web/app/app.css:1-6` — Tailwind v4 entry point; `@theme` block defines only `--font-sans`. No semantic color, radius, or spacing tokens.
- `web/app/app.css:8-15` — `html, body` hardcode `bg-white dark:bg-gray-950` directly in CSS, not via a token — this is the literal the brand background token needs to replace.
- No file under `web/app` matches `*component*` (directory search), confirmed empty result — there is no shared component directory to extend.
- `web/package.json:8-13` — dependency list has no component/utility library (no shadcn, radix-ui, clsx, class-variance-authority, lucide). Introducing tokens/components here is a greenfield addition, not an extension.

### Typography (brand font missing)

- `web/app/root.tsx:9-16` — `links` loads only Google Fonts "Inter" (ital,opsz,wght axes). Fraunces (brand's serif display face, used for `h1`/`h2`/the logo wordmark on shelfmark.online) is not loaded anywhere in the repo.

### redundancy.tsx — literal values standing in for tokens

- `web/app/routes/redundancy.tsx:163` — `rounded-lg border border-gray-200 p-4 dark:border-gray-800` (upload section container).
- `web/app/routes/redundancy.tsx:177` — input border/background: `border-gray-300 ... dark:border-gray-700 dark:bg-gray-900`.
- `web/app/routes/redundancy.tsx:197` — primary action button: `bg-gray-900 ... text-white ... dark:bg-gray-100 dark:text-gray-900` (no pill radius, no accent color — contradicts the brand's gold pill CTA).
- `web/app/routes/redundancy.tsx:204`, `:250`, `:253` — error text: `text-red-600 dark:text-red-400`, repeated three times with no shared error-message component.
- `web/app/routes/redundancy.tsx:216` — batch-progress chip border: `border-gray-200 dark:border-gray-800`.
- `web/app/routes/redundancy.tsx:227`, `:260`, `:275` — report section container and table border rules: `border-gray-200`/`border-gray-100` + dark variants — candidates for the brand's single `--rule` divider token (brand style note: thin 1px rule dividers, not cards/shadows).
- `web/app/routes/redundancy.tsx:291` — "Strategic" badge: `bg-amber-100 ... text-amber-900 dark:bg-amber-900 dark:text-amber-100` — amber is not the brand accent (`#c9a227` is a specific gold, not Tailwind's `amber` scale).
- `web/app/routes/redundancy.tsx:296`, `:307` — "Retire candidate" badge and retire-flag text: `bg-red-100 ... text-red-900 dark:bg-red-900 dark:text-red-100` / `text-red-600 dark:text-red-400` — no shared `destructive` token.
- `web/app/routes/redundancy.tsx:317` — empty-state row: `text-gray-500`, plain text inside the existing table (acceptable empty-state shape, but still an untokenized color).

### Architecture: blank screen during auth resolution

- `web/app/lib/require-auth.tsx:27-29` — while `loading` is `true`, `RequireAuth` returns `null`.
- `web/app/lib/require-auth.tsx:33` — once resolved, if `!me` or `wrongRole`, it also returns `null` (before the `useEffect` redirect fires).
- `redundancy.tsx:331-337` wraps its entire content in `RequireAuth`, so every visit — including a direct link — renders a blank page (background falls through to the untokenized `bg-white dark:bg-gray-950` in `web/app/app.css:10`) before either the real content or a redirect appears. No spinner, no brand mark, nothing shown during this window.

### Comparison: other routes show the same pattern, confirming it's systemic not local

- `web/app/routes/dashboard.tsx:16-33` — same literal `bg-gray-900`/`dark:bg-gray-100` button, same `underline` link styling, no shared `Button`/`Link` component.
- `web/app/routes/home.tsx:1-13` — still the unmodified React Router starter template (`Welcome` component, default meta title "New React Router App") — confirms the baseline note in `context/foundation/roadmap.md` ("only a template demo page") is still current. Out of scope for this change (not the view being audited) but worth flagging as a later charge for whichever change touches the public landing route.
- No route file or shared module renders the Shelfmark logo/wordmark anywhere in `web/app` — the bookshelf-spine SVG mark exists only on the external marketing site (shelfmark.online), fetched live for this audit, not in this repository.

## Code References

- `web/app/app.css:1-15` — Tailwind v4 entry, only token today is `--font-sans`, body background hardcoded.
- `web/app/root.tsx:9-23` — global `<head>` links (fonts) and `<Layout>`; no logo, no Fraunces.
- `web/app/routes/redundancy.tsx:158-328` — the audited view; see literal-value list above.
- `web/app/lib/require-auth.tsx:12-40` — shared auth gate; blank-render-during-loading charge.
- `web/app/routes/dashboard.tsx:14-38`, `web/app/routes/home.tsx:1-13` — comparison evidence that the gap is systemic.
- `web/package.json:8-20` — full dependency list (no UI/component library present).

## Architecture Insights

Every route in `web/app/routes/` independently rebuilds its own container, borders, buttons, and status-pill colors directly from Tailwind's default gray/red/amber palettes. There is no single place that currently decides what "primary", "surface", "muted", or "destructive" mean for this app — so "redundancy-view-brand-tokens" is introducing a design system, not extending one, per the three conditions the `/10x-ui` skill requires for that case: (1) flagged as a dependency/pattern addition for the user to approve in planning, (2) scoped to the token block plus whatever components `redundancy.tsx` actually needs (not a full component library), (3) nothing existing is being displaced since no prior system exists.

## Historical Context (from prior changes)

- `context/foundation/roadmap.md` "Baseline" section (not quantitatively re-verified beyond the one claim checked above) states the frontend is "a scaffolded SPA ... but only a template demo page; no project-specific views yet" as of 2026-10-15 — confirmed still true for `home.tsx` specifically as of this commit (`web/app/routes/home.tsx:1-13`, unchanged `Welcome`/default meta).
- No prior `context/changes/**/` or `context/archive/**/` folder addresses UI/branding; this is the first design-related change in the project's history.

## Related Research

None — no other `research.md` in this repository discusses styling, tokens, or branding.

## Open Questions

- Whether the eventual plan should also add a minimal shared `Logo` component (SVG) in this same change, or defer it to a separate "app shell/nav" change — the mark currently exists nowhere in-app; `redundancy.tsx` itself has no header area today to place it in. Left for `/10x-plan` to scope.
- Whether `RequireAuth`'s blank-render-during-loading gap should be fixed inside this token change (quick, token-driven fix: swap `return null` for a themed loading skeleton) or deferred as a separate change, since it's a shared file touched by every route, not just `redundancy.tsx`. Flagged as a charge below; scope decision left to `/10x-plan`.

## Charges

1. **[Missing tokens]** No color/typography tokens exist anywhere in the repo (`web/app/app.css:1-6` defines only `--font-sans`); every color in `redundancy.tsx` (15 hits: lines 163, 177, 197, 204, 216, 227, 250, 253, 260, 275, 291, 296, 307, 317, plus border repeats) is a literal Tailwind gray/red/amber class. **User impact**: the screen users actually use looks like an unstyled Tailwind scaffold, carrying none of the Shelfmark brand (dark editorial palette, gold accent) visible on shelfmark.online.
2. **[Missing tokens]** `web/app/root.tsx:9-16` loads only Inter; Fraunces (the brand's serif display face for headings) is never loaded, so `redundancy.tsx:160,164,228` headings render in default sans weight. **User impact**: headings read as generic UI text instead of the brand's editorial identity.
3. **[Missing shared component]** The Shelfmark bookshelf-spine SVG mark is not present anywhere under `web/app` (confirmed by directory/file search); no route renders a logo or product mark. **User impact**: there is no brand presence inside the product at all — a user who saw the marketing site sees nothing recognizable once logged in.
4. **[Accidental architecture]** `web/app/lib/require-auth.tsx:27-29,33` returns `null` during loading and on failed auth/role checks, so `redundancy.tsx` (wrapped in `RequireAuth` at `:331-337`) renders a blank, untokenized page (`web/app/app.css:10` background) before content or redirect appears. **User impact**: every direct visit to the report — the architect's main entry point — briefly flashes a blank screen with no loading indicator.
5. **[Missing tokens]** Status badges use Tailwind's `amber`/`red` palette directly (`redundancy.tsx:291,296,307`) instead of a semantic `accent`/`destructive` token, and Tailwind `amber` does not match the brand's specific gold (`#c9a227`). **User impact**: once brand tokens land elsewhere on the page, these badges will visibly clash rather than read as "important" and "dangerous" in the brand's own voice.

No charges are deferred — all five are in scope for this change (token source + `redundancy.tsx`); charge 3 (logo) and charge 4 (auth loading state) touch shared files (`root.tsx`, `require-auth.tsx`) used by every route, which is expected and acceptable since the token source itself must be shared.
