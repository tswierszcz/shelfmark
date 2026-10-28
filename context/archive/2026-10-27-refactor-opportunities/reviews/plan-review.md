<!-- PLAN-REVIEW-REPORT -->
# Plan Review: Jeden właściciel znaczników strategicznych

- **Plan**: context/changes/refactor-opportunities/plan.md
- **Mode**: Deep (weryfikacja inline, bez subagenta)
- **Date**: 2026-10-28
- **Verdict**: SOUND
- **Findings**: 0 critical, 3 warnings, 3 observations

## Verdicts

| Dimension | Verdict |
|-----------|---------|
| End-State Alignment | PASS |
| Lean Execution | PASS |
| Architectural Fitness | PASS |
| Blind Spots | WARNING |
| Plan Completeness | WARNING |

## Grounding

7/7 paths ✓, 8/8 symbols ✓, Progress↔Phase 3/3 (16 rows) ✓, brief↔plan ✓. Sprawdzone w kodzie: `CreateLibraries` przyjmuje dowolny ekosystem (`store.go:47-51`); handler grupuje wiersze raportu po `LibraryID` i pomija wiersze bez kategorii (`project_handlers.go:184-215`); brak `TestMain`, więc `go test -list` nie wymaga Dockera; `docs/reference/contract-surfaces.md` nie istnieje (sprawdzenie powierzchni pominięte).

## Findings

### F1 — Porównanie kolejności uploadów nie ma klucza ani resetu bazy

- **Severity**: ⚠️ WARNING
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Blind Spots
- **Location**: Phase 1, zmiana 1, ostatni scenariusz
- **Detail**: Dwa przebiegi na tej samej bazie dzielą biblioteki, a przebiegi na dwóch bazach mają różne `id` i `upload_batch_id`. Porównanie całego JSON-a da czerwony test bez błędu w kodzie.
- **Fix**: Między kolejnościami czyścić bazę przez `resetCatalogTables` (`json_contract_test.go:83`) i porównywać raporty po purlu.
- **Decision**: FIXED

### F2 — Ręczne porównanie „przed i po” nie ma zapisanego stanu „przed”

- **Severity**: ⚠️ WARNING
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Blind Spots
- **Location**: Phase 3, Manual 3.6
- **Detail**: Żaden krok nie zapisuje stanu sprzed fazy 3, a porównanie „na oko” jest słabe.
- **Fix**: Przed edycjami fazy 3 zapisać JSON raportu (`jq -S`) z binarki z commitu fazy 2; po fazie 3 zrobić `diff` na tej samej jednorazowej bazie.
- **Decision**: FIXED

### F3 — Kontrakt indeksu nie mówi o wierszach bez kategorii

- **Severity**: ⚠️ WARNING
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Blind Spots
- **Location**: Phase 3, zmiany 1–2
- **Detail**: Wiersze z `CategoryID == nil` istnieją. `Strategic` liczone przez `*row.CategoryID` spowoduje panic. „Nil-map panic” to zła nazwa, bo odczyt z nil-mapy jest w Go bezpieczny.
- **Fix**: Dla wiersza bez kategorii `Strategic` i `RetireCandidate` = false bez pytania indeksu; pusty indeks zwraca false.
- **Decision**: FIXED

### F4 — Hook końca tury nie uruchamia testów internal/catalog

- **Severity**: 💡 OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Plan Completeness
- **Location**: Phase 3, zmiana 4
- **Detail**: `.claude/hooks/end-of-turn.sh:78` pomija cały pakiet.
- **Fix**: Zastąpić twierdzenie zdaniem „działa bez Dockera, uruchomić jawnie”.
- **Decision**: FIXED

### F5 — Archiwizacja wchłoniętej zmiany wymaga wcześniejszego commitu

- **Severity**: 💡 OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Plan Completeness
- **Location**: Phase 1, Manual 1.6
- **Detail**: `/10x-archive` blokuje się przy niezacommitowanych zmianach w folderze zmiany i ostrzega o statusie `new`.
- **Fix**: Archiwizować po commicie fazy 1 i potwierdzić ostrzeżenia jako zmiana wchłonięta.
- **Decision**: FIXED

### F6 — Placeholder `<sha fazy 1>` w kryterium 3.1

- **Severity**: 💡 OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Plan Completeness
- **Location**: Phase 3, Automated 3.1
- **Detail**: Polecenie nie daje się uruchomić dosłownie.
- **Fix**: „<SHA z wiersza 1.1 w Progress>”.
- **Decision**: FIXED
