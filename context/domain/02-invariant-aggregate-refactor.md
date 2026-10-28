---
title: "Agregat-strażnik wyboru strategicznego (CategoryStrategy)"
created: 2026-10-28
type: refactor-plan
head: 50ff522
source_prompt: .claude/prompts/m4l5-2-invariant-aggregate-refactor.md
builds_on: context/domain/domain-distillation.md
---

# Plan refaktoru: agregat-strażnik wyboru strategicznego

Produkt to plan, nie implementacja. Kod produkcyjny nie był zmieniany. Wszystkie `plik:linia` sprawdzone na `50ff522`.

## Krok 0. Kontekst

- **Cel produktu.** „Nobody answers "what is redundant" — only "what is vulnerable."” (`context/foundation/prd.md:24`). Architekt oznacza wybór strategiczny w kategorii, a raport flaguje bezpośrednie biblioteki niestrategiczne jako kandydatów do wycofania (`prd.md:36`, `prd.md:73`).
- **Stack.** Go 1.27 + pgx na Postgres 16; SPA React Router 8. Pakiet `backend/internal/catalog` łączy handlery HTTP, logikę i SQL; nie ma osobnej warstwy domenowej (`context/domain/evidence/0-sources.md`).
- **Warstwy, w których żyje logika znaczników:**
  - API: `handlers.go:352-425`;
  - store i SQL w transakcji: `strategic.go:55-153`;
  - model odczytu: `strategic.go:201-245`;
  - persystencja: `0003_strategic_marks.up.sql:3-13`;
  - UI: `web/app/routes/organization.tsx:74-110`, `:224-243`.

## Krok 1. Niezmienniki biznesowe

Pełna lista ze statusami jest w `context/domain/domain-distillation.md` (R-01–R-16). Tutaj są te, które konkurują o #1.

| id | niezmiennik | źródło |
|---|---|---|
| I-1 | W (kategoria, ekosystem) są najwyżej 2 aktywne wybory strategiczne. | „Normally one strategic library per category per ecosystem, but up to two are allowed.” `prd.md:73`; „No category/ecosystem can ever hold more than 2 active strategic marks.” `archive/2026-10-20-org-data-fixes-and-user-management/plan-brief.md:69` |
| I-2 | Wybór strategiczny dotyczy biblioteki **w tej kategorii** (i bezpośredniej). | „Architect can mark a direct library within a category as strategic” `prd.md:73` |
| I-3 | Każde oznaczenie i odznaczenie ma niepuste uzasadnienie. | „Required on both mark and unmark” `archive/2026-10-17-mark-strategic-libraries/plan-brief.md:22` |
| I-4 | Kandydat do wycofania = bezpośrednia, niestrategiczna biblioteka w przejrzanej kategorii; przechodnia nigdy. | `archive/2026-10-19-project-redundancy-report/plan-brief.md:26`, `…/plan.md:193` |
| I-5 | „Categorized” ⇔ biblioteka ma ≥1 kategorię. | `archive/2026-10-27-batch-status-uncategorized-libraries/plan-brief.md:24` |
| I-6 | Upload projektu jest Architect-only; raporty dla Architect i Viewer. | `prd.md:101` |
| I-7 | Tożsamość biblioteki = purl; tożsamość projektu = nazwa bez wielkości liter. | `0002_catalog_schema.up.sql:24`, `0004_project_reports.up.sql:9` |

## Krok 2. Klasyfikacja i wybór #1

