# Evidence: rules guarded in the code

Zakres: `.` (backend/ + web/) wg `context/domain/evidence/0-sources.md`, HEAD 24a8920. Pominięto ścieżki wykluczone w inwentarzu (m.in. `backend/hooktest/`, `.github/`, `web/build/`).
Warstwy przeszukane: migracje SQL (`backend/internal/db/migrations/*.up.sql`), tabela tras z rolami (`backend/cmd/shelfmark/routes.go`), middleware (`backend/internal/auth/middleware.go`), handlery i store (`backend/internal/catalog/*.go`, `backend/internal/auth/*.go`), kolejka i pipeline (`queue.go`, `pipeline.go`), parsery (`csv.go`, `sbom.go`, `project_sbom.go`, `purl.go`), klienci zewnętrzni (`depsdev.go`, `github.go`, `openrouter.go`), UI (`web/app/routes/*.tsx`, `web/app/lib/require-auth.tsx`, `web/app/components/nav.tsx`).
Wzorce: `CHECK|UNIQUE|NOT NULL|REFERENCES`, `errors.Is|Err[A-Z]`, `writeCatalogError|writeUserError`, `RequireRole`, `Begin(`, `DELETE|UPDATE`, w UI `disabled|required|role|isArchitect|trim()`.
Nie pokryto: testy czytane tylko jako lista nazw (dowód, że reguła jest testowana, nie źródło reguły); `backend/internal/obs/*` (logowanie, nie reguły biznesowe); klient `web/app/lib/*.ts` poza fragmentami cytowanymi.

Wszystkie opisy reguł w kolumnie „reguła” to **inference** (moje sformułowanie); cytaty z kodu w backtickach są **evidence**.

## Guarded rules

### Uprawnienia (role) — kto może wykonać operację

| reguła (inference) | guarded at | paths covered | strictness | failure handling |
|---|---|---|---|---|
| Istnieją dokładnie trzy role: admin, architect, viewer. | data: `backend/internal/db/migrations/0006_users.up.sql:5` `CHECK (role IN ('admin', 'architect', 'viewer'))`; app: `backend/internal/auth/handlers.go:123-128` (`"role must be one of admin, architect, viewer"`); `backend/internal/auth/accounts.go:12-15` | `UsersStore.Create` (`users_store.go:56`), `SeedFromEnvIfEmpty` (`users_store.go:131-139`) | hard (data) | named error 400 (handler); w seedzie — błąd bootu (`main.go:37-39`) |
| Każda chroniona trasa wymaga ważnej sesji i roli z listy trasy; brak sesji → 401, zła rola → 403. | app: `backend/internal/auth/middleware.go:29-58`; tabela tras `backend/cmd/shelfmark/routes.go:35-61`, opakowanie `routes.go:71-73` | wszystkie trasy `/api/*` z niepustym `Roles`; publiczne: `/healthz`, `/api/login`, `/api/logout`, SPA (`routes.go:44-46,61`) | hard (app) | generic error (pusty 401/403) |
| Tylko admin wgrywa katalog, listuje bibliotekę z `error_detail`, ręcznie przypina kategorię, zarządza użytkownikami. | app: `routes.go:48-50,52,54` (`admin`) | POST/GET `/api/users`, POST `/api/catalog/upload`, GET `/api/catalog/libraries`, POST `.../categories` | hard (app) | 403 |
| Tylko architect oznacza/odznacza bibliotekę jako strategiczną i wgrywa SBOM projektu. | app: `routes.go:56-58` (`architect`) | POST/DELETE `.../strategic`, POST `/api/projects/upload` | hard (app) | 403 |
| Architect i viewer (nie admin) widzą widok organizacji, projekty i raport redundancji. | app: `routes.go:40,55,59,60` (`architectViewer`) | GET `/api/organization/libraries`, `/api/projects`, `/api/projects/{id}/report` | hard (app) | 403 |
| Postęp batcha widzą admin i architect („lets the architect who uploads a project SBOM poll its categorization batch”). | app: `routes.go:37-39,51` | GET `/api/catalog/batches/{batchID}` | hard (app) — bez sprawdzenia, czyj to batch (inference: każdy admin/architect widzi dowolny batch) | 403 |
| To samo w UI: nawigacja i trasy ukryte wg roli. | UI: `web/app/components/nav.tsx:8-13,32`; `web/app/lib/require-auth.tsx:18-31` (przekierowanie na `/dashboard`); `web/app/routes/catalog.tsx:348`, `users.tsx:148`, `organization.tsx:314`, `redundancy.tsx:366` | widoki SPA | client-only (zdublowane hard (app) na API) | przekierowanie |
| Przyciski „Mark strategic”/„Unmark” i formularz uploadu SBOM projektu widzi tylko architect. | UI: `web/app/routes/organization.tsx:18,243,257`; `web/app/routes/redundancy.tsx:68,206` | widoki SPA | client-only (zdublowane hard (app) na API) | element ukryty |
| Rola w sesji pochodzi z podpisanego cookie, ważnego 24 h; nie jest ponownie sprawdzana w tabeli `users`. | app: `backend/internal/auth/session.go:21` (`sessionTTL = 24 * time.Hour`), `session.go:143-157` | każda chroniona trasa | hard (app) dla podpisu/wygaśnięcia; **inference**: zmiana/usunięcie konta nie unieważnia sesji (w zakresie nie znaleziono ścieżki zmiany roli ani usuwania użytkownika — grep `DELETE|UPDATE` w `backend/**/*.go`) | 401 |

