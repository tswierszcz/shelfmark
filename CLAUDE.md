<!-- BEGIN @przeprogramowani/10x-cli -->

## Zestaw narzędzi AI 10xDevs — Moduł 1, Lekcja 3

Przygotuj szkielet projektu dla stosu wybranego w Lekcji 2, używając **łańcucha bootstrap**:

```
(/10x-init  →  /10x-shape  →  /10x-prd)  →  /10x-tech-stack-selector  →  /10x-bootstrapper
```

Łańcuch PRD pochodzi z Lekcji 1, a selektor stosu technologicznego z Lekcji 2 — oba zostały ponownie uwzględnione w tej lekcji, aby można było poprawić PRD lub zmienić stos w trakcie pracy. `/10x-bootstrapper` jest głównym tematem lekcji. Łańcuch kończy się tutaj w v1; przyszła Lekcja 4 skonfiguruje kontekst agenta (`CLAUDE.md`, `AGENTS.md`).

### Router zadań — od czego zacząć

| Skill | Użyj go, gdy |
| --- | --- |
| **Bootstrap (główny temat lekcji)** | |
| `/10x-bootstrapper` | Masz przekazanie w `context/foundation/tech-stack.md` (utworzone przez `/10x-tech-stack-selector`) i jesteś gotowy przygotować szkielet projektu w bieżącym katalogu. Skill odczytuje przekazanie, wyszukuje wybraną kartę w rejestrze starterów, uruchamia jej CLI za pomocą jednej z trzech strategii cwd (utworzenie szkieletu w katalogu tymczasowym, a następnie przeniesienie plików wyżej; utworzenie szkieletu bezpośrednio w bieżącym katalogu; klonowanie repozytorium startera bez zachowywania jego historii git), zawsze zachowuje `context/`, odkłada inne kolizje jako rodzeństwo `.scaffold`, wykonuje lekką kontrolę aktualności przed utworzeniem szkieletu i pogłębiony audyt po jego utworzeniu oraz zapisuje dziennik weryfikacji w `context/changes/bootstrap-verification/verification.md`. Użyj PO `/10x-tech-stack-selector`. |
| **W razie potrzeby uruchom ponownie wcześniejszy etap** | |
| `/10x-init` / `/10x-shape` / `/10x-prd` / `/10x-tech-stack-selector` | Dołączone, aby można było poprawić PRD lub zmienić stos w trakcie pracy. Jeśli `/10x-bootstrapper` zgłosi odmowę z powodu rozbieżności rejestru albo zmienisz zdanie co do startera, uruchom ponownie `/10x-tech-stack-selector`, aby ponownie wygenerować `tech-stack.md`, a następnie wywołaj ponownie. |

### Jak łańcuch przekazuje pracę

- `/10x-tech-stack-selector` (Lekcja 2) zapisuje `context/foundation/tech-stack.md` z frontmatterem zawierającym 4 klucze (`starter_id`, `package_manager`, `project_name`, `hints`) oraz jedn akapit treści `## Why this stack`.
- `/10x-bootstrapper` odczytuje ten plik W CAŁOŚCI (bez fallbacku do historii rozmowy). Jeśli go nie ma, skill odmawia, podając jednolinijkowe przekierowanie do `/10x-tech-stack-selector`, i zatrzymuje się — bez wbudowanego mini-przekazania, bez trybu samodzielnego w v1.
- Wybrane `starter_id` jest wyszukiwane w `/skills/10x-tech-stack-selector/references/starter-registry.yaml`. Skill korzysta z tego rejestru; nie jest jego właścicielem. Walidator CI (`scripts/validate-starter-registry-sync.mjs`) zapobiega odwoływaniu się przez bootstrapper do `starter_id`, którego nie ma w rejestrze.
- Skill zapisuje `context/changes/bootstrap-verification/verification.md` jako dziennik ścieżki audytowej uruchomienia. Schemat znajduje się w `/skills/10x-bootstrapper/references/verification-log-schema.md`.

### Co bootstrapper obejmuje (a czego NIE obejmuje)

- **Obejmuje (v1)**: tworzenie szkieletu przez `cmd_template` wybranej karty (delegowanie do CLI, nie generowanie plików inline), trzy strategie cwd wybierane z `bootstrapper-config.yaml` (`subdir-then-move`, `native-cwd`, `git-clone`), rygorystyczną politykę konfliktów tworzącą rodzeństwo `.scaffold` + zawsze zachowującą `context/`, dwa etapy weryfikacji (lekka kontrola aktualności przed utworzeniem szkieletu + pogłębiony audyt zależny od języka po jego utworzeniu), podsumowanie audytu według poziomów istotności, pełny dziennik weryfikacji na dysku.
- **NIE obejmuje w v1 (celowo)**: generowania `AGENTS.md` / `CLAUDE.md` (odroczone do przyszłej Lekcji 4 — „Architektura pamięci”); nakładek rozmieszczenia elementów certyfikacyjnych dla poszczególnych starterów (należą do przyszłego skilla kontekstu agenta, nie tutaj); plików workflow CI; fallbacku AI-as-bridge dla stosów spoza rejestru (odroczone do v2 — w v1 tech-stack-selector w trybie łańcuchowym już ogranicza wybór do rejestru, więc taki przypadek nie może wystąpić); trybu samodzielnego, w którym użytkownik podaje stos inline bez przekazania (odroczone do v2); działań kompensacyjnych dla `bootstrapper_confidence: best-effort` lub `quality_override: true` (widoczne w rozmowie, ale bez automatycznego działania następczego — to również zadanie przyszłego skilla architektury pamięci).

