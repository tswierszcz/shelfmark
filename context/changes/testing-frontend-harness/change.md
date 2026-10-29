---
change_id: testing-frontend-harness
title: Testing frontend harness
status: impl_reviewed
created: 2026-10-29
updated: 2026-10-29
archived_at: null
---

## Notes

Opened for rollout Phase 4 of context/foundation/test-plan.md: "Frontend functional test harness".
Risks covered: #5 (a frontend route mishandles an error/empty/role-mismatch response and crashes or misrenders). Test types planned: unit/component (new setup — no frontend runner installed yet).
Risk response intent: #5 — prove a route renders the correct state (data/empty/error/loading) for a given role without crashing, regardless of the API response shape; challenge "manual clicking from the plan's Verification step is a permanent substitute"; avoid testing only happy-path rendering.