### Konta użytkowników

| reguła (inference) | guarded at | paths covered | strictness | failure handling |
|---|---|---|---|---|
| Email konta jest unikalny. | data: `0006_users.up.sql:3` `email text NOT NULL UNIQUE`; app: `users_store.go:62-64` → `ErrEmailTaken`; `handlers.go:136-139` → 409 `"email already in use"` | `Create`, seed | hard (data) — **inference**: unikalność rozróżnia wielkość liter (brak `lower()`), w przeciwieństwie do nazw projektów | named error (409) |
| Hasło nowego konta ma co najmniej 8 znaków; email i hasło wymagane. | app: `backend/internal/auth/handlers.go:115-122` (`"password must be at least 8 characters"`) ; UI: `web/app/routes/users.tsx:66,80` tylko `required` (bez `minLength`) | POST `/api/users`; seed z env (`users_store.go:111-140`) nie sprawdza długości (hash podany gotowy) | hard (app) dla API | named error 400 |
| Konta seedowe tworzone są tylko, gdy tabela `users` jest pusta. | app: `users_store.go:111-118` (`if count > 0 { return nil }`) | boot (`main.go:37`) | hard (app) | — |

### Biblioteka (libraries) i ingest

| reguła (inference) | guarded at | paths covered | strictness | failure handling |
|---|---|---|---|---|
| Biblioteka jest identyfikowana przez purl; ten sam purl to ta sama biblioteka. | data: `0002_catalog_schema.up.sql:24` `purl text UNIQUE NOT NULL`; app: `store.go:84-86` `ON CONFLICT (purl) DO NOTHING` | `CreateLibraries` z `UploadHandler` (`handlers.go:127`) i `ProjectUploadHandler` (`project_handlers.go:81`) | hard (data) — **inference**: purl zapisywany surowo (`purl.go:28` `Purl: raw`), więc różne wersje i różny zapis tego samego pakietu to różne biblioteki | silently corrects: ponowny purl pomijany bez informacji (komentarz `store.go:72-75` „safe no-op”) |
| Akceptowane są wyłącznie purle typu `maven`. | app: `backend/internal/catalog/purl.go:19-21` (`"unsupported purl type %q (only %q is supported)"`); ekosystem ustawiany na stałe `purl.go:31`; default DB `0002...:26` `ecosystem text NOT NULL DEFAULT 'maven'` | `ParsePurlCSV` (`csv.go:30`), `ParseCycloneDXSBOM` (`sbom.go:49`), `ParseProjectSBOM` (`project_sbom.go:78`) | hard (app) — DB nie ma CHECK na `ecosystem` | logs and continues → wiersz trafia do `skipped` w odpowiedzi (`sbom.go:50-53`), upload trwa |
| Plik uploadu musi być CycloneDX (`bomFormat`), CSV lub SBOM; `kind` ∈ {`sbom`,`csv`}; limit 10 MB. | app: `handlers.go:40,73,87-91`; `sbom.go:30-32`; `project_sbom.go:32-34` | oba uploady | hard (app) | named error 400 |
| Status biblioteki ma 5 wartości: `pending`, `processing`, `categorized`, `needs_manual`, `failed`. | data: `0002_catalog_schema.up.sql:28-29` CHECK | `UpdateLibraryStatus` (`store.go:273-291`) | hard (data) dla wartości; **przejścia nie są pilnowane** (UPDATE bez warunku na bieżący status, `store.go:275-280`) | DB error → generic |
| `categorized` tylko gdy dołączono ≥1 kategorię z taksonomii; inaczej `needs_manual`. | app: `pipeline.go:90-96` (`if attached == 0 { ... "needs_manual" }`, komentarz: `"categorized" with no category would read as success while the library is invisible`) | `ProcessLibrary` (jedyny zapisujący `categorized`) | hard (app) | silently corrects → `needs_manual` |
| Brak repo GitHub lub niepewna odpowiedź LLM → `needs_manual` (nie błąd). | app: `pipeline.go:46-48`, `68-70`; `openrouter.go:218-222` (niepoprawny JSON → `confident=false`) | pipeline | hard (app) | silently corrects (`needs_manual`, bez `error_detail`) |
| Po 3 nieudanych próbach biblioteka przechodzi w `failed` z `error_detail`. | app: `queue.go:20` `maxAttempts = 3`; `queue.go:232-264`; `markFailed` `queue.go:269-276` | `Queue.process` | hard (app) | named status `failed` + log Error |
| `error_detail` widzi tylko admin. | app: `handlers.go:286-288` (`l.ErrorDetail = nil` — „admin-only pipeline diagnostic”); raport projektu nie kopiuje pola (`project_handlers.go` budowa `Library` bez `ErrorDetail`) | GET `/api/organization/libraries`, GET report | hard (app) (read path) | — |

