---
change_id: delete-project
title: Architect can delete a project
status: archived
created: 2026-10-28
updated: 2026-10-29
archived_at: 2026-10-29T18:28:10Z
---

## Notes

Architect może usunąć projekt (domknięcie litery D w CRUD dla odznaki 10xBuilder). Usunięcie projektu usuwa jego członkostwo bibliotek (project_libraries), ale nie biblioteki z katalogu ani znaczniki strategiczne. Kontekst: FK project_libraries ma ON DELETE CASCADE (backend/internal/db/migrations/0004_project_reports.up.sql:12-13); dziś brak ścieżek usuwania encji w kodzie (context/domain/evidence/3-code-rules.md). Termin certyfikacji 1: 2026-11-04.