### Polityka konfliktów

Gdy skill przenosi pliki z tymczasowego katalogu szkieletu do bieżącego katalogu roboczego, stosuje rygorystyczną macierz:

- **`context/**`** — wszystko, co szkielet próbował zapisać w `context/`, jest **odrzucane**. Twoje `context/` jest źródłem prawdy dla łańcucha bootstrap (PRD, przekazanie tech-stack, plany, ramy) i nigdy nie jest nadpisywane.
- **`.gitignore`** — scalanie przez dopisanie: istniejące linie pozostają w kolejności, a następnie linie ze szkieletu są odduplikowywane względem Twojego zestawu i dopisywane z komentarzem-separatorem. Semantyka ignorowania Git jest addytywna, więc łączenie jest bezpieczne.
- **`package.json`, `README.md`, `CLAUDE.md`, `AGENTS.md`, root-level `*.md`** — wygrywa istniejący plik; kopia ze szkieletu trafia jako rodzeństwo `<filename>.scaffold`. Możesz użyć `diff README.md README.md.scaffold`, aby zobaczyć, co dostarczył starter, a co już było.
- **Wszystko inne** — jest przenoszone bez komunikatu, jeśli nie ma konfliktu, albo odkładane jako `<filename>.scaffold`, jeśli konflikt występuje. Macierz nigdy nie usuwa plików użytkownika.

Dla strategii `git-clone` (10x-astro-starter i podobne): sklonowane `.git/` jest usuwane przed przeniesieniem wyżej, dzięki czemu historia upstreamowego startera nie przedostaje się do Twojego repozytorium. Następnie inicjalizujesz własną historię (`git init`).

### Dziennik weryfikacji

Każde uruchomienie zapisuje `context/changes/bootstrap-verification/verification.md`. Sekcje:

- **`## Hand-off`** — dosłowna kopia frontmatteru tech-stack.md oraz treści `## Why this stack`.
- **`## Pre-scaffold verification`** — tabela ustaleń dotyczących aktualności (wersja pakietu npm + `time.modified` dla starterów JS; GitHub `pushed_at` dla każdego startera z GitHub `docs_url`).
- **`## Scaffold log`** — rozstrzygnięte wywołanie CLI, kod wyjścia, przeniesione pliki, konflikty przedstawione jako rodzeństwo `.scaffold`, obsługa `.gitignore`.
- **`## Post-scaffold audit`** — pełne dane wyjściowe audytu dla każdego języka (`npm audit --json` dla JS, `pip-audit` dla Python, `cargo audit` dla Rust itd.). Podział według poziomów istotności: CRITICAL i HIGH są prezentowane inline na czacie, MODERATE i LOW tylko w dzienniku. Podział na bezpośrednie i przechodnie zależności, jeśli narzędzie go obsługuje.
- **`## Hints recorded but not acted on`** — każda wskazówka z przekazania, którą bootstrapper odczytał, ale na której nie działał w v1. Kompletność ścieżki audytowej dla przyszłego skilla architektury pamięci.
- **`## Next steps`** — tekst wskazujący dalsze kroki. v1 podaje „your project is scaffolded and verified — happy hacking” i oznacza przyszły skill z Lekcji 4 jako następne ogniwo łańcucha.

Folder (`context/changes/bootstrap-verification/`) celowo nie zawiera `change.md`. Uruchomienia bootstrap są jednorazowymi artefaktami, a nie śledzonymi zmianami workflow — folder zawiera dziennik i nic więcej. Ponowne uruchomienia stosują zabezpieczenie ostrzegające i wymagające potwierdzenia przed nadpisaniem; furtką awaryjną jest `verification-v2.md` (i tak dalej).

### Ścieżki foundation używane przez tę lekcję

- `context/foundation/tech-stack.md` — wejście (z Lekcji 2)
- `context/changes/bootstrap-verification/verification.md` — wyjście (dziennik ścieżki audytowej)
- `context/foundation/lessons.md` — powtarzające się reguły i pułapki
- `docs/reference/contract-surfaces.md` — rejestr kluczowych nazw

### Uniwersalny język

Dostarczony skill nie zawiera odniesień do 10xDevs / kohort / certyfikacji. Audyt po utworzeniu szkieletu jest wybierany według `language_family` na podstawie niewielkiej tabeli wyszukiwania; kohorty, których stos trafia do `java`, `php`, `dart` lub kombinacji wielu języków, zobaczą w dzienniku linię „no built-in audit tool for this ecosystem” oraz rekomendowane narzędzie zewnętrzne, a nie fałszywy wpis „0 findings”.

Skille nie mogą zapisywać do `context/archive/`. Zarchiwizowane zmiany są niezmienne; jeśli rozstrzygnięta ścieżka docelowa zaczyna się od `context/archive/`, przerwij z komunikatem: „This change is archived. Open a new change with `/10x-new` instead.”

<!-- END @przeprogramowani/10x-cli -->