### Kategorie (taksonomia) i przypisania

| reguła (inference) | guarded at | paths covered | strictness | failure handling |
|---|---|---|---|---|
| Taksonomia to zamknięty, seedowany zbiór 14 nazw; nazwy unikalne. | data: `0002_catalog_schema.up.sql:1-20` (`name text UNIQUE NOT NULL`, INSERT seed); brak endpointu tworzenia kategorii (routes.go:43-62) | — | hard (data) | — |
| Pipeline przypisuje tylko kategorie obecne w taksonomii. | app: `pipeline.go:74-78` („LLM returned a name outside the taxonomy; nothing to attach it to”) | `ProcessLibrary` | hard (app) + FK `0002...:40` | silently corrects (nazwa pomijana) |
| Przypisanie ma źródło `github` / `llm` / `manual`; para (biblioteka, kategoria) jednokrotna. | data: `0002...:41-42` CHECK + PK | `AttachCategory` (`store.go:296-308`) | hard (data) | — |
| Kategoria w użyciu nie może zostać usunięta. | data: `0002...:40` `ON DELETE RESTRICT`; `0003...:5` | brak ścieżki usuwania w kodzie | hard (data) | — |
| Ręczne przypisanie kategorii przez admina wygrywa nad wcześniejszym źródłem (upsert nadpisuje `source`). | app: `store.go:300` `ON CONFLICT ... DO UPDATE SET source = EXCLUDED.source`; `handlers.go:335` z `"manual"` | `AttachCategoryHandler` | soft — patrz „Writers that work against a guard” | — |
| Nie dołącza się kategorii do nieistniejącej biblioteki/kategorii. | data: FK `0002...:39-40` | `AttachCategoryHandler` | hard (data) | **generic error** 500 `"failed to attach category"` (`handlers.go:335-337`) — brak mapowania FK na 400/404, w odróżnieniu od `MarkStrategic` |

### Oznaczenie strategiczne (strategic_marks)

