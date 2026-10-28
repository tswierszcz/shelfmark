---
date: 2026-10-27T23:55:00+01:00
researcher: Claude Opus 5.5 (Claude Code), dla Tomasza Świerszcza
git_commit: 8bcf698
branch: shape/shelfmark
repository: tswierszcz/shelfmark
topic: "Refactor opportunities: które problemy z analizy przepływu raportu redundancji warto naprawić, w jakim kształcie i kolejności"
tags: [research, refactor-opportunities, catalog-store, http-contract, strategic-marks, m4-l4]
status: complete
last_updated: 2026-10-27
last_updated_by: Claude Opus 5.5 (Claude Code)
last_updated_note: "Weryfikacja twierdzeń strukturalnych rankingu ast-grep 0.51.0 na c2340d0 (kod bez zmian od 8bcf698); korekty w miejscu w formacie 'X (raport: Y)'; ranking bez zmian"
verification_commit: c2340d0
---

# Research: refactor opportunities

**Data**: 2026-10-27 · **Commit**: 8bcf698 · **Gałąź**: shape/shelfmark · **Repozytorium**: tswierszcz/shelfmark

## Pytanie badawcze

Które z problemów zapisanych w `context/changes/redundancy-flow-analysis/research.md` warto naprawić, w jakim docelowym kształcie i w jakiej kolejności? Etap eksploracji: żadnego refaktoru, żadnej decyzji. Wynik to ranking propozycji dla osobnej sesji planowania.

Priory (traktowane jako zebrane dowody, nie wyprowadzane na nowo): `context/changes/redundancy-flow-analysis/research.md` (T1–T12, weryfikacja V1–V17), `context/map/repo-map.md` (strefy ryzyka 1–2), `context/map/evidence/3-structure.md`.

Oznaczenia: **[D]** dowód (kod, dokument, commit), **[I]** wniosek, **[?]** nieznane.

## Lista i klasyfikacja kandydatów

Pełna tabela z uzasadnieniem: `context/changes/refactor-opportunities/candidates.md`.

| Id | Problem | Klasyfikacja |
|---|---|---|
| K1 | Zakres „wyboru strategicznego”: limit per (kategoria, ekosystem), raport per kategoria | KANDYDAT → **przeprojektowanie pojęcia biznesowego** (patrz niżej) |
| K2 | Jeden `Store` dla pięciu możliwości; te same tabele z `store.go` i `project.go`; raport czyta `strategic_marks` z pominięciem metod znaczników | KANDYDAT |
| K3 | Kontrakt HTTP Go ↔ TS trzymany konwencją | KANDYDAT |
| K4 | Status paczki liczy tylko nowe purle; ponowne kolejkowanie | KANDYDAT → **przeprojektowanie pojęcia biznesowego** |
| K5 | Zduplikowana logika (parser SBOM, polling i `isBatchSettled`, `errorMessage`) | KANDYDAT (trzy grupy) |
| K6 | Granice plików w pakiecie `catalog` | KANDYDAT |
| K7 | Upload bez wspólnej transakcji | KANDYDAT |
| K8 | Dwa znaczenia `is_direct` | KANDYDAT (mały) |
| W1–W6 | Brak testu wieloprojektowego, e2e poza CI, nieaktualne komentarze, nazwa `created`, brak rejestru kontraktów | WEJŚCIA (koszt i osłony) |

Metoda: trzech subagentów w trybie eksploracji (obecny kształt; historia i intencjonalność; wykonalność migracji). Rozstrzygające cytaty z archiwum i kod (`catalog.tsx:19-27`, `redundancy.tsx:22-28`, `archive/*-empty-batch-status/plan.md:20`, `archive/*-mark-strategic-libraries/plan-brief.md:25`, `archive/*-project-redundancy-report/plan.md:21, 32, 45, 164`, `archive/*-data-layer-connectivity/plan.md:11`) sprawdził główny agent.

## Korekta priorów

- **[D] `isBatchSettled` to nie kopia, tylko dwie semantyki.** Dla pustej paczki `{}` `catalog.tsx:21-27` zwraca `false`, a `redundancy.tsx:26-28` zwraca `true` (komentarz `redundancy.tsx:22-25`). Weryfikacja V14 w researchu L3 nazwała je kopiami, bo wzorzec ast-grep porównywał sygnatury, nie ciała. Do decyzji na etapie planowania; raport L3 zostaje bez zmian.

## Kandydaci

### K1. Zakres wyboru strategicznego

