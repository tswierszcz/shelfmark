# Evidence: language in the documents

Źródło: wyłącznie dokumenty z inwentarza `0-documents.md`; kodu nie otwierano. Wszystkie cytaty i `path:line` sprawdzone w tej sesji. Terminy zostawione w języku źródła (głównie angielski; plan refaktoru jest po polsku).

**Przeczytane w całości (requirements + reference/engineering w części reguł):**
- `context/foundation/prd.md` (cały), `context/foundation/shape-notes.md` (cały), `context/foundation/archive/shape-notes-2026-10-15-2157.md` (cały, starsza wersja z 2026-10-10), `idea-check.md` (cały, PL).
- `context/foundation/roadmap.md` (cały — narrative, ale krótki i nośny), `context/foundation/test-plan.md` (cały, pod kątem reguł), `context/foundation/lessons.md` (cały).
- `docs/runbook.md` (tylko seed accounts i smoke flow), `AGENTS.md:3` (opis produktu), `docs/samples/smoke-catalog.csv` (nagłówek).

**Próbka narrative:** tytuły wszystkich 19 folderów `context/archive/*/change.md` i 3 folderów `context/changes/` (bez wykluczonego `redundancy-flow-analysis`). W całości przeczytane plan-briefy (9): `seed-classification-catalog`, `mark-strategic-libraries`, `project-redundancy-report`, `org-data-fixes-and-user-management`, `batch-status-uncategorized-libraries`, `empty-batch-status`, `refactor-opportunities`, `testing-access-control-data-integrity`, oraz `testing-redundancy-report-correctness/change.md`. Z pełnych `plan.md` czytane tylko fragmenty znalezione grepem (statusy biblioteki, `source`, ekosystem/„przejrzana”, nazwa projektu): `seed-classification-catalog`, `mark-strategic-libraries`, `project-redundancy-report`, `org-data-fixes-and-user-management`, `batch-status-uncategorized-libraries`, `refactor-opportunities`. Tylko tytuły (10): data-layer-connectivity, minimal-auth-role-scaffold, redundancy-view-brand-tokens, app-nav-menu, home-landing-page, projects-empty-list-null-fix, views-brand-tokens-rollout, testing-pipeline-input-hardening, observability-logging-queue-supervision (brief przejrzany grepem), prod-deploy.

**Pominięte:** `tech-stack.md`, `infrastructure.md`, `health-check.md`, `test-stack.md` (decyzje techniczne, brak słów biznesowych wg inwentarza); `context/changes/bootstrap-verification/`, `impl-review-ci/` (wg inwentarza mało treści domenowej); wykluczone przez zlecenie: `context/map/`, `context/changes/redundancy-flow-analysis/`, `refactor-opportunities/research.md` i `candidates.md`.

## Terms

