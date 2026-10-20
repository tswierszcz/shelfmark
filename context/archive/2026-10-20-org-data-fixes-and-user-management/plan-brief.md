# Org data fixes and user management — Plan Brief

> Full plan: `context/changes/org-data-fixes-and-user-management/plan.md`

## What & Why

A Milestone 1 completeness audit found two real data gaps behind roadmap items already marked `done`: FR-006's "Direct" column on `/organization` is a dead stub (always "Yes"), and FR-007's "up to two strategic libraries per category/ecosystem" rule is never enforced. The user also asked for a simple admin page to add users, which forces a related decision: today's 3 login accounts are hardcoded in `.env` and loaded once at startup — adding a 4th account currently requires editing env vars and restarting.

## Starting Point

`libraries.is_direct` defaults to `true` and nothing ever updates it, while the correct per-project direct/transitive data already exists in `project_libraries`. `strategic_marks` has no count/cap logic anywhere. `auth.LoadAccounts()` reads `ADMIN_EMAIL`/`ARCHITECT_EMAIL`/`VIEWER_EMAIL` (+ password hashes) into a static map captured once by `LoginHandler` — there is no reload path.

## Desired End State

`/organization`'s Direct column reflects real usage. Marking a 3rd library strategic in an already-full category/ecosystem is rejected with a clear error. An admin opens `/users`, fills in email/password/role, and the new account can log in immediately — no `.env` edit, no restart, no CLI step.

## Key Decisions Made

| Decision | Choice | Why (1 sentence) | Source |
|---|---|---|---|
| Change scope | One combined change, not split bugfix/feature | User chose speed over strict separation of concerns | User (prior turn) |
| Account storage model | Full replacement of `.env` accounts with a `users` DB table | User chose the cleaner long-term architecture over a dual env+DB fallback | User (prior turn) |
| `.env` requirement after migration | Required only on first boot (empty table) | Keeps deployment simple once the DB has rows; avoids permanently-unused required env vars | Plan (AskUserQuestion) |
| New-user password input | Admin types plaintext, server hashes with bcrypt | Matches "very simple" intent — no CLI (`hashpw`) step for the admin | Plan (AskUserQuestion) |
| Users page scope | Add-form + read-only list of existing users | Avoids duplicate-email guesswork; store needs a list query for bootstrap anyway | Plan (AskUserQuestion) |
| Password validation | Minimum 8 characters | One small guardrail against trivially weak accounts, cheap to add alongside bcrypt | Plan (AskUserQuestion) |
| FR-006 fix approach | Compute `is_direct` live via `EXISTS` over `project_libraries`, don't maintain a cached column | Always correct, no write-path to keep in sync, trivial query cost at this project's scale | Plan (research) |
| FR-007 cap enforcement | Reject the 3rd distinct library server-side (409), inside a transaction with the count check | PRD says "up to two," not "evict the oldest"; a transaction avoids a concurrent-mark race | Plan (research) |

## Scope

**In scope:**
- Fix `/organization`'s dead Direct column (FR-006).
- Enforce the 2-strategic-library cap per category/ecosystem (FR-007).
- New `users` DB table, migrated/seeded from existing `.env` accounts on first boot.
- `POST /api/users` (create) and `GET /api/users` (list) endpoints, admin-only.
- New `/users` frontend page: add-user form + list, linked from `Nav` for admins only.

**Out of scope:**
- Editing or deleting existing users, resetting passwords.
- Email format validation, "confirm password" field.
- Session revocation when a user is added or changed.
- Any change to existing `RequireAuth` role rules.

## Architecture / Approach

Backend-first: fix the two read-path bugs in `catalog` (Phase 1, no schema dependency on auth work), then build the `users` table bottom-up — schema → store → bootstrap seed → handlers → `main.go` wiring, all as one phase since login would be broken by a partial version of this work. Frontend page comes last, consuming the new API. A final phase re-verifies everything together, since Phase 2 changes how every existing role logs in.

## Phases at a Glance

| Phase | What it delivers | Key risk |
|---|---|---|
| 1. Fix FR-006 + FR-007 | Live `is_direct` computation; enforced strategic cap | Low — isolated query/logic changes, no schema migration needed for FR-007 |
| 2. Users table + bootstrap + CRUD | DB-backed accounts, one-time env seed, admin API | Medium-high — changes how every role logs in; startup ordering changes |
| 3. Admin "Users" page | `/users` route: add-user form + list | Low — standard form + table, existing component patterns |
| 4. End-to-end verification | Fresh-DB bootstrap, new-user login, cap rejection, Direct column, all proven together | Low — verification only, no new code |

**Prerequisites:** None — builds on the existing `catalog` and `auth` packages, no external services beyond the existing Postgres instance.
**Estimated effort:** ~1 session across 4 phases; Phase 2 is the one that needs careful manual verification (restart behavior) before moving on.

## Open Risks & Assumptions

- Assumes the SBOM formats currently in use carry enough information for `project_sbom.go` to distinguish direct from transitive dependencies — if not, FR-006's fix will correctly show "No" for everything, which is accurate but not very useful (this is the same Open Question already tracked in the PRD, not new to this change).
- The one-way `.env`→DB seed means there's no automated path back to env-based accounts if the `users` table is ever corrupted; recovery is a manual `psql` fix, documented in the plan's Migration Notes.

## Success Criteria (Summary)

- `/organization`'s Direct column is never wrong for a library with upload history.
- No category/ecosystem can ever hold more than 2 active strategic marks.
- An admin can create a working login for a new teammate in under a minute, with zero restarts or terminal access.