| reguła (inference) | guarded at | paths covered | strictness | failure handling |
|---|---|---|---|---|
| Najwyżej 2 różne biblioteki strategiczne na parę (kategoria, ekosystem). | app: `backend/internal/catalog/strategic.go:94-106` (`COUNT(DISTINCT sm.library_id) ... if count >= 2 { return ErrStrategicCapReached }`), serializacja `strategic.go:83-92` (`pg_advisory_xact_lock`), timeout `strategic.go:50,73`; handler `handlers.go:374-376` 409 `"already 2 strategic libraries marked for this category and ecosystem"` | `MarkStrategic` — jedyny writer `active = true` (grep `strategic_marks` w kodzie nie-testowym) | hard (app) — brak constraintu/triggera w DB (`0003_strategic_marks.up.sql`) | named error 409; busy → 503 `ErrStrategicMarkBusy` (`handlers.go:378-380`) |
| Oznaczenie/odznaczenie wymaga niepustego uzasadnienia; autor = email z sesji. | data: `0003...:7-8` `justification text NOT NULL`, `marked_by text NOT NULL`; app: `handlers.go:358-361`, `402-405` (`strings.TrimSpace(...) == ""` → `"missing or blank justification"`), `handlers.go:369,413` (`session.Email`); UI: `organization.tsx:74-77,96-99` | POST/DELETE strategic | hard (app) dla niepustości (DB przyjmie `''` lub spacje) | named error 400 |
| Biblioteka i kategoria muszą istnieć. | app+data: `strategic.go:63-66` (`ErrInvalidReference`), FK `0003...:4-5`, mapowanie `strategic.go:118-121`; `handlers.go:370-372` 400 | MarkStrategic | hard (data) | named error 400 |
| Odznaczyć można tylko aktywne oznaczenie; odznaczenie jest miękkie (`active = false`), historia nadpisywana. | app: `strategic.go:139-149` (`WHERE ... AND active = true`; `ErrStrategicMarkNotFound`); `handlers.go:414-416` 404 | UnmarkStrategic | hard (app) | named error 404 |
| **Oznaczyć jako strategiczną można tylko w kategorii, do której biblioteka jest przypisana.** | UI: `organization.tsx:224-243` (przycisk renderowany tylko w pętli `library.categories.map`) | widok organizacji | **client-only** — `MarkStrategic` (`strategic.go:55-130`) nie sprawdza `library_categories`; bezpośredni POST na dowolną istniejącą parę przejdzie (evidence: brak zapytania o `library_categories` w `strategic.go`) | — |
| Kategoria „przejrzana” = istnieje ≥1 aktywne oznaczenie w niej (dowolny ekosystem); kandydat do wycofania = bezpośrednia zależność w przejrzanej kategorii, nie będąca wyborem strategicznym. | app (read path): `strategic.go:201-245` — `isReviewed` („today means at least one library, in any ecosystem … open decision K1”, `strategic.go:230-232`), `isRetireCandidate` `strategic.go:243-245`; użycie `project.go:224-227` | GET report | hard (app), jedno źródło reguły (`strategicIndex`) | — (kategoria-less wiersz nigdy nie jest kandydatem, `project.go:224`) |

### Projekt i raport redundancji

| reguła (inference) | guarded at | paths covered | strictness | failure handling |
|---|---|---|---|---|
| Nazwa projektu unikalna bez względu na wielkość liter; ponowny upload tej samej nazwy aktualizuje ten sam projekt. | data: `0004_project_reports.up.sql:9` `CREATE UNIQUE INDEX projects_name_lower_idx ON projects (lower(name))`; app: `project.go:52-56` `ON CONFLICT (lower(name)) DO UPDATE SET updated_at = now()` | `FindOrCreateProject` | hard (data) | silently corrects — upload pod istniejącą nazwą (także przez innego architekta) przejmuje projekt; `created_by` zostaje pierwotny (inference z `project.go:55`) |
| Nazwa projektu wymagana (po trim). | app: `project_handlers.go:50-53`; UI: `redundancy.tsx:174,238` | POST `/api/projects/upload` | hard (app) | named error 400 |
| Członkostwo bibliotek w projekcie jest w całości zastępowane przy każdym uploadzie. | app: `project.go:107-144` (DELETE + INSERT w jednej transakcji) | upload projektu | hard (app) | — |
| Biblioteka jest bezpośrednia, jeśli SBOM nie pozwala ustalić inaczej (konserwatywny default). | app: `project_sbom.go:89-94` (`direct := true`), opis `project_sbom.go:16-25` | `ParseProjectSBOM` | hard (app) | silently corrects (brak grafu → wszystko `direct`) |
| Flaga „direct” należy do relacji projekt–biblioteka, nie do biblioteki; w katalogu biblioteka jest „direct”, jeśli jest bezpośrednia w którymkolwiek projekcie. | data: `0004...:14` `is_direct boolean NOT NULL`, usunięcie z `libraries` `0005_drop_libraries_is_direct.up.sql:1`; app: `store.go:130` (`EXISTS (... pl.is_direct)`) | ListLibraries | hard (data) | — |
| Ponowny upload SBOM projektu kolejkuje tylko biblioteki `pending`, aby nie nadpisać ręcznych korekt. | app: `project_handlers.go:96-105` (komentarz: „enqueueing an already-categorized or manually-corrected library would silently overwrite its library_categories.source”) | upload projektu | soft — patrz niżej | — |
| Raport dla nieistniejącego projektu → 404; pusty batch → 200 z pustą mapą. | app: `project_handlers.go:168-175`; `handlers.go:152-169` | GET report, GET batch | hard (app) | named error 404 / brak błędu |

