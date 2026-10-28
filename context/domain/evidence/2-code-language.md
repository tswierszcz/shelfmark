# Evidence: language in the code

Zakres: `backend/` (bez `hooktest/`, `*_test.go` tylko pośrednio) + `web/app/` (bez `web/build`, `node_modules`, `.react-router`), HEAD 24a8920. Warstwy: persistence (7 par migracji, SQL w Go), typy domenowe (struktury Go w `internal/catalog`, `internal/auth`; typy TS w `web/app/lib`), operacje (metody `Store`, `Queue`, `ProcessLibrary`), entry points (`backend/cmd/shelfmark/routes.go`, `web/app/routes.ts`), UI (route'y, `components/`). Pakiety `internal/db` i `internal/obs` przejrzane tylko pod kątem nazw — czysto techniczne. Etykiety UI zebrane wzorcami `rg` (nagłówki, `<th>`, `<option>`, `placeholder`, tekst w JSX) — próbkowane, nie wyczerpujące. Ustalenia oparte wyłącznie na komentarzu oznaczone `comment`.

## Concepts and their names across layers

| koncept | persistence | domain type | operations | entry points | UI | etykieta dla użytkownika (język) |
|---|---|---|---|---|---|---|
| Biblioteka | `libraries` (`0002_catalog_schema.up.sql:22`), klucz `purl text UNIQUE` (`:24`) | `Library` (`backend/internal/catalog/store.go:32`), `NewLibrary` (`store.go:25`); TS `Library` (`web/app/lib/catalog.ts:6`), `OrganizationLibrary` (`web/app/lib/organization.ts:10`), `ProjectReportLibrary` (`web/app/lib/projects.ts:17`) | `CreateLibraries` (`store.go:76`), `ListLibraries` (`store.go:128`), `LibrariesByPurls` (`project.go:73`), `ProcessLibrary` (`pipeline.go:35`) | `GET /api/catalog/libraries` (`routes.go:52`), `GET /api/organization/libraries` (`routes.go:55`) | `catalog.tsx:220`, `organization.tsx:122` | "Libraries" (EN) |
| Purl / ekosystem | `purl`, `ecosystem text NOT NULL DEFAULT 'maven'` (`0002…up.sql:24,26`) | pola `Purl`, `Ecosystem` (`store.go:34,36`) | `ParsePurl` (`purl.go:13`) — akceptuje tylko `maven` (`purl.go:19-20`) | filtr `?ecosystem=` (`handlers.go:190,237`) | `<th>Purl</th>` (`organization.tsx:205`), placeholder `"maven, npm…"` (`organization.tsx:172`, `catalog.tsx:270`) | "Purl", "Ecosystem" (EN) |
| Kategoria / taksonomia | `categories` (`0002…up.sql:1`) z 14 zasianymi nazwami (`:6-20`) | `Category` (`store.go:18`); TS `Category` (`catalog.ts:1`) | `ListTaxonomy` (`store.go:104`), `taxonomyIDByName` (`pipeline.go:116`) | `GET /api/catalog/categories` → `ListTaxonomyHandler` (`routes.go:53`) | `listTaxonomy` (`catalog.ts:91`) | "Category", "Categories" (EN, `organization.tsx:146,209`) |
| Przypisanie kategorii (+ źródło) | `library_categories`, `source CHECK IN ('github','llm','manual')` (`0002…up.sql:38,41`) | brak typu; tylko `string source` | `AttachCategory` (`store.go:296`), `CategoryNamesByLibrary` (`store.go:232`) | `POST /api/catalog/libraries/{libraryID}/categories` (`routes.go:54`) | kolumna "Add category" (`catalog.tsx:288`) | "Add category" (EN) |
| Kategoryzacja (pipeline + kolejka) | `libraries.status`, `github_repo`, `error_detail` (`0002…:27-29`, `0007…:1`) | `Categorizer` (`pipeline.go:18`), `RepoFinder` (`depsdev.go:18`), `TopicsFetcher` (`github.go:15`), `Queue` (`queue.go:48`), `Enqueuer` (`handlers.go:19`) | `ProcessLibrary`, `Categorize` (`openrouter.go:199`), `FindRepo` (`depsdev.go:56`, `openrouter.go:163`), `Enqueue`/`Sweep`/`markFailed` (`queue.go:97,161,269`) | boot sweep `go catalogQueue.Sweep(ctx, catalog.StuckLibraryAge)` (`main.go:57`) | `LibraryStatusBadge` (`components/library-status-badge.tsx:5`) | "Processing", "Needs review", "Failed" (EN) |
| Partia uploadu (batch) | `upload_batch_id uuid NOT NULL` + indeks (`0002…:30,35`) | `BatchProgress map[string]int` (`store.go:58`); pole `UploadBatchID` (`store.go:39`), `LibraryFilter.BatchID` (`store.go:52`); TS `BatchProgress` (`catalog.ts:34`) | `newBatchID` (`handlers.go:429`), `BatchProgress` (`store.go:311`) | `GET /api/catalog/batches/{batchID}` (`routes.go:51`) | `describeBatch` (`redundancy.tsx:35`) | " — done", "N needs review", " — done: no new libraries to categorize" (EN, `redundancy.tsx:51-61`) |
| Upload katalogu | — | `uploadResponse` (`handlers.go:42`), `SkippedRow` (`sbom.go:13`) | `UploadHandler` (`handlers.go:71`), `ParseCycloneDXSBOM` (`sbom.go:24`), `ParsePurlCSV` (`csv.go:13`) | `POST /api/catalog/upload` (`routes.go:50`), pole `kind` = `"sbom"`/`"csv"` (`handlers.go:87-89`) | `uploadCatalogFile(file, kind)` (`catalog.ts:45-47`) | "Upload", "Kind", "SBOM" (EN, `catalog.tsx:161,165,173`) |
| Oznaczenie strategiczne | `strategic_marks` (`active`, `justification`, `marked_by`, `marked_at`) (`0003_strategic_marks.up.sql:3-8`), indeks częściowy `WHERE active` (`:13`) | `StrategicMark` (`strategic.go:34`); `strategicPair`, `strategicIndex` (`strategic.go:196,206`); TS `StrategicMark` (`organization.ts:3`) | `MarkStrategic` (`strategic.go:55`), `UnmarkStrategic` (`strategic.go:137`), `StrategicMarksByLibrary` (`strategic.go:158`), `isStrategic` (`strategic.go:226`) | `POST`/`DELETE /api/organization/libraries/{libraryID}/categories/{categoryID}/strategic` (`routes.go:56-57`) | badge (`organization.tsx:239`, `redundancy.tsx:329`), filtr (`organization.tsx:179,188-189`), pole `placeholder="Justification"` (`organization.tsx:267`) | "Strategic", "Strategic only", "Non-strategic only", "Justification" (EN) |
| Projekt | `projects` (`0004_project_reports.up.sql:1`), unikalny `lower(name)` (`:9`), `created_by` (`:4`) | `Project` (`project.go:13`); TS `Project` (`projects.ts:3`) | `FindOrCreateProject` (`project.go:51`), `ListProjects` (`project.go:164`) | `GET /api/projects` (`routes.go:59`) | select projektu (`redundancy.tsx:275,283`) | "Project", "Project name", "Select a project…" (EN, `redundancy.tsx:212,275,283`) |
| Skład projektu (bezpośrednia/przechodnia) | `project_libraries (project_id, library_id, is_direct)` (`0004…:11,14`) | `ProjectLibraryEntry` (`project.go:23`) | `ReplaceProjectLibraries` (`project.go:107`), `ParseProjectSBOM` (`project_sbom.go:26`) | `POST /api/projects/upload` (`routes.go:58`) | `<th>Direct</th>` (`redundancy.tsx:301`, `organization.tsx:207`) | "Direct" (EN); "transitive" tylko w kodzie/komentarzu (`project_sbom.go:14`) i w copy strony startowej (`home.tsx:32`) |
| Raport redundancji | tabela nie istnieje; nazwa migracji `0004_project_reports` | `ProjectReportRow` (`project.go:33`), `projectReportCategory`/`projectReportLibrary` (`project_handlers.go:137,147`); TS `ProjectReportCategory`/`ProjectReportLibrary` (`projects.ts:10,17`) | `ProjectReport` (`project.go:193`), `isRetireCandidate` (`strategic.go:243`), `isReviewed` (`strategic.go:233`) | `GET /api/projects/{projectID}/report` (`routes.go:60`); SPA `route("redundancy", …)` (`routes.ts:10`) | `redundancy.tsx:204,271` | "Redundancy report", "Report", "Retire?", "Retire candidate" (EN, `redundancy.tsx:204,271,304,331`) |
| Użytkownik / rola | `users (email, password_hash, role CHECK IN ('admin','architect','viewer'))` (`0006_users.up.sql:1,5`) | `Role` + `RoleAdmin`/`RoleArchitect`/`RoleViewer` (`auth/accounts.go:9-14`), `Account` (`accounts.go:18`), `UserSummary` (`users_store.go:20`), `Session` (`session.go:24`); TS `Me` (`auth.ts:11`), `User` (`users.ts:1`) | `UsersStore.Create/List/GetByEmail` (`users_store.go:56,74,40`), `SeedFromEnvIfEmpty` (`users_store.go:111`), `RequireRole` (`middleware.go:29`) | `/api/login`, `/api/logout`, `/api/me`, `/api/users` (`routes.go:45-49`) | `users.tsx:54,57,112`; nav z rolami (`components/nav.tsx:25-31`) | "Users", "Add user", "Role", "Log in"; "Zaloguj" (PL, `home.tsx:44`) |

Pojęcia z UI bez odpowiednika w kodzie (szukane `rg -il` w `backend` + `web/app`): "Capabilities" (`home.tsx:21`), "Health" z "Last release, maintainers, licence, archived status" (`home.tsx:26-27`), "Cost of change" / "consolidation" (`home.tsx:31-32`) — słowa `licen`, `maintainer`, `consolidat` występują tylko w `web/app/routes/home.tsx`; `capabilit` dodatkowo tylko w komentarzu `store.go:2` (`comment`). `evidence` dla braku w tym zakresie.

## One concept, many names

- **Kategoria ↔ taksonomia ↔ capability.** Tabela i typ `categories`/`Category` (`0002…:1`, `store.go:18`); operacja i handler `ListTaxonomy`/`ListTaxonomyHandler` (`store.go:104`, `handlers.go:301`) pod ścieżką `/api/catalog/categories` (`routes.go:53`); komentarz pakietu: "a curated capability taxonomy" (`store.go:2`, `comment`); strona startowa: "Capabilities … tagged against a curated taxonomy" (`home.tsx:21-22`). `evidence`.
- **Raport projektu ↔ raport redundancji.** Migracja `0004_project_reports`, endpoint `/api/projects/{projectID}/report`, typ `ProjectReportRow`; ekran i trasa SPA `redundancy` (`routes.ts:10`), nagłówek "Redundancy report" (`redundancy.tsx:204`), pozycja menu "Redundancy" (`nav.tsx:30`). W backendzie słowo "redundancy" pojawia się tylko w komentarzach (`project.go:29,188`, `strategic.go:201`, `project_handlers.go:144,154`). `evidence`.
- **Lista bibliotek: "catalog" ↔ "organization".** Ten sam `Store.ListLibraries` obsługuje `ListLibrariesHandler` (`handlers.go:185`, rola admin) i `ListOrganizationLibrariesHandler` (`handlers.go:232`, architect/viewer); UI rozdziela na "Catalog" i "Organization" (`nav.tsx:27,29`). Typ TS `OrganizationLibrary = Library & { is_direct; strategic }` (`organization.ts:10-12`). `evidence`; że to jeden koncept widziany przez dwie role — `inference`.
- **`needs_manual` ↔ "Needs review" ↔ "need review".** Status w bazie `needs_manual` (`0002…:29`); badge "Needs review" (`library-status-badge.tsx:7-8`); podsumowanie partii `needReview` / "needs review" (`redundancy.tsx:54,57`). Filtry w UI pokazują surowe `needs_manual` (`catalog.tsx:237`, `organization.tsx:139`). `evidence`.
- **`pending` + `processing` → "Processing".** Dwa stany scalone w jedną etykietę (`library-status-badge.tsx:11-13`). `evidence`.
- **Użytkownik: `users` ↔ `Account` ↔ `UserSummary` ↔ `Session` ↔ `Me` ↔ `User`.** (`0006_users.up.sql:1`, `accounts.go:18`, `users_store.go:20`, `session.go:24`, `auth.ts:11`, `users.ts:1`). `GetByEmail` zwraca `Account` (`users_store.go:40`). `evidence`.
- **Nazwa/ID strategiczny: "strategic mark" ↔ "strategic designation" ↔ "strategic pick".** Typ `StrategicMark`; komentarze "active strategic designation" (`strategic.go:32,52`) i "strategic pick" (`strategic.go:194,239`) — `comment`.

## One name, many concepts

- **`is_direct`.** (a) per projekt: `project_libraries.is_direct boolean NOT NULL` (`0004…:14`), ustawiane z grafu SBOM (`project_handlers.go:115`); (b) na `Library` jako pochodna "bezpośrednia w którymkolwiek projekcie": `EXISTS (SELECT 1 FROM project_libraries pl WHERE pl.library_id = l.id AND pl.is_direct) AS is_direct` (`store.go:130`), pole `Library.IsDirect` (`store.go:40`); (c) historycznie kolumna `libraries.is_direct` dodana w `0003_strategic_marks.up.sql:1` i usunięta w `0005_drop_libraries_is_direct.up.sql:1`. Ta sama nazwa JSON `is_direct` w `Library` i `ProjectReportRow` (`project.go:40`). `evidence`.
- **"review".** (a) `needs_manual` → "Needs review" = kategoryzacja wymaga człowieka (`library-status-badge.tsx:8`); (b) `isReviewed(categoryID)` = kategoria ma co najmniej jedno aktywne oznaczenie strategiczne: "which today means at least one library, in any ecosystem, holds an active strategic mark in it" (`strategic.go:230-233`). `evidence`.
- **`created` w odpowiedzi uploadu.** Upload katalogu: `Created: len(created)` gdzie `created` = biblioteki z nowej partii (`handlers.go:132,146`); upload projektu: `Created: len(resolved)` = wszystkie rozpoznane purle, także istniejące wcześniej (`project_handlers.go:90,128`). `evidence`.
- **`marked_by` / `created_by`.** Kolumny tekstowe wypełniane `session.Email` (`handlers.go:369,413`, `project_handlers.go:107`) — nazwa sugeruje osobę, wartość to e-mail, nie klucz do `users`. `evidence`.
- **`ecosystem`.** W danych zawsze `"maven"` (`purl.go:31`, `0002…:26`), a w UI placeholder sugeruje `"maven, npm…"` (`organization.tsx:172`). Limit strategiczny liczy się per (kategoria, ekosystem) (`strategic.go:83-104`). `evidence`.

## Implicit concepts

- **Partia uploadu** — brak tabeli; żyje jako `upload_batch_id` na `libraries` (`0002…:30`) i `map[string]int` (`store.go:58`). Partia obejmuje tylko biblioteki nowo wstawione: `ON CONFLICT (purl) DO NOTHING` (`store.go:86`), a `BatchProgress` liczy `WHERE upload_batch_id = $1` (`store.go:313`); UI ma na to osobny komunikat "no new libraries to categorize" (`redundancy.tsx:51`). `evidence`.
- **Status kategoryzacji** — brak enuma w Go; `Status string` (`store.go:38`) porównywany jako literały w `pipeline.go:47,69,93,96`, `queue.go:223,272`, `project_handlers.go:102`, `store.go:203`; zbiór dozwolonych wartości tylko w `CHECK` (`0002…:29`). Web powtarza go jako `SETTLED_STATUSES` w dwóch plikach (`catalog.tsx:17`, `redundancy.tsx:19`) i jako listy `<option>` (`catalog.tsx:234-238`, `organization.tsx:136-140`). `evidence`.
- **Źródło przypisania** (`github`/`llm`/`manual`) — tylko `CHECK` (`0002…:41`) i literały (`pipeline.go:80-82`, `handlers.go:335`); nie jest zwracane do UI w przejrzanych typach TS (`catalog.ts:6-15`). `evidence` dla typów, `unknown` czy jest gdziekolwiek wyświetlane (nie znaleziono `source` w `web/app/lib`).
- **Wersja biblioteki vs artefakt.** Tożsamość = pełny purl (`purl text UNIQUE`, `0002…:24`; `Purl: raw`, `purl.go:29`), a `name` = `namespace/name` bez wersji (`purl.go:23-26`). Brak pojęcia "artefakt niezależny od wersji". `inference`: dwie wersje tego samego artefaktu to dwie biblioteki, a limit `COUNT(DISTINCT sm.library_id)` (`strategic.go:96`) liczy je osobno.
- **Kandydat do wycofania (retire candidate)** — brak tabeli i flagi w bazie; wyliczany w locie: `isDirect && idx.isReviewed(categoryID) && !idx.isStrategic(libraryID, categoryID)` (`strategic.go:244`), ustawiany w `project.go:226`. `evidence`.
- **Biblioteka "zablokowana" (stuck/orphaned)** — tylko wiek + status: `StuckLibraryAge = 10 * time.Minute` (`queue.go:42`), `ListStuckLibraries` (`store.go:199-203`). `evidence`.
- **Limit strategiczny (cap = 2)** — literał `if count >= 2` (`strategic.go:104`), komunikat "already 2 strategic libraries marked for this category and ecosystem" (`handlers.go:375`); brak stałej nazwanej. `evidence`.

## States and transitions

**`libraries.status`**: `pending | processing | categorized | needs_manual | failed` (`0002_catalog_schema.up.sql:29`, domyślnie `'pending'` `:28`). Wszystkie zapisy przez `UpdateLibraryStatus` (`store.go:273-281`), który zawsze nadpisuje `error_detail` ("any transition away from \"failed\" clears a previous failure reason", `store.go:272`).

| przejście | gdzie |
|---|---|
| (insert) → `pending` | DEFAULT w `0002…:28`; `CreateLibraries` (`store.go:84-86`) |
| `pending` → `processing` | `queue.go:223` |
| → `needs_manual` (brak repo / LLM niepewny / 0 przypisanych kategorii) | `pipeline.go:47`, `pipeline.go:69`, `pipeline.go:93` |
| → `categorized` | `pipeline.go:96` |
| → `failed` (po `maxAttempts = 3`, `queue.go:20`, lub panice) | `queue.go:272` |
| `pending`/`processing` starsze niż 10 min → ponowne zakolejkowanie | `queue.go:161`, `store.go:203`, `main.go:57` |
| ręczne przypisanie kategorii | `AttachCategoryHandler` wywołuje tylko `AttachCategory(..., "manual")` (`handlers.go:335`) — **nie zmienia statusu** (`evidence`); biblioteka `needs_manual` pozostaje `needs_manual` po korekcie admina (`inference`) |

Nie znaleziono maszyny stanów ani walidacji przejść w Go — każde wywołanie może zapisać dowolny status (`store.go:273-281` bez warunku na poprzedni status). `evidence`.

**`strategic_marks.active`**: `true` przy `MarkStrategic` (upsert, `strategic.go:111-115`), `false` przy `UnmarkStrategic` (`strategic.go:139-141`, tylko gdy `active = true`). Wiersz nie jest usuwany; `justification`/`marked_by`/`marked_at` są nadpisywane przy każdej zmianie, więc historia poprzedniego oznaczenia ginie (`evidence` z SQL; że to utrata historii — `inference`).

**`users.role`**: `admin | architect | viewer` (`0006_users.up.sql:5`, `accounts.go:12-14`, `auth.ts:13`, `users.ts:1`). Brak operacji zmiany roli w przejrzanych trasach (`routes.go:48-49` — tylko create/list). `evidence`.

**`library_categories.source`**: `github | llm | manual`; upsert nadpisuje źródło (`store.go:300`). Ochrona przed nadpisaniem ręcznej korekty jest tylko w handlerze projektu (enqueue wyłącznie `pending`, `project_handlers.go:96-104`, uzasadnienie w komentarzu — `comment`).

## Where business logic lives

Brak warstwy domenowej: typy w `internal/catalog` to wiersze bazy ("Library is a full libraries row", `store.go:31`; "Project is a full projects row", `project.go:11`). Logika rozproszona:

- **Kategoryzacja** — funkcja proceduralna `ProcessLibrary` (`pipeline.go:35-97`) + kolejka z ponawianiem (`queue.go:208-269`). Warstwa: serwis/pipeline.
- **Limit 2 strategicznych na (kategoria, ekosystem)** — metoda store'a w transakcji z advisory lockiem (`strategic.go:83-106`); baza nie ma ograniczenia (tylko PK `(library_id, category_id)`, `0003…`). Warstwa: store (SQL).
- **Wymóg uzasadnienia** — tylko handler: "missing or blank justification" (`handlers.go:359,403`); kolumna `justification text NOT NULL` (`0003…:7`) dopuszcza pusty string. Warstwa: route handler + częściowo DB.
- **Strategic / reviewed / retire candidate** — `strategicIndex`, opisany jako "the single owner of the \"strategic\", \"reviewed\" and \"retire candidate\" rules" (`strategic.go:202-204`, `comment`); używany w `ProjectReport` (`project.go:226`). Filtr "strategic" na liście bibliotek używa osobnego predykatu SQL `activeStrategicMarkExists` (`strategic.go:275`). Warstwa: typ pomocniczy w pakiecie store.
- **Direct/transitive** — parser SBOM (`project_sbom.go:10-25`), z fallbackiem "every parsed purl is reported as direct" (`project_sbom.go:20-21`, `comment`). Warstwa: parser wejścia.
- **Tylko Maven** — `ParsePurl` (`purl.go:19-21`). Warstwa: parser wejścia.
- **Unikalność projektu po nazwie bez wielkości liter** — DB `projects_name_lower_idx` (`0004…:9`) + `FindOrCreateProject` (`project.go:51`). Warstwa: DB + store.
- **Uprawnienia ról** — tabela tras (`routes.go:35-61`) + `RequireRole` (`middleware.go:29`); w UI powielone w `LINKS` (`nav.tsx:25-31`). Warstwa: entry point.
- **Interpretacja postępu partii** — tylko UI (`describeBatch`, `redundancy.tsx:35-62`; `SETTLED_STATUSES` w `catalog.tsx:17`, `redundancy.tsx:19`).

## Unknowns

- Czy po ręcznym przypisaniu kategorii status powinien przejść na `categorized` — kod tego nie robi (`handlers.go:335`); intencja nieznana.
- Czy "reviewed" ma być per ekosystem — komentarz mówi "open decision K1" (`strategic.go:232`, `comment`), a limit strategiczny jest per ekosystem (`strategic.go:99`) — niespójność skali, rozstrzygnięcie `unknown`.
- Znaczenie "redundancy" jako miary: w backendzie nie znaleziono żadnego zliczania bibliotek na kategorię w projekcie (szukano `redundan` w `backend/`, trafienia tylko w komentarzach); raport to lista wierszy z flagami. Czy to cała definicja redundancji — `unknown`.
- Funkcje z copy strony startowej ("Health", "Cost of change", "runtime or test", "how many projects") — brak nazw w kodzie w zakresie `backend/` + `web/app/`.
- `source` przypisania — nie znaleziono w typach TS ani route'ach web; czy jest gdzieś prezentowane — `unknown`.
- Czy `LoadAccounts`/`Account` (env) to pojęcie domenowe, czy tylko seed — używane wyłącznie w `SeedFromEnvIfEmpty` (`users_store.go:120`) i jako typ zwrotny `GetByEmail`.

## Commands run

- `git ls-files backend web | grep -E '\.(go|ts|tsx|sql)$'`
- `cat -n` / `grep -n` na `backend/internal/db/migrations/*.up.sql`
- `cat -n backend/cmd/shelfmark/routes.go web/app/routes.ts`
- `rg -n '^(type|func) |^const|…' backend` (deklaracje typów i funkcji)
- `rg -n 'IsDirect|is_direct'`, `rg -n '"(pending|processing|categorized|needs_manual|failed)"…' backend web/app`
- `rg -n '^export (type|…)' web/app/lib/*.ts`; `rg` etykiet JSX (`<h1-3>`, `<th>`, `<option>`, `placeholder`) w `web/app/routes/*.tsx`
- `rg -il` dla: redundan, capabilit, consolidat, licen, maintainer, transitive, taxonomy, retire, review, orphan|stuck, needs_manual|manual
- `rg -n 'session.Email|markedBy|createdBy'`, `rg -n 'LoadAccounts|Accounts'`, `rg -n 'Sweep\(|StuckLibraryAge' backend/cmd`
- odczyty fragmentów (`sed -n`): `store.go`, `project.go`, `strategic.go`, `pipeline.go`, `handlers.go`, `project_handlers.go`, `project_sbom.go`, `sbom.go`, `purl.go`, `auth/accounts.go`, `home.tsx`, `dashboard.tsx`, `redundancy.tsx`, `catalog.tsx`, `components/library-status-badge.tsx`, `components/nav.tsx`
