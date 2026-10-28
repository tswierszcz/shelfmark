---
title: "Anti-corruption layer dla tożsamości pakietu (purl / packageurl-go)"
created: 2026-10-28
type: refactor-plan
head: b655dc3
source_prompt: .claude/prompts/m4l5-3-anti-corruption-layer.md
builds_on: context/domain/domain-distillation.md
---

# Plan refaktoru: anti-corruption layer dla tożsamości pakietu

Produkt to plan, nie implementacja. Kod produkcyjny nie był zmieniany. `plik:linia` sprawdzone na `b655dc3`; kod biblioteki `packageurl-go` sprawdzony w module cache (`github.com/package-url/packageurl-go@v0.1.7/packageurl.go`).

## Krok 0. Kontekst

**Deklaracje dokumentów o wymienialności i rozszerzalności:**
- „Shelfmark classifies and reports on libraries from the Java/Maven ecosystem; Java/Maven support is must-have for the MVP.” (`context/foundation/prd.md:80`)
- „Python ecosystem support is a nice-to-have for the MVP — it may slip past the MVP without harming the primary flow.” (`prd.md:81`)
- „No ecosystems other than Java/Maven (and optionally Python)” (`prd.md:107`)
- Wybór strategiczny jest „scoped per technology/ecosystem” (`prd.md:73`)
- „No SPDX support — CycloneDX only, per the SBOM format decision.” (`archive/2026-10-16-seed-classification-catalog/plan.md:29`)

Dokumenty traktują więc ekosystem jako wymiar domeny, który ma się rozszerzać (Python), a format SBOM jako świadomie zamrożony.

**Stack i zależności zewnętrzne:**
- `backend/go.mod`: `cyclonedx-go v0.12.0`, `golang-migrate/v4`, `pgx/v5`, `packageurl-go v0.1.7`, `x/crypto`.
- Usługi przez HTTP: deps.dev (`depsdev.go`), GitHub (`github.go`), OpenRouter (`openrouter.go`).
- Web: tylko `react`, `react-router`, `@react-router/node`, `isbot` (`web/package.json`). Granica klient/serwer to Go ↔ TS, więc żadne SDK nie jest współdzielone.

**Warstwy:** jeden pakiet `backend/internal/catalog`. Parsery, pipeline, adaptery zewnętrzne, store i handlery żyją w nim obok siebie; brak osobnej warstwy domenowej (`context/domain/evidence/0-sources.md`).

## Krok 1. Przeciekające zależności

| id | zależność | kto ją dziś zna (plik:linia) | sygnał przecieku |
|---|---|---|---|
| L-1 | `packageurl-go` + kształt współrzędnej Maven (group / artifact / version) | parsowanie: `catalog/purl.go:14`, `:19-20`, `catalog/pipeline.go:36-40`; współrzędne Maven w sygnaturze portu: `catalog/depsdev.go:19`; w adapterach: `depsdev.go:56-61` (`systems/maven`, `group + ":" + artifact`), `openrouter.go:163-171` („Maven library with group ID … artifact ID”); w pipeline: `pipeline.go:40-42`, `:103-105`; literał ekosystemu: `purl.go:31` (`Ecosystem: "maven"`); default DB: `0002_catalog_schema.up.sql:26` | ten sam obiekt biblioteki rekonstruowany w dwóch miejscach; kształt Maven w sygnaturze domenowego portu; ekosystem jako literał |
| L-2 | `cyclonedx-go` | `catalog/sbom.go:25-26`, `catalog/project_sbom.go:27-28` | zduplikowana rekonstrukcja: dekodowanie, `bomFormat`, pętla komponentów (`sbom.go:43-56` vs `project_sbom.go:72-93`); typy biblioteki nie wychodzą poza parsery |
| L-3 | `pgx/v5`, `pgconn`, `pgerrcode` | `catalog/store.go`, `catalog/project.go`, `catalog/strategic.go`, `auth/users_store.go`, `db/*.go`; kody błędów PG mapowane w `strategic.go` i `auth/users_store.go` | wiele plików, ale jedna warstwa (persystencja); handlery nie importują pgx |
| L-4 | OpenRouter (HTTP) | `catalog/openrouter.go` za interfejsami `Categorizer` (`pipeline.go:18`) i `RepoFinder` (`depsdev.go:18`) | już odizolowany portem; jedyny przeciek to kształt Maven w `RepoFinder` (L-1) |

## Krok 2. Klasyfikacja i wybór #1