## Soft and client-only rules

- **client-only: strategiczność tylko w przypisanej kategorii** — `organization.tsx:224-243` vs. `strategic.go:55-130` (brak sprawdzenia `library_categories`). Bypass: `POST /api/organization/libraries/{lib}/categories/{dowolna istniejąca kategoria}/strategic` jako architect. Skutek (inference): taki znak liczy się do limitu 2 i sprawia, że kategoria jest „reviewed” (`strategic.go:217-220`), co może oznaczyć inne biblioteki projektu jako retire candidates.
- **soft: ochrona ręcznych korekt przed pipeline'em** — `project_handlers.go:96-105` kolejkuje tylko `pending`, ale:
  - Boot sweep re-kolejkuje `pending` i `processing` (`store.go:199-206`, `queue.go:161-175`); jeśli admin ręcznie przypisze kategorię bibliotece wciąż w tych stanach, ponowny przebieg `ProcessLibrary` wykona `AttachCategory(..., "github"|"llm")` (`pipeline.go:80-85`) i nadpisze `source = 'manual'` (`store.go:300`). Inference, nie potwierdzone testem.
  - `project_handlers.go:102` kolejkuje każdą bibliotekę `pending` — także taką, która jest już w kolejce z innego uploadu (możliwe podwójne przetwarzanie; inference).
- **soft: przejścia statusu** — CHECK ogranicza wartości (`0002...:29`), ale `UpdateLibraryStatus` (`store.go:273-291`) pozwala na dowolne przejście (UPDATE bez warunku na bieżący status). Jedynymi writerami statusu są kolejka i pipeline (`queue.go:223,272`, `pipeline.go:47,69,93,96`). Nie znalazłem ścieżki biznesowej, która by to obchodziła; ryzyko jest teoretyczne.
- **soft: limit 2 strategicznych** — tylko aplikacja (advisory lock), brak constraintu w DB (`0003_strategic_marks.up.sql:1-13`). Jedyny writer w kodzie przechodzi przez check, więc *hard (app)*; zapis bezpośrednio w SQL ominąłby regułę.
- **client-only (UI pomocniczo)**: `required`/`disabled` w formularzach (`catalog.tsx:185,191,321`, `users.tsx:66,80,103`, `redundancy.tsx:217,232,238`) — wszystkie zdublowane na serwerze poza wspomnianą regułą strategiczną.

## Writers that work against a guard

- `AttachCategoryHandler` (`handlers.go:325-342`) dodaje kategorię ręcznie, ale **nie zmienia statusu** biblioteki: biblioteka `needs_manual` lub `failed` z ręcznie przypisaną kategorią pozostaje w tym statusie (evidence: handler wywołuje tylko `AttachCategory`, `handlers.go:335`). Działa przeciw regule „`categorized` ⇔ ≥1 kategoria” (`pipeline.go:90-96`) w odwrotnym kierunku: kategoria jest, status mówi, że nie.
- `ProcessLibrary` (`pipeline.go:80-85`) nadpisuje `source` istniejącego przypisania przy ponownym przebiegu (upsert `store.go:300`) — przeciw ochronie ręcznych korekt (`project_handlers.go:96-101`).
- `ProcessLibrary` nigdy nie usuwa wcześniej dołączonych kategorii (grep `DELETE` — tylko `project.go:114`), więc ponowny przebieg kumuluje przypisania (inference).
- `FindOrCreateProject` (`project.go:52-56`) — upsert po `lower(name)` pozwala dowolnemu architektowi zastąpić członkostwo cudzego projektu (`ReplaceProjectLibraries`); brak sprawdzenia `created_by`.
- `UnmarkStrategic` (`strategic.go:139-141`) nadpisuje `justification`/`marked_by` oryginalnego oznaczenia uzasadnieniem odznaczenia — historia oznaczenia jest tracona (jeden wiersz na parę, PK `0003...:10`).

## Failure handling

