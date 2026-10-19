# Shelfmark brand design tokens — redundancy report view — Implementation Plan

## Overview

Introduce Shelfmark's brand design tokens (sourced live from shelfmark.online) as a single dark theme in `web/app/app.css`, add three hand-rolled shared components (`Logo`, `Button`, `Badge`), apply them to `web/app/routes/redundancy.tsx` — the app's main product screen — and fix the blank-flash loading state in `web/app/lib/require-auth.tsx`. Close with a kitchen-sink route proving all 7 UI states and a `CLAUDE.md` rule so the next agent keeps using the tokens.

## Current State Analysis

- `web/app/app.css` defines only `--font-sans` inside `@theme`; no color/radius/spacing tokens exist. `html, body` hardcode `bg-white dark:bg-gray-950`.
- `web/app/root.tsx` loads only Google Fonts "Inter"; no Fraunces.
- No `web/app/components/` directory exists; `package.json` has no shadcn/radix/cva/clsx.
- `web/app/routes/redundancy.tsx` has 15 literal Tailwind gray/red/amber hits (full list in `research.md` Code References).
- `web/app/lib/require-auth.tsx:27-29,33` returns `null` while `loading` is true and when auth/role checks fail, so every visit to `redundancy.tsx` (wrapped in `RequireAuth`) can flash a blank, untokenized page.
- 49 total `dark:`-variant class usages exist across `web/app/routes/*.tsx` and `app.css`, driven by `prefers-color-scheme` — this plan removes that mechanism in favor of a single always-dark brand theme (decision below).

## Desired End State

`redundancy.tsx` renders with Shelfmark's brand palette (dark editorial theme, gold accent, Fraunces headings, bookshelf-spine logo), reading every color/spacing decision from tokens in `app.css` rather than literal Tailwind classes. A new kitchen-sink route proves all 7 UI states. The loading flash before auth resolves is replaced by a themed indicator. A rule in `CLAUDE.md` tells the next agent where tokens/components live.

**Verification**: open `/redundancy` and `/dev/kitchen-sink-redundancy` in a browser; `npm run typecheck` and `npm run build` succeed in `web/`; the hardcoded-value scan against `redundancy.tsx` returns 0 hits (down from 15).

### Key Discoveries:

- Tailwind v4 publishes any `--color-*` custom property inside `@theme` as a utility class suffix automatically (e.g. `--color-accent` → `bg-accent`, `text-accent`, `border-accent`) — no `@theme inline` indirection needed since we are not keeping a separate `.dark` variant.
- shelfmark.online defines no error/destructive color — only `--accent` (gold) and a neutral ink/bg scale. This plan must derive a destructive token (see Critical Implementation Details).
- `web/app/routes.ts` is a flat array of `route(path, file)` calls — adding the kitchen-sink route is a one-line addition.

## What We're NOT Doing

- Not touching `catalog.tsx`, `organization.tsx`, `login.tsx`, `dashboard.tsx`, or `home.tsx` — each gets its own future change inheriting these same tokens.
- Not adding shadcn/ui, radix, cva, or any component library — three components are hand-rolled Tailwind/React.
- Not keeping a light theme or `prefers-color-scheme` responsive behavior — single fixed dark brand theme, per decision below.
- Not fixing the `!me` / `wrongRole` branch of `require-auth.tsx` (`:33`) — that redirect fires near-instantly and was not the flagged charge; only the `loading` branch (charge 4) is in scope.
- Not installing Playwright or any screenshot-diff runner — the kitchen-sink route plus manual screenshots are the visual gate, per decision below.
- Not adding a lint/pre-commit hardcoded-value guard — the repo has no lint tooling (no eslint/biome/prettier) at all; introducing one is a separate, larger decision than this change's scope.
- Not rebranding `home.tsx` (still the unmodified React Router starter template) — flagged in `research.md` as a future charge for whichever change touches the public landing route.

## Implementation Approach

Work bottom-up: establish token values and font loading first (Phase 1), then build the three shared components and rewrite `redundancy.tsx` to use them (Phase 2), then fix the shared loading-state gap (Phase 3), then prove the result with a kitchen-sink route and persist the convention as an agent rule (Phase 4). This matches the `/10x-ui` skill's required order: environment/library → token values → one view → states.

