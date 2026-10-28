# Jeden właściciel znaczników strategicznych — Plan Brief

> Full plan: `context/changes/refactor-opportunities/plan.md`
> Research: `context/changes/refactor-opportunities/research.md`

## What & Why

Refaktor nr 1 z rankingu L4: tabela `strategic_marks` i jej reguły („strategiczna”, „kategoria przejrzana”, „kandydat do wycofania”) dostają jednego właściciela, plik `strategic.go`. To jedyny twardy niezmiennik domeny, naprawiany już dwa razy. Jego reguły rozjechały się między limitem (per ekosystem) a raportem (per kategoria). Decyzja produktowa K1 (FR-007) ma później zmienić jedno miejsce, nie trzy.

## Starting Point

Znaczniki czytają trzy miejsca: metody znaczników w `store.go:369-521`, raport (własny JOIN `project.go:201` i prywatna `reviewedCategories` `project.go:242`) oraz filtr `ListLibraries` (`store.go:187`). Testy raportu obejmują tylko jeden projekt. Faza 3 test-planu (ryzyko #4) jest wstrzymana w osobnej zmianie.

## Desired End State

`strategic.go` jest jedynym plikiem produkcyjnym z SQL-em na `strategic_marks`. Raport bierze wszystkie trzy predykaty z jednego indeksu znaczników. Zachowanie raportu jest identyczne, potwierdzone testami z wieloma projektami, które jednocześnie zamykają Fazę 3 test-planu.

## Key Decisions Made

| Decision | Choice | Why (1 sentence) | Source |
| --- | --- | --- | --- |
| Który refaktor | #1 jeden właściciel znaczników | Największy dług w strefie ryzyka 2, przygotowuje grunt pod K1, a pierwszy krok jest mały | Research / Plan |
| Relacja z Fazą 3 test-planu | Faza 1 = cała Faza 3 (ryzyko #4), wstrzymana zmiana archiwizowana jako wchłonięta | Jeden fixture i jedna zmiana zamiast podwójnej pracy | Plan |
| Kształt właściciela | Indeks aktywnych znaczników w Go (jedno zapytanie), raport bez JOIN-a | Reguła w jednej testowalnej funkcji; zbiór ograniczony limitem 2/kategoria/ekosystem | Plan |
| Filtr `ListLibraries` | Predykat SQL dostarczany przez `strategic.go` | Filtr musi zostać w SQL, ale jego treść ma jednego właściciela | Plan |
| Semantyka ekosystemu („przejrzana”) | Test charakteryzujący utrwala dzisiejsze globalne zachowanie, bez rozstrzygania | K1 to decyzja produktowa dla L5; refaktor nie może jej zmienić po cichu | Research / Plan |
| Kolejność | Testy → przeniesienie 1:1 → zmiana odczytów | Review oddziela „przeniesione” od „zmienione”; każda faza osobno odwracalna | Research |

## Scope

**In scope:**
- Testy raportu z wieloma projektami (6 scenariuszy, obie kolejności uploadów) i test charakteryzujący ekosystem
- `strategic.go` / `strategic_test.go`: przeniesienie kodu i testów znaczników
- Indeks znaczników dla raportu, predykat dla `ListLibraries`, usunięcie `reviewedCategories`
- Aktualizacja `test-plan.md` (Faza 3 `complete`, cookbook §6.3) i archiwizacja wchłoniętej zmiany

**Out of scope:**
- Decyzja K1 (FR-007 per ekosystem) i K4 (członkostwo w paczce): idą do L5
- Pełny podział `Store` na repozytoria i interfejsy
- Kontrakt HTTP/JSON, typy TS, migracje, widok SQL
- Zmiany w zapytaniu limitu, advisory locku i test seams

## Architecture / Approach

`strategic.go` (ten sam pakiet, ten sam `*Store`) dostaje metodę, która jednym zapytaniem buduje indeks aktywnych par (biblioteka, kategoria). Indeks odpowiada na trzy pytania: strategiczna, przejrzana, kandydat do wycofania. `ProjectReport` pobiera kategorie i `is_direct` jak dotąd, a flagi bierze z indeksu. `ListLibraries` wkleja predykat `EXISTS` zwrócony przez `strategic.go`. Zapis znaczników (`MarkStrategic` z limitem i lockiem) tylko zmienia plik.

## Phases at a Glance

| Phase | What it delivers | Key risk |
| --- | --- | --- |
| 1. Testy charakteryzujące raportu | Ryzyko #4 pokryte; osłona dla refaktoru; test-plan Faza 3 `complete` | Testy zielone od razu, więc trzeba je raz zobaczyć na czerwono |
| 2. Wydzielenie `strategic.go` | Kod i testy znaczników w osobnych plikach, bez zmian treści | Przypadkowa zmiana przy przenoszeniu (diff `--color-moved`) |
| 3. Jeden właściciel reguł | Indeks w Go, raport bez JOIN-a, predykat filtra, `strategic_marks` tylko w `strategic.go` | Zmiana wyniku raportu: pilnują jej testy z fazy 1 |

**Prerequisites:** Docker do testów `internal/catalog` (~3 min); kod bez zmian od c2340d0.
**Estimated effort:** ~2 sesje, 3 fazy, każda z osobnym commitem.

## Open Risks & Assumptions

- Testy charakteryzujące utrwalają zachowanie, które może być błędem produktowym (K1). Są nazwane jako charakteryzujące, a L5 zmieni je świadomie.
- Zakładamy, że `libraries.ecosystem` się nie zmienia (`store.go:409-410`). Refaktor tego założenia nie rusza.

## Success Criteria (Summary)

- `grep`/ast-grep: `strategic_marks` w SQL tylko w `strategic.go`; `reviewedCategories` nie istnieje.
- Testy z wieloma projektami z fazy 1 przechodzą bez zmian po fazie 3.
- JSON raportu z jednorazowej instancji jest identyczny przed fazą 3 i po niej (`diff` pusty), a widok `/redundancy` wygląda tak samo.
