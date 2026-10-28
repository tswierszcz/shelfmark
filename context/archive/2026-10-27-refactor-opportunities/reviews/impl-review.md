<!-- IMPL-REVIEW-REPORT -->
# Implementation Review: Jeden właściciel znaczników strategicznych

- **Plan**: context/changes/refactor-opportunities/plan.md
- **Scope**: Full plan
- **Reviewed phases**: 1, 2, 3
- **Date**: 2026-10-28
- **Verdict**: APPROVED
- **Findings**: 0 critical, 0 warnings, 4 observations

## Verdicts

| Dimension | Verdict |
|-----------|---------|
| Plan Adherence | PASS |
| Scope Discipline | PASS |
| Safety & Quality | PASS |
| Architecture | PASS |
| Pattern Consistency | PASS |
| Success Criteria | PASS |

## Evidence

- Diff `79c2261..HEAD`: 6 plików backendu + dokumenty zmiany; każdy plik z planu jest w diffie, nic spoza planu.
- Plan drift: wszystkie zmiany MATCH; `project_report_test.go` identyczny między 86dccdd a HEAD; faza 2 to czyste przeniesienie (tylko nagłówki `package`/`import` nie są ruchem); nic z „What We're NOT Doing” nie zostało dotknięte (brak zmian w `web/`, migracjach, `handlers.go`, zapytaniu limitu i locku).
- Kryteria Automated uruchomione ponownie: `go vet ./...` + `go test ./internal/catalog/...` ok (178,7 s); `TestStrategicIndex` ok bez Dockera; `reviewedCategories` brak; metody znaczników poza `store.go`; ast-grep: `strategic_marks` w SQL tylko w `strategic.go` (`:92, :107, :134, :161, :246, :272`).
- Wiersze Manual mają dowody: 1.4 break-check (czerwony test po usunięciu `!row.Strategic`), 2.4 kontrola mechaniczna przeniesienia, 3.6 pusty `diff` JSON raportu na jednorazowej bazie (binarka 13225cf vs 9495cbe), 1.6 commit archiwizacji 85cf137.
- Spójność danych lepsza niż przed refaktorem: `Strategic` i `RetireCandidate` pochodzą teraz z jednego odczytu znaczników (wcześniej z dwóch zapytań).

## Findings

### F1 — Parametr aliasu w `activeStrategicMarkExists` wklejany do SQL

- **Severity**: 💡 OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Safety & Quality
- **Location**: backend/internal/catalog/strategic.go:271
- **Detail**: Funkcja skleja `alias` z tekstem SQL. Jedyne wywołanie (`store.go:162`) podaje stałą `"l"`, a komentarz zastrzega stałą z kodu, więc dziś nic nie jest wstrzykiwalne. Kontrakt trzyma jednak tylko komentarz.
- **Fix**: Usunąć parametr i zaszyć alias `l` (jedyny używany) albo zostawić z komentarzem.
- **Decision**: FIXED

### F2 — `ErrInvalidReference` został w `store.go`

- **Severity**: 💡 OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Pattern Consistency
- **Location**: backend/internal/catalog/store.go:19
- **Detail**: Jedynym producentem błędu jest `MarkStrategic` (dziś w `strategic.go`), a konsumentem `handlers.go:370`. Plan go nie wymieniał, więc pozostanie było zgodne z planem; błędy znaczników nie są jednak zgrupowane.
- **Fix**: Przenieść `ErrInvalidReference` z komentarzem do `strategic.go` obok pozostałych błędów znaczników.
- **Decision**: FIXED

### F3 — Test tabelaryczny odtwarza gałąź „brak kategorii” zamiast ją wywoływać

- **Severity**: 💡 OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Success Criteria
- **Location**: backend/internal/catalog/strategic_test.go:471
- **Detail**: Przypadek „bezpośrednia, bez kategorii” sprawdza warunek `CategoryID == nil` napisany w teście, nie w produkcji. Prawdziwą ścieżkę pokrywa `TestProjectReportHandler_ListsUncategorizedLibrariesWithStatus` (`project_handlers_test.go:647`), więc luka jest kosmetyczna.
- **Fix**: Zostawić; ewentualnie usunąć ten przypadek z testu indeksu, bo indeks nie zna pojęcia braku kategorii.
- **Decision**: FIXED

### F4 — Subtest porównania kolejności zależy od rodzeństwa

- **Severity**: 💡 OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Pattern Consistency
- **Location**: backend/internal/catalog/project_report_test.go:370
- **Detail**: `upload_order_does_not_change_reports` kończy się `t.Fatalf`, gdy ktoś uruchomi pojedynczy subtest przez `-run` (brak raportów z obu kolejności). Pada głośno, nie flakuje.
- **Fix**: Zamienić `t.Fatalf` na `t.Skip` z komunikatem, gdy brak raportów z obu kolejności.
- **Decision**: FIXED