**Obecny kształt.**
- [D] Limit liczony per (kategoria, ekosystem): ekosystem odczytany w `store.go:391-395`, advisory lock na `kategoria:ekosystem` (`store.go:406-414`), `l.ecosystem = $2` (`store.go:425-428`).
- [D] Raport per kategoria: `project.go:207-208`, `reviewedCategories` `project.go:242-244`.
- [D] Schemat nie zna ekosystemu jako wymiaru pojęcia. `strategic_marks` ma PK `(library_id, category_id)` (migracja 0003); `categories` to jedna globalna taksonomia (0002); ekosystem to atrybut biblioteki.
- [D] `store.go:409-410` dokumentuje ukryty niezmiennik: limit jest poprawny, dopóki `libraries.ecosystem` się nie zmienia.

**Historia i intencjonalność.** Werdykt: **świadoma decyzja, której przesłanka rozpadła się dzień później.**
- [D] 10-17/18: S-02 bez limitu: „Unbounded — no limit enforced … deviates from PRD FR-007” (`archive/2026-10-17-mark-strategic-libraries/plan-brief.md:25`).
- [D] 10-19: plan raportu: „catalog is Maven-only … no additional ecosystem scoping is needed” (`archive/2026-10-19-project-redundancy-report/plan.md:21`).
- [D] 10-20: commit 74daf0f dodał limit per ekosystem; ani jego plan, ani review nie wspominają `ProjectReport`.
- [D] PRD od początku mówi per ekosystem (`context/foundation/prd.md:73`, FR-007), a „tylko Maven” to zakres MVP (`prd.md:80-81, 107`).

**Wykonalność.** [I] Zmiana kodu jest mała (S: jeden warunek SQL i klucz złożony w mapie, bez migracji, bez zmiany szwu Go ↔ TS, bo `ecosystem` już jest w JSON raportu, `projects.ts:21`). Pierwszy krok: test charakteryzujący, który zasila bibliotekę spoza Mavena przez `CreateLibraries` (omija `ParsePurl`) i utrwala dzisiejsze zachowanie.

**Werdykt badania: STOP — to przeprojektowanie pojęcia biznesowego.** Czy „kategoria przejrzana” i „wybór strategiczny” są per kategoria czy per (kategoria, ekosystem), rozstrzyga produkt (FR-007), nie struktura kodu. To przedmiot analizy domenowej (L5), nie rankingu refaktorów.

### K2. Wspólny `Store` i odczyty `strategic_marks` z wielu miejsc

**Obecny kształt.**
- [D] Jeden typ `Store{pool}` (`store.go:94-101`) z 18 metodami: 11 w `store.go`, 7 w `project.go`. Bez interfejsu (V8).
- [D] 13 funkcji produkcyjnych przyjmuje `*Store` wprost: 11 handlerów w `handlers.go` i `project_handlers.go`, `NewQueue` `queue.go:71`, `ProcessLibrary` `pipeline.go:35` (raport: 14).
- [D] Pięć literałów SQL czytających `strategic_marks` w pięciu metodach: `StrategicMarksByLibrary` (`store.go:494`), zapytanie limitu w `MarkStrategic` (`store.go:425`), filtr `Strategic` w `ListLibraries` (`store.go:187`) i SQL raportu (`project.go:201, 244`); poza blokiem metod znaczników (`store.go:369-521`) trzy: `store.go:187`, `project.go:201, 244` (raport: trzy ścieżki odczytu, bez zapytania limitu).
- [D] Dwa równoległe sposoby „rozwiązania bibliotek po insercie”: `ListLibraries(BatchID)` (`handlers.go:132`; raport: `:131`) i `LibrariesByPurls` (`project.go:73`, powód w komentarzu `project.go:68-72`).

**Historia i intencjonalność.** Werdykt: **świadoma konwencja; koncentracja w jednym typie narosła, nie została zaprojektowana.**
- [D] „a `Store` struct wrapping `*pgxpool.Pool`” (`archive/2026-10-16-seed-classification-catalog/plan.md:75`); interfejsy tylko dla klientów zewnętrznych (`plan.md:39`).
- [D] Testy na prawdziwym Postgresie z wyboru (`archive/2026-10-16-data-layer-connectivity/plan-brief.md:27`), więc brak interfejsu nie był kosztem.
- [D] `project.go` jako drugi plik na tym samym `Store` „matching `Store`'s existing convention” (`archive/2026-10-19-project-redundancy-report/plan.md:85`); bezpośredni odczyt `strategic_marks` zaplanowany (`plan.md:95`).
- [I] Żaden dokument nie rozważa podziału `Store` per możliwość.

