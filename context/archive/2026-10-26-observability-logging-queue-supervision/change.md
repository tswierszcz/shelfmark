---
change_id: observability-logging-queue-supervision
title: Observability logging contract and queue supervision
status: archived
created: 2026-10-26
updated: 2026-10-26
archived_at: 2026-10-26T18:44:20Z
---

## Notes

Fixes 1 and 2 from the observability audit `context/audits/observability/2026-10-26_catalog-seeding-redundancy-report-login-strategic-marking.md` (commit e1fea03):

1. Logging contract + server layer: slog JSON logger on stdout (Fly.io collects it); error writer that logs `err` with method, route pattern, request id, user id; request-id + access-log + panic-recovery middleware; `http.Server` with timeouts and graceful shutdown. Closes P1, P2, P3, CS9, RR8, LU2, SM1, SM2 (partially LU3, LU7, SM8, P5).
2. Queue supervision: `recover` per job, log every attempt and every status-write failure with library id / batch / purl, boot sweep for stuck `pending`/`processing` rows, drain on shutdown. Closes CS1, CS2, CS3, CS4, RR5.

Constraints: never log passwords, raw emails or session cookies; client-facing error messages stay unchanged.
