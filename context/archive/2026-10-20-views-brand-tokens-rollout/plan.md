# Brand tokens rollout — remaining views — Implementation Plan

## Overview

Apply the existing Shelfmark brand tokens (`web/app/app.css`) and shared components (`Logo`, `Button`, `Badge` in `web/app/components/`) to `login.tsx`, `dashboard.tsx`, `catalog.tsx`, and `organization.tsx`. No new tokens or components — same mechanical substitution already proven on `redundancy.tsx`.

## Desired End State

All four routes render with the dark brand theme (no literal `gray-*`/`red-*`/`amber-*` classes), using `Button` for primary actions and `Badge` for the "Strategic" status pill. `Logo` appears on `login.tsx` (first screen a logged-out user sees) and `dashboard.tsx` (first screen after login).

**Verification**: `cd web && npm run typecheck && npm run build` succeed; hardcoded-value grep scan (same pattern used in `redundancy-view-brand-tokens/plan.md` Phase 2) returns 0 hits on all four files; manual look at each route in the browser.

## What We're NOT Doing

- Not touching `home.tsx` (unmodified starter template, out of scope per prior change).
- Not adding new tokens, components, or dependencies.
- Not changing any data-fetching, routing, or business logic — styling-only substitution.

## Phase 1: login.tsx + dashboard.tsx

- `login.tsx`: card `border-gray-200` → `border-rule bg-bg-soft`; inputs `border-gray-300 dark:border-gray-700 dark:bg-gray-900` → `border-rule bg-bg-soft text-ink`; error text `text-red-600 dark:text-red-400` → `text-destructive`; submit `<button>` → `<Button type="submit" disabled={submitting}>`; add `<Logo className="mb-6" />` above the `<h1>`; heading gets `font-serif`.
- `dashboard.tsx`: heading gets `font-serif`; add `<Logo className="mb-6" />` above the `<h1>`; logout `<button>` → `<Button type="button" onClick={handleLogout}>`.

## Phase 2: catalog.tsx

- All `border-gray-200`/`border-gray-800` section/table borders → `border-rule` (+ `bg-bg-soft` on section containers).
- All `border-gray-300 ... dark:border-gray-700 dark:bg-gray-900` selects/inputs → `border-rule bg-bg-soft text-ink`.
- Error text (`text-red-600 dark:text-red-400`) → `text-destructive`.
- Upload/Add `<button>` elements → `<Button>` (Add button uses a compact size via `className` override).
- Empty-state `text-gray-500` → `text-ink-faint`.
- Headings get `font-serif`.

## Phase 3: organization.tsx

- Same border/input/error substitutions as Phase 2.
- "Strategic" status pill (`bg-amber-100 ... dark:bg-amber-900 dark:text-amber-100`) → `<Badge tone="accent">Strategic</Badge>`.
- "Mark strategic"/"Unmark"/"Confirm mark"/"Confirm unmark" buttons → `<Button variant="secondary">` / `<Button>`.
- Empty-state `text-gray-500` → `text-ink-faint`.
- Headings get `font-serif`.

## Success Criteria

### Automated

- `cd web && npm run typecheck`
- `cd web && npm run build`
- Hardcoded-value scan returns 0 on all four files.

### Manual

- `/login`, `/dashboard`, `/catalog`, `/organization` all render dark brand theme, no white/gray flashes, buttons and badges visually consistent with `/redundancy`.

## Progress

### Phase 1: login.tsx + dashboard.tsx
- [x] Implemented
- [x] Typecheck/build pass

### Phase 2: catalog.tsx
- [x] Implemented
- [x] Typecheck/build pass

### Phase 3: organization.tsx
- [x] Implemented
- [x] Typecheck/build pass

All phases verified together: `npm run typecheck`, `npm run build`, and the hardcoded-value grep scan (0 hits on all four files) all passed after the full rollout.