**Wykonalność.**
- [D] Zasięg: `store.go` Ca 8, Ce 0 (`context/map/evidence/3-structure.md`); 11 handlerów, `main.go:42`, 66 konstrukcji `NewStore` w 6 plikach testowych (V8).
- [D] Osłony: testy store i handlerów na Postgresie, dobre per metoda; brak testu „znacznik ustawiony przez `MarkStrategic` → widoczny w raporcie” poza jednym przypadkiem (`project_handlers_test.go:456`).
- [I] Pierwszy krok (szew, nie refaktor): przenieść odczyty `strategic_marks` w raporcie (`project.go:207-208, 242-264`) za nazwaną metodę w zakresie znaczników (`store.go:384-521`), bez zmiany zachowania.
- [I] Koszt pełnego podziału: **L** (sygnatury 11 handlerów i 66 konstrukcji w testach). Koszt pierwszego kroku: **S**.

### K3. Kontrakt HTTP Go ↔ TS

**Obecny kształt.**
- [D] Brak osobnych DTO: kształty JSON to struktury domenowe z tagami lub anonimowe osadzanie (`projectReportLibrary{Library; IsDirect; Categories}` `project_handlers.go:147-151`, `libraryResponse` `handlers.go:176-180`).
- [D] Raport wysyła pola nieobecne w typie TS (`upload_batch_id` jako `""`); `ProjectReportRow` ma tagi JSON, choć nigdy nie jest serializowany (`project.go:33-45`; raport: `:34-44`).
- [D] Typy TS to ręczne, częściowe widoki per trasa (`catalog.ts:6-15` pomija `is_direct`, `strategic`, `error_detail`; `organization.ts:10-13` dodaje `is_direct` i `strategic`, a `error_detail` nie ma w żadnym typie TS; raport: `organization.ts` dodaje wszystkie trzy).
- [D] Jedyna kontrola maszynowa: `json_contract_test.go` (tylko `[]` vs `null`, tylko po stronie Go).

**Historia i intencjonalność.** Werdykt: **mieszany.**
- [D] Ręczne typy TS świadome: „plain `fetch`, `errorMessage` helper, exported types” (`archive/2026-10-17-mark-strategic-libraries/plan.md:187`); OpenAPI/codegen nigdzie nierozważane.
- [D] `SkippedRow` bez tagów: przypadek z 10-16 (dd27197), zauważony 10-26 i świadomie odłożony (`archive/2026-10-26-testing-access-control-data-integrity/plan.md:39`).
- [D] Zduplikowany klucz `is_direct`: przypadek (osadzenie `Library` + pole per projekt, 0d560b6; przesłanianie nigdzie nieomówione). Od 74daf0f osadzone pole jest zawsze `false` na tej ścieżce (`archive/2026-10-20-org-data-fixes-and-user-management/plan.md:83`).

**Wykonalność.**
- [D] Zasięg: 4 pary endpointów (tabela „Szew HTTP” w researchu L3). Dodanie tagów do `SkippedRow` zmienia klucze JSON i wymaga zmiany `catalog.ts:24-25` w tym samym commicie; testy Go czytają pola struktur, więc nie zauważą.
- [D] Luka w osłonach: test raportu dekoduje do typu Go (`project_handlers_test.go:489-490`, asercje `:500-543`; raport: `:500-543`), więc nie wykryje rozjazdu z widokiem TS. CI: `typecheck` widzi tylko TS, `go vet` tylko Go.
- [I] Pierwszy krok: testy „złotego zestawu kluczy” w `json_contract_test.go` dla 4 endpointów (dekodowanie do `map[string]any`, dokładny zbiór kluczy).
- [I] Koszt: **M** (mało kodu, ale każda zmiana w dwóch językach; odwracalne przez expand/contract).

### K4. Członkostwo w paczce

**Obecny kształt.**
- [D] Paczka nie jest bytem: brak tabeli, `upload_batch_id` to jedna kolumna NOT NULL na `libraries` (0002), zapisywana tylko przy pierwszym insercie (`store.go:115-117`). Znaczy „upload, który pierwszy wprowadził purl”, nie „biblioteki tego uploadu”.
- [D] Kolejka bez deduplikacji: `Enqueue` wysyła do kanału (`queue.go:97-107`), `UpdateLibraryStatus` bez compare-and-swap (`store.go:305-312`).

