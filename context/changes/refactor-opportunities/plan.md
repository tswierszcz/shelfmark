# Jeden właściciel znaczników strategicznych — plan implementacji

## Overview

Refaktor nr 1 z rankingu `research.md`. Tabela `strategic_marks` i jej reguły („strategiczna”, „kategoria przejrzana”, „kandydat do wycofania”, zakres limitu) mają jednego właściciela: plik `strategic.go` w pakiecie `catalog`. Zachowanie się nie zmienia. Zanim cokolwiek przeniesiemy, dzisiejsze zachowanie raportu utrwalają testy z wieloma projektami. Te testy to jednocześnie cała Faza 3 test-planu (ryzyko #4). Dzięki temu decyzja produktowa K1 (FR-007: czy „przejrzana” liczy się per ekosystem), podejmowana w L5, zmieni jedną funkcję i jeden test charakteryzujący.

## Current State Analysis

- Wiedza o znacznikach jest rozproszona w trzech miejscach pakietu `catalog`:
  - metody znaczników w `store.go:369-521` (`MarkStrategic`, `UnmarkStrategic`, `StrategicMarksByLibrary`, test seams `afterCapCheck`, `strategicLockTimeout`);
  - raport: własny `LEFT JOIN strategic_marks` (`project.go:201-209`), prywatna metoda `reviewedCategories` (`project.go:242-264`) i reguła kandydata do wycofania w pętli skanowania (`project.go:227`);
  - filtr `Strategic` w `ListLibraries`: własne `EXISTS` (`store.go:186-193`).
- Weryfikacja R3: pięć literałów SQL czyta `strategic_marks` (`store.go:187, 425, 494`, `project.go:201, 244`), dwa piszą (`store.go:441, 468`).
- Limit liczy się per (kategoria, ekosystem) (`store.go:391-428`). „Przejrzana” liczy się globalnie per kategoria (`project.go:244`). Tego rozjazdu nie rozstrzygamy, utrwalamy go testem (K1 → L5).
- Typy i błędy znaczników leżą w `store.go`: `ErrStrategicMarkNotFound` (:21), `ErrStrategicCapReached` (:31), `ErrStrategicMarkBusy` (:36), `StrategicMark` (:80-85).
- Testy raportu mają jeden projekt (`setUpProjectReportFixture`, `project_handlers_test.go:412`). Jedyny test raportu ze znacznikiem to `project_handlers_test.go:437` (V7). Nie istnieje test z wieloma projektami ani z odwrotną kolejnością uploadów (V6).
- Testy znaczników w `store_test.go`: 270, 320, 372, 400, 446, 703, 779, 881, 914; filtr: `store_test.go:508`; handlery: `handlers_test.go:476, 544, 632, 787, 828`.

## Desired End State

- `backend/internal/catalog/strategic.go` jest jedynym plikiem produkcyjnym z literałem SQL zawierającym `strategic_marks`.
- Raport bierze „strategiczna”, „przejrzana” i „kandydat do wycofania” z jednego indeksu znaczników zbudowanego w `strategic.go`. Nie ma już JOIN-a do `strategic_marks` ani `reviewedCategories`.
- `ListLibraries` filtruje predykatem dostarczonym przez `strategic.go`.
- Testy z wieloma projektami (ryzyko #4) i test charakteryzujący ekosystem przechodzą przed refaktorem i po nim bez zmian.
- Faza 3 test-planu ma status `complete`, a zmiana `testing-redundancy-report-correctness` jest zarchiwizowana jako wchłonięta.

Weryfikacja: `go test ./internal/catalog/...` na zielono po każdej fazie, plus kontrola ast-grep (Faza 3).

### Key Discoveries:

- Raport już dziś wykonuje dwa zapytania bez wspólnego snapshotu (`reviewedCategories`, potem raport, `project.go:195-209`). Indeks w Go nie pogarsza więc spójności, a „strategiczna” i „przejrzana” pochodzą z tego samego odczytu.
- Zbiór aktywnych znaczników jest ograniczony limitem: najwyżej 2 × kategorie × ekosystemy wierszy. Wczytanie go w całości jest tanie.
- `CreateLibraries` przyjmuje dowolne `Ecosystem` (`NewLibrary`, `store.go:47-51`). Biblioteka spoza Mavena powstaje więc z pominięciem `ParsePurl`, czyli bez strażnika z `purl.go:19`.
- Wzorzec testu handlera: `doProjectUpload` (`project_handlers_test.go:121`), `AttachCategory(..., "manual")` zamiast LLM, `ProjectReportHandler` + `req.SetPathValue("projectID", …)` (`:437-490`).

## What We're NOT Doing

- Nie rozstrzygamy K1 (czy „przejrzana” / „wybór strategiczny” jest per ekosystem). Test charakteryzujący utrwala dzisiejsze zachowanie, a decyzja należy do L5.
- Nie dzielimy `Store` na repozytoria ani interfejsy (K2 w pełni). `strategic.go` to plik w tym samym pakiecie z metodami na tym samym `*Store`.
- Nie zmieniamy kontraktu HTTP, JSON ani typów TS. Nie dodajemy migracji ani widoku SQL.
- Nie ruszamy zapytania limitu w `MarkStrategic` poza przeniesieniem pliku. Advisory lock, `lock_timeout` i test seams zostają bez zmian.
- Nie zmieniamy członkostwa w paczce (K4) ani `isBatchSettled`.

## Implementation Approach

Kolejność to: udokumentuj zachowanie → przenieś bez zmian → zmień kształt odczytów. Każda faza kończy się zielonym `go test` i osobnym commitem, więc każdą da się osobno wycofać. Faza 1 nie dotyka kodu produkcyjnego, a jej testy są osłoną dla faz 2 i 3. Faza 2 to czyste przeniesienie, które pozwala oddzielić w review „co przeniesiono” od „co zmieniono”. Faza 3 zmienia zapytania i jako jedyna niesie ryzyko zmiany zachowania. Pilnują go testy z fazy 1.

## Critical Implementation Details

- **Dowód, że testy coś chronią.** Testy z fazy 1 przechodzą od razu, bo utrwalają istniejące zachowanie. Zgodnie z cookbookiem test-planu (§6.1) każdy nowy test trzeba raz zobaczyć na czerwono: celowo popsuć produkcyjną linię (np. usunąć `!row.Strategic` albo zawęzić `reviewedCategories` do projektu), zobaczyć czerwony test, przywrócić. Nie commitować zepsutego stanu.
- **Testy z Dockerem.** `internal/catalog` uruchamia Postgresa w kontenerze (~3 min), a hook końca tury pomija ten pakiet. Testy trzeba więc uruchomić jawnie.

## Phase 1: Testy charakteryzujące raportu (Faza 3 test-planu)

### Overview

Utrwalamy dzisiejsze zachowanie raportu przy wielu projektach i przy znacznikach ustawionych przez `MarkStrategic`. Bez zmian w kodzie produkcyjnym. Ta faza domyka Fazę 3 test-planu (ryzyko #4).

### Changes Required:

#### 1. Testy raportu z wieloma projektami

**File**: `backend/internal/catalog/project_report_test.go` (nowy)

**Intent**: Pokryć ryzyko #4: przy N bezpośrednich zależnościach w jednej kategorii raport oznacza jako kandydatów do wycofania dokładnie te bez znacznika, niezależnie od kolejności uploadów między projektami. Odrzucamy założenie „jeden projekt wystarczy”.

**Contract**: Testy na poziomie `ProjectReportHandler` (prawdziwy Postgres, `startTestPostgres`). Fixture: dwa projekty z co najmniej trzema wspólnymi albo częściowo wspólnymi bibliotekami Maven. Co najmniej jedna biblioteka jest bezpośrednia w jednym projekcie i przechodnia w drugim. Biblioteki dostają kategorie przez `AttachCategory(..., "manual")`, a znaczniki przez `MarkStrategic`. Scenariusze (podtesty, każdy z oczekiwanym zbiorem `retire_candidate` per projekt):
- kategoria z jednym znacznikiem: kandydatami są dokładnie bezpośrednie biblioteki bez znacznika, w każdym projekcie;
- biblioteka ze znacznikiem przechodnia w drugim projekcie: nie jest kandydatem, a `strategic` = true;
- kategoria przejrzana znacznikiem na bibliotece spoza danego projektu: bezpośrednie biblioteki tego projektu w tej kategorii są kandydatami („przejrzana” jest globalna między projektami);
- kategoria bez żadnego aktywnego znacznika: zero kandydatów;
- po `UnmarkStrategic` ostatniego znacznika w kategorii kandydaci znikają;
- kolejność uploadów A→B i B→A daje identyczne raporty obu projektów. Między kolejnościami baza jest czyszczona przez `resetCatalogTables` (`json_contract_test.go:83`). Raporty porównuje się po purlu (`is_direct`, kategorie, `strategic` i `retire_candidate` per kategoria), nie jako cały JSON, bo `id` i `upload_batch_id` różnią się między przebiegami.

Pomocnik fixture'a może być nowy albo rozszerzać `setUpProjectReportFixture`. Istniejący test `:437` zostaje bez zmian.

#### 2. Test charakteryzujący semantykę ekosystemu

**File**: `backend/internal/catalog/project_report_test.go`

**Intent**: Utrwalić dzisiejsze zachowanie: kategoria jest „przejrzana”, gdy ma aktywny znacznik w dowolnym ekosystemie, choć limit liczy się per ekosystem. Dzięki temu refaktor nie zmieni tego po cichu, a decyzja K1 w L5 będzie jawną zmianą tego testu.

**Contract**: Biblioteka z `Ecosystem: "npm"` utworzona przez `CreateLibraries` (z pominięciem `ParsePurl`), przypięta do kategorii i oznaczona przez `MarkStrategic`. Bezpośrednia biblioteka Maven w projekcie w tej samej kategorii jest dziś kandydatem do wycofania. Nazwa testu zawiera `Characterization`. Komentarz odsyła do K1 / FR-007 i `context/changes/refactor-opportunities/research.md`, a nie podaje tego zachowania jako wymagania.

#### 3. Test-plan i wchłonięta zmiana

**File**: `context/foundation/test-plan.md`, `context/changes/testing-redundancy-report-correctness/change.md`

**Intent**: Zapisać, że Faza 3 test-planu została zrealizowana w tej zmianie, i dopisać wzorzec testu z wieloma projektami do cookbooka.

**Contract**: Wiersz 3 w §3: status `complete`, kolumna zmiany wskazuje `context/changes/refactor-opportunities/`. §6.3 zamiast „TBD”: 3–5 punktów wzorca (fixture z dwoma projektami przez `doProjectUpload`, kategorie przez `AttachCategory(..., "manual")`, znaczniki przez `MarkStrategic`, obie kolejności uploadów, asercja zbioru kandydatów per projekt). W `change.md` wstrzymanej zmiany dopisać notatkę „Absorbed by refactor-opportunities Phase 1” (status bez zmian, archiwizacja to wiersz Manual).

### Success Criteria:

#### Automated Verification:

- Testy z wieloma projektami i test charakteryzujący przechodzą: `cd backend && go test ./internal/catalog/ -run 'TestProjectReport' -count=1`
- Cały pakiet i vet przechodzą: `cd backend && go vet ./... && go test ./internal/catalog/... -count=1`
- Brak zmian w kodzie produkcyjnym: `git diff --stat HEAD -- backend/internal/catalog ':!*_test.go'` jest puste

#### Manual Verification:

- Dowód, że testy coś chronią: celowe zepsucie reguły raportu (np. usunięcie `!row.Strategic` w `project.go:227`) daje czerwony test z wieloma projektami; po przywróceniu jest zielono
- Przegląd scenariuszy testu względem ryzyka #4 w `test-plan.md:62` (kolejność uploadów, kategoria bez znaczników, agregacja między projektami)
- Wchłonięta zmiana zarchiwizowana: `/10x-archive testing-redundancy-report-correctness`, uruchomione po commicie fazy 1 (archive blokuje się przy niezacommitowanych zmianach w folderze). Ostrzeżenia (status `new`, brak planu i impl-review) potwierdzić jako zmiana wchłonięta

**Implementation Note**: Po fazie 1 zatrzymaj się na potwierdzenie wierszy Manual przed przejściem do fazy 2.

---

## Phase 2: Wydzielenie `strategic.go`

### Overview

Czyste przeniesienie wszystkiego, co dotyczy znaczników, z `store.go` do nowego pliku. Bez zmiany ani jednej linii logiki.

### Changes Required:

#### 1. Kod znaczników

**File**: `backend/internal/catalog/strategic.go` (nowy), `backend/internal/catalog/store.go`

**Intent**: Dać znacznikom własny plik, w którym w fazie 3 zamieszkają wszystkie odczyty i reguły.

**Contract**: Do `strategic.go` przechodzą 1:1, razem z komentarzami: `ErrStrategicMarkNotFound`, `ErrStrategicCapReached`, `ErrStrategicMarkBusy`, `StrategicMark`, `afterCapCheck`, `strategicLockTimeout`, `MarkStrategic`, `UnmarkStrategic`, `StrategicMarksByLibrary`. Bez zmian sygnatur i nazw. Importy w obu plikach uporządkowane przez `gofmt`/`goimports`.

#### 2. Testy znaczników

**File**: `backend/internal/catalog/strategic_test.go` (nowy), `backend/internal/catalog/store_test.go`

**Intent**: Trzymać testy obok przeniesionego kodu.

**Contract**: Przenieść 1:1 `TestMarkStrategic_*`, `TestUnmarkStrategic_*`, `TestStrategicMarksByLibrary_*`. `TestListLibraries_StrategicFilter` zostaje w `store_test.go`. Wspólne pomocniki testowe zostają tam, gdzie są.

### Success Criteria:

#### Automated Verification:

- Ten sam zbiór testów przed i po fazie: `cd backend && go test ./internal/catalog/ -list '.*' | sort` daje identyczny wynik jak przed fazą
- Cały pakiet i vet przechodzą: `cd backend && go vet ./... && go test ./internal/catalog/... -count=1`
- `store.go` nie zawiera już `MarkStrategic`, `UnmarkStrategic` ani `StrategicMarksByLibrary`: `grep -nE 'func \(s \*Store\) (Mark|Unmark)Strategic|StrategicMarksByLibrary\(' backend/internal/catalog/store.go` jest puste

#### Manual Verification:

- Przegląd diffu: tylko przeniesione bloki, bez zmian w treści (np. `git diff --color-moved=dimmed-zebra`)

**Implementation Note**: Po fazie 2 zatrzymaj się na potwierdzenie przeglądu diffu.

---

## Phase 3: Jeden właściciel reguł znaczników

### Overview

Raport i filtr `ListLibraries` przestają czytać `strategic_marks` samodzielnie. Pytają `strategic.go`.

### Changes Required:

#### 1. Indeks aktywnych znaczników

**File**: `backend/internal/catalog/strategic.go`

**Intent**: Jedno miejsce, które odpowiada na pytania: „czy biblioteka jest strategiczna w kategorii”, „czy kategoria jest przejrzana” i „czy bezpośrednia biblioteka jest kandydatem do wycofania”. Decyzja K1 zmieni w przyszłości tylko to miejsce.

**Contract**: Niewyeksportowany typ indeksu i metoda `Store` budująca go jednym zapytaniem o aktywne pary (biblioteka, kategoria). Indeks udostępnia trzy predykaty: strategiczna(libID, catID), przejrzana(catID), kandydat do wycofania(libID, catID, isDirect). Ostatni koduje dzisiejszą regułę z `project.go:227`: kategoria jest znana, biblioteka bezpośrednia, kategoria przejrzana i para nie jest strategiczna. Dla wiersza raportu bez kategorii (`CategoryID == nil`) `Strategic` i `RetireCandidate` są `false` bez pytania indeksu. „Przejrzana” znaczy dziś „co najmniej jeden aktywny znacznik w kategorii, w dowolnym ekosystemie”, a komentarz przy predykacie odsyła do K1. Pusty indeks (brak aktywnych znaczników) zwraca `false` dla każdego zapytania.

#### 2. Raport przez indeks

**File**: `backend/internal/catalog/project.go`

**Intent**: Usunąć z raportu bezpośrednią wiedzę o `strategic_marks`.

**Contract**: `ProjectReport` buduje indeks przez metodę z `strategic.go`. Zapytanie raportu traci `LEFT JOIN strategic_marks` i kolumnę `strategic`. `Strategic` i `RetireCandidate` w `ProjectReportRow` ustawiają predykaty indeksu. `reviewedCategories` zostaje usunięta. Komentarz nad `ProjectReport` opisuje regułę przez odesłanie do indeksu. Kształt `[]ProjectReportRow`, kolejność wierszy (`ORDER BY l.purl, c.name`) i wiersze bez kategorii bez zmian.

#### 3. Predykat filtra

**File**: `backend/internal/catalog/strategic.go`, `backend/internal/catalog/store.go`

**Intent**: Usunąć ostatni literał `strategic_marks` spoza `strategic.go`.

**Contract**: Niewyeksportowana funkcja w `strategic.go` zwraca predykat SQL „biblioteka o aliasie X ma aktywny znacznik” (dziś `EXISTS (SELECT 1 FROM strategic_marks sm WHERE sm.library_id = l.id AND sm.active)`). `ListLibraries` woła ją z aliasem `l` dla obu gałęzi (`Strategic` true/false). Alias jest stałą z kodu, nie wejściem użytkownika.

#### 4. Test jednostkowy indeksu

**File**: `backend/internal/catalog/strategic_test.go`

**Intent**: Sprawdzić regułę kandydata do wycofania bez Dockera, tabelą przypadków.

**Contract**: Test tabelaryczny na indeksie zbudowanym z par w pamięci: bezpośrednia/przechodnia × strategiczna/nie × kategoria przejrzana/nie × brak kategorii, plus pusty indeks. Nie wymaga Postgresa: działa bez Dockera w kilka sekund (`go test ./internal/catalog/ -run TestStrategicIndex`). Hook końca tury pomija cały pakiet `internal/catalog` (`.claude/hooks/end-of-turn.sh:78`), więc test trzeba uruchomić jawnie.

### Success Criteria:

#### Automated Verification:

- Testy z fazy 1 przechodzą bez zmian w ich plikach: `git diff --stat <SHA z wiersza 1.1 w Progress> -- backend/internal/catalog/project_report_test.go` jest puste, a `cd backend && go test ./internal/catalog/ -run 'TestProjectReport' -count=1` przechodzi
- Test jednostkowy indeksu przechodzi: `cd backend && go test ./internal/catalog/ -run 'TestStrategicIndex' -count=1`
- Cały pakiet i vet przechodzą: `cd backend && go vet ./... && go test ./internal/catalog/... -count=1`
- `strategic_marks` w literałach SQL tylko w `strategic.go`: reguła ast-grep (string literal, `regex: strategic_marks`) na `backend/internal/catalog` bez `*_test.go` zwraca trafienia wyłącznie w `strategic.go` (poza atrybutem logu `handlers.go:272`, który nie jest SQL-em)
- `reviewedCategories` nie istnieje: `grep -rn reviewedCategories backend/` jest puste

#### Manual Verification:

- Raport w działającej aplikacji wygląda tak samo jak przed zmianą. Przed edycjami fazy 3 zapisać `GET /api/projects/{id}/report` (`jq -S`) z binarki zbudowanej na commicie fazy 2. Po fazie 3 zrobić `diff` na tej samej jednorazowej bazie (diff pusty) i dopiero potem obejrzeć widok. Lokalna instancja na jednorazowej bazie, `docs/samples/smoke-sbom.cdx.json` w dwóch projektach, jeden znacznik strategiczny, porównanie widoku `/redundancy` przed refaktorem i po nim (pary jackson-databind/gson i guava/commons-lang3)

**Implementation Note**: Po fazie 3 zatrzymaj się na potwierdzenie wiersza Manual. Następny krok w łańcuchu to `/10x-impl-review refactor-opportunities`.

---

## Testing Strategy

### Unit Tests:

- Tabela przypadków indeksu znaczników (faza 3): każda kombinacja bezpośrednia/strategiczna/przejrzana oraz brak kategorii i pusty indeks.

### Integration Tests:

- Raport z wieloma projektami (faza 1): sześć scenariuszy ryzyka #4, obie kolejności uploadów.
- Test charakteryzujący ekosystem (faza 1): dzisiejsza globalna semantyka „przejrzana”.
- Istniejące: `store_test.go` (limit, współbieżność, lock timeout), `handlers_test.go` (409, 503, role), `project_handlers_test.go:437`.

### Manual Testing Steps:

1. Faza 1: celowo zepsuć regułę raportu, zobaczyć czerwony test, przywrócić.
2. Faza 2: przejrzeć diff jako przeniesienie (`--color-moved`).
3. Faza 3: przed edycjami zapisać JSON raportu z binarki z commitu fazy 2, po fazie 3 zrobić `diff` na tej samej jednorazowej bazie, potem obejrzeć `/redundancy`.

## Performance Considerations

Raport zamiast JOIN-a wczytuje wszystkie aktywne znaczniki. Liczbę wierszy ogranicza limit (2 na kategorię i ekosystem), więc jest to rząd dziesiątek. Liczba zapytań raportu się nie zmienia: dwa przed i dwa po (indeks zamiast `reviewedCategories`).

## Migration Notes

Brak migracji i zmian w schemacie. Każda faza to osobny commit i da się ją wycofać `git revert` bez wpływu na dane.

## References

- Research: `context/changes/refactor-opportunities/research.md` (ranking #1, weryfikacja R1–R18)
- Kandydaci: `context/changes/refactor-opportunities/candidates.md`
- Analiza przepływu: `context/changes/redundancy-flow-analysis/research.md` (T1, T5, V2, V4, V6, V7)
- Test-plan: `context/foundation/test-plan.md` (ryzyko #4 `:45, :62`, Faza 3 `:77`, cookbook §6.2–6.3)
- Wstrzymana zmiana: `context/changes/testing-redundancy-report-correctness/change.md`
- Kod: `store.go:159-227, 369-521`, `project.go:186-264`, `project_handlers_test.go:412-545`

## Progress

> Convention: `- [ ]` pending, `- [x]` done. Append ` — <commit sha>` when a step lands. Do not rename step titles. See `references/progress-format.md`.

### Phase 1: Testy charakteryzujące raportu (Faza 3 test-planu)

#### Automated

- [x] 1.1 Testy z wieloma projektami i test charakteryzujący przechodzą — 86dccdd
- [x] 1.2 Cały pakiet i vet przechodzą — 86dccdd
- [x] 1.3 Brak zmian w kodzie produkcyjnym — 86dccdd

#### Manual

- [x] 1.4 Dowód, że testy coś chronią (zepsucie reguły → czerwony test) — 86dccdd
- [x] 1.5 Przegląd scenariuszy testu względem ryzyka #4 — 86dccdd
- [x] 1.6 Wchłonięta zmiana zarchiwizowana — 85cf137

### Phase 2: Wydzielenie strategic.go

#### Automated

- [x] 2.1 Ten sam zbiór testów przed i po fazie — 13225cf
- [x] 2.2 Cały pakiet i vet przechodzą — 13225cf
- [x] 2.3 store.go nie zawiera metod znaczników — 13225cf

#### Manual

- [x] 2.4 Przegląd diffu: tylko przeniesione bloki — 13225cf

### Phase 3: Jeden właściciel reguł znaczników

#### Automated

- [x] 3.1 Testy z fazy 1 przechodzą bez zmian w ich plikach
- [x] 3.2 Test jednostkowy indeksu przechodzi
- [x] 3.3 Cały pakiet i vet przechodzą
- [x] 3.4 strategic_marks w literałach SQL tylko w strategic.go
- [x] 3.5 reviewedCategories nie istnieje

#### Manual

- [x] 3.6 Raport w działającej aplikacji wygląda tak samo jak przed zmianą