| id | (a) rdzeniowy | (b) rozsmarowany (warstwy / pliki) | (c) egzekwowany |
|---|---|---|---|
| I-1 | Core: od niego zależy, które biblioteki raport flaguje | 3 warstwy: SQL w transakcji (`strategic.go:83-106`), API (`handlers.go:374-380`), UI (`organization.tsx:88`) | **egzekwowany** hard (app); brak ograniczenia w DB |
| I-2 | Core: zły cel psuje I-4 dla wszystkich projektów | 2 warstwy, ale tylko UI pilnuje: przycisk przy kategoriach biblioteki (`organization.tsx:224-243`); serwer nic (`strategic.go:55-130`) | **naruszalny**: client-only dla kategorii, brak strażnika dla „direct” |
| I-3 | Supporting (audyt decyzji) | 3 warstwy: UI `organization.tsx:74-77,96-99`, API `handlers.go:358-361,402-405`, DB `0003…up.sql:7` | egzekwowany hard (app); DB przyjmuje `''` |
| I-4 | Core | 1 miejsce: `strategic.go:243-245` (po refaktorze L4) | egzekwowany (read model) |
| I-5 | Supporting | `pipeline.go:90-96` vs `handlers.go:335` | sprzeczny (D-04) |
| I-6 | Generic | `routes.go:35-61` | egzekwowany |

**Wybór: I-2 rozpatrywany razem z I-1 i I-3.** Są to warunki jednej operacji: „ustal wybór strategiczny w (kategoria, ekosystem)”.

Uzasadnienie:
- I-2 jest Core i jedynym niezmiennikiem Core, którego serwer w ogóle nie pilnuje.
- Jeden `POST /api/organization/libraries/{lib}/categories/{cat}/strategic` z dowolną istniejącą parą przechodzi. Taki znacznik:
  - zalicza kategorię jako przejrzaną (`strategic.go:213-216`, `:233`), więc zmienia flagi I-4 we wszystkich projektach;
  - zużywa limit I-1.
- I-1 i I-3 są dziś egzekwowane, ale w różnych miejscach: limit w SQL store'a, uzasadnienie tylko w handlerze. Agregat zbiera wszystkie trzy warunki w jednej metodzie.

Odpowiada to pozycji #1 w rankingu destylacji (D-02, R-02).

## Krok 3. Diagnoza

**Gdzie reguła żyje dziś:**

| warstwa | miejsce | co robi | ocena |
|---|---|---|---|
| UI | `web/app/routes/organization.tsx:224-243` | przycisk „Mark strategic” renderowany tylko w pętli po `library.categories` | **jedyny strażnik I-2** (client-only); omijalny bezpośrednim wywołaniem API |
| UI | `organization.tsx:74-77`, `:96-99` | `trim()` uzasadnienia, „Justification is required.” | kopia I-3, zdublowana na serwerze |
| UI | `organization.tsx:217` | pokazuje `is_direct`, ale nie warunkuje przycisku | „direct” z I-2 niepilnowane nawet w UI |
| API | `handlers.go:358-361`, `:402-405` | `strings.TrimSpace(req.Justification) == ""` → 400 | I-3 żyje tylko tutaj; `Store.MarkStrategic` przyjmie pusty string od każdego innego wywołującego |
| API | `handlers.go:370-380` | mapuje `ErrInvalidReference` → 400, `ErrStrategicCapReached` → 409, `ErrStrategicMarkBusy` → 503 | poprawne mapowanie; brak błędu „biblioteka nie w kategorii” |
| store (tx) | `strategic.go:62-68` | czyta tylko `libraries.ecosystem` | nie sprawdza `library_categories` ani `project_libraries` |
| store (tx) | `strategic.go:83-106` | advisory lock na `kategoria:ekosystem`, `COUNT(DISTINCT …)`, `count >= 2` | I-1 egzekwowany; limit jako literał bez nazwy |
| store (tx) | `strategic.go:111-123` | upsert; FK violation → `ErrInvalidReference` | FK pilnuje tylko istnienia biblioteki i kategorii, nie przypisania |
| store | `strategic.go:137-150` | unmark: `UPDATE … WHERE active = true`, brak → `ErrStrategicMarkNotFound` | nadpisuje uzasadnienie oznaczenia; nie wymaga niepustego uzasadnienia |
| DB | `0003_strategic_marks.up.sql:3-13` | PK `(library_id, category_id)`, FK, `justification text NOT NULL`, indeks `WHERE active` | brak CHECK na niepuste uzasadnienie; brak FK do `library_categories` |
| read model | `strategic.go:201-245` | `isReviewed`, `isRetireCandidate` | ufa każdemu aktywnemu wierszowi, więc zły cel z zapisu przechodzi do raportu |