| term | variants | kind | definition (quoted) | source |
|---|---|---|---|---|
| Library | libraries; „biblioteka” (PL); dependency | entity | „classifies libraries by capability (what each library actually does)” | `context/foundation/prd.md:26`; `AGENTS.md:3` |
| purl | package URL | value (identyfikator biblioteki) | „Input: a purl (package URL) identifying the library.” | `context/foundation/prd.md:87`; `prd.md:35` |
| Ecosystem | technology/ecosystem; Maven/Java, npm/JS, Python | value | „scoped per technology/ecosystem (e.g. one strategic choice for Maven/Java, a separate one for npm/JS, since no single library spans both)” | `prd.md:73`; `prd.md:80-81`; `prd.md:107` |
| Capability | capabilities | thesis/value | „classifies libraries by capability (what each library actually does)” | `prd.md:26` |
| Category | capability category; „kategoria” | entity (słownikowa) | „one or more capability categories drawn from a curated taxonomy” | `prd.md:87`; `prd.md:73` |
| Taxonomy | curated taxonomy; seeded taxonomy; „taksonomia capabilities” | entity/value set | „drawn from a curated taxonomy” ; w planie: „Seeded baseline (14 Java/Maven categories) via migration” | `prd.md:87`; `context/archive/2026-10-16-seed-classification-catalog/plan-brief.md:25` |
| Classification base | classification knowledge base; knowledge base; library catalog; catalog; „baza wiedzy” | entity (agregat zbioru) | „an internal knowledge base that classifies libraries by capability”; Admin „populates and corrects the classification knowledge base (library catalog + capability categories)” | `prd.md:26`; `prd.md:97` |
| Catalog seeding | seed (the catalog), seeding the classification base | operation | „Admin enters a purl list or SBOM; the system derives purls from the SBOM.” | `prd.md:35`; `prd.md:87` |
| Categorization | categorization process; categorize | operation | „For each library, Shelfmark determines a capability classification from its GitHub repository data, with an LLM making the final categorization decision.” | `prd.md:85`; `prd.md:59` |
| Cascade | sourcing detail (cascading); „find-repo → fetch-topics → categorize cascade” | policy/operation | „1) … pulls its topic tags as a first-pass categorization; 2) an LLM is asked whether that categorization is correct … 3) where neither source yields a confident category, the admin enters one manually” | `prd.md:89`; `seed-classification-catalog/plan-brief.md:36` |
| Topic tags | GitHub topics, tags | value | „pulls its topic tags as a first-pass categorization” | `prd.md:89` |
| Category source | `source`: `github` / `llm` / `manual` | value | „`source text not null check (source in ('github','llm','manual'))`” | `context/archive/2026-10-16-seed-classification-catalog/plan.md:64` |
| Library status | `pending`, `processing`, `categorized`, `needs_manual`, `failed`; UI: „Needs review”, „Failed”, „Processing” | state | „`needs_manual`, which means "the pipeline ran successfully but found nothing confident," versus `failed`, which means "the pipeline itself errored."” | `seed-classification-catalog/plan.md:63`, `:208`; `context/archive/2026-10-27-batch-status-uncategorized-libraries/plan.md:20` |
| Batch | upload batch, batch id; „paczka” (PL) | entity (techniczno-biznesowa) | brak definicji; opisany przez zachowanie: „The batch line says `— done` only when every library is categorized, otherwise `— done: N need review, M failed`” | `batch-status-uncategorized-libraries/plan-brief.md:15`; `context/archive/2026-10-27-empty-batch-status/plan-brief.md:7` |
| SBOM | project SBOM; CycloneDX; „SBOM-y” | value (dokument wejściowy) | „An organization's own dependency data (SBOMs)”; „contains no private/customer data — only library coordinates and their roles” | `prd.md:26`; `prd.md:42`; `seed-classification-catalog/plan-brief.md:21` |
| CSV list of purls | purl list | value (dokument wejściowy) | „providing a CSV list of purls, as an alternative to uploading an SBOM” | `prd.md:61` |
| Project | „projekt” | entity | „Project identity: Free-text name, case-insensitive match-or-create on upload” | `context/archive/2026-10-19-project-redundancy-report/plan-brief.md:21`; `prd.md:36` |
| Direct / transitive | direct dependency, transitive; role (in project); `is_direct` | value/state (per projekt) | „Direct vs. transitive role is shown when the SBOM input supports it”; „direct/transitive varies per project, not globally per library” | `prd.md:43`; `project-redundancy-report/plan-brief.md:11` |
| Strategic library | strategic/standard; strategic choice; strategic pick; standard; strategic mark | state / policy | „mark which library in a category is strategic/standard”; „Normally one strategic library per category per ecosystem, but up to two are allowed.” | `prd.md:36`; `prd.md:73` |
| Strategic mark | `strategic_marks`, mark/unmark; „znacznik strategiczny” | entity (decyzja) | „New `strategic_marks` table, keyed `(library_id, category_id)`, with an `active` flag rather than row deletion” | `context/archive/2026-10-17-mark-strategic-libraries/plan-brief.md:23` |
| Justification | „uzasadnienie” | value | „Required on both mark and unmark” | `mark-strategic-libraries/plan-brief.md:22`; `prd.md:98` |
| Recommendation | standard / to retire | entity/thesis | Architect „sets and changes recommendations (standard / to retire) with justification” | `prd.md:98-99` |
| Candidate to retire | retire candidate; non-strategic; to retire; „kandydat do wycofania” | state (wyliczany) | „Direct libraries in a category/ecosystem not marked strategic are implicitly candidates to retire.” | `prd.md:73`; `prd.md:54`; `project-redundancy-report/plan-brief.md:26` |
| Reviewed category | unreviewed / never-reviewed category; „kategoria przejrzana” | state (wyliczany) | „„Przejrzana” znaczy dziś „co najmniej jeden aktywny znacznik w kategorii, w dowolnym ekosystemie”” | `context/archive/2026-10-27-refactor-opportunities/plan.md:165`; `project-redundancy-report/plan.md:95` |
| Redundancy | functional redundancy; „redundancja funkcjonalna” | measure/thesis | „flagging redundancy (two libraries in the same category within one project)” | `prd.md:36`; `prd.md:24`; `prd.md:26` |
| Redundancy report | project report, category report; organization-level report | entity (widok/odczyt) | „report of all its libraries by category, with direct dependencies highlighted … and showing whether a library is the organization's marked strategic choice” | `prd.md:69`; `prd.md:36`; `prd.md:51` |
| My organization view | organization's library list; `/organization` | operation/widok | „Architect can view the organization's library list, filterable by strategic/standard status, with direct dependencies emphasized.” | `prd.md:71`; `prd.md:36` |
| Organization | organization instance; single organization instance | entity/kontekst | „the MVP is a single organization instance” | `prd.md:110` |
| Admin | — | role | „Manages users/roles …; populates and corrects the classification knowledge base” | `prd.md:97` |
| Architect | platform engineer (persona) | role | „Uploads project SBOMs for analysis; sets and changes recommendations (standard / to retire) with justification” | `prd.md:98`; `prd.md:30` |
| Viewer | User (FR-008) | role | „Views reports and current recommendations; no edit capabilities” | `prd.md:99`; `prd.md:75` |
| User (account) | users, seed accounts | entity | „Accounts are now managed through an in-app admin page backed by a database table” | `prd.md:103`; `docs/runbook.md:35` |
| Health metrics | health metrics enrichment; „sygnały zdrowia” | measure (odłożone) | „fetches current health metrics for a library from the web, as an enrichment on top of the categorization” | `prd.md:39`; `context/foundation/roadmap.md:155` |
| Attack surface | — | thesis | „the number of libraries in use is itself the attack surface” | `prd.md:24` |
| „what is redundant” vs „what is vulnerable” | — | thesis | „Nobody answers "what is redundant" — only "what is vulnerable."” | `prd.md:24`; `prd.md:108` |
| Remediated builds | „remediated” replacement builds; Chainguard | thesis (kontekst) | „increasingly building "remediated" replacement builds” | `prd.md:22`; `context/foundation/archive/shape-notes-2026-10-15-2157.md:202` |