## Critical Implementation Details

- **Destructive color has no brand source.** shelfmark.online defines `--accent: #c9a227` (gold) but no error/danger color. This plan introduces `--color-destructive: #c75146` (a muted brick red chosen to sit within the warm, dark editorial palette) since Tailwind's default `red-*` scale clashes with the brand tone. This is a derived value, not one pulled from the live site — call this out in the PR/review as a judgment call, not a brand fact.
- **Font link consolidation.** `root.tsx` currently loads Inter alone via `family=Inter:ital,opsz,wght@0,14..32,100..900`. shelfmark.online's own combined link is `family=Fraunces:opsz,wght@9..144,400;9..144,600&family=Inter:wght@400;500`. Phase 1 replaces the existing Inter link with this exact combined URL (matching brand source precisely) rather than appending a second stylesheet tag — this narrows Inter's loaded weight range from the current `100..900` to `400;500`. Since every other route today only ever renders default Tailwind font weights (400/500/600/700 via `font-semibold`/`font-medium`), this is expected to be a no-op for them visually, but is a shared-file change affecting every route, not just `redundancy.tsx` — flag any visual regression found elsewhere during manual verification as a new charge for that route's own future change, not something to fix here.
- **Kitchen-sink needs no auth and no new dependency.** `/dev/kitchen-sink-redundancy` renders static fixture data (no `RequireAuth` wrapper, no real API calls) so all 7 states (including `hover` and `focus-visible`, which only need a real mouse/keyboard against real rendered elements) are reachable by a human reviewer directly in the browser — no JS state-forcing or Storybook needed.

## Phase 1: Design tokens & brand environment

### Overview

Define the brand's semantic color/font tokens as a single always-dark theme in `app.css`, and load Fraunces alongside Inter in `root.tsx`.

### Changes Required:

#### 1. Brand tokens and global background

**File**: `web/app/app.css`

**Intent**: Replace the empty token set and hardcoded light/dark background with Shelfmark's brand palette as a single fixed dark theme, so every future view can read `bg-bg`, `text-ink`, `bg-accent`, etc. instead of literal Tailwind colors.

**Contract**: Inside `@theme`, add `--color-bg: #14110f`, `--color-bg-soft: #1b1714`, `--color-ink: #ece7df`, `--color-ink-dim: #a39b90`, `--color-ink-faint: #6d6459`, `--color-accent: #c9a227`, `--color-rule: #2e2823`, `--color-destructive: #c75146`, and `--font-serif: "Fraunces", Georgia, serif`. Replace the `html, body { @apply bg-white dark:bg-gray-950; @media (prefers-color-scheme: dark) { color-scheme: dark; } }` block with `html, body { @apply bg-bg text-ink; color-scheme: dark; }` — no media query, since there is only one theme.

#### 2. Brand typography

**File**: `web/app/root.tsx`

**Intent**: Load Fraunces (brand serif, used for headings and the logo wordmark) alongside Inter, matching shelfmark.online's own font request.

**Contract**: Replace the existing `stylesheet` link's `href` in the `links` array with `https://fonts.googleapis.com/css2?family=Fraunces:opsz,wght@9..144,400;9..144,600&family=Inter:wght@400;500&display=swap` (single combined request, matching the brand source exactly — see Critical Implementation Details on the Inter weight-range narrowing).

### Success Criteria:

#### Automated Verification:

- Type checking passes: `cd web && npm run typecheck`
- Build succeeds: `cd web && npm run build`

#### Manual Verification:

- Running `npm run dev` and opening any existing route (e.g. `/login`) shows a dark `#14110f` background instead of white, with no console font-loading errors.
- `view-source` or devtools confirms Fraunces is requested and applied where `font-serif` is used (will be empty until Phase 2, but the font request itself should succeed with no 404).

---

## Phase 2: redundancy.tsx view + shared components

### Overview