**Gdzie egzekwowanie jest niespójne:**
- I-3: UI i handler pilnują, store i DB nie (DB przyjmie `''`).
- I-1: tylko store; DB nie zna limitu.
- I-2: tylko UI, i to tylko część o kategorii.

**Gdzie błąd jest połykany zamiast zatrzymywać operację:**
- Ponowne oznaczenie tej samej pary to cichy upsert, który nadpisuje uzasadnienie (`strategic.go:111-115`). Jest to celowe („rather than erroring”, `strategic.go:52-54`), ale bez śladu poprzedniego uzasadnienia.
- Oznaczenie biblioteki spoza kategorii nie daje żadnego błędu: wiersz powstaje.

**Testy utrwalające lukę:** `backend/internal/catalog/strategic_test.go` ma 23 wywołania `MarkStrategic` i 0 wywołań `AttachCategory`. Testy limitu (`:62`, `:250`, `:326`, `:424`) oznaczają biblioteki, które nie są przypięte do kategorii. Wymuszenie I-2 na serwerze najpierw zaczerwieni te testy.

## Krok 4. Projekt agregatu-strażnika

**Granica agregatu.** `CategoryStrategy` to wybór strategiczny jednej pary (kategoria, ekosystem):
- tożsamość: `(categoryID, ecosystem)`;
- stan: aktywne znaczniki tej pary;
- niezmienniki: I-1, I-2, I-3.

Granica pokrywa się z kluczem advisory locka, który już dziś serializuje zapis (`strategic.go:83-86`). Biblioteka nie należy do agregatu. Agregat dostaje jej migawkę `Candidate` (id, ekosystem, przypisane kategorie, czy bezpośrednia gdziekolwiek), wczytaną w tej samej transakcji.

**Dwie otwarte decyzje produktowe są parametrem, nie kodem na sztywno:**
- **Q-02** (czy wymagać „direct”) → `Policy.RequireDirect`. Domyślnie `false`, czyli dzisiejsze zachowanie dla tej części.
- **Q-01** (czy „przejrzana” per ekosystem) dotyczy modelu odczytu, nie zapisu. Agregat już jest per ekosystem; zmiana Q-01 dotknie tylko `strategicIndex`.

```go
// Błędy domenowe (nazwane, fail-fast).
var (
	ErrStrategicCapReached      // istnieje: strategic.go:25
	ErrStrategicMarkNotFound    // istnieje: strategic.go:20
	ErrInvalidReference         // istnieje: strategic.go:16
	ErrJustificationRequired    = errors.New("catalog: justification is required")
	ErrLibraryNotInCategory     = errors.New("catalog: library is not attached to this category")
	ErrLibraryNotDirect         = errors.New("catalog: library is not a direct dependency of any project")
	ErrEcosystemMismatch        = errors.New("catalog: library belongs to a different ecosystem")
)

const strategicCap = 2 // zamiast literału strategic.go:104

type Policy struct{ RequireDirect bool } // Q-02

type Candidate struct {
	LibraryID     string
	Ecosystem     string
	CategoryIDs   map[string]bool // z library_categories
	DirectAnywhere bool           // EXISTS po project_libraries.is_direct
}

type activeMark struct{ LibraryID, Justification, MarkedBy string }

type CategoryStrategy struct {
	categoryID, ecosystem string
	policy               Policy
	marks                map[string]activeMark // libraryID -> mark
	changed              *markChange           // jedna zmiana na transakcję
}

func (cs *CategoryStrategy) Mark(c Candidate, justification, by string) error {
	if strings.TrimSpace(justification) == "" { return ErrJustificationRequired }
	if c.Ecosystem != cs.ecosystem             { return ErrEcosystemMismatch }
	if !c.CategoryIDs[cs.categoryID]           { return ErrLibraryNotInCategory }
	if cs.policy.RequireDirect && !c.DirectAnywhere { return ErrLibraryNotDirect }
	if _, already := cs.marks[c.LibraryID]; !already && len(cs.marks) >= strategicCap {
		return ErrStrategicCapReached
	}
	cs.marks[c.LibraryID] = activeMark{c.LibraryID, justification, by}
	cs.changed = &markChange{kind: marked, libraryID: c.LibraryID}
	return nil
}

func (cs *CategoryStrategy) Unmark(libraryID, justification, by string) error {
	if strings.TrimSpace(justification) == "" { return ErrJustificationRequired }
	if _, ok := cs.marks[libraryID]; !ok        { return ErrStrategicMarkNotFound }
	delete(cs.marks, libraryID)
	cs.changed = &markChange{kind: unmarked, libraryID: libraryID, justification: justification, by: by}
	return nil
}
```