## Lifecycles

**Library (status)** — evidence, plany S-01 i późniejsze; PRD nie nazywa stanów.
- Stany: „`status in ('pending','processing','categorized','needs_manual','failed')`” (`seed-classification-catalog/plan.md:63`).
- `pending` → `categorized` | `needs_manual`: „libraries transitioning from `pending` → `categorized` (or `needs_manual`)” (`seed-classification-catalog/plan.md:220`). Porusza: pipeline/kolejka (system).
- → `needs_manual`: „If no repo is found by any finder, or the LLM returns no confident categories” (`seed-classification-catalog/plan.md:200`); doprecyzowane później: „A library whose LLM result attaches no taxonomy category ends `needs_manual`, not `categorized`.” (`batch-status-uncategorized-libraries/plan.md:21`).
- → `failed`: „a non-transient error … or exhausted retries sets `status: "failed"`” (`seed-classification-catalog/plan.md:208`); „A panic in categorization no longer restarts the server; the library ends `failed` with a reason.” (`context/archive/2026-10-26-observability-logging-queue-supervision/plan-brief.md:60`).
- `needs_manual` → (kategoryzowana ręcznie): „Manually attach a category to a `needs_manual` library and confirm it now shows as categorized” (`seed-classification-catalog/plan.md:308`). Porusza: Admin.
- Odzysk `pending`/`processing`: „Boot-only sweep of `pending`/`processing` older than 10 min” (`observability-logging-queue-supervision/plan-brief.md:26`).
- Brak w dokumentach: przejście `failed` → ponowna kategoryzacja (re-categorize jest „Out of scope”, `batch-status-uncategorized-libraries/plan-brief.md:30`). `unknown`.

