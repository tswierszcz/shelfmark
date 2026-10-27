---
change_id: testing-access-control-data-integrity
title: Access control and data-integrity regression tests
status: archived
created: 2026-10-26
updated: 2026-10-26
archived_at: 2026-10-26T21:15:42Z
---

## Notes

Open a change folder for rollout Phase 2 of context/foundation/test-plan.md: "Access control & data-integrity regressions". Risks covered: #3 (disallowed role reaches a protected endpoint), #2 (nil-slice → JSON null recurs in a list endpoint), #6 (strategic-marks cap of 2 per (category, ecosystem) stops being enforced). Test types planned: unit + integration. Risk response intent: #3: a request from a disallowed role gets 403 (or 401 without a session) on every protected endpoint, not just the happy-path ones — table-driven per route × role. #2: every list-returning endpoint ships [] (never null) on an empty result — including endpoints added after the original fix. #6: a third active strategic mark in the same (category, ecosystem) is rejected with an explicit error — the "2 OK, 3rd rejected" boundary. After creating the folder, follow the downstream continuation rule.