Ponowne oznaczenie tej samej biblioteki zostaje dozwolone i aktualizuje uzasadnienie (dzisiejszy kontrakt `strategic.go:52-54`, test `strategic_test.go:12`). Zmiana tej semantyki byłaby nową decyzją, nie częścią tego planu.

**Repozytorium.** Jedno miejsce, które ładuje i zapisuje agregat w JEDNEJ transakcji:

```go
type StrategyRepository interface {
	// Begin + SET LOCAL lock_timeout + pg_advisory_xact_lock(category:ecosystem),
	// potem SELECT aktywnych znaczników tej pary. Zwraca ErrStrategicMarkBusy przy timeoucie.
	LoadForUpdate(ctx, tx pgx.Tx, categoryID, ecosystem string, p Policy) (*CategoryStrategy, error)
	// Migawka biblioteki w tej samej tx: ecosystem, library_categories, EXISTS project_libraries.is_direct.
	// ErrInvalidReference, gdy biblioteka lub kategoria nie istnieje.
	Candidate(ctx, tx pgx.Tx, libraryID string) (Candidate, error)
	// Zapisuje cs.changed (upsert active=true albo UPDATE active=false).
	Save(ctx, tx pgx.Tx, cs *CategoryStrategy) error
}

// Usługa w pakiecie catalog, zastępuje Store.MarkStrategic / UnmarkStrategic.
func (s *Store) MarkStrategic(ctx, libraryID, categoryID, justification, by string) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		c, err := repo.Candidate(ctx, tx, libraryID)               // ecosystem z migawki
		if err != nil { return err }
		cs, err := repo.LoadForUpdate(ctx, tx, categoryID, c.Ecosystem, s.policy)
		if err != nil { return err }
		if err := cs.Mark(c, justification, by); err != nil { return err } // rollback, nic nie zapisane
		return repo.Save(ctx, tx, cs)
	})
}
```

Kolejność „najpierw migawka, potem lock” zachowuje dzisiejsze założenie, że `libraries.ecosystem` się nie zmienia (`strategic.go:80-82`). Migawkę kategorii trzeba czytać **po** locku, a nie przed nim: tylko wtedy przypisanie i znacznik są spójne w tej samej transakcji. Ścieżek odpinania kategorii dziś nie ma (jedyny SQL `DELETE` w kodzie produkcyjnym `backend/` to `project.go:114`, członkostwo projektu), więc warunek I-2 raz spełniony pozostaje spełniony. Jeśli odpinanie powstanie, będzie musiało przejść przez ten agregat.

**Cienkie API.** `handlers.go:352-425` sprowadza się do: parse path i body → `store.MarkStrategic` / `UnmarkStrategic` → mapowanie błędu:

| błąd domenowy | HTTP |
|---|---|
| `ErrJustificationRequired` | 400 |
| `ErrInvalidReference` | 400 |
| `ErrLibraryNotInCategory`, `ErrEcosystemMismatch`, `ErrLibraryNotDirect` | 422 (albo 400, zgodnie z istniejącą konwencją pakietu; do decyzji w planie) |
| `ErrStrategicCapReached` | 409 |
| `ErrStrategicMarkBusy` | 503 |
| `ErrStrategicMarkNotFound` | 404 |

Sprawdzanie `TrimSpace` znika z handlera, bo przenosi się do agregatu. UI zostaje przy swoich podpowiedziach, ale przestaje być jedynym strażnikiem.

## Krok 5. Before/after, plan faz, testy

**Before / after:**