**Batch** — evidence, częściowy.
- „settled, all `categorized` → `— done`”; „settled with `needs_manual`/`failed` → `— done: N need review, M failed`” (`batch-status-uncategorized-libraries/plan.md:119-120`).
- Pusta paczka: „Re-uploading an SBOM whose libraries are all known creates an empty batch” → „`— done: no new libraries to categorize`” (`empty-batch-status/plan-brief.md:7`, `:13`).
- Stan „settled” nie ma definicji poza zachowaniem UI; członkostwo w paczce nazwane jako otwarta decyzja „K4” (`context/archive/2026-10-27-refactor-opportunities/plan-brief.md:38`). `unknown` co do treści K4.

**Strategic mark** — evidence.
- mark → `active`; unmark → nieaktywny, wiersz zostaje: „with an `active` flag rather than row deletion” (`mark-strategic-libraries/plan-brief.md:23`). Porusza: Architect; warunek: justification „Required on both mark and unmark” (`:22`); limit 2 per (category, ecosystem) (`org-data-fixes-and-user-management/plan-brief.md:28`).
- Historia: „Full audit history of every mark/unmark event (only the latest action is kept)” — poza zakresem (`mark-strategic-libraries/plan-brief.md:41`).

**Project** — evidence.
- Tworzenie: „case-insensitive match-or-create on upload” (`project-redundancy-report/plan-brief.md:21`).
- Re-upload: „Replace the project's current library set, no history” (`project-redundancy-report/plan-brief.md:22`).
- Brak: rename/merge/usuwanie („No project rename/merge tooling”, `project-redundancy-report/plan.md:38`).

**User account** — evidence, częściowy.
- Seed: „Seed accounts are created only when the `users` table is empty (first boot).” (`docs/runbook.md:35`). Tworzy Admin w `/users` (`org-data-fixes-and-user-management/plan-brief.md:15`). Edycja/usuwanie/reset hasła: „Out of scope” (`:39`).

## Rules