Add `Logo`, `Button`, and `Badge` as hand-rolled components reading only from the new tokens, then rewrite every literal-value hit in `redundancy.tsx` to use them.

### Changes Required:

#### 1. Logo component

**File**: `web/app/components/logo.tsx` (new)

**Intent**: Render the Shelfmark bookshelf-spine mark plus wordmark as a single reusable component, so the brand has an in-app presence (charge 3 from `research.md`).

**Contract**: Exports `Logo({ className }: { className?: string })`. Renders the exact SVG markup from shelfmark.online (4 rounded rects, fills `#c9a227` / `#ece7df` with `.85`/`.35` opacity as on the live site) at `width=28 height=28`, followed by `<span className="font-serif font-semibold">Shelfmark</span>`, wrapped in a `flex items-center gap-3` container.

#### 2. Button component

**File**: `web/app/components/button.tsx` (new)

**Intent**: Replace the ad hoc `bg-gray-900 ... dark:bg-gray-100` button styling in `redundancy.tsx` with one pill-shaped, token-driven button matching the brand's CTA style.

**Contract**: Exports `Button({ variant = "primary", className, ...props }: { variant?: "primary" | "secondary" } & ComponentProps<"button">)`. `variant="primary"` → `rounded-full bg-accent text-bg font-medium px-4 py-2 disabled:opacity-50`. `variant="secondary"` → `rounded-full border border-rule text-ink px-4 py-2`. Forwards all other button props (`type`, `disabled`, `onClick`, `children`) unchanged.

#### 3. Badge component

**File**: `web/app/components/badge.tsx` (new)

**Intent**: Replace the literal `amber-100`/`red-100` status pills in `redundancy.tsx` with a single token-driven badge (charge 5 from `research.md`).

**Contract**: Exports `Badge({ tone, children }: { tone: "accent" | "destructive"; children: ReactNode })`. `tone="accent"` → `rounded bg-accent/15 text-accent px-2 py-0.5 text-xs`. `tone="destructive"` → `rounded bg-destructive/15 text-destructive px-2 py-0.5 text-xs`.

#### 4. Apply tokens and components to the view

**File**: `web/app/routes/redundancy.tsx`

**Intent**: Replace every literal Tailwind gray/red/amber hit identified in `research.md` (lines 163, 177, 197, 204, 216, 227, 250, 253, 260, 275, 291, 296, 307, 317) with the new tokens and components, and add a `Logo` to the top of the page so the view carries brand identity.

**Contract**:
- Section containers (`:163`, `:227`) and table/row rules (`:260`, `:275`): `rounded-lg border border-gray-200 p-4 dark:border-gray-800` → `rounded-lg border border-rule bg-bg-soft p-4` (drop `dark:` variants entirely — single theme).
- Inputs (`:177`), select (`:238` from earlier read): `border-gray-300 ... dark:border-gray-700 dark:bg-gray-900` → `border border-rule bg-bg-soft text-ink rounded px-3 py-2`.
- Submit button (`:197`): replace raw `<button>` with `<Button type="submit" disabled={...}>`.
- Logout/secondary-style elements: none in this view; skip.
- Error text (`:204`, `:250`, `:253`): `text-red-600 dark:text-red-400` → `text-destructive text-sm`.
- Batch-progress chip (`:216`): `border-gray-200 dark:border-gray-800` → `border border-rule`.
- "Strategic" badge (`:291`): `<Badge tone="accent">Strategic</Badge>`.
- "Retire candidate" badge (`:296`) and retire-flag text (`:307`): `<Badge tone="destructive">Retire candidate</Badge>` / `text-destructive`.
- Empty-state text (`:317`): `text-gray-500` → `text-ink-faint`.
- Add `<Logo />` as the first element inside `<main>`, above the `<h1>`.
- Headings (`:160,164,228`): add `font-serif` to the existing `text-xl font-semibold` / `text-lg font-medium` classes.

### Success Criteria:

#### Automated Verification:

