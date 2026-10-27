---
change_id: org-data-fixes-and-user-management
title: Fix FR-006/FR-007 data gaps and add DB-backed user management
status: archived
created: 2026-10-20
updated: 2026-10-20
archived_at: 2026-10-20T15:01:50Z
---

## Notes

Bundles three pieces of work, by user decision (one combined change rather than splitting bugfixes from the new feature):

1. **FR-006 fix** — `libraries.is_direct` defaults to `true` at seed time and is never updated afterward; the "Direct" column on `/organization` always reads "Yes" regardless of actual usage. Needs to reflect real direct/transitive status (derived from the union of project SBOM uploads, same data `project_libraries.is_direct` already has correctly per-project).
2. **FR-007 fix** — "up to two strategic libraries per category/ecosystem" is not enforced anywhere (`MarkStrategicHandler`, `store.MarkStrategic`, and the `/organization` UI all allow unlimited marks). Needs a hard cap of 2, enforced server-side (not just UI-side).
3. **New feature: admin user management page.** A simple page (no email validation) where an admin can add a new user (email, password, role). User decision: fully replace the current `.env`-based accounts (`ADMIN_EMAIL`/`ADMIN_PASSWORD_HASH`, `ARCHITECT_EMAIL`/`ARCHITECT_PASSWORD_HASH`, `VIEWER_EMAIL`/`VIEWER_PASSWORD_HASH`) with a `users` DB table — at startup, migrate the 3 existing env-configured accounts into the table so existing logins keep working, then the DB becomes the single source of truth going forward. This is a deliberate deviation from the PRD's Access Control section ("accounts/roles provisioned via instance configuration") — the PRD will need a follow-up note/update once this ships.

Found via a Milestone 1 completeness audit (see conversation) that otherwise confirmed FR-001–FR-005, FR-008, and US-01 as genuinely implemented, not just stubbed.
