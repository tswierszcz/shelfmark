---
artifact: domain-distillation
created: 2026-10-28
repo_root: /opt/src/shelfmark
code_scope: . (backend/ + web/)
head: 24a8920
dirty: context/domain/ (this run's output, uncommitted)
documents: 3 requirements, 21 narrative (9 plan-briefs and fragments of 6 plans read, 10 by title only), 4 reference, 5 engineering
previous: none
evidence: context/domain/evidence/
glossary: context/domain/glossary.md
---

# Destylacja domeny: Shelfmark

## Podsumowanie

Shelfmark odpowiada organizacji na pytanie „co jest redundantne” w jej zależnościach. Biblioteki (purl) są klasyfikowane według capability w kurowanej taksonomii, a architekt zestawia z nimi SBOM-y projektów i oznacza biblioteki strategiczne. Bezpośrednie biblioteki, które nie są wyborem strategicznym w przejrzanej kategorii, stają się kandydatami do wycofania. **Core** to analiza redundancji: znaczniki strategiczne, kategoria przejrzana, kandydat do wycofania, raport projektu. Katalog klasyfikacji i ingest SBOM są **Supporting**, a konta i role **Generic**.

W kodzie brakuje 4 z 24 terminów Core/Supporting. Trzy z nich to pojęcia Core: miara „redundancy”, raport na poziomie organizacji i jawna rekomendacja „to retire”. Czwarty, metryki zdrowia, należy do Supporting.

Najmocniejszy rozjazd (D-02) dotyczy oznaczania biblioteki jako strategicznej. PRD mówi o „direct library within a category”, ale serwer przyjmuje znacznik na dowolnej parze biblioteka–kategoria. Taki znacznik od razu zalicza kategorię jako przejrzaną i zmienia flagi „retire” we wszystkich projektach. Dlatego to jest **#1**.

Rozjazdy według strony naprawy: kod 3, dokumentacja 2, decyzja 4. Otwartych pytań: 4.

## Zmiany od poprzedniego uruchomienia

Pierwsze uruchomienie: nie ma poprzedniego raportu.

## Język wszechobecny

| id | termin | rodzaj | definicja (cytat) | źródło | w kodzie (path:line) albo **missing in code** | inne nazwy w kodzie |
|---|---|---|---|---|---|---|
| T-01 | Library | encja | „classifies libraries by capability (what each library actually does)” | `context/foundation/prd.md:26` | `Library` `backend/internal/catalog/store.go:32`, tabela `libraries` `backend/internal/db/migrations/0002_catalog_schema.up.sql:22` | `OrganizationLibrary`, `ProjectReportLibrary` (TS) |
| T-02 | purl | wartość (tożsamość) | „Input: a purl (package URL) identifying the library.” | `prd.md:87` | `purl text UNIQUE` `0002_catalog_schema.up.sql:24`, `ParsePurl` `purl.go:13` | — (tożsamość = pełny purl z wersją) |
| T-03 | Ecosystem | wartość | „scoped per technology/ecosystem (e.g. one strategic choice for Maven/Java, a separate one for npm/JS…)” | `prd.md:73` | `Ecosystem` `store.go:36`; zawsze `maven` (`purl.go:19-21,31`) | — |
| T-04 | Category | encja słownikowa | „one or more capability categories drawn from a curated taxonomy” | `prd.md:87` | `Category` `store.go:18`, tabela `categories` `0002…up.sql:1` | taxonomy, capability (komentarz `store.go:2`) |
| T-05 | Taxonomy | zbiór wartości | „drawn from a curated taxonomy” | `prd.md:87` | `ListTaxonomy` `store.go:104`, seed 14 nazw `0002…up.sql:6-20` | `/api/catalog/categories` |
| T-06 | Classification base / catalog | agregat zbioru | „an internal knowledge base that classifies libraries by capability” | `prd.md:26` | pakiet `backend/internal/catalog`, trasy `/api/catalog/*` (`routes.go:50-54`) | „organization” (ta sama lista w widoku architekta, `handlers.go:232`) |
| T-07 | Categorization | operacja | „Shelfmark determines a capability classification from its GitHub repository data, with an LLM making the final categorization decision.” | `prd.md:85` | `ProcessLibrary` `pipeline.go:35`, `Categorizer` `pipeline.go:18`, `Queue` `queue.go:48` | cascade, pipeline |
| T-08 | Category source | wartość | „`source text not null check (source in ('github','llm','manual'))`” | `archive/2026-10-16-seed-classification-catalog/plan.md:64` | `library_categories.source` `0002…up.sql:41` | — (tylko literały, bez typu) |
| T-09 | Library status | stan | „`needs_manual`, which means "the pipeline ran successfully but found nothing confident," versus `failed`…” | `seed-classification-catalog/plan.md:63` | `libraries.status` `0002…up.sql:28-29`, `Status string` `store.go:38` | „Needs review”, „Processing” (`components/library-status-badge.tsx:7-13`) |
| T-10 | Batch | encja niejawna | brak definicji; zachowanie: „The batch line says `— done` only when every library is categorized…” | `archive/2026-10-27-batch-status-uncategorized-libraries/plan-brief.md:15` | `upload_batch_id` `0002…up.sql:30`, `BatchProgress` `store.go:58` | — (bez tabeli i typu) |
| T-11 | SBOM | dokument wejściowy | „contains no private/customer data — only library coordinates and their roles” | `prd.md:42` | `ParseCycloneDXSBOM` `sbom.go:24`, `ParseProjectSBOM` `project_sbom.go:26` | CycloneDX |
| T-12 | Project | encja | „Project identity: Free-text name, case-insensitive match-or-create on upload” | `archive/2026-10-19-project-redundancy-report/plan-brief.md:21` | `Project` `project.go:13`, `projects_name_lower_idx` `0004_project_reports.up.sql:9` | — |
| T-13 | Direct / transitive | wartość per projekt | „direct/transitive varies per project, not globally per library” | `project-redundancy-report/plan-brief.md:11` | `project_libraries.is_direct` `0004…up.sql:14` | `Library.IsDirect` = „direct w którymkolwiek projekcie” (`store.go:130`) — patrz B-02 |
| T-14 | Strategic library / strategic mark | decyzja (encja) | „Normally one strategic library per category per ecosystem, but up to two are allowed.” | `prd.md:73` | `StrategicMark` `strategic.go:34`, `strategic_marks` `0003_strategic_marks.up.sql:3-8`, `MarkStrategic` `strategic.go:55` | standard, strategic pick, strategic designation (komentarze) |
| T-15 | Justification | wartość | „Required on both mark and unmark” | `archive/2026-10-17-mark-strategic-libraries/plan-brief.md:22` | `strategic_marks.justification` `0003…up.sql:7` | — |
| T-16 | Candidate to retire | stan wyliczany | „Direct libraries in a category/ecosystem not marked strategic are implicitly candidates to retire.” | `prd.md:73` | `isRetireCandidate` `strategic.go:243`, `ProjectReportRow.RetireCandidate` `project.go:44` | „Retire?” (`redundancy.tsx:304`) |
| T-17 | Reviewed category | stan wyliczany | „Only flag a direct library when a *different* library is strategic in that category” | `project-redundancy-report/plan-brief.md:26` | `isReviewed` `strategic.go:233` | — |
| T-18 | Strategic cap | reguła | „No category/ecosystem can ever hold more than 2 active strategic marks.” | `archive/2026-10-20-org-data-fixes-and-user-management/plan-brief.md:69` | literał `count >= 2` `strategic.go:104`, `ErrStrategicCapReached` `strategic.go:25` | — (bez nazwanej stałej) |
| T-19 | Redundancy report (projektu) | odczyt | „report of all its libraries by category, with direct dependencies highlighted…” | `prd.md:69` | `ProjectReport` `project.go:193`, `GET /api/projects/{projectID}/report` `routes.go:60` | „redundancy” tylko w SPA (`routes.ts:10`, `redundancy.tsx:204`) |
| T-20 | My organization view | odczyt | „Architect can view the organization's library list, filterable by strategic/standard status” | `prd.md:71` | `ListOrganizationLibrariesHandler` `handlers.go:232`, `organization.tsx` | catalog (ta sama lista dla admina) |
| T-21 | Redundancy (miara) | miara | „flagging redundancy (two libraries in the same category within one project)” | `prd.md:36` | **missing in code** — szukano `redundan`, zliczeń na kategorię w `backend/` i `web/app/routes/redundancy.tsx`: tylko komentarze i nazwa trasy | — |
| T-22 | Organization-level report | odczyt | „An organization-level report shows redundancy across all uploaded projects” | `prd.md:36` | **missing in code** — szukano `cross.?project`, `organi[sz]ation.?(level\|wide)`, `org.?report` w `backend/` i `web/app/`: tylko nazwa testu | — |
| T-23 | Recommendation (standard / to retire) | decyzja | „sets and changes recommendations (standard / to retire) with justification” | `prd.md:98` | **missing in code** jako jawna decyzja — szukano `retire` w `backend/` i `web/app/`: tylko wyliczana flaga (`strategic.go:243`) i etykiety | — |
| T-24 | Health metrics | miara (odłożona) | „fetches current health metrics for a library from the web, as an enrichment” | `prd.md:39` | **missing in code** — `health` w `backend/internal` i `web/app` tylko w `home.tsx:26` (copy) i `db.HealthCheck` (techniczne) | — |

W dowodach jest 33 terminów z dokumentów, w tabeli 24. Pominięte: 4 terminy Generic (Admin, Architect, Viewer, User account; w kodzie `Role…` `auth/accounts.go:9-14`, `users` `0006_users.up.sql:1`), Topic tags, CSV list i Category source jako wejścia pomocnicze oraz 4 tezy (Capability, Attack surface, „what is redundant”, Remediated builds).

Brakujące w kodzie: 2 miary (T-21, T-24), 1 odczyt (T-22), 1 decyzja (T-23).

**Pojęcia tylko w kodzie:**
- „stuck library” (`StuckLibraryAge` `queue.go:42`) — *searched in docs*: występuje jako „boot-only sweep” w briefie observability.
- `error_detail` (`0007_libraries_error_detail.up.sql:1`) — *not in the docs sample*.
- `marked_by` / `created_by` jako e-mail — *searched in docs*.

## Wieloznaczności i granice kontekstów

| id | słowo | znaczenie A (kontekst, źródło) | znaczenie B (kontekst, źródło) | proponowane nazwy |
|---|---|---|---|---|
| B-01 | redundancy | miara: dwie biblioteki w tej samej kategorii w jednym projekcie (analiza, `prd.md:36`) | decyzja: bezpośrednia biblioteka, która nie jest wyborem strategicznym (rekomendacja, `project-redundancy-report/plan-brief.md:7`) | „redundancy” dla miary, „candidate to retire” dla decyzji |
| B-02 | `is_direct` / Direct | rola w projekcie (`0004…up.sql:14`, `prd.md:43`) | „direct w którymkolwiek projekcie” w widoku organizacji (`store.go:130`, `org-data-fixes…/plan-brief.md:27`) | „direct in project” i „direct anywhere” |
| B-03 | review | `needs_manual`, czyli kategoryzacja czeka na człowieka (katalog, `library-status-badge.tsx:7-8`) | kategoria ma aktywny znacznik strategiczny (analiza, `strategic.go:230-233`) | „needs manual categorization” i „reviewed category” |
| B-04 | batch | biblioteki, które upload pierwszy wprowadził do katalogu (katalog, `store.go:84-86`, `0002…up.sql:30`) | to, na co czeka użytkownik po uploadzie (projekt, `batch-status-uncategorized-libraries/plan-brief.md:15`) | otwarte — Q-03 |

Granica, która z tego wynika: **katalog klasyfikacji** (status, review = brak kategorii, batch) i **analiza organizacji** (direct w projekcie, reviewed = znacznik, redundancy). Dzielą tabelę `libraries` i jeden `Store`.

## Poddomeny

| poddomena | klasa | pojęcia | dlaczego |
|---|---|---|---|
| Analiza redundancji | **Core** | T-13–T-23 | „Nobody answers "what is redundant" — only "what is vulnerable."” (`prd.md:24`); kryterium sukcesu MVP part 2 (`prd.md:36`) |
| Katalog klasyfikacji | Supporting | T-01–T-10, T-24 | „Building and seeding this classification base is the first necessary stage” (`prd.md:26`) — konieczny, ale nie tu produkt wygrywa |
| Ingest SBOM / purl | Supporting | T-02, T-11 | wejście dla obu poddomen; tylko Maven (`prd.md:107`) |
| Tożsamość i dostęp | Generic | role, konta, sesje | logowanie i role rozwiązane jak wszędzie (`prd.md:91-103`) |

## Niezmienniki i polityki

### Znacznik strategiczny (Core)

| id | kandydat | niezmiennik (cytat) | źródło | status | gdzie pilnowany | luka |
|---|---|---|---|---|---|---|
| R-01 | Strategic mark set per (category, ecosystem) | „No category/ecosystem can ever hold more than 2 active strategic marks.” | `org-data-fixes…/plan-brief.md:69`, `prd.md:73` | enforced | hard (app): `strategic.go:83-106` (advisory lock + count), 409/503 `handlers.go:374-380` | brak ograniczenia w DB (`0003…up.sql`) |
| R-02 | Cel znacznika | „Architect can mark a direct library within a category as strategic” | `prd.md:73` | declared | client-only: przycisk tylko przy kategoriach biblioteki (`organization.tsx:224-243`) | `MarkStrategic` (`strategic.go:55-130`) nie czyta `library_categories` ani `project_libraries`; „direct” nie jest sprawdzane nigdzie (szukano `is_direct` w `strategic.go`, `handlers.go`, `organization.tsx`) |
| R-03 | Uzasadnienie | „Required on both mark and unmark” | `mark-strategic-libraries/plan-brief.md:22` | enforced | hard (app): `handlers.go:358-361,402-405` | DB przyjmuje `''` (`0003…up.sql:7`) |

### Kandydat do wycofania (Core)

| id | kandydat | niezmiennik (cytat) | źródło | status | gdzie pilnowany | luka |
|---|---|---|---|---|---|---|
| R-04 | Reguła kandydata | „Only flag a direct library when a *different* library is strategic in that category”; „transitive → never flagged” | `project-redundancy-report/plan-brief.md:26`, `project-redundancy-report/plan.md:193` | enforced | hard (app), jeden właściciel: `strategic.go:243-245`, użycie `project.go:224-227` | — |
| R-05 | Zakres ekosystemu | „Direct libraries in a category/ecosystem not marked strategic are implicitly candidates to retire.” | `prd.md:73` | declared | „reviewed” liczone globalnie (`strategic.go:230-233`), limit per ekosystem (`strategic.go:94-100`) | kod sam nazywa to „open decision K1”; dziś niewidoczne, bo zawsze `maven` |
| R-06 | Każda biblioteka w raporcie | „Every uploaded library appears in the project report.” | `batch-status-uncategorized-libraries/plan-brief.md:53` | enforced | hard (app): `LEFT JOIN` w `project.go:201-209`, test `project_handlers_test.go:647` | — |

### Biblioteka i kategoryzacja (Supporting)

| id | kandydat | niezmiennik (cytat) | źródło | status | gdzie pilnowany | luka |
|---|---|---|---|---|---|---|
| R-07 | Status ⇔ kategorie | „A categorized library with no category is a status lie”; „Manually attach a category to a `needs_manual` library and confirm it now shows as categorized” | `batch-status-uncategorized-libraries/plan-brief.md:24`, `seed-classification-catalog/plan.md:308` | contradicted | pipeline: `pipeline.go:90-96` | `AttachCategoryHandler` (`handlers.go:335`) przypina kategorię bez zmiany statusu — D-04 |
| R-08 | Ochrona ręcznej korekty | „enqueueing an already-categorized or manually-corrected library would silently overwrite its `library_categories.source`” | `project-redundancy-report/plan.md:164` | declared | soft: tylko `pending` w `project_handlers.go:98-104` | boot sweep może ponownie przetworzyć `pending`/`processing` i upsert nadpisze `source` (`store.go:300`) |
| R-09 | Tożsamość = purl | „`ON CONFLICT (purl) DO NOTHING` so re-uploading an existing purl is a safe no-op” | `seed-classification-catalog/plan.md:75` | enforced | hard (data): `0002…up.sql:24`, `store.go:84-86` | — |
| R-10 | Tylko Maven | „No ecosystems other than Java/Maven (and optionally Python)” | `prd.md:107` | enforced | hard (app): `purl.go:19-21` | brak CHECK na `ecosystem` w DB |

### Batch (Supporting)

| id | kandydat | niezmiennik (cytat) | źródło | status | gdzie pilnowany | luka |
|---|---|---|---|---|---|---|
| R-11 | Zakończenie pustego batcha | „Settle such a batch immediately with the user-approved wording `— done: no new libraries to categorize`.” | `archive/2026-10-27-empty-batch-status/plan.md:5` | contradicted | client-only: `redundancy.tsx:22-28` | `catalog.tsx:21-27` traktuje `{}` jako niezakończony — D-05 |
| R-12 | Członkostwo w batchu | „No change to how known libraries are re-processed. They are still not re-enqueued.” | `empty-batch-status/plan.md:20` | contradicted | — | `project_handlers.go:101-103` kolejkuje znane biblioteki `pending` spoza batcha — D-06 |

### Projekt i dostęp

| id | kandydat | niezmiennik (cytat) | źródło | status | gdzie pilnowany | luka |
|---|---|---|---|---|---|---|
| R-13 | Tożsamość projektu | „case-insensitive match-or-create on upload”; „Replace the project's current library set, no history” | `project-redundancy-report/plan-brief.md:21-22` | enforced | hard (data): `0004…up.sql:9`, transakcja `project.go:107-144` | — |
| R-14 | Direct per projekt | „Direct/transitive is inherently per-project; a global column would let one project's upload corrupt another's report” | `project-redundancy-report/plan-brief.md:23` | enforced | hard (data): `0004…up.sql:14`, `0005_drop_libraries_is_direct.up.sql:1` | — |
| R-15 | Polityka dostępu | „Project SBOM upload is Architect-only; project lists and redundancy reports are available to Architect and Viewer” | `prd.md:101` | enforced | hard (app): `routes.go:35-61`, `middleware.go:29-58` | rola z cookie, nie odświeżana z DB przez 24 h (`session.go:21`) |
| R-16 | Brak danych prywatnych w SBOM | „SBOM data uploaded to the system contains no private/customer data — only library coordinates and their roles.” | `prd.md:42` | enforced | z konstrukcji: zapisywane są tylko purl, nazwa, ekosystem i `is_direct` (`store.go:25-29`, `0004…up.sql:14`) | sam plik nie jest walidowany pod kątem treści (nie musi: nie jest przechowywany) |

**Reguły tylko w kodzie:**
- `error_detail` widzi tylko admin (`handlers.go:286-288`).
- Pipeline przypina tylko nazwy z taksonomii (`pipeline.go:74-78`).
- Po 3 próbach biblioteka przechodzi w `failed` (`queue.go:20`).
- Odzyskiwanie „stuck” działa tylko przy starcie (`main.go:57`). Nie znaleziono ścieżki ponownej kategoryzacji `failed`/`needs_manual` (`store.go:86`, `store.go:203`).
- Dowolny architekt może zastąpić członkostwo projektu o tej samej nazwie (`project.go:52-56`, bez sprawdzenia `created_by`).
- `UnmarkStrategic` nadpisuje uzasadnienie oznaczenia (`strategic.go:139-141`).
- deps.dev traktuje każdy status ≠200 jako „brak mapowania” (`depsdev.go:74-78`).

## Rozjazd modelu i kodu

| id | dokumenty mówią (najnowsze) | kod robi | dowód (path:line) | skutek | naprawa |
|---|---|---|---|---|---|
| D-01 | strategic i kandydat do wycofania są „scoped per technology/ecosystem” (`prd.md:73`) | limit per (kategoria, ekosystem), ale „reviewed” globalnie per kategoria | `strategic.go:94-100`, `strategic.go:230-233`; test charakteryzujący `project_report_test.go:393` | przy drugim ekosystemie znacznik npm zrobi kandydatów z bibliotek Maven | decision (Q-01) |
| D-02 | „mark a direct library within a category as strategic” (`prd.md:73`) | serwer przyjmuje znacznik dla dowolnej istniejącej pary (biblioteka, kategoria); przynależność do kategorii pilnuje tylko UI | `strategic.go:55-130`, `organization.tsx:224-243` | jedno wywołanie API zalicza kategorię jako przejrzaną i zmienia flagi „retire” we wszystkich projektach; zużywa też limit | code |
| D-03 | strategiczna ma być „direct library” (`prd.md:73`) | „direct” nie jest sprawdzane ani na serwerze, ani w UI | `strategic.go:55-130`, `organization.tsx:217-243` | biblioteka wyłącznie przechodnia może zostać standardem | decision (Q-02) |
| D-04 | ręcznie skategoryzowana biblioteka `needs_manual` ma się pokazać jako categorized (`seed-classification-catalog/plan.md:308`) | ręczne przypięcie nie zmienia statusu | `handlers.go:335` | biblioteka z kategorią wisi jako „Needs review”, a batch pokazuje fałszywy problem | code |
| D-05 | pusty batch jest zakończony: „Settle such a batch immediately” (`empty-batch-status/plan.md:5`) | strona katalogu traktuje `{}` jako niezakończony i odpytuje 60 s | `catalog.tsx:21-27` vs `redundancy.tsx:22-28` | upload katalogu ze znanymi purlami wygląda na zawieszony | code |
| D-06 | znane biblioteki „are still not re-enqueued” (`empty-batch-status/plan.md:20`) | upload projektu kolejkuje znane biblioteki `pending`, które nie należą do nowego batcha | `project_handlers.go:101-103`, `store.go:84-86` | batch może być „done”, a biblioteki projektu wciąż się przetwarzają (K4) | decision (Q-03) |
| D-07 | „An organization-level report shows redundancy across all uploaded projects” (`prd.md:36`) | brak takiego odczytu; S-03 zostawił go „Out of scope … a future roadmap slice” (`project-redundancy-report/plan-brief.md:25`), a roadmapa ma M-1 jako done (`roadmap.md:23`) bez slice'a na ten raport | T-22 (szukane) | kryterium sukcesu PRD niezrealizowane, a nikt tego nie śledzi | decision (Q-04) |
| D-08 | redundancja = „two libraries in the same category within one project” (`prd.md:36`) | raport flaguje tylko bezpośrednie biblioteki niestrategiczne w przejrzanej kategorii | `strategic.go:243-245` | PRD i produkt mówią „redundancy” o czym innym (B-01) | docs — późniejsza decyzja: `project-redundancy-report/plan-brief.md:7`, `:26` |
| D-09 | Architect „sets and changes recommendations (standard / to retire)” (`prd.md:98`) | jest tylko znacznik strategiczny; „to retire” jest wyliczane | `strategic.go:243`, `routes.go:56-57` | PRD obiecuje jawną decyzję „to retire” | docs — późniejsza decyzja: `project-redundancy-report/plan-brief.md:26` (flaga wyliczana) |

## Nieaktualne dokumenty

- `prd.md:93`, `prd.md:97` („via configuration”) zastąpione przez `prd.md:103` (konta w bazie, strona admina).
- `shape-notes.md:53` (Admin wgrywa SBOM projektu) zastąpione przez `prd.md:101` i `testing-access-control-data-integrity/plan-brief.md:22`.
- `mark-strategic-libraries/plan-brief.md:25` („Unbounded — no limit enforced”) zastąpione przez `org-data-fixes…/plan-brief.md:28`.
- `archive/shape-notes-2026-10-15-2157.md:131` (wersjonowanie SBOM), `:145` (system sugeruje standard), `:125` (zarządzanie taksonomią) — porzucone w PRD i planach.
- Open Question w `prd.md:114` (czy SBOM niesie direct/transitive) — rozstrzygnięte w `seed-classification-catalog/plan-brief.md:21`, PRD niezaktualizowany.
- Do tego D-08 i D-09 (strona *docs*).

## Ranking

| # | kandydat | rodzaj | poddomena | status | powiązane ID | powód | done when |
|---|---|---|---|---|---|---|---|
| 1 | Cel znacznika strategicznego | agregat (strategic mark set) | Core | declared | R-02, D-02, D-03, Q-02 | przesunięty w górę: blast radius — jedno wywołanie zmienia flagi „retire” w każdym projekcie | `POST …/libraries/{lib}/categories/{cat}/strategic` dla kategorii, do której biblioteka nie jest przypięta, zwraca 4xx i nie zapisuje wiersza w `strategic_marks` |
| 2 | Status po ręcznej kategoryzacji | agregat (Library) | Supporting | contradicted | R-07, D-04 | sprzeczność z udokumentowanym przejściem | po ręcznym przypięciu kategorii do biblioteki `needs_manual` jej `status` = `categorized` |
| 3 | Pusty batch na stronie katalogu | polityka (batch) | Supporting | contradicted | R-11, D-05 | dwie strony mówią co innego o tym samym `{}` | upload katalogu z samymi znanymi purlami pokazuje „done” bez odpytywania; `rg 'function isBatchSettled' web/app` zwraca jedną definicję |
| 4 | Członkostwo w batchu (K4) | agregat niejawny (Batch) | Supporting | contradicted | R-12, D-06, B-04, Q-03 | dokument opiera decyzję `{}` na fałszywej przesłance | dla uploadu projektu ze znaną biblioteką `pending` batch nie pokazuje „done”, dopóki ta biblioteka się nie zakończy — albo dokument opisuje, że batch obejmuje tylko nowe purle |
| 5 | Zakres ekosystemu (K1) | agregat (strategic mark set) | Core | declared | R-05, D-01, Q-01 | dziś niewidoczne (tylko Maven), ale pierwszy drugi ekosystem zmieni flagi | test `…_Characterization` (`project_report_test.go:393`) zastąpiony testem reguły zgodnej z decyzją Q-01 |
| 6 | Ochrona ręcznej korekty | polityka | Supporting | declared | R-08 | ochrona tylko w jednym handlerze | ponowny przebieg pipeline'u dla biblioteki z kategorią `manual` zostawia `source = 'manual'` |
| 7 | Raport organizacji | odczyt | Core | — (missing) | T-22, D-07, Q-04 | kryterium sukcesu bez właściciela | roadmapa ma slice z tym raportem albo PRD go usuwa |

**#1:** Przesunięty z „declared” nad pozycje „contradicted”, bo ma największy zasięg: jedno wywołanie API zmienia wynik Core we wszystkich projektach.
`MarkStrategic` (`strategic.go:55-130`) nie sprawdza, czy biblioteka należy do kategorii ani czy jest bezpośrednia, a UI tylko ukrywa przycisk (`organization.tsx:224-243`).
Taki znacznik zalicza kategorię jako „reviewed” (`strategic.go:230-233`) i zużywa limit R-01, więc reguła kandydata do wycofania działa na złych danych.

## Pytania

| id | pytanie | kto odpowie | odblokowuje |
|---|---|---|---|
| Q-01 | Czy „kategoria przejrzana” i wybór strategiczny mają być liczone per ekosystem (jak limit), czy globalnie per kategoria? | właściciel produktu | D-01, R-05, ranking #5 |
| Q-02 | Czy biblioteka musi być bezpośrednia (w co najmniej jednym projekcie), żeby można ją było oznaczyć jako strategiczną? | właściciel produktu | D-03, ranking #1 |
| Q-03 | Co użytkownik ma rozumieć przez batch: biblioteki nowo wprowadzone przez upload czy wszystko, na co ten upload czeka? | właściciel produktu | D-06, R-12, B-04, ranking #4 |
| Q-04 | Czy raport redundancji na poziomie organizacji jest nadal w zakresie MVP? | właściciel produktu | D-07, ranking #7 |

## Niewiadome i ograniczenia

- Dokumenty: jeden PRD, jedna user story (US-01), brak glosariusza i ADR. Słownictwo w czasie żyje w planach archiwalnych. Z 19 zmian archiwalnych 10 znanych tylko z tytułów, a z 6 planów czytano tylko fragmenty.
- Ekosystem w danych zawsze `maven`, więc reguły zależne od ekosystemu (R-01, R-05) nie są obserwowalne w produkcji.
- Testy czytane tylko po nazwach (dowód testowania, nie źródło reguł). `backend/internal/obs` i `web/app/lib` przeczytane częściowo.
- Brak ścieżek usuwania encji, więc kaskady FK nie są osiągalne z aplikacji (nieocenione jako reguły).
- Copy strony startowej („Health”, „Cost of change”, licencje, maintainerzy) nie ma odpowiedników w kodzie i dokumentach poza `prd.md:39` (metryki zdrowia odłożone, `roadmap.md:155`).
- Priory (repo-map, analiza przepływu, research L4) nie były czytane przez subagentów. Synteza zgadza się z nimi co do K1 i K4.