| miejsce dziś | before | after |
|---|---|---|
| `organization.tsx:224-243` | jedyny strażnik „w tej kategorii” | podpowiedź UX; serwer odrzuca 4xx, UI pokazuje komunikat z `setFormError` (`organization.tsx:88`) |
| `organization.tsx:74-77,96-99` | walidacja uzasadnienia | bez zmian (UX), zdublowana przez agregat |
| `handlers.go:358-361,402-405` | `TrimSpace` w handlerze | usunięte; błąd `ErrJustificationRequired` z agregatu |
| `handlers.go:370-380,414-416` | mapowanie 3–4 błędów | jedna funkcja mapująca błędy domenowe (tabela wyżej) |
| `strategic.go:55-130` | lookup ekosystemu + lock + count + upsert w jednej metodzie store'a | `Candidate` → `LoadForUpdate` → `CategoryStrategy.Mark` → `Save`, ta sama transakcja i ten sam lock |
| `strategic.go:104` | `count >= 2` | `strategicCap` w agregacie |
| `strategic.go:137-150` | UPDATE bez wymogu uzasadnienia | `CategoryStrategy.Unmark` z I-3 |
| `0003_strategic_marks.up.sql:7` | `justification text NOT NULL` przyjmuje `''` | opcjonalna nowa migracja `0008_…` z `CHECK (btrim(justification) <> '')`, najpierw sprawdzić dane produkcyjne |
| `strategic.go:201-245` | read model ufa każdemu wierszowi | bez zmian w kodzie; ufa wierszom, których zapis przeszedł przez agregat |

**Plan faz** (projekt ma runner Go na prawdziwym Postgresie i dyscyplinę test-first z `/10x-tdd`):

1. **Testy jako specyfikacja (test-first, czerwone).**
   - Napraw fixture'y istniejących testów limitu: przypnij kategorię przez `AttachCategory(..., "manual")` przed `MarkStrategic` (`strategic_test.go:62`, `:250`, `:326`, `:424` i pozostałe; `handlers_test.go:476`, `:544`, `:787`, `:828`). Same fixture'y przechodzą już na obecnym kodzie.
   - Dopisz czerwone testy nielegalnych operacji (lista niżej).