| # | rule (quoted) | binds | entity / policy | source |
|---|---|---|---|---|
| R1 | „Normally one strategic library per category per ecosystem, but up to two are allowed.” | Strategic library, Category, Ecosystem | entity (strategic mark set per category+ecosystem) | `prd.md:73` |
| R1a | „No category/ecosystem can ever hold more than 2 active strategic marks.” ; 3rd → 409 „also when marks arrive concurrently” | Strategic mark | entity | `org-data-fixes-and-user-management/plan-brief.md:69`; `context/foundation/test-plan.md:64` |
| R2 | „Architect can mark a direct library within a category as strategic” | Strategic mark, Direct | entity | `prd.md:73` |
| R3 | „Direct libraries in a category/ecosystem not marked strategic are implicitly candidates to retire.” | Candidate to retire, Direct, Strategic | policy | `prd.md:73`; `prd.md:54` |
| R3a | „Only flag a direct library when a *different* library is strategic in that category” (unreviewed category nie jest flagowana) | Candidate to retire, Reviewed category | policy (zawęża R3) | `project-redundancy-report/plan-brief.md:26`; `project-redundancy-report/plan.md:95` |
| R3b | „transitive → never flagged” | Transitive, Candidate to retire | policy | `project-redundancy-report/plan.md:193` |
| R4 | „the category simply has no strategic library yet — the system does not force a choice.” | Category, Strategic | policy | `prd.md:74` |
| R5 | „Justification … Required on both mark and unmark” | Strategic mark, Justification | entity | `mark-strategic-libraries/plan-brief.md:22` |
| R6 | „SBOM data uploaded to the system contains no private/customer data — only library coordinates and their roles.” | SBOM | policy | `prd.md:42` |
| R7 | „Direct vs. transitive role is shown when the SBOM input supports it” ; fallback: „fall back to all-direct if absent” | Direct/transitive, SBOM | policy | `prd.md:43`; `project-redundancy-report/plan-brief.md:28` |
| R8 | „Direct/transitive is inherently per-project; a global column would let one project's upload corrupt another's report” | Direct/transitive, Project | entity (Project membership) | `project-redundancy-report/plan-brief.md:23` |
| R9 | „Project SBOM upload is Architect-only; project lists and redundancy reports are available to Architect and Viewer” | Architect, Viewer, Project, Report | policy (access) | `prd.md:101` |
| R10 | Viewer: „no edit capabilities” | Viewer | policy | `prd.md:99` |
| R11 | Catalog seeding/kategoryzacja tylko Admin: „All of this is Admin-only, consistent with PRD Access Control.” | Admin, Catalog | policy | `seed-classification-catalog/plan-brief.md:69`; `prd.md:59-65` |
| R12 | „A categorized library with no category is a status lie” → zero kategorii = `needs_manual` | Library status, Category | entity (Library) | `batch-status-uncategorized-libraries/plan-brief.md:24` |
| R13 | „Every uploaded library appears in the project report.” | Project, Report, Library | policy | `batch-status-uncategorized-libraries/plan-brief.md:53` |
| R14 | „An upload the categorizer can't place no longer reads as a clean "done".” | Batch | policy | `batch-status-uncategorized-libraries/plan-brief.md:52` |
| R15 | Re-upload nie nadpisuje kategoryzacji: enqueue „only the ones with `Status == "pending"` … enqueueing an already-categorized or manually-corrected library would silently overwrite its `library_categories.source`” | Library, Category source | entity (Library) | `project-redundancy-report/plan.md:164` |
| R16 | Re-upload istniejącego purl: „`ON CONFLICT (purl) DO NOTHING` so re-uploading an existing purl is a safe no-op” (purl unikalny) | purl, Library | entity | `seed-classification-catalog/plan.md:75` |
| R17 | „Re-uploading a project by name updates its existing report rather than creating a duplicate.” | Project | entity | `project-redundancy-report/plan-brief.md:74` |
| R18 | „Shelfmark recommends strategic libraries; it does not block.” | Recommendation | policy (non-goal) | `prd.md:109` |
| R19 | „Seed accounts are created only when the `users` table is empty (first boot).” | User | policy | `docs/runbook.md:35` |
| R20 | Hasło nowego użytkownika: „Minimum 8 characters” | User | entity | `org-data-fixes-and-user-management/plan-brief.md:26` |
| R21 | Lista nigdy `null`: „Every Go handler that returns a list must guarantee a non-nil slice before marshaling” | (techniczna, nie domenowa) | policy (kontrakt API) | `context/foundation/lessons.md:9` |
| R22 | „The cap lock is transaction-scoped and keyed by the pair the rule is defined on.” | Strategic mark | entity (mechanizm R1a) | `testing-access-control-data-integrity/plan-brief.md:38` |

## Multiple meanings