**Historia i intencjonalność.** Werdykt: **dwie świadome decyzje połączone bez analizy; późniejszy plan opiera się na fałszywej przesłance.**
- [D] „set once at first insert and never updated” (`archive/2026-10-19-project-redundancy-report/plan.md:10`).
- [D] Kolejkowanie tylko `pending` dodane po plan-review F3, w celu ochrony skategoryzowanych wierszy.
- [D] `archive/2026-10-27-empty-batch-status/plan.md:20`: „They are still not re-enqueued” — nieprawda dla znanych bibliotek wciąż `pending` (`project_handlers.go:102-103`). Na tej przesłance stoi decyzja `{}` = zakończona.

**Wykonalność.** [I] Pełna naprawa: **L** (migracja + semantyka po obu stronach szwu). Pierwszy krok: test charakteryzujący „ponowny upload, gdy pierwsza paczka jest `pending`” z nagrywającym fake'iem `Enqueuer` (`project_handlers_test.go:387, 809`).

**Werdykt badania: STOP — to przeprojektowanie pojęcia biznesowego** („co zawiera paczka”). Do analizy domenowej (L5).

### K5. Zduplikowana logika

**Obecny kształt i historia.**
- [D] **Parser SBOM (Go).** Skopiowane: dekodowanie, sprawdzenie `bomFormat`, inicjalizacja `skipped`, wczesny powrót i pętla (`sbom.go:25-56` vs `project_sbom.go:27-93`). Plan wymagał tego samego zachowania, nie wspólnego kodu („Reuses `ParsePurl`/skip semantics identically”, `archive/2026-10-19-project-redundancy-report/plan.md:126`). Werdykt: **przypadek.**
- [D] **Polling i `isBatchSettled` (TS).** Skopiowane z polecenia planu (`plan.md:243`) i już rozjechane: inna semantyka `{}` (patrz „Korekta priorów”), znacznik nieaktualnego pollingu tylko w `redundancy.tsx:129-170`. Werdykt: **przypadek, który już się rozjechał.**
- [D] **`errorMessage` (TS).** 4 identyczne kopie (V15), świadoma konwencja „own `errorMessage` helper, no shared client” (`plan.md:218`). Werdykt: **świadome.**

**Wykonalność.**
- [D] Parser Go: osłony są (`sbom_test.go:30, 54, 61`, `project_sbom_test.go:146`, czyste testy jednostkowe uruchamiane w CI bez Dockera). Koszt **S**.
- [I] TS: osłoną jest tylko `typecheck` i `build`; scalenie `isBatchSettled` wymaga decyzji o semantyce `{}` (zależy od K4). Koszt **S**, ale weryfikacja ręczna.

### K6. Granice plików w pakiecie `catalog`

**Obecny kształt.**
- [D] `handlers.go` (439 linii): 8 handlerów czterech możliwości + pomocniki pakietowe `Enqueuer` (:19), `maxUploadBytes` (:40), `writeCatalogError` (:57, z typem odpowiedzi i komentarzem :48-64; 26 wywołań tu, 14 w `project_handlers.go`; raport: 27, liczone z definicją), `newBatchID` (:429).
- [D] Martwy kod: `noopEnqueuer` / `NewNoopEnqueuer` (`handlers.go:23-33`) bez żadnych wywołań, także w testach.
- [D] Powtórzony prolog multipart w obu handlerach uploadu (`handlers.go:73-102` vs `project_handlers.go:30-66`).

**Historia i intencjonalność.** Werdykt: **jeden pakiet świadomie; położenie pomocników przypadkowe.**
- [D] „All new backend code lives in the existing `catalog` package” (`archive/2026-10-19-project-redundancy-report/plan.md:45`) i „extend (not fork)” (`archive/2026-10-17-mark-strategic-libraries/plan.md:16`).
- [D] Wcześniejsza konwencja mówiła co innego: „one package per domain concern, each with its own small, focused files” (`archive/2026-10-16-data-layer-connectivity/plan.md:11`).

**Wykonalność.** [D] Przeniesienie plików w obrębie pakietu: bez zmian symboli, `routes.go`, `main.go`, migracji ani TS. Osłony: kompilator i `handlers_test.go`, w tym testy logowania 4xx/5xx (`:711, :762`). Koszt **S**. Podział na osobne pakiety Go: **L**, bo wymaga najpierw K2 (współdzielona między plikami niewyeksportowana metoda `projectExists`, wołana z `project_handlers.go:168`; `reviewedCategories` jest wołana tylko z `project.go:195`; raport: obie współdzielone).