| id | (a) warstwy / pliki | (b) koszt wymiany dziś | (c) dokumenty deklarują wymienialność? |
|---|---|---|---|
| L-1 | 4 role (parser wejścia, pipeline, port, 2 adaptery) + DB default; 5 plików Go + migracja; 2 implementacje portu w produkcji, fałszywe w `pipeline_test.go` i `queue_test.go` | wysoki: dodanie Pythona (`pkg:pypi`) albo zmiana parsera purl dotyka parsera, pipeline'u, sygnatury portu, obu adapterów i literału ekosystemu | **tak**: Python jako dopuszczony drugi ekosystem (`prd.md:81`, `:107`), wybór strategiczny per ekosystem (`prd.md:73`) |
| L-2 | 2 pliki, 1 warstwa (parser wejścia) | niski–średni: format zamrożony | **nie**: „No SPDX support” (`seed-classification-catalog/plan.md:29`) |
| L-3 | 1 warstwa persystencji | wysoki, ale nikt tego nie planuje | nie |
| L-4 | 1 plik | niski: port już istnieje | domyślnie tak (env `OPENROUTER_MODEL`, runbook „Secrets”) i już spełnione |

**Wybór: L-1, tożsamość pakietu (purl) i współrzędna Maven.**
- Jedyny przeciek, przy którym dokumenty deklarują rozszerzenie (drugi ekosystem), a kod go nie dotrzymuje: port domenowy `RepoFinder` jest zdefiniowany w kategoriach Maven.
- Dotyka tożsamości kluczowego bytu (`Library`, T-01/T-02 w destylacji), a od ekosystemu zależą niezmienniki Core (limit per ekosystem R-01, zakres „przejrzanej” K1).
- L-2 jest tańsza i dotyczy formatu świadomie zamrożonego, więc zostaje jako refaktor higieny (pozycja #3 rankingu L4), nie jako ACL.

## Krok 3. Diagnoza

**Duplikacja rekonstrukcji tego samego bytu.** Purl jest parsowany przez bibliotekę dwa razy, w dwóch rolach:
- `purl.go:14`: `packageurl.FromString(raw)` przy ingest. Wynik jest spłaszczany do `NewLibrary{Purl: raw, Name: namespace/name, Ecosystem: "maven"}` (`purl.go:23-32`), a obiekt biblioteki ginie.
- `pipeline.go:36-40`: `packageurl.FromString(lib.Purl)` ponownie, przy kategoryzacji, tylko po to, żeby odzyskać `Namespace`, `Name`, `Version`, które parser już miał.

**Przeciek przez granicę portu.** Port domenowy, który pipeline zna zamiast konkretnych usług, ma sygnaturę Maven: `FindRepo(ctx, group, artifact, version string)` (`depsdev.go:19`). W efekcie:
- `DepsDevClient` zakłada system `maven` i składa nazwę `group:artifact` (`depsdev.go:59-61`);
- `OpenRouterClient` pyta LLM o „Maven library with group ID … artifact ID” (`openrouter.go:169`);
- `findRepo` w pipeline przenosi trzy luźne stringi (`pipeline.go:103-105`).

Biblioteka z innego ekosystemu nie ma „group”, więc nie przejdzie przez ten port bez zmiany sygnatury we wszystkich implementacjach.

**Ekosystem jako literał, nie wartość pochodna.** `Ecosystem: "maven"` (`purl.go:31`) nie pochodzi z `parsed.Type`, mimo że parser go zna. DB ma `DEFAULT 'maven'` (`0002…up.sql:26`). Limit strategiczny i advisory lock używają `libraries.ecosystem` (`strategic.go:62-68`, `:83-86`), więc poprawność R-01 zależy od tego, czy ten literał zawsze odpowiada typowi purla.

**Deklaracja vs kod.** PRD dopuszcza Pythona (`prd.md:81`, `:107`) i definiuje wybór strategiczny per ekosystem (`prd.md:73`). Kod pozwala przyjąć tylko `maven` (`purl.go:19-20`) i w trzech innych miejscach zakłada Maven bez sprawdzania. Odrzucenie innego typu przy wejściu jest zgodne z MVP; zakładanie Maven za portem już nie.

**Tożsamość: surowy string zamiast kanonicznej postaci.** `Purl: raw` (`purl.go:29`) zapisuje purl tak, jak przyszedł. Biblioteka dostarcza kanoniczną postać: `PackageURL.ToString()` z kodowaniem procentowym i posortowanymi kwalifikatorami (`packageurl-go@v0.1.7/packageurl.go:439-486`, sortowanie `:297`, `:343-344`). Dwa zapisy tego samego pakietu (kolejność kwalifikatorów, kodowanie) dają dziś dwa wiersze `libraries` mimo `UNIQUE` (`0002…up.sql:24`), a więc dwie biblioteki dla limitu i raportu.

**Groźne przecieki klient/serwer:** brak. Web nie ma zależności od żadnej z tych bibliotek; UI dostaje gotowe pola `purl`, `name`, `ecosystem` (`web/app/lib/catalog.ts:6-15`).

## Krok 4. Projekt ACL

**Value object `PackageRef`** to jedyne miejsce wiedzy o kształcie purla i o tym, jak z niego wyprowadzić ekosystem i współrzędną dla usług zewnętrznych. Plik `backend/internal/catalog/packageref.go`; tylko on importuje `packageurl-go`.

```go
// PackageRef is the domain identity of a library version: an immutable,
// canonical package URL plus what the domain needs from it.
type PackageRef struct {
	canonical string    // PackageURL.ToString(): canonical form, the identity
	ecosystem Ecosystem // derived from purl type, never a literal elsewhere
	namespace string
	name      string
	version   string
}

type Ecosystem string
const EcosystemMaven Ecosystem = "maven"
// const EcosystemPyPI Ecosystem = "pypi" — added when Python ships (prd.md:81)

var supportedEcosystems = map[string]Ecosystem{packageurl.TypeMaven: EcosystemMaven}

var ErrUnsupportedEcosystem = errors.New("catalog: unsupported package ecosystem")
var ErrInvalidPackageRef    = errors.New("catalog: invalid package url")

func ParsePackageRef(raw string) (PackageRef, error) {
	p, err := packageurl.FromString(raw)
	if err != nil { return PackageRef{}, fmt.Errorf("%w: %q", ErrInvalidPackageRef, raw) }
	eco, ok := supportedEcosystems[p.Type]
	if !ok { return PackageRef{}, fmt.Errorf("%w: %q", ErrUnsupportedEcosystem, p.Type) }
	return PackageRef{canonical: p.ToString(), ecosystem: eco,
		namespace: p.Namespace, name: p.Name, version: p.Version}, nil
}

func (r PackageRef) String() string        { return r.canonical }      // persisted as libraries.purl
func (r PackageRef) Ecosystem() Ecosystem  { return r.ecosystem }      // persisted as libraries.ecosystem
func (r PackageRef) DisplayName() string   { /* namespace/name, as purl.go:23-26 today */ }
func (r PackageRef) Version() string       { return r.version }
func (r PackageRef) Coordinate() Coordinate { return Coordinate{Ecosystem: r.ecosystem, Namespace: r.namespace, Name: r.name, Version: r.version} }
```

**Wąski port.** Pipeline zna tylko port z wartością domenową, nie trzy stringi Maven:

```go
// Coordinate is what an external lookup needs, per ecosystem; adapters map it
// to their own wire shape (deps.dev system name, prompt wording).
type Coordinate struct {
	Ecosystem Ecosystem
	Namespace, Name, Version string
}

type RepoFinder interface {
	FindRepo(ctx context.Context, c Coordinate) (repo string, found bool, err error)
}
```

**Adaptery** mapują `Coordinate` na swój kształt i są jedynym miejscem, które wie, jak usługa nazywa ekosystem:
- `DepsDevClient`: `Ecosystem → system` (`maven` → `maven`, w przyszłości `pypi` → `pypi`), nazwa pakietu `namespace:name` dla Maven, sama `name` dla PyPI;
- `OpenRouterClient`: tekst promptu per ekosystem;
- nieobsługiwany ekosystem w adapterze → `found=false` (to oczekiwany wynik kaskady, jak dziś przy 404, `depsdev.go:51-55`).

**Ingest.** `ParseCycloneDXSBOM`, `ParseProjectSBOM` i `ParsePurlCSV` wołają `ParsePackageRef` zamiast `ParsePurl`. `NewLibrary` dostaje `Ref PackageRef` albo pola wyprowadzone z niego (`String()`, `DisplayName()`, `Ecosystem()`). Przyczyna odrzucenia w `SkippedRow` pozostaje tekstem dla klienta.

**Pipeline.** `ProcessLibrary` wczytuje `PackageRef` z zapisanego `lib.Purl` przez ten sam `ParsePackageRef`, bez drugiego importu biblioteki, i przekazuje `ref.Coordinate()` do `findRepo`.

## Krok 5. Dowód izolacji i before/after

**Co dotyka wymiana biblioteki purl** (np. na własny parser albo nowszą wersję o innym API) po refaktorze:
- `backend/internal/catalog/packageref.go` (nowy) i jego testy. **Nic więcej.**
- Bez zmian: tabele (`libraries.purl` i `libraries.ecosystem` pozostają tekstem), API (`purl`, `name`, `ecosystem` w JSON, `store.go:32-41`), UI (`web/app/lib/catalog.ts`, `organization.ts`, `projects.ts`), adaptery (znają `Coordinate`, nie bibliotekę).

**Co dotyka dodanie drugiego ekosystemu (Python)** po refaktorze:
- `packageref.go`: wpis w `supportedEcosystems` i stała `EcosystemPyPI`;
- adaptery: mapowanie `Coordinate` na ich kształt (`depsdev.go`, `openrouter.go`).

Pipeline, store, handlery, raport i UI się nie zmieniają, bo ekosystem płynie jako wartość. Ograniczenie, które zostaje: limit per ekosystem (R-01) już działa dla każdej wartości `libraries.ecosystem`, ale zakres „przejrzanej” (K1, Q-01) wymaga decyzji produktowej, zanim drugi ekosystem będzie widoczny.

**Before / after:**

| miejsce | before | after |
|---|---|---|
| `purl.go:13-32` | `packageurl.FromString`, sprawdzenie `TypeMaven`, spłaszczenie do `NewLibrary`, literał `"maven"` | usunięty lub cienka nakładka na `ParsePackageRef`; ekosystem z typu purla |
| `pipeline.go:36-40` | drugi `packageurl.FromString`, rozpakowanie `Namespace/Name/Version` | `ParsePackageRef(lib.Purl)` → `ref.Coordinate()` |
| `pipeline.go:103-105` | `findRepo(ctx, finders, group, artifact, version)` | `findRepo(ctx, finders, coord)` |
| `depsdev.go:19` | `FindRepo(ctx, group, artifact, version string)` | `FindRepo(ctx, c Coordinate)` |
| `depsdev.go:59-61` | stały `systems/maven`, `group + ":" + artifact` | mapowanie `c.Ecosystem` → system deps.dev i nazwa pakietu w adapterze |
| `openrouter.go:163-171` | prompt „Maven library with group ID … artifact ID” | prompt wybierany po `c.Ecosystem` w adapterze; `sanitizeForPrompt` bez zmian |
| `sbom.go:49`, `project_sbom.go:78`, `csv.go:39` | `ParsePurl(raw)` | `ParsePackageRef(raw)` |
| `0002_catalog_schema.up.sql:26` | `DEFAULT 'maven'` | bez zmian (migracje się nie cofają); kod zawsze podaje ekosystem jawnie |
| UI (`web/app/lib/*.ts`) | dostaje `purl`, `name`, `ecosystem` jako gotowe pola | bez zmian: dalej gotowe dane domenowe, nie obiekt biblioteki |

**Pytanie zależne od kontraktu biblioteki, rozstrzygnięte w ACL.** Czy identyfikatorem biblioteki jest surowy string, czy postać kanoniczna?
- Biblioteka definiuje kanoniczną serializację zgodną ze specyfikacją purl (`ToString`, „returns a canonical string representation … according to [SPEC]”, `packageurl.go:439-442`), z posortowanymi kwalifikatorami (`packageurl.go:343-344`).
- **Decyzja w ACL:** `PackageRef.String()` = postać kanoniczna; to ona trafia do `libraries.purl` dla nowych wierszy.
- Zachowanie dla istniejących wierszy: zapisane purle z SBOM-ów z wtyczki Maven są już w praktyce kanoniczne. Faza 4 zawiera zapytanie, które to sprawdza na danych, zanim cokolwiek zostanie przepisane.
- Pytanie produktowe, którego ACL nie rozstrzyga: czy kwalifikatory (np. `?type=jar`) mają odróżniać biblioteki. Dziś je odróżniają; ACL zachowuje to zachowanie i daje jedno miejsce na zmianę.

## Krok 6. Weryfikacja i plan

**Kryterium sukcesu:**

```bash
rg -l 'package-url/packageurl-go' backend --glob '!*_test.go'
```

Po refaktorze zwraca wyłącznie `backend/internal/catalog/packageref.go`. Dziś zwraca `catalog/purl.go` i `catalog/pipeline.go`.

Kryterium pomocnicze: `rg -n '"maven"|TypeMaven|group, artifact' backend --glob '!*_test.go'` trafia tylko w `packageref.go` i w mapowania adapterów (`depsdev.go`, `openrouter.go`).

| plik | zna `packageurl-go` / kształt Maven dziś | po refaktorze |
|---|---|---|
| `catalog/purl.go` | tak (`:14`, `:19-20`, `:31`) | nie (usunięty lub nakładka) |
| `catalog/pipeline.go` | tak (`:36-40`, `:103-105`) | nie (zna `PackageRef`, `Coordinate`) |
| `catalog/depsdev.go` | kształt Maven w porcie i adapterze (`:19`, `:56-61`) | tylko mapowanie `Coordinate` → deps.dev |
| `catalog/openrouter.go` | kształt Maven w prompcie (`:163-171`) | tylko mapowanie `Coordinate` → prompt |
| `catalog/sbom.go`, `project_sbom.go`, `csv.go` | pośrednio przez `ParsePurl` | pośrednio przez `ParsePackageRef` |
| `catalog/packageref.go` | — | jedyny importer `packageurl-go` |

**Plan faz** (konwencja projektu: zmiana 10x, fazy z osobnym commitem, Go testy na prawdziwym Postgresie, test-first tam, gdzie da się bez Dockera):

1. **`PackageRef` test-first (bez Dockera).** Testy tabelaryczne `ParsePackageRef`: Maven poprawny, nie-Maven → `ErrUnsupportedEcosystem`, śmieci → `ErrInvalidPackageRef`, kanonizacja (kwalifikatory w odwrotnej kolejności → ten sam `String()`), `DisplayName` = dzisiejsze `namespace/name`. Istniejące testy `purl_test.go` jako regresja zachowania.
2. **Ingest przez ACL.** Parsery (`sbom.go`, `project_sbom.go`, `csv.go`) używają `ParsePackageRef`; `NewLibrary.Ecosystem` z `ref.Ecosystem()`. Testy parserów (`sbom_test.go`, `project_sbom_test.go`, `csv_test.go`) bez zmian w asercjach.
3. **Port `RepoFinder(Coordinate)`.** Zmiana sygnatury portu, obu adapterów i fałszywych implementacji w `pipeline_test.go` i `queue_test.go`; pipeline bez importu biblioteki. Testy adapterów: dla `Coordinate{Ecosystem: maven}` zbudowany URL deps.dev i prompt są identyczne jak dziś.
4. **Kanoniczność danych.** Zapytanie na kopii danych produkcyjnych (runbook: „Test `migrate up` against a copy of the data”): ile `libraries.purl` różni się od swojej postaci kanonicznej. Jeśli zero, koniec. Jeśli więcej, osobna decyzja o scaleniu duplikatów, poza tym planem.
5. **Bramka kryterium sukcesu.** Kryteria `rg` z góry jako automatyczny check w Progress planu.

**Ryzyka:**
- Zmiana `RepoFinder` jest zmianą interfejsu, którą widzą testy kolejki i pipeline'u. Mechaniczna, ale w kilku plikach.
- Kanonizacja może zmienić zapis nowego purla względem starego zapisu tego samego pakietu, stąd faza 4 przed włączeniem.

## Podsumowanie

Najgorszym przeciekiem jest tożsamość pakietu: biblioteka `packageurl-go` i kształt współrzędnej Maven. Purl jest parsowany dwa razy (`purl.go:14`, `pipeline.go:36`), a port domenowy `RepoFinder` ma sygnaturę Maven (`group, artifact, version`, `depsdev.go:19`), którą powtarzają oba adaptery i pipeline. Ekosystem jest literałem `"maven"` (`purl.go:31`), a nie wartością wyprowadzoną z purla. To rozjazd intencji z kodem: PRD dopuszcza Pythona (`prd.md:81`, `:107`) i definiuje wybór strategiczny per ekosystem (`prd.md:73`). CycloneDX (L-2) też jest zduplikowany, ale to format świadomie zamrożony, więc nie wymaga ACL.

Projekt wprowadza value object `PackageRef` w jednym pliku, jedynym importerze `packageurl-go`, oraz wąski port `RepoFinder(Coordinate)`. Adaptery deps.dev i OpenRouter mapują `Coordinate` na swój kształt. Kontrakt biblioteki rozstrzyga też pytanie o tożsamość: ACL zapisuje kanoniczny `ToString()` z posortowanymi kwalifikatorami zamiast surowego stringa, a faza 4 sprawdza to najpierw na kopii danych.

Kryterium sukcesu: `rg -l packageurl-go` w kodzie produkcyjnym zwraca tylko `packageref.go`. Wymiana biblioteki dotyka wtedy jednego pliku, a dodanie Pythona: ACL i dwóch adapterów.
