# Shelfmark brand design tokens — redundancy view — Plan Brief

> Full plan: `context/changes/redundancy-view-brand-tokens/plan.md`
> Research: `context/changes/redundancy-view-brand-tokens/research.md`

## What & Why

Shelfmark's production app currently looks like an unstyled Tailwind scaffold, while a real brand already exists at shelfmark.online (dark editorial theme, gold accent, Fraunces serif, bookshelf-spine logo). This change introduces that brand as design tokens and applies it to `redundancy.tsx`, the architect's main screen — the first of several per-view changes that will each inherit the same tokens.

## Starting Point

`web/app/app.css` defines only a font token; there are no color tokens, no shared components, and no component library in `package.json`. `redundancy.tsx` has 15 literal Tailwind gray/red/amber classes, and the shared `require-auth.tsx` renders a blank page while auth resolves.

## Desired End State

`/redundancy` renders in Shelfmark's brand palette, built entirely from tokens and three new shared components (`Logo`, `Button`, `Badge`). A themed loading indicator replaces the blank flash on entry. A new `/dev/kitchen-sink-redundancy` route proves all 7 UI states, and `CLAUDE.md` tells future agents where the tokens and components live.

## Key Decisions Made

| Decision | Choice | Why | Source |
| --- | --- | --- | --- |
| Theme strategy | Single fixed dark theme, no `prefers-color-scheme` | shelfmark.online has no light variant; the brand *is* dark | Plan (user-confirmed) |
| Background scope | Global `html`/`body` now | One source of truth; other views inherit it when they get their own change | Plan (user-confirmed) |
| Component approach | Hand-rolled Tailwind, zero new deps | Only 3 simple components needed; matches the skill's "scoped to what the view needs" rule | Plan (user-confirmed) |
| `require-auth.tsx` loading fix | Fix now (themed skeleton) | It's charge 4 from research and hits every visit to the main product screen | Plan (user-confirmed) |
| Visual gate | New in-app kitchen-sink route | No Playwright/screenshot runner exists; this works on the current stack and is reusable for future views | Plan (user-confirmed) |
| Destructive color | Derived `#c75146`, not brand-sourced | shelfmark.online defines no error color; Tailwind's default red clashes with the warm palette | Plan (research gap) |

## Scope

**In scope:** brand tokens in `app.css`, Fraunces font load, `Logo`/`Button`/`Badge` components, full `redundancy.tsx` rewrite (15 literal hits → 0), `require-auth.tsx` loading-state fix, kitchen-sink route, `CLAUDE.md` rule, closing all 5 research charges.

**Out of scope:** every other route (`catalog`, `organization`, `login`, `dashboard`, `home`) — future changes; shadcn/ui or any component library; light theme; the post-redirect blank-null branch in `require-auth.tsx`; Playwright/screenshot-diff tooling; a lint/pre-commit hardcoded-value guard (no lint infra exists).

## Architecture / Approach

Tokens are defined once in `web/app/app.css`'s `@theme` block (Tailwind v4 auto-publishes `--color-*` as utility classes like `bg-accent`). Three components in a new `web/app/components/` directory consume those tokens. `redundancy.tsx` is rewritten to use both. A themed loading state closes the shared `require-auth.tsx` gap. A kitchen-sink route and a `CLAUDE.md` rule make the convention discoverable and reusable for the next view.

## Phases at a Glance

| Phase | What it delivers | Key risk |
| --- | --- | --- |
| 1. Design tokens & brand environment | Color/font tokens in `app.css`, Fraunces loaded in `root.tsx` | Global background/font change affects every route immediately, before they get their own brand pass |
| 2. redundancy.tsx view + shared components | `Logo`/`Button`/`Badge` built; all 15 literal hits replaced | Destructive color is a derived guess, not brand-sourced — may need revisiting |
| 3. require-auth.tsx loading state | Themed loading indicator replaces blank flash | Touches a file shared by every route |
| 4. States & visual gate | Kitchen-sink route, screenshots, `CLAUDE.md` rule, charges closed | No automated screenshot-diff — regressions rely on manual review until a later module adds Playwright |

**Prerequisites:** none — repo and dependencies already support this (Tailwind v4 already installed).
**Estimated effort:** ~1 session across 4 phases; no new external dependencies.

## Open Risks & Assumptions

- The derived destructive color (`#c75146`) is a judgment call, not a brand fact — may need a real design review later.
- Narrowing `root.tsx`'s Inter font-weight range from `100..900` to `400;500` (to match shelfmark.online's own link exactly) touches every route; expected to be a no-op visually but not explicitly verified on routes outside this change's scope.
- No automated visual regression protection exists yet (Module 3's `/10x-e2e-setup` could add Playwright later if this becomes a recurring pain point).

## Success Criteria (Summary)

- `/redundancy` visually matches the Shelfmark brand and functions end-to-end (upload + report).
- Zero literal Tailwind palette/arbitrary-value hits remain in `redundancy.tsx`.
- All 7 UI states are visible and reviewable at `/dev/kitchen-sink-redundancy`.
- The next agent touching any view finds the token/component convention documented in `CLAUDE.md`.