| miejsce | zachowanie | klasyfikacja |
|---|---|---|
| `depsdev.go:74-78` | każdy status ≠ 200 (w tym rate limit, 5xx) = „brak mapowania” (`"Not found, rate-limited, or otherwise unresolvable ... Not an error."`) → przejście do LLM-findera | **swallowed silently** (bez logu) |
| `depsdev.go:81-86` | błąd dekodowania odpowiedzi 2xx = „brak mapowania” | **swallowed silently** |
| `openrouter.go:218-219` | niepoprawny JSON od LLM → `confident=false` → `needs_manual` (`pipeline.go:68-70`), bez `error_detail` | **silently corrects** |
| `pipeline.go:74-78` | nazwa kategorii spoza taksonomii pomijana | **silently corrects** |
| `store.go:84-86` | istniejący purl pomijany (`DO NOTHING`); w `UploadHandler` liczba `created` liczy tylko nowe w batchu (`handlers.go:132-146`) | **silently corrects** |
| `csv.go:39-43`, `sbom.go:49-53`, `project_sbom.go:78-82` | wiersz z niewspieranym/nieparsowalnym purlem trafia do `skipped` w odpowiedzi | logs and continues (raportowane klientowi, nie logowane) |
| `queue.go:222-229` | nieudany zapis `processing` → log Error i return; wiersz zostaje `pending` do następnego restartu | **logs and continues** |
| `queue.go:257-259`, `236-239` | shutdown w trakcie prób → biblioteka zostaje `processing` | logs and continues (sweep przy następnym boocie) |
| `queue.go:97-106` | Enqueue po shutdown → job porzucony (`"enqueue after shutdown, job dropped"`) | **logs and continues** |
| `queue.go:212-219` | panic → log + `failed` | named status |
| `handlers.go:79-85`, `project_handlers.go:42-48` | błąd `ParseMultipartForm` inny niż limit rozmiaru ignorowany | swallowed silently (format, nie reguła biznesowa) |
| `handlers.go:335-337` | FK violation przy ręcznym przypisaniu → 500 | generic error |
| `strategic.go`, `users_store.go:62-64` | `ErrStrategicCapReached`, `ErrStrategicMarkBusy`, `ErrInvalidReference`, `ErrStrategicMarkNotFound`, `ErrEmailTaken` | named error |

## Atomicity

| operacja | transakcja? | cytat | uwagi |
|---|---|---|---|
| Oznaczenie strategiczne (lookup ekosystemu, lock, count, upsert) | tak | `strategic.go:56-60,125` | lock per (kategoria, ekosystem) `strategic.go:83-86`; komentarz `strategic.go:80-82`: poprawne „only while libraries.ecosystem is never updated” |
| Zastąpienie członkostwa projektu (DELETE + INSERT) | tak | `project.go:108-112,139` | |
| Seed kont | tak | `users_store.go:125-129,141` | sprawdzenie `Count` (`users_store.go:112`) **poza** transakcją — dwa równoległe booty mogą oba zobaczyć 0 (inference; UNIQUE na email by wtedy odrzucił drugi) |
| **Upload SBOM projektu** (CreateLibraries → LibrariesByPurls → Enqueue → FindOrCreateProject → ReplaceProjectLibraries) | **nie** | `project_handlers.go:81,90,102-103,107,117` | **finding**: błąd w `FindOrCreateProject`/`ReplaceProjectLibraries` zostawia utworzone i zakolejkowane biblioteki oraz (w drugim przypadku) projekt z poprzednim lub pustym członkostwem i świeżym `updated_at`; zwracane 500 |
| Upload katalogu (CreateLibraries → ListLibraries → Enqueue) | nie (CreateLibraries to `pgx.Batch` na poolu, bez `Begin`) | `store.go:81-98`, `handlers.go:127-140` | **inference**: `SendBatch` poza transakcją — błąd w połowie zostawia część wierszy; te `pending` podejmie dopiero boot sweep po 10 min i restarcie |
| Kategoryzacja biblioteki (N × AttachCategory → UpdateLibraryStatus) | nie | `pipeline.go:80-96`, `store.go:296-308` | **finding**: błąd po części przypisań zostawia przypisane kategorie przy statusie `processing`; po 3 próbach `failed` z częściowymi kategoriami |
| Raport projektu (loadStrategicIndex, potem zapytanie raportu) | nie | `project.go:194-209` | dwa odczyty bez wspólnego snapshotu (drobne, inference) |

## Scheduled and asynchronous work