- **„Redundancy”** — (a) „two libraries in the same category within one project” (`prd.md:36`); (b) „which of its direct dependencies are redundant — i.e. not the organization's strategic pick for that category” (`project-redundancy-report/plan-brief.md:7`). (a) to miara bez strategic marks, (b) to decyzja zależna od marks. W starszych notatkach (c) „more than one direct runtime dependency” z wyłączeniem test-scope (`archive/shape-notes-2026-10-15-2157.md:159`). `evidence`; granica między „pomiarem redundancji” a „rekomendacją retire”.
- **`is_direct` / „Direct”** — globalne na bibliotece (`/organization`) vs per projekt: „share a field name but mean different things on two pages” (`project-redundancy-report/plan-brief.md:69`); później globalne przeliczane z projektów: „Compute `is_direct` live via `EXISTS` over `project_libraries`” (`org-data-fixes-and-user-management/plan-brief.md:27`). `inference`: na `/organization` „Direct” = „bezpośrednia w co najmniej jednym projekcie”, w raporcie = „bezpośrednia w tym projekcie”.
- **„add their own categories” (FR-004)** — PRD: „Admin can add their own categories to a library” (`prd.md:65`); plan S-01: „means attaching an existing taxonomy category to a library, not creating new taxonomy entries” (`seed-classification-catalog/plan.md:34`). PRD dopuszcza też LLM: „the LLM adds its own categories” (`prd.md:35`) — nie wiadomo, czy spoza taksonomii; później: nazwy spoza taksonomii nie są przypinane (`batch-status-uncategorized-libraries/plan.md:13`).
- **„Strategic” vs „standard”** — używane zamiennie: „strategic/standard” (`prd.md:36`, `prd.md:71`); „Recommendation (standard / to retire)” (`prd.md:98`). Synonimy, ale „recommendation” jest szersze (obejmuje „to retire” jako jawną decyzję), a w kodzie-planach istnieje tylko mark strategic; „to retire” jest wyliczane (`prd.md:73`). `inference`.
- **„Catalog”** — „library catalog” jako baza wiedzy (`prd.md:97`) i jako strona Admina `/catalog` (`seed-classification-catalog/plan-brief.md:15`); „organization's library list” (`prd.md:71`) to ta sama tabela bibliotek widziana przez Architekta (`mark-strategic-libraries/plan-brief.md:15`). W single-organization instance katalog wspólny i „organizacja” się pokrywają. `inference`.
- **„Reviewed category” scope** — limit liczony per (category, ecosystem), „przejrzana” globalnie per category: „Limit liczy się per (kategoria, ekosystem) … „Przejrzana” liczy się globalnie per kategoria … Tego rozjazdu nie rozstrzygamy” (`refactor-opportunities/plan.md:14`). Otwarta decyzja „K1”.

## Conflicting statements