### K7. Upload bez wspólnej transakcji

- [D] Obecny kształt: 5 niezależnych wywołań store na puli (`project_handlers.go:81, 90, 101-105, 107, 117`); transakcyjny jest tylko `ReplaceProjectLibraries` (`project.go:107-144`); w całym pakiecie `pool.Begin` występuje jeszcze tylko w `MarkStrategic` (`store.go:385`), poza ścieżką uploadu. Brak abstrakcji jednostki pracy.
- [D] Historia: **świadome i udokumentowane.** „needs no spanning transaction … deliberately last” (`archive/2026-10-19-project-redundancy-report/plan.md:164`), po plan-review F4 „Safe as designed”.
- [I] Skutek błędu w połowie to zmarnowana praca (biblioteki skategoryzowane bez projektu), nie uszkodzenie danych. Przesłanka nadal w dużej mierze aktualna.
- [I] Wykonalność: nowy szew transakcyjny na 5 metodach i przesunięcie kolejkowania za commit. Koszt **M**.

### K8. Dwa znaczenia `is_direct`

- [D] Obecny kształt: katalogowe „bezpośrednia w jakimkolwiek projekcie” (`store.go:161`) vs per projekt (`project_libraries.is_direct`), pod tym samym kluczem JSON z tego samego pola `Library.IsDirect` (`store.go:62`). Wartość pola zależy od zapytania, które zbudowało strukturę (`LibrariesByPurls`, `ListStuckLibraries` zostawiają `false`).
- [D] Historia: **świadome ograniczenie**, zaakceptowane 10-19 („accepted as out of scope … revisit if this causes real confusion”, `archive/2026-10-19-project-redundancy-report/plan.md:32`), ale znaczenie katalogowe zmieniono 10-20 (74daf0f), a notatki nie zaktualizowano.
- [I] Wykonalność: zmiana nazwy przez szew Go ↔ TS, expand/contract. Koszt **S–M**. Pokrywa się z K3.

## Refactor opportunities (ranked)

Ocena: koszt długu (ryzyko, dowody z mapy i researchu) wobec kosztu zmiany (zasięg, osłony). Pozycje K1 i K4 są poza rankingiem jako przeprojektowanie pojęć biznesowych.

### 1. Jeden właściciel znaczników strategicznych (z K2, przygotowuje grunt pod K1)

- **Obecny → docelowy kształt:** pięć literałów odczytu `strategic_marks` w pięciu metodach, z czego trzy poza metodami znaczników (`store.go:187`, `project.go:201, 244`; raport: trzy ścieżki odczytu `store.go:187`, `store.go:494`, `project.go:201, 244`) i reguła „kategoria przejrzana” ukryta w prywatnej metodzie raportu → jedna nazwana część `Store` (plik `strategic.go` w tym samym pakiecie), która jest jedynym miejscem czytającym i zapisującym tę tabelę, w tym regułę „przejrzana” i zakres limitu.
- **Dlaczego #1:**
  - Dług jest w strefie ryzyka 2 mapy.
  - Tu siedzi jedyny twardy niezmiennik domeny, naprawiany już dwa razy.
  - Każda decyzja w K1 (FR-007 per ekosystem) wyląduje dziś w dwóch miejscach, które już raz się rozjechały (`store.go:428` vs `project.go:244`).
  - Koszt pierwszego kroku jest mały, bo pakiet i sygnatury handlerów się nie zmieniają.
- **Blast radius:** `store.go:369-521`, `project.go:194-264`, `store.go:159-227` (filtr `Strategic`); bez migracji i bez zmian TS. Testy: `store_test.go:270-914`, `project_handlers_test.go:437`, `handlers_test.go:787`.
- **Szkic ścieżki:**
  1. test raportu z dwoma projektami i znacznikami ustawionymi przez `MarkStrategic` (W1);
  2. wydzielenie metod znaczników do osobnego pliku;
  3. przeniesienie `reviewedCategories` i warunku JOIN-a raportu za metodę znaczników;
  4. filtr `Strategic` w `ListLibraries` przez tę samą metodę lub predykat.

  Każdy krok osobno odwracalny.
