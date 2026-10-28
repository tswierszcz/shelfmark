# Usuwanie projektu — Plan Brief

> Full plan: `context/changes/delete-project/plan.md`

## What & Why

Architect może usunąć projekt. Shelfmark nie ma dziś żadnej ścieżki usuwania danych, a odznaka 10xBuilder wymaga pełnego CRUD „w sposób sensowny dla domeny” (termin 1: 2026-11-04). Usunięcie nieaktualnego projektu to najbardziej naturalne „D” w tej domenie.

## Starting Point

Projekty powstają i są nadpisywane przy uploadzie SBOM, ale nie da się ich usunąć. Baza jest już przygotowana: `project_libraries` ma `ON DELETE CASCADE` do `projects`, a wszystko, co zależy od projektu (direct, flagi retire), liczy się przy odczycie.

## Desired End State

`DELETE /api/projects/{id}` dla architekta zwraca 204 i usuwa projekt z listy, a 404 dla nieistniejącego lub błędnego id. Na `/redundancy` architekt usuwa wybrany projekt dwuetapowym przyciskiem. Biblioteki i znaczniki strategiczne zostają, a raporty innych projektów się nie zmieniają.

## Key Decisions Made

| Decision | Choice | Why (1 sentence) |
| --- | --- | --- |
| Kto usuwa | Tylko Architect | Ta sama rola tworzy projekty (upload Architect-only, `prd.md:101`) |
| Własność | Każdy architekt, bez sprawdzania `created_by` | Spójne z dzisiejszym nadpisywaniem projektu o tej samej nazwie; jedna organizacja |
| Potwierdzenie w UI | Dwuetapowy przycisk „Delete project” → „Confirm delete <nazwa>” / „Cancel” | Bez okna systemowego, na tokenach i komponencie `Button`, łatwe do E2E |
| Testy | Go (store, handler, role) + E2E Playwright | Test z perspektywy użytkownika to wymóg Buildera; Go pilnuje reguł danych |
| Twarde vs miękkie usuwanie | Twarde, przez kaskadę FK | Projekt nie ma historii, a odtwarza się ponownym uploadem |
| Biblioteki osierocone | Zostają w katalogu | Katalog jest wspólny dla organizacji |
| Nie-UUID w ścieżce | 404 zamiast dzisiejszego 500 | Nowa trasa od razu robi to dobrze; stara poza zakresem |

## Scope

**In scope:**
- `Store.DeleteProject` + `ErrProjectNotFound`, `DeleteProjectHandler`, trasa z rolą architect, wpis w `expectedRoles`
- testy Go: znikanie, biblioteki i znaczniki zostają, inny raport bez zmian, direct przeliczane, 404
- `deleteProject` w `projects.ts`, wariant `destructive` w `Button`, dwuetapowy przycisk na `/redundancy`
- E2E `project-delete.spec.ts`; zdanie o usuwaniu w Access Control w PRD

**Out of scope:**
- soft-delete i cofanie, usuwanie bibliotek, kategorii i użytkowników
- usuwanie przez Admina, sprawdzanie autora projektu
- naprawa 500 dla nie-UUID w istniejącym raporcie projektu

## Architecture / Approach

Jedno zapytanie `DELETE FROM projects WHERE id = $1`; kaskada FK usuwa członkostwo. Cienki handler mapuje `ErrProjectNotFound` na 404, a role pilnuje istniejąca tabela tras i jej test. W UI stan potwierdzenia żyje w widoku `/redundancy`, a przycisk destrukcyjny to nowy wariant istniejącego `Button`.

## Phases at a Glance

| Phase | What it delivers | Key risk |
| --- | --- | --- |
| 1. Backend | Trasa `DELETE` 204/404, role, testy Go, PRD | Usunięcie czegoś poza projektem: pilnują testy na bibliotekach, znacznikach i drugim raporcie |
| 2. Frontend i E2E | Dwuetapowy przycisk dla architekta, wariant `destructive`, test Playwright | Przypadkowe usunięcie: pilnuje dwuetapowe potwierdzenie |

**Prerequisites:** Docker (testy Go na Postgresie), lokalna baza `shelfmark_e2e` i `E2E_*` w `.env` do Playwrighta.
**Estimated effort:** 1–2 sesje, 2 fazy.

## Open Risks & Assumptions

- E2E nie działa w CI, więc wynik Playwrighta jest weryfikowany lokalnie.
- Zakładamy, że brak sprawdzania autora jest akceptowalny dla jednej organizacji (jak przy uploadzie).

## Success Criteria (Summary)

- Architekt usuwa projekt z `/redundancy` i projekt znika z listy oraz z API.
- Biblioteki, znaczniki strategiczne i raporty innych projektów są bez zmian.
- Viewer nie może usunąć projektu ani w UI, ani przez API (403).