| topic | older | newer (obowiązuje) | note |
|---|---|---|---|
| Limit strategic marks | PRD: „up to two are allowed” (`prd.md:73`, 2026-10-15) → S-02: „**Unbounded** — no limit enforced … deviates from PRD FR-007” (`mark-strategic-libraries/plan-brief.md:25`, 2026-10-17) | „Reject the 3rd distinct library server-side (409)” (`org-data-fixes-and-user-management/plan-brief.md:28`, 2026-10-20) + race fixed (`testing-access-control-data-integrity/plan-brief.md:23`, 2026-10-26) | reguła odwrócona i przywrócona; najnowsza = PRD |
| Kto wgrywa project SBOM | „Admin … uploads project SBOMs” (`shape-notes.md:53`); „admin and architect can import SBOMs” (`archive/shape-notes-2026-10-15-2157.md:19`) | „Project SBOM upload is Architect-only” (`prd.md:101`); decyzja „Keep code … correct the PRD” (`testing-access-control-data-integrity/plan-brief.md:22`, 2026-10-26) | PRD korygowany do kodu |
| Provisioning kont | „accounts/roles provisioned via instance configuration” (`prd.md:93`), Admin „Manages users/roles (via configuration)” (`prd.md:97`) | „Accounts are now managed through an in-app admin page backed by a database table” (`prd.md:103`) | sprzeczność wewnątrz tego samego PRD; nowsze = `prd.md:103` (zmiana 2026-10-20) |
| Historia SBOM projektu | „creates a new version … previous versions are kept” (`archive/shape-notes-2026-10-15-2157.md:131`, 2026-10-10) | „Replace the project's current library set, no history” (`project-redundancy-report/plan-brief.md:22`) | wersjonowanie porzucone |
| Kto wskazuje standard | „System can suggest the standard library … the architect accepts or overrides” (`archive/shape-notes-2026-10-15-2157.md:145`) | Architect sam oznacza (`prd.md:73`); brak sugestii w PRD | FR-013 zniknął z PRD v1 |
| Zarządzanie taksonomią | „Admin can manage the capability taxonomy (add, change, remove categories)” (`archive/shape-notes-2026-10-15-2157.md:125`) | „No taxonomy management UI … seeded via migration” (`seed-classification-catalog/plan.md:34`) | |
| Organization-level report | PRD: „An organization-level report shows redundancy across all uploaded projects” (`prd.md:36`) | S-03: „Cross-project org report — Out of scope … a future roadmap slice” (`project-redundancy-report/plan-brief.md:25`); roadmap mimo to „M-1 … done” (`roadmap.md:23`) | brak slice'a w roadmapie realizującego ten raport (`roadmap.md:40-46`) |
| Ekosystem w statusie strategic | PRD: strategic „scoped per technology/ecosystem” (`prd.md:73`) | S-03: „since the catalog is Maven-only … no additional ecosystem scoping is needed when checking strategic status” (`project-redundancy-report/plan.md:21`); utrwalone jako K1 (`refactor-opportunities/plan.md:14`) | nierozstrzygnięte (K1) |
| Redundancja w starszej wizji | test-scope i transitive nie liczą się: „transitive and test-scope libraries are shown but do not trigger redundancy” (`archive/shape-notes-2026-10-15-2157.md:159`) | PRD nie wspomina scope runtime/test (`prd.md:36`, `prd.md:73`) | scope zniknął z języka |

## Unknowns

- **K1** — czy „reviewed category”/strategic choice ma być per ekosystem; dokumenty nazywają to decyzją produktową „dla L5” (`refactor-opportunities/plan-brief.md:26`, `:38`). Treść decyzji: `unknown`.
- **K4** — „członkostwo w paczce” (`refactor-opportunities/plan-brief.md:38`); definicji nie znaleziono w przeczytanych dokumentach (szukano grepem „K4|paczk” w `refactor-opportunities/plan.md`; research.md wykluczony).
- **Organization-level report** — wymagany w PRD (`prd.md:36`), poza zakresem S-03; brak w przeczytanych dokumentach informacji, czy istnieje.
- **Open Question PRD** — czy format SBOM niesie direct/transitive (`prd.md:114`); plan S-01 odpowiada „CycloneDX … its dependency graph carries the direct/transitive distinction” (`seed-classification-catalog/plan-brief.md:21`), ale PRD nie zamknął pytania.
- **Recommendation „to retire” jako jawna decyzja** — PRD Access Control mówi o ustawianiu „standard / to retire” (`prd.md:98`), FR-007 o „implicitly candidates” (`prd.md:73`); w przeczytanych planach brak operacji jawnego „mark to retire”.
- **Health metrics** — odłożone (`roadmap.md:155`), FR-005 „optionally showing health metrics” (`prd.md:69`); brak definicji metryk.
- Brak glosariusza i ADR w repo (potwierdza `0-documents.md`); słownictwo żyje w planach archiwalnych.

## Commands run

- `cat` obu plików inwentarza; `cat -n` na `prd.md`, `shape-notes.md`, `roadmap.md`, `archive/shape-notes-2026-10-15-2157.md`, `idea-check.md`, `test-plan.md`, `lessons.md`, 9 plan-briefów / change.md.
- `ls` folderów `context/archive/*/`, `context/changes/*/`; `wc -l` i `head` tytułów `change.md`.
- `grep -n` po planach: statusy biblioteki, `K1|reviewed|przejrzan|ecosystem`, `K4|paczk`, `source|taxonomy`, `case-insensitive|name`; `grep` w `docs/runbook.md` i `AGENTS.md`; `head -3 docs/samples/smoke-catalog.csv`.