- **Pierwszy krok-prerekwizyt:** test wieloprojektowy raportu (ryzyko #4, W1). Jest wspólny z wstrzymaną Fazą 3 test-planu.

### 2. Kontrakt HTTP pilnowany testem (K3, wchłania K8 i W4)

- **Obecny → docelowy kształt:** struktury domenowe z osadzaniem i ręczne, częściowe typy TS, bez kontroli kluczy → jawne struktury odpowiedzi (DTO) dla 4 endpointów przepływu, z tagami (w tym `SkippedRow`), bez przesłoniętego `is_direct`, oraz testy złotego zestawu kluczy w `json_contract_test.go`.
- **Dlaczego #2:**
  - Rozjazd kontraktu to dokładnie klasa błędu, która dwa razy uderzyła w ten przepływ (`null` zamiast `[]`, jedyny wpis w `lessons.md`).
  - Dziś żaden automat w CI nie widzi obu stron szwu.
  - Koszt średni, a pierwszy krok (testy kluczy) zmienia tylko testy.
- **Blast radius:** `project_handlers.go:13-19, 137-151`, `handlers.go:176-180`, `sbom.go:13-16`, `store.go:54-64`, `projects.ts`, `catalog.ts:6-34`, `organization.ts:10-13`; testy `json_contract_test.go`, `project_handlers_test.go:437-543`, e2e.
- **Szkic ścieżki:**
  1. testy złotych kluczy, które utrwalą dzisiejszy stan;
  2. DTO raportu bez osadzania `Library`, z tymi samymi kluczami (zmiana bez zmiany przewodu);
  3. tagi `SkippedRow` metodą expand/contract (oba klucze, potem usunięcie starego);
  4. rozdzielenie nazw `is_direct` (K8) tą samą metodą.
- **Pierwszy krok-prerekwizyt:** testy złotych zestawów kluczy dla `POST /api/projects/upload`, `GET /api/projects`, `GET /api/projects/{id}/report`, `GET /api/catalog/batches/{id}`.

### 3. Higiena pakietu `catalog` przy istniejących osłonach (K5-Go + K6)

- **Obecny → docelowy kształt:** skopiowany parser SBOM i pomocniki HTTP w pliku handlerów katalogu → wspólna funkcja przetwarzania komponentu CycloneDX używana przez oba parsery, osobny plik pomocników HTTP (`writeCatalogError`, `newBatchID`, `maxUploadBytes`, prolog multipart), usunięty martwy `noopEnqueuer`.
- **Dlaczego #3:** najtańsze (S), z gotowymi osłonami (czyste testy parserów w CI, testy logowania handlerów); zmniejsza ryzyko, że dwa parsery rozjadą się tak jak polling w TS. Niższa pozycja, bo dług jest dziś mniej groźny: kopie parsera są jeszcze zgodne.
- **Blast radius:** `sbom.go`, `project_sbom.go`, `csv.go` (wspólny `SkippedRow`), `handlers.go`, `project_handlers.go`; bez zmian kontraktu, migracji i TS.
- **Szkic ścieżki:** (1) nowy plik z pomocnikami HTTP, przeniesienie bez zmian; (2) usunięcie `noopEnqueuer`; (3) wspólna funkcja komponentu, oba parsery przez nią.
- **Pierwszy krok-prerekwizyt:** brak nowego; istniejące testy wystarczą. Warto dopisać brakujące gałęzie `ParseProjectSBOM` (błąd dekodowania, zły `bomFormat`, brak komponentów), które dziś ma tylko `ParseCycloneDXSBOM`.

### Rozważone i odrzucone (na tym etapie)

| Kandydat | Dlaczego poza rankingiem |
|---|---|
| K1 zakres wyboru strategicznego | przeprojektowanie pojęcia biznesowego (FR-007 per ekosystem); decyzja produktowa przed kodem; po decyzji zmiana S w miejscu przygotowanym przez #1 |
| K4 członkostwo w paczce | przeprojektowanie pojęcia biznesowego („co zawiera paczka”); koszt L z migracją; najpierw test charakteryzujący podwójne kolejkowanie |
| K2 pełny podział `Store` na repozytoria per możliwość | koszt L (11 handlerów, 66 konstrukcji w testach) przy świadomej konwencji jednego `Store` i testów na Postgresie; #1 bierze z niego najcenniejszy kawałek |
| K5-TS polling / `isBatchSettled` | zależy od decyzji o semantyce `{}` (K4); `errorMessage` ×4 to świadoma konwencja |
| K6 podział na osobne pakiety Go | wymaga najpierw K2 |
| K7 transakcja uploadu | świadoma, udokumentowana decyzja, której przesłanka w dużej mierze trzyma; skutek błędu to zmarnowana praca, nie uszkodzenie danych |
| K8 dwa znaczenia `is_direct` | wchłonięte przez #2 (ta sama technika expand/contract na tym samym szwie) |

## Weryfikacja twierdzeń (ast-grep)

ast-grep 0.51.0, commit c2340d0 (kod `backend/` i `web/` bez zmian od 8bcf698, `git diff --stat` puste), katalogi `backend/internal/catalog`, `backend/cmd`, `web/app/lib`. Każde zero z ast-grep sprawdzone grepem. Jak w L3: wywołania metod Go przez reguły YAML `kind: call_expression` + `selector_expression`; wzorzec liniowy `$R.ParseMultipartForm($$$)` dał 0 przy 2 trafieniach grepa. Weryfikacja dotyczy twierdzeń strukturalnych rankingu i sekcji K, z których ranking korzysta; cytaty z archiwum i historia commitów są poza jej zakresem. **Ranking się nie zmienia.**

| # | Twierdzenie (pozycja) | Werdykt | Dowód (plik:linia) | Metoda |
|---|---|---|---|---|
| R1 | `Store` ma 18 metod: 11 w `store.go`, 7 w `project.go` (K2) | potwierdzone | `store.go:107, 135, 159, 230, 263, 304, 327, 342, 384, 466, 487`; `project.go:51, 73, 107, 149, 164, 194, 242`; grep `^func (s \*Store)`: 18 | `func ($S *Store) $M($$$A) $R { $$$B }` |
| R2 | 14 funkcji produkcyjnych przyjmuje `*Store` wprost (K2) | **obalone liczbowo → 13** | 11 handlerów (`handlers.go:71, 156, 185, 232, 301, 325, 352, 396`, `project_handlers.go:28, 163, 233`), `queue.go:71`, `pipeline.go:35` | reguła `parameter_declaration` z `pointer_type` `*Store`; grep |
| R3 | Trzy ścieżki odczytu `strategic_marks` (#1, K2) | doprecyzowane | 5 literałów odczytu: `store.go:187, 425, 494`, `project.go:201, 244`; zapisy `store.go:441, 468`; poza blokiem znaczników (`store.go:369-521`) czytają tylko `store.go:187`, `project.go:201, 244`. Pominięty odczyt to zapytanie limitu w `MarkStrategic`, które już jest w bloku znaczników, więc wniosek #1 trzyma | reguła: string literal `regex: strategic_marks` |
| R4 | Reguła „kategoria przejrzana” ukryta w prywatnej metodzie raportu (#1) | potwierdzone | `reviewedCategories` `project.go:242`, jedyne wywołanie `project.go:195` (z `ProjectReport`) | reguła `call_expression` |
| R5 | `projectExists` i `reviewedCategories` to niewyeksportowane metody współdzielone między plikami (K6) | doprecyzowane | między plikami tylko `projectExists` (`project_handlers.go:168`); `reviewedCategories` zostaje w `project.go`. Argument „podział pakietów wymaga najpierw K2” trzyma się na `projectExists` | reguła `call_expression` |
| R6 | Dwa sposoby rozwiązania bibliotek po insercie (K2) | potwierdzone | `ListLibraries(BatchID)` `handlers.go:132`; `LibrariesByPurls` `project_handlers.go:90` | reguła `call_expression` |
| R7 | Brak DTO: kształty JSON to struktury domenowe z osadzaniem (#2) | potwierdzone | osadzenie `Library` tylko w `libraryResponse` (`handlers.go:176-180`) i `projectReportLibrary` (`project_handlers.go:147-151`); `Library` `store.go:54-64` | `type $N struct { $$$ }`, grep `^\s*Library$` |
| R8 | `ProjectReportRow` ma tagi JSON, ale nie jest serializowany (#2) | potwierdzone | definicja `project.go:33-45`; handler koduje `[]projectReportLibrary` (`project_handlers.go:219-225`), `ProjectReportRow` tylko jako typ pośredni | grep `ProjectReportRow` |
| R9 | `SkippedRow` bez tagów (#2, #3) | potwierdzone | `sbom.go:13-16` (V11) | `type SkippedRow struct { $$$ }` |
| R10 | `json_contract_test.go` sprawdza tylko `[]` vs `null` (#2) | potwierdzone | pomocniki `assertNoNullArrays` (:23), `assertEmptyArrayField` (:59); handlery w teście: listy bibliotek (2), `BatchProgressHandler`, `UploadHandler`, `ProjectUploadHandler`; brak `ProjectReportHandler` i `ListProjectsHandler`; żadnej asercji zbioru kluczy | grep `Handler(` |
| R11 | Test raportu dekoduje do typu Go (#2) | potwierdzone | `var resp []projectReportLibrary` + `json.Unmarshal` `project_handlers_test.go:489-490` | grep, `json.Unmarshal($$$)` |
| R12 | Typy TS to częściowe widoki; `organization.ts` dodaje `is_direct`, `strategic`, `error_detail` (#2) | doprecyzowane | `catalog.ts:6-15` bez trzech pól; `OrganizationLibrary = Library & {is_direct, strategic}` (`organization.ts:10-13`); `error_detail` (`store.go:63`, `omitempty`) nie ma w żadnym pliku `web/app` | `export type $N = { $$$ }`, grep |
| R13 | `handlers.go` ma 439 linii i pomocniki pakietowe (#3) | potwierdzone | `wc -l`: 439; `Enqueuer` :19, `maxUploadBytes` :40, `writeCatalogError` :57, `newBatchID` :429 | grep |
| R14 | `writeCatalogError`: 27 wywołań w `handlers.go`, 14 w `project_handlers.go` (#3) | **obalone liczbowo → 26** | ast-grep 26 + 14; grep 27 w `handlers.go` liczy też definicję | `writeCatalogError($$$)` |
| R15 | `noopEnqueuer` / `NewNoopEnqueuer` to martwy kod bez wywołań, także w testach (#3) | potwierdzone | jedyne wystąpienia w `handlers.go:23-32` (definicje i komentarze) w całym `backend/` | grep |
| R16 | Powtórzony prolog multipart w obu uploadach (#3) | potwierdzone | `MaxBytesReader` `handlers.go:73` / `project_handlers.go:30`; `ParseMultipartForm(32 << 20)` `:79` / `:42`; `FormFile("file")` `:93` / `:56` | `http.MaxBytesReader($$$)`; grep (wzorzec metody: 0) |
| R17 | `ParseProjectSBOM` nie ma testów błędu dekodowania, złego `bomFormat` i braku komponentów (#3) | potwierdzone | `TestParseProjectSBOM` (`project_sbom_test.go:146`) ma 4 podtesty, żaden nie sprawdza błędu; gałęzie istnieją w `project_sbom.go:28-33`; `ParseCycloneDXSBOM` ma je w `sbom_test.go:54, 61` | grep `t.Run` |
| R18 | Upload: 5 niezależnych wywołań, transakcyjne tylko `ReplaceProjectLibraries` (K7) | potwierdzone, doprecyzowane | `project_handlers.go:81, 90, 103, 107, 117`; `pool.Begin` w produkcji: `project.go:108` i `store.go:385` (`MarkStrategic`, poza uploadem) | grep `store\.`, `Begin(` |

Dwa twierdzenia obalone liczbowo (R2, R14), żadne co do kierunku. Zmiany w treści: K2 (R2, R3, R6), K3 (R8, R11, R12), K6 (R5, R14), K7 (R18), pozycja #1 (R3). Kotwice z sekcji „Historia i intencjonalność” (archiwum, commity) nie były przedmiotem tej weryfikacji.

## Open Questions

- **[?]** FR-007: czy „kategoria przejrzana” jest per ekosystem? Właściciel produktu. Blokuje K1.
- **[?]** Czy paczka ma oznaczać „biblioteki tego uploadu”? Właściciel produktu. Blokuje K4 i scalenie `isBatchSettled`.
- **[?]** Czy dwa workery mogą równolegle przetwarzać tę samą bibliotekę (ponowne kolejkowanie + `Sweep`)? Nieuruchomione.
- **[?]** Czy strona katalogu odpytuje 60 s przy uploadzie samych znanych purli (`{}` = niezakończona)? Wniosek z kodu, nieuruchomiony.
- **[?]** Czy CI faktycznie uruchamia testy integracyjne z Dockerem? Nie widać tego z repo.

## Related Research

- `context/changes/redundancy-flow-analysis/research.md` — źródło T1–T12 i weryfikacji V1–V17.
- `context/map/repo-map.md` — strefy ryzyka 1 i 2.
- `context/changes/testing-redundancy-report-correctness/` — wstrzymana Faza 3 test-planu; jej test to prerekwizyt #1.