2. **Agregat w pamięci (test-first, bez Dockera).** `CategoryStrategy` + `Candidate` + błędy domenowe z testem tabelarycznym, w stylu `TestStrategicIndex_RetireCandidateRule` (`strategic_test.go:471`).
3. **Repozytorium i transakcja.** `LoadForUpdate` / `Candidate` / `Save` przejmują SQL z `strategic.go:55-150`. Testy współbieżności (`strategic_test.go:326`, `:391`, `:424`) muszą przejść bez zmian w asercjach; seam `afterCapCheck` przechodzi do miejsca między `Mark` a `Save`.
4. **Cienki handler.** Usunięcie `TrimSpace` z handlerów, jedna funkcja mapująca błędy, testy handlerów 400/409/422/503/404.
5. **Opcjonalnie: strażnik w DB** dla I-3 (migracja `0008`), dopiero po sprawdzeniu danych produkcyjnych (runbook: migracje się nie cofają, `docs/runbook.md`, sekcja „Migrations don't roll back”).

Decyzja Q-02 (direct) jest tylko przełącznikiem `Policy.RequireDirect`: plan nie zależy od odpowiedzi. Q-01 (zakres „przejrzanej”) jest poza tym planem i należy do modelu odczytu (`strategicIndex`).

**Przypadki testowe niezmiennika:**

| # | operacja | stan przed | oczekiwane |
|---|---|---|---|
| L1 | Mark biblioteki przypiętej do kategorii, para pusta | 0 znaczników | OK, 1 aktywny |
| L2 | Mark drugiej przypiętej biblioteki | 1 znacznik | OK, 2 aktywne |
| L3 | Ponowny Mark już oznaczonej biblioteki | 2 znaczniki | OK, uzasadnienie zaktualizowane, wciąż 2 |
| L4 | Unmark oznaczonej, z uzasadnieniem | 2 znaczniki | OK, 1 aktywny |
| L5 | Mark po Unmark (zwolnione miejsce) | 1 znacznik | OK, 2 aktywne |
| L6 | Mark w innym ekosystemie tej samej kategorii przy pełnej parze Maven | 2 Maven | OK (limit per ekosystem) |
| N1 | Mark trzeciej przypiętej biblioteki | 2 znaczniki | `ErrStrategicCapReached` → 409, brak wiersza |
| N2 | Mark biblioteki **nieprzypiętej** do kategorii | 0 znaczników | `ErrLibraryNotInCategory`, brak wiersza, kategoria nie staje się „reviewed” w raporcie |
| N3 | Mark z uzasadnieniem `"   "` bezpośrednio przez `Store` | — | `ErrJustificationRequired`, brak wiersza |
| N4 | Unmark z pustym uzasadnieniem przez `Store` | 1 znacznik | `ErrJustificationRequired`, znacznik nadal aktywny |
| N5 | Unmark nieoznaczonej | 0 znaczników | `ErrStrategicMarkNotFound` → 404 |
| N6 | Mark nieistniejącej biblioteki lub kategorii | — | `ErrInvalidReference` → 400 |
| N7 | Mark przechodniej (nigdzie direct) przy `RequireDirect=true` | — | `ErrLibraryNotDirect`; przy `false` OK (dzisiejsze zachowanie) |
| N8 | Dwa równoległe Mark na pełnej-1 parze | 1 znacznik | dokładnie jeden sukces, jeden `ErrStrategicCapReached` (istniejący `strategic_test.go:326`, `:424`) |
| N9 | Lock zajęty dłużej niż timeout | — | `ErrStrategicMarkBusy` → 503 (istniejący `strategic_test.go:391`) |
| R1 | Raport po N2 | — | żadna biblioteka w tej kategorii nie jest kandydatem do wycofania (`project_report_test.go` jako regresja) |

**Nowe nazwy „load-bearing”.** Rejestr kontraktów `docs/reference/contract-surfaces.md` nie istnieje (sprawdzone). Gdyby go założyć, wpisy to:
- `CategoryStrategy`, `Candidate`, `StrategyRepository`, `strategicCap`;
- `ErrJustificationRequired`, `ErrLibraryNotInCategory`, `ErrLibraryNotDirect`, `ErrEcosystemMismatch`;
- nowe kody odpowiedzi API (422 lub 400) dla `POST/DELETE /api/organization/libraries/{libraryID}/categories/{categoryID}/strategic`, które UI musi pokazać.

Glosariusz `context/domain/glossary.md` dostaje „Category strategy” jako nazwę pary (kategoria, ekosystem) z jej znacznikami.

## Podsumowanie

Wybrany niezmiennik to wybór strategiczny w parze (kategoria, ekosystem): najwyżej 2 znaczniki (I-1), tylko na bibliotece z tej kategorii (I-2), zawsze z uzasadnieniem (I-3). Jest Core, bo od niego zależy, które biblioteki raport każe wycofać. Jest też najsłabiej egzekwowany: część „w tej kategorii” pilnuje dziś wyłącznie UI (`organization.tsx:224-243`), a serwer (`strategic.go:55-130`) przyjmuje znacznik na dowolnej parze i tym samym zmienia flagi „retire” we wszystkich projektach.

Projekt wprowadza agregat `CategoryStrategy`, ładowany pod tym samym advisory lockiem co dziś, z metodami `Mark` i `Unmark`, które zwracają nazwane błędy domenowe. Repozytorium wykonuje całość w jednej transakcji, a handler sprowadza się do parse → metoda → mapowanie błędu. Otwarta decyzja Q-02 (czy wymagać „direct”) jest przełącznikiem polityki, więc plan od niej nie zależy.

Najważniejsze ryzyko: istniejące testy limitu oznaczają biblioteki nieprzypięte do kategorii (23 wywołania `MarkStrategic`, 0 `AttachCategory` w `strategic_test.go`), więc faza 1 zaczyna od poprawy fixture'ów. Plan ma 5 faz, z czego pierwsze dwie test-first, i 16 przypadków testowych.