- Type checking passes: `cd web && npm run typecheck`
- Build succeeds: `cd web && npm run build`
- Hardcoded-value scan returns 0 hits on the view: `grep -ncE '#[0-9a-fA-F]{3,8}\b|rgba?\(|hsla?\(|oklch\(|-\[[0-9.]+(px|rem)\]|\b(bg|text|border|ring|outline|from|via|to|fill|stroke|shadow|divide)-(slate|gray|zinc|neutral|stone|red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose|white|black)\b' web/app/routes/redundancy.tsx` prints `0` (the SVG fill hexes inside `logo.tsx` are expected and out of scope for this scan — they're the fixed brand asset, not a token gap)

#### Manual Verification:

- `/redundancy` visually matches the brand: dark background, gold accent button, Fraunces headings, visible logo.
- Upload form and project-report table still function end-to-end (smoke test: pick a project, confirm table renders).
- "Strategic" and "Retire candidate" badges are visually distinct from each other and legible against the dark background.

**Implementation Note**: Pause here for manual confirmation before proceeding to Phase 3.

---

## Phase 3: require-auth.tsx loading state

### Overview

Replace the blank `null` render during `loading` with a themed indicator, closing charge 4 from `research.md`.

### Changes Required:

#### 1. Themed loading state

**File**: `web/app/lib/require-auth.tsx`

**Intent**: Show a brand-consistent loading indicator instead of a blank page while auth state resolves, so visiting `/redundancy` directly never flashes an untokenized blank screen.

**Contract**: Change the `if (loading) { return null; }` branch to return a minimal centered element, e.g. `<div className="flex min-h-screen items-center justify-center bg-bg text-ink-faint text-sm">Loading…</div>`. Leave the `if (!me || wrongRole) { return null; }` branch unchanged (out of scope — see "What We're NOT Doing").

### Success Criteria:

#### Automated Verification:

- Type checking passes: `cd web && npm run typecheck`

#### Manual Verification:

- Throttle network (or add a brief artificial delay during manual testing) and reload `/redundancy`: a themed "Loading…" state appears instead of a blank flash before content renders.

---

## Phase 4: States & visual gate

### Overview

Prove the view's 7 UI states side by side in a new kitchen-sink route, take reference screenshots, and persist the token/component convention as a `CLAUDE.md` rule.

### Changes Required:

#### 1. Kitchen-sink route registration

**File**: `web/app/routes.ts`

**Intent**: Register the new dev-only review route.

**Contract**: Add `route("dev/kitchen-sink-redundancy", "routes/dev.kitchen-sink-redundancy.tsx")` to the exported array.

#### 2. Kitchen-sink page

**File**: `web/app/routes/dev.kitchen-sink-redundancy.tsx` (new)

**Intent**: Render the view's key tokenized elements (upload button, status badges, table rows, error text) in all 7 states side by side with static fixture data, with no auth guard, so a reviewer can see and interact with (hover/tab into) every state in one page load.

**Contract**: Default export renders, labeled in sections: **default** (normal `Button`, a populated fixture row); **hover**/**focus-visible** (the same real `Button`/table row — reviewer mouses over / tabs to them; no forced state needed); **disabled** (`<Button disabled>`); **error** (the error-text pattern with fixture message); **empty** (the empty-state table row, "No libraries recorded for this project."); **loading** (the batch-progress chip pattern with a fixture "processing…" value). No `RequireAuth` wrapper; no network calls; all data is inline fixture data.

#### 3. Persist the convention

**File**: `CLAUDE.md`

**Intent**: Tell the next agent where brand tokens and shared components live, so new views extend this system instead of reintroducing literal Tailwind colors.

**Contract**: Append a new section **after** the existing `<!-- END @przeprogramowani/10x-cli -->` line (never inside the CLI-managed block), documenting: tokens live in `web/app/app.css` `@theme` (`--color-bg`, `--color-bg-soft`, `--color-ink`, `--color-ink-dim`, `--color-ink-faint`, `--color-accent`, `--color-rule`, `--color-destructive`); shared components live in `web/app/components/` (`Logo`, `Button`, `Badge` — check this directory before building a new one); no literal Tailwind palette colors or arbitrary hex/rgb values in route files — use the tokens above; kitchen-sink reference lives at `/dev/kitchen-sink-redundancy`.

#### 4. Close out the charges

**File**: `context/changes/redundancy-view-brand-tokens/research.md`

**Intent**: Record that all 5 charges were addressed, for `/10x-impl-review` and future readers.

**Contract**: Under `## Charges`, append a one-line resolution note to each of the 5 numbered charges (e.g. "— Fixed in Phase 2/3/1, see `plan.md`").

### Success Criteria:

#### Automated Verification:

- Type checking passes: `cd web && npm run typecheck`
- Build succeeds: `cd web && npm run build`
- Hardcoded-value scan still returns 0 hits on `redundancy.tsx` (re-run the Phase 2 command)

#### Manual Verification:

- Opening `/dev/kitchen-sink-redundancy` shows all 7 states clearly labeled and visually distinguishable; hovering and tab-focusing the real elements shows token-driven state changes.
- Desktop-width and one mobile-width screenshot taken of both `/redundancy` and the kitchen-sink page, attached to the change for review.
- `CLAUDE.md` renders correctly with the new section after, not inside, the `<!-- END @przeprogramowani/10x-cli -->` marker.

---

## Testing Strategy

### Manual Testing Steps:

1. Run `npm run dev` in `web/`, visit `/redundancy` as an architect/viewer user, confirm brand styling and that upload + report flows still work end-to-end.
2. Reload `/redundancy` with network throttled to see the themed loading state.
3. Visit `/dev/kitchen-sink-redundancy`, tab through and hover over interactive elements to confirm all 7 states.
4. Re-run the hardcoded-value grep scan against `redundancy.tsx` and confirm it returns `0`.

## Performance Considerations

None identified — this change only affects static CSS tokens, font loading, and component markup; no new runtime dependencies or data-fetching changes.

## Migration Notes

Not applicable — no data model or API changes.

## References

- Related research: `context/changes/redundancy-view-brand-tokens/research.md`
- Brand source (live, fetched 2026-10-19): `https://shelfmark.online/`

## Progress

> Convention: `- [ ]` pending, `- [x]` done. Append ` — <commit sha>` when a step lands. Do not rename step titles. See `references/progress-format.md`.

### Phase 1: Design tokens & brand environment

#### Automated

- [x] 1.1 Type checking passes: `cd web && npm run typecheck` — 245ac12
- [x] 1.2 Build succeeds: `cd web && npm run build` — 245ac12

#### Manual

- [x] 1.3 Any existing route shows dark `#14110f` background, no font-loading errors — 245ac12
- [x] 1.4 Fraunces font request succeeds (no 404) — 245ac12

### Phase 2: redundancy.tsx view + shared components

#### Automated

- [x] 2.1 Type checking passes: `cd web && npm run typecheck`
- [x] 2.2 Build succeeds: `cd web && npm run build`
- [x] 2.3 Hardcoded-value scan on `redundancy.tsx` returns 0 hits

#### Manual

- [x] 2.4 `/redundancy` visually matches the brand (dark bg, gold CTA, Fraunces headings, logo visible)
- [x] 2.5 Upload form and project-report table function end-to-end
- [x] 2.6 "Strategic" and "Retire candidate" badges are visually distinct and legible

### Phase 3: require-auth.tsx loading state

#### Automated

- [ ] 3.1 Type checking passes: `cd web && npm run typecheck`

#### Manual

- [ ] 3.2 Throttled reload of `/redundancy` shows themed "Loading…" instead of a blank flash

### Phase 4: States & visual gate

#### Automated

- [ ] 4.1 Type checking passes: `cd web && npm run typecheck`
- [ ] 4.2 Build succeeds: `cd web && npm run build`
- [ ] 4.3 Hardcoded-value scan on `redundancy.tsx` still returns 0 hits

#### Manual

- [ ] 4.4 `/dev/kitchen-sink-redundancy` shows all 7 states, clearly labeled and distinguishable
- [ ] 4.5 Desktop + one mobile-width screenshots taken of both `/redundancy` and the kitchen sink
- [ ] 4.6 `CLAUDE.md` addition renders correctly, outside the CLI-managed block
