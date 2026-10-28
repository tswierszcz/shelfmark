# Usuwanie projektu — plan implementacji

## Overview

Architect może usunąć projekt. Znika wiersz `projects` i, przez kaskadę FK, jego członkostwo w `project_libraries`. Biblioteki zostają w katalogu, a znaczniki strategiczne pozostają nietknięte. Domyka to literę D w CRUD wymaganą dla odznaki 10xBuilder (termin 1: 2026-11-04). Bez migracji.

## Current State Analysis

- Brak jakiejkolwiek ścieżki usuwania encji w kodzie (`context/domain/evidence/3-code-rules.md`, „Unknowns”). Jedyna trasa `DELETE` to odznaczenie strategiczne (`backend/cmd/shelfmark/routes.go:57`).
- **Kaskada już istnieje:** `project_id uuid NOT NULL REFERENCES projects (id) ON DELETE CASCADE` (`backend/internal/db/migrations/0004_project_reports.up.sql:12`). Jeden `DELETE FROM projects WHERE id = $1` usuwa całe członkostwo.
- Wszystko, co zależy od projektu, jest liczone przy odczycie:
  - „direct w którymkolwiek projekcie” to `EXISTS` po `project_libraries` (`backend/internal/catalog/store.go:130`);
  - flagi strategic/retire wylicza `strategicIndex` (`backend/internal/catalog/strategic.go:201-245`).
  Po usunięciu nie trzeba niczego przeliczać.
- Wzorce do powtórzenia:
  - `projectExists` + 404 w `ProjectReportHandler` (`backend/internal/catalog/project.go:146-158`, `project_handlers.go:163-175`);
  - mapowanie błędów `writeCatalogError` (`handlers.go:57`);
  - tabela tras z rolami (`routes.go:35-61`);
  - regresyjna blokada ról `expectedRoles` (`backend/cmd/shelfmark/routes_test.go:18`), w której nowa trasa bez wpisu wywala test.
- `projectID`, który nie jest UUID, kończy się dziś błędem pgx, czyli 500 (otwarte pytanie z `context/changes/redundancy-flow-analysis/research.md`).
- UI:
  - projekt wybiera się w `<select id="project-picker">` na `/redundancy` (`web/app/routes/redundancy.tsx:276-288`); stan `projects`, `selectedProjectId`, `isArchitect` (`redundancy.tsx:68, 80, 83`);
  - klient `web/app/lib/projects.ts` ma wzorzec `fetch` + `errorMessage` (`projects.ts:65-85`);
  - `Button` ma tylko warianty `primary` i `secondary` (`web/app/components/button.tsx:3-6`);
  - w aplikacji nie ma żadnego potwierdzenia akcji destrukcyjnej.
- E2E: `web/tests/e2e/sbom-upload-report.spec.ts` uploaduje SBOM pod unikalną nazwą i sprząta przez `pg` (`:39-43`).

## Desired End State

- `DELETE /api/projects/{projectID}` (rola architect) zwraca:
  - 204 po usunięciu;
  - 404 dla nieistniejącego projektu i dla `projectID`, który nie jest UUID;
  - 403 dla admina i viewera, 401 bez sesji.
- Po usunięciu:
  - projekt nie występuje w `GET /api/projects`, a jego raport zwraca 404;
  - biblioteki są nadal w katalogu, znaczniki strategiczne bez zmian;
  - raport każdego innego projektu jest identyczny jak przed usunięciem.
- Na `/redundancy` architekt widzi przy wybranym projekcie „Delete project”. Po kliknięciu pojawiają się „Confirm delete <nazwa>” i „Cancel”. Potwierdzenie usuwa projekt, znika on z listy, a wybór się czyści. Viewer przycisku nie widzi.
- `prd.md` (Access Control) mówi, że usuwanie projektu jest Architect-only.

### Key Discoveries:

- Kaskada FK robi całą pracę na danych (`0004_project_reports.up.sql:12`), więc nie potrzeba transakcji obejmującej kilka tabel.
- `routes_test.go:18` wymusza świadomy wpis roli dla każdej nowej trasy i sprawdza 401/403 na prawdziwym muxie.
- `CLAUDE.md`: rozszerzaj kontrakt istniejącego komponentu zamiast duplikować; tokeny tylko z `@theme` (`--color-destructive` istnieje).

## What We're NOT Doing