| reguła zależna od pracy w tle | trigger | handler | uwagi |
|---|---|---|---|
| Kategoryzacja nowych bibliotek | `queue.Enqueue` z `handlers.go:138-140` i `project_handlers.go:101-104`; pula 5 workerów `main.go:49`, `queue.go:84-87` | `Queue.process` → `ProcessLibrary` (`queue.go:208-265`, `pipeline.go:35`) | trigger dociera do handlera (evidence) |
| Ponowienie przy błędzie przejściowym | pętla `queue.go:232-260`, `retryBackoff = 500ms` `queue.go:23` | `ProcessLibrary` | 3 próby, potem `failed`; brak późniejszego ponowienia dla `failed`/`needs_manual` (sweep bierze tylko `pending`/`processing`, `store.go:203`) — **finding**: nie znaleziono w kodzie ścieżki ponownej kategoryzacji `failed`/`needs_manual` (ponowny upload purla jest no-op, `store.go:86`) |
| Odzyskanie bibliotek „utkniętych” (`pending`/`processing` > 10 min) | **tylko przy starcie**: `main.go:57` `go catalogQueue.Sweep(ctx, catalog.StuckLibraryAge)`; `StuckLibraryAge = 10 * time.Minute` `queue.go:42` | `Queue.Sweep` `queue.go:161-179` → `ListStuckLibraries` `store.go:199-226` | **finding**: brak okresowego triggera (grep `Sweep(` w kodzie nie-testowym — tylko `main.go:57`); wiersz osierocony w trakcie działania (np. `queue.go:225-229`, porzucony Enqueue `queue.go:97-106`) czeka do następnego restartu/deployu |
| Wygaśnięcie sesji | sprawdzane leniwie przy każdym żądaniu `session.go:153-155` | `VerifySession` | brak zadania w tle (niepotrzebne — sesja bezstanowa) |

## Unknowns

- Czy `libraries.ecosystem` może kiedykolwiek przyjąć wartość inną niż `maven` — kod wymusza `maven` (`purl.go:19-21,31`), ale limit strategiczny jest liczony per ekosystem (`strategic.go:96-100`), a „reviewed” globalnie (`strategic.go:230-235`). Nie ustaliłem, która granica jest zamierzona (kod sam wskazuje „open decision K1”).
- Czy ręczne przypisanie kategorii powinno zmieniać status biblioteki — kod tego nie robi; brak testu, który by to rozstrzygał (przejrzane nazwy testów w `handlers_test.go`: `TestAttachCategoryHandler_AttachesAndRejectsMalformedBody`).
- Czy wielu architektów może dzielić/nadpisywać projekty tej samej nazwy — brak guardu właściciela; nie znaleziono testu dotyczącego innego `created_by`.
- Brak ścieżek usuwania bibliotek, projektów, kategorii i użytkowników w kodzie (grep `DELETE` w `backend/**/*.go`, bez testów): reguły kaskad FK (`ON DELETE CASCADE` w `0002...:39`, `0003...:4`, `0004...:12-13`) są obecnie nieosiągalne z aplikacji.
- Nie weryfikowałem w UI komunikatów 409/503 dla limitu strategicznego poza tym, że błąd serwera trafia do `setFormError` (`organization.tsx:88,110`).

## Commands run

- `git ls-files backend web | grep -E '\.(go|ts|tsx|sql)$' ... | xargs wc -l`
- `cat -n` migracji `*.up.sql`, `routes.go`, `handlers.go`, `strategic.go`, `store.go`, `project.go`, `project_handlers.go`, `queue.go`, `pipeline.go`, `csv.go`, `sbom.go`, `project_sbom.go`, `purl.go`, `auth/{middleware,accounts,handlers,session,users_store}.go`, `cmd/shelfmark/main.go`
- `grep -n` w `openrouter.go`, `depsdev.go`, `github.go`; `sed -n` fragmentów `depsdev.go`, `openrouter.go`
- `grep -n -E 'disabled|role|confirm\(|required|...'` w `web/app/{routes,lib,components}`; `sed -n` fragmentów `organization.tsx`, `catalog.tsx`, `redundancy.tsx`, `users.tsx`, `require-auth.tsx`, `routes.ts`
- `grep -rn -E 'DELETE|UPDATE |Begin\('` w `backend/**/*.go` (bez testów)
- `grep -n '^func Test'` w `strategic_test.go`, `handlers_test.go`, `project_handlers_test.go`, `queue_test.go`, `pipeline_test.go`