- Brak soft-delete, kosza i cofania. Projekt odtwarza się ponownym uploadem SBOM; PRD i tak nie trzyma historii projektu („Replace the project's current library set, no history”, `archive/2026-10-19-project-redundancy-report/plan-brief.md:22`).
- Brak usuwania bibliotek osieroconych przez usunięty projekt. Zostają w katalogu, bo katalog jest wspólny dla organizacji.
- Brak sprawdzania autora (`created_by`). Każdy architekt może usunąć dowolny projekt, tak samo jak dziś może nadpisać projekt o tej samej nazwie (`project.go:52-56`).
- Brak usuwania dla roli Admin. Admin nie widzi projektów (`routes.go:59-60`).
- Brak usuwania użytkowników, bibliotek i kategorii (osobne decyzje).
- Brak zmiany zachowania 500 dla nie-UUID w istniejącym `GET /api/projects/{projectID}/report` (poza zakresem; nowa trasa od razu robi to dobrze).

## Implementation Approach

Najpierw backend z testami (faza 1), potem UI i E2E, które z niego korzystają (faza 2). Każda faza to osobny commit z zielonymi bramkami.

## Phase 1: Backend — usuwanie projektu

### Overview

Operacja w store, handler, trasa z rolą architect, testy Go i aktualizacja PRD.

### Changes Required:

#### 1. Store

**File**: `backend/internal/catalog/project.go`

**Intent**: Usunąć projekt jednym zapytaniem; kaskada usuwa członkostwo. Rozróżnić „nie ma takiego projektu” od błędu bazy.

**Contract**: `func (s *Store) DeleteProject(ctx context.Context, projectID string) error`. Zwraca nowy nazwany błąd `ErrProjectNotFound`, gdy usunięto 0 wierszy **albo** gdy `projectID` nie jest poprawnym UUID (pgx/Postgres zgłasza `invalid_text_representation`, `pgerrcode.InvalidTextRepresentation`). Każdy inny błąd jest opakowany jako `catalog: failed to delete project: %w`.

#### 2. Handler i trasa

**File**: `backend/internal/catalog/project_handlers.go`, `backend/cmd/shelfmark/routes.go`

**Intent**: Cienki handler dla nowej trasy, zgodny z wzorcem `ProjectReportHandler`.

**Contract**:
- `func DeleteProjectHandler(store *Store) http.HandlerFunc` dla `DELETE /api/projects/{projectID}`;
- odpowiedzi: 204 bez treści; `ErrProjectNotFound` → 404 `"project not found"` przez `writeCatalogError`; inny błąd → 500 (logowany jak pozostałe 5xx);
- wpis w `routes()` z rolami `architect`.

#### 3. Blokada ról

**File**: `backend/cmd/shelfmark/routes_test.go`

**Intent**: Świadomy wpis nowej trasy w regresyjnej tabeli ról.

**Contract**: `"DELETE /api/projects/{projectID}": {auth.RoleArchitect}` w `expectedRoles`. Istniejący test sam sprawdzi 401 bez sesji i 403 dla admina oraz viewera.

#### 4. Testy Go

**File**: `backend/internal/catalog/project_handlers_test.go`

**Intent**: Pokryć reguły danych usuwania na prawdziwym Postgresie.

**Contract**: testy z prefiksem `TestDeleteProject`:
- dwa projekty ze wspólną biblioteką i znacznikiem strategicznym; usunięcie projektu A daje 204, A znika z `ListProjects`, a raport A daje 404;
- biblioteki A nadal są w `ListLibraries`, znacznik strategiczny nadal aktywny (`StrategicMarksByLibrary`);
- raport B identyczny przed i po (porównanie po purlu, jak w `project_report_test.go`);
- biblioteka direct tylko w A przestaje być direct w widoku organizacji (`ListLibraries` `IsDirect`);
- nieistniejący UUID daje 404, a `"not-a-uuid"` daje 404, nie 500.

#### 5. PRD

**File**: `context/foundation/prd.md`

**Intent**: Dokument dostępu zgodny z kodem.

**Contract**: zdanie pod tabelą ról (`prd.md:101`) rozszerzone: „Project SBOM upload and project deletion are Architect-only; …”.

### Success Criteria:

#### Automated Verification:

- Testy usuwania przechodzą: `cd backend && go test ./internal/catalog/ -run 'TestDeleteProject' -count=1`
- Blokada ról przechodzi z nową trasą: `cd backend && go test ./cmd/shelfmark/ -count=1`
- Cały backend i vet przechodzą: `cd backend && go vet ./... && go test ./... -count=1`

#### Manual Verification:

- Na jednorazowej instancji `curl -X DELETE` jako architekt daje 204, jako viewer 403, a dla nie-UUID 404

**Implementation Note**: Po fazie 1 zatrzymaj się na potwierdzenie wiersza Manual.

---

## Phase 2: Frontend i E2E

### Overview

Klient API, dwuetapowy przycisk na `/redundancy` (tylko architekt), wariant destrukcyjny przycisku i test E2E z perspektywy użytkownika.

### Changes Required:

#### 1. Klient API

**File**: `web/app/lib/projects.ts`

**Intent**: Wywołanie nowej trasy w stylu istniejących funkcji.

**Contract**: `export async function deleteProject(projectId: string): Promise<void>`; `DELETE /api/projects/${encodeURIComponent(projectId)}`; przy `!res.ok` rzuca `Error(await errorMessage(res, "Failed to delete project"))`.

#### 2. Wariant przycisku

**File**: `web/app/components/button.tsx`, `web/app/routes/dev.kitchen-sink-redundancy.tsx`

**Intent**: Akcja destrukcyjna ma własny wygląd na tokenach, przez rozszerzenie istniejącego komponentu.

**Contract**: `variant?: "primary" | "secondary" | "destructive"`, klasy oparte o `destructive` z `@theme` (np. `bg-destructive text-bg`). Kitchen-sink dostaje stan „destructive” obok istniejących.

#### 3. Widok `/redundancy`

**File**: `web/app/routes/redundancy.tsx`

**Intent**: Architekt może usunąć wybrany projekt bez ryzyka przypadkowego kliknięcia.

**Contract**:
- przy wybranym projekcie i `isArchitect`: przycisk „Delete project” (`variant="secondary"`);
- po kliknięciu zamienia się w „Confirm delete <nazwa projektu>” (`variant="destructive"`) i „Cancel”;
- potwierdzenie woła `deleteProject`, usuwa projekt z `projects`, czyści `selectedProjectId` i raport, a błąd pokazuje w istniejącym miejscu błędów (`text-destructive`);
- zmiana wybranego projektu lub „Cancel” wraca do stanu początkowego;
- viewer nie widzi przycisku.

#### 4. E2E

**File**: `web/tests/e2e/project-delete.spec.ts` (nowy)

**Intent**: Dowód z perspektywy użytkownika, że architekt może usunąć projekt.

**Contract**:
- według wzoru `sbom-upload-report.spec.ts`: upload SBOM pod unikalną nazwą, wybór projektu, „Delete project” → „Confirm delete …”;
- asercje: projekt znika z `#project-picker`, a po przeładowaniu strony nadal go nie ma;
- sprzątanie bibliotek testowych przez `pg` jak w istniejącym spec.

### Success Criteria:

#### Automated Verification:

- Typecheck i build przechodzą: `cd web && npm run typecheck && npm run build`
- Nowy E2E przechodzi: `cd web && npx playwright test tests/e2e/project-delete.spec.ts`
- Cały zestaw E2E przechodzi: `cd web && npx playwright test`

#### Manual Verification:

- Na `/redundancy` jako architekt: „Delete project” → „Confirm delete <nazwa>”; „Cancel” wraca do stanu początkowego, potwierdzenie usuwa projekt z listy
- Jako viewer przycisku „Delete project” nie ma
- Kitchen-sink pokazuje wariant `destructive` na tokenach, czytelny w jasnym i ciemnym motywie

**Implementation Note**: Po fazie 2 zatrzymaj się na potwierdzenie wierszy Manual. Następnie `/10x-impl-review delete-project`.

---

## Testing Strategy

### Unit Tests:

- Brak czystej logiki do testów jednostkowych; reguły leżą w SQL i kaskadzie, więc testy idą na Postgresie.

### Integration Tests:

- `TestDeleteProject*` (faza 1): 204 i znikanie, biblioteki i znaczniki zostają, raport innego projektu bez zmian, direct przeliczane, 404 dla nieistniejącego i nie-UUID.
- `routes_test.go`: 401 bez sesji, 403 dla admina i viewera.

### Manual Testing Steps:

1. Faza 1: `curl -X DELETE` na jednorazowej instancji dla trzech ról i dla nie-UUID.
2. Faza 2: przejście „Delete → Cancel” i „Delete → Confirm” jako architekt; brak przycisku jako viewer.

## Performance Considerations

Jedno `DELETE` po kluczu głównym plus kaskada po indeksie PK `project_libraries (project_id, library_id)`. Bez znaczenia przy tej skali.

## Migration Notes

Brak migracji. Rollback obrazu jest bezpieczny, bo schemat się nie zmienia; usunięte projekty odtwarza się ponownym uploadem SBOM.

## References

- `backend/internal/db/migrations/0004_project_reports.up.sql:11-16`
- `backend/internal/catalog/project.go:146-191`, `project_handlers.go:163-260`
- `backend/cmd/shelfmark/routes.go:35-61`, `routes_test.go:14-40`
- `web/app/routes/redundancy.tsx:68-200, 270-290`, `web/app/lib/projects.ts:65-85`, `web/app/components/button.tsx`
- `web/tests/e2e/sbom-upload-report.spec.ts`
- `context/domain/domain-distillation.md` (R-13 tożsamość projektu), `context/foundation/prd.md:91-103`
- Wymóg CRUD: `.claude/skills/10x-idea-check/references/10xdevs-4-certification.md`

## Progress

> Convention: `- [ ]` pending, `- [x]` done. Append ` — <commit sha>` when a step lands. Do not rename step titles. See `references/progress-format.md`.

### Phase 1: Backend — usuwanie projektu

#### Automated

- [x] 1.1 Testy usuwania przechodzą — cc6d8d3
- [x] 1.2 Blokada ról przechodzi z nową trasą — cc6d8d3
- [x] 1.3 Cały backend i vet przechodzą — cc6d8d3

#### Manual

- [ ] 1.4 curl DELETE na jednorazowej instancji: 204 / 403 / 404

### Phase 2: Frontend i E2E

#### Automated

- [x] 2.1 Typecheck i build przechodzą
- [x] 2.2 Nowy E2E przechodzi
- [x] 2.3 Cały zestaw E2E przechodzi

#### Manual

- [ ] 2.4 Delete / Cancel / Confirm na /redundancy jako architekt
- [ ] 2.5 Viewer nie widzi przycisku
- [ ] 2.6 Kitchen-sink pokazuje wariant destructive
