---
name: 10x-impl-review-ci
description: >
  Run implementation review non-interactively in CI against a PR: discovers
  the plan, checks drift/safety/patterns/test coverage, writes
  context/changes/<change-id>/reviews/impl-review-ci.md, commits it to the PR
  branch, and posts a summary comment. Use whenever the request mentions CI,
  GitHub Actions, GHA, Claude Code Action, automated PR review, or "review
  this PR in CI".
argument-hint: (none — discovers plan and PR automatically from the CI environment)
allowed-tools:
  - Read
  - Write
  - Glob
  - Grep
  - Bash
  - Agent
  - TaskCreate
  - TaskUpdate
  - TaskList
  - TaskGet
---

# Przegląd implementacji (CI/CD)

> **Shelfmark patch.** Ta kopia jest zmodyfikowana dla Shelfmark: raport trafia do `reviews/impl-review-ci.md` (nie `impl-review.md`, aby nigdy nie nadpisać lokalnego, przetriażowanego przeglądu), a nagłówek raportu zawiera pole `Reviewed phases`; commit raportu nie ma `[skip ci]` (GitHub pomija wtedy wszystkie zdarzenia `pull_request` na tym HEAD, także zmiany etykiet, więc `impl-review-override` nie mógłby przeliczyć werdyktu). Po aktualizacji umiejętności zastosuj ten patch ponownie.

Uruchamiaj w GitHub Actions (zwykle za pośrednictwem Claude Code Action) dla pull requesta. Analizuj zmiany w PR względem planu, który według deklaracji implementuje. Twórz ślad audytowy jako zatwierdzony plik raportu oraz zwięzły komentarz do PR.

**Ta umiejętność nie prowadzi interakcji i nie edytuje kodu.** Bez pytań, bez pętli triage, bez edycji źródeł. Czyta, analizuje, zapisuje jeden plik raportu, zatwierdza go, publikuje komentarze przeglądu inline przy oznaczonych wierszach oraz publikuje podsumowujący komentarz do PR. To całe zadanie.

**Kryteria przeglądu — jak porównywać implementację z planem, jak oceniać każdy wymiar i jak formułować ustalenia — znajdują się w `references/impl-review-instructions.md`. Przeczytaj ten plik raz po załadowaniu tej umiejętności; Kroki 1–5 orkiestrują go (zbierają dowody, uruchamiają kontrole, normalizują ustalenia) i odwołują się do niego zamiast powtarzać kryteria.** Poniższe kroki obejmują tylko mechanikę harnessu: odkrywanie planu, diff, delegowanie do subagentów, kontrakt pliku raportu, commit oraz publikowanie komentarzy inline/podsumowujących.

## Kontekst operacyjny

Załóż, że działasz nieinteraktywnie na efemerycznym runnerze Linux z:

- **Historią Git** — `origin/<base-ref>` jest pobrany; merge-base PR jest możliwy do ustalenia.
- **CLI `gh`** — uwierzytelnionym przez `GH_TOKEN`/`GITHUB_TOKEN`; może odczytywać metadane PR i publikować komentarze.
- **Łańcuchem narzędzi projektu** — zainstalowanym przed uruchomieniem tej umiejętności (Node/pnpm, Python, Go, cokolwiek wykorzystuje projekt). Polecenia testów i lintowania powinny działać.
- **Zmiennymi środowiskowymi** — `PR_NUMBER`, `GITHUB_BASE_REF` (gałąź bazowa) oraz wszystkim, co eksportuje workflow.
- **Subagentami** — możesz uruchamiać agentów `general-purpose` do równoległego zbierania dowodów.

Żaden użytkownik nie obserwuje procesu. Nigdy nie wywołuj `AskUserQuestion` — nie ma nikogo, kto mógłby odpowiedzieć. Jeśli coś jest niejednoznaczne, wybierz konserwatywną interpretację i odnotuj ją w raporcie.

## Krok 0: Znajdź plan

TaskCreate: "Impl-Review (CI)" / activeForm "Odkrywanie planu"

Plan jest źródłem prawdy — przegląd porównuje PR z deklarowaną intencją planu. Bez planu nie ma sensownego przeglądu.

**Główne źródło: konwencja.** Projekty korzystające z workflow opartego na planach przechowują pliki planów w `context/changes/<change-id>/plan.md` w gałęzi PR — w katalogu głównym repozytorium lub, w monorepo, pod poddrzewem workspace (np. `projects/<app>/context/changes/<change-id>/plan.md`). Znajdź najnowszy plik będący częścią tego PR:

```bash
BASE="${GITHUB_BASE_REF:-master}"
# The leading `:(glob)**/` matches context/changes/ at the repo root AND
# under any workspace subtree, so this works in single-package repos and
# monorepos alike.
PLAN=$(git diff --name-only "origin/${BASE}...HEAD" -- ':(glob)**/context/changes/**/plan.md' \
  | sort -r \
  | head -1)
```

Sortowanie odwrotnie leksykograficzne wybiera ostatnio zmodyfikowany folder zmiany (`<change-id>` zazwyczaj ma prefiks daty dla aktywnej pracy, a porządek alfabetyczny jest wystarczający jako rozstrzygnięcie remisu; w monorepo faworyzuje również workspace, którego ścieżka sortuje się jako ostatnia, co jest akceptowalnym rozstrzygnięciem remisu).

**Nadpisanie: jawne odwołanie w treści PR.** Jeśli opis PR zawiera wiersz taki jak `Plan: context/changes/<change-id>/plan.md`, ma on pierwszeństwo przed konwencją. Obsługuje to połączone PR, wskrzeszone starsze plany oraz PR reorganizujące pliki planów, ale implementujące inny plan:

```bash
PR_BODY=$(gh pr view "$PR_NUMBER" --json body -q .body 2>/dev/null || echo "")
OVERRIDE=$(printf '%s' "$PR_BODY" \
  | grep -oE 'Plan:[[:space:]]*[^[:space:]`]*context/changes/[^[:space:]`]+/plan\.md' \
  | head -1 \
  | sed -E 's/Plan:[[:space:]]*//')
[ -n "$OVERRIDE" ] && PLAN="$OVERRIDE"
```

**Łagodne zakończenie, gdy nie znaleziono planu.** Większość PR w dojrzałym repozytorium nie jest oparta na planach (dokumentacja, commity chore, hotfixy). Nie powoduj niepowodzenia workflow — opublikuj neutralny komentarz i zakończ kodem 0:

```bash
if [ -z "$PLAN" ] || [ ! -f "$PLAN" ]; then
  gh pr comment "$PR_NUMBER" --body "🔍 **impl-review (CI)** — no plan detected in this PR. Skipping review.

To enable a review, include a \`Plan: context/changes/<change-id>/plan.md\` line in the PR description, or ensure the branch touches a plan file under \`context/changes/<change-id>/\`."
  exit 0
fi
```

Zapisz ustaloną ścieżkę planu — każdy kolejny krok się do niej odwołuje.

## Krok 1: Załaduj i przeanalizuj plan

TaskUpdate: activeForm "Ładowanie planu"

Przeczytaj cały plik planu (bez offsetu, bez limitu) i wyodrębnij jego zobowiązania — planowane ścieżki plików, Automated/Manual Verification, wyłączenia, decyzje architektoniczne — zgodnie z **"Read the plan as the baseline"** w `references/impl-review-instructions.md` (anatomia planu i pięć celów ekstrakcji). PR zawsze jest przeglądany jako całość — częściowe przeglądy CI faz są podatne na błędy bez plików stanu i rzadko przydatne w praktyce.

### Oblicz diff

```bash
BASE="${GITHUB_BASE_REF:-master}"
git fetch origin "$BASE" --depth=50 2>/dev/null || true
CHANGED_FILES=$(git diff --name-only "origin/${BASE}...HEAD")
```

Zakres z trzema kropkami daje diff merge-base — to, co ten PR rzeczywiście dodaje, a nie wszystko, co wydarzyło się od rozdzielenia gałęzi.

Porównaj zmienione pliki z planowanymi plikami:

- **W planie ORAZ w diffie** → oczekiwana zmiana; zweryfikuj, czy intencja się zgadza.
- **W diffie, ale NIE w planie** → nieplanowany dodatek; sprawdź względem listy wyłączeń, a następnie oznacz jako scope creep, jeśli nie jest jawnie wyłączony.
- **W planie, ale NIE w diffie** → możliwie brakująca implementacja; oznacz.

Nie wczytuj wstępnie każdego zmienionego pliku źródłowego do głównego kontekstu. Deleguj to subagentom poniżej — utrzymuj główny kontekst zawierający tylko tekst planu i podsumowanie diffu.

## Krok 2: Równoległe zbieranie dowodów

TaskUpdate: activeForm "Zbieranie dowodów"

Uruchom równolegle trzech subagentów, każdego z ukierunkowanym kontekstem. Nie przekazuj całego planu wszystkim — każdy agent potrzebuje tylko tego, co jest istotne dla jego pytania. Kryteria oceny stosowane przez każdego agenta znajdują się w `references/impl-review-instructions.md` pod **"The review dimensions"** — przekaż każdemu agentowi jego dane wejściowe i wskaż jego wymiar; powinien raportować ustalenia w formacie dla danego wymiaru określonym przez referencję.

### Agent 1 — Wykrywanie rozbieżności z planem

`subagent_type: "general-purpose"`

Przekaż mu: wyodrębniony tekst "Changes Required" (dla każdej fazy) oraz listę planowanych ścieżek plików. Stosuje **wymiar 1 (Plan drift)** — MATCH / DRIFT / MISSING / EXTRA dla każdej planowanej zmiany.

### Agent 2 — Bezpieczeństwo, jakość i zgodność ze wzorcami

`subagent_type: "general-purpose"`

Przekaż mu: listę zmienionych plików źródłowych (z wyłączeniem plików testowych i samego pliku planu) oraz katalog główny projektu. Stosuje **wymiar 2** — bezpieczeństwo / wydajność / niezawodność / bezpieczeństwo danych oraz porównanie ze wzorcami sąsiednimi skalowane do rozmiaru zmiany.

### Agent 3 — Pokrycie testami

`subagent_type: "general-purpose"`

Przekaż mu: sekcję Success Criteria planu (wyodrębnij tekst przed uruchomieniem), listę plików diffu podzieloną na `source_files` i `test_files`, katalog główny projektu oraz listę "What We're NOT Doing". Stosuje **wymiar 3** — wyodrębnia zobowiązania testowe, dopasowuje je do artefaktów, skanuje pod kątem niepokrytego zachowania, uruchamia polecenia testowe z planu i respektuje jawne rezygnacje.

Jeśli raport subagenta staje się zbyt obszerny, poproś go o zwrócenie tylko ustaleń o najwyższej wadze, a nie pełnego śladu dochodzenia.

## Krok 3: Zweryfikuj nietestowe automatyczne kontrole

TaskUpdate: activeForm "Weryfikowanie automatycznych kontroli"

Agent 3 uruchomił już polecenia Automated Verification związane z testami. Teraz uruchom wszystko inne — lint, build, kontrolę formatu, typecheck i wszelkie inne polecenia nietestowe z checkboxów. Dla każdego:

```bash
echo "→ running: $cmd"
$cmd
echo "exit: $?"
```

Zapisz polecenie, status powodzenia/niepowodzenia, skrócone wyjście (zwykle wystarczy pierwszych 40 i ostatnich 20 wierszy). Aby ustalić, jak interpretowane są checkboxy Manual Verification i jak nieudane kontrole mapują się na oceny, postępuj zgodnie z **"Verify the success criteria"** w referencji.

## Krok 4: Oceń każdy wymiar

TaskUpdate: activeForm "Ocenianie"

Przypisz PASS / WARNING / FAIL do każdego z siedmiu wymiarów i wyprowadź ogólny werdykt (APPROVED / NEEDS ATTENTION / REJECTED), korzystając z reguł w **"Grade each dimension"** i **"Overall verdict"** referencji.

## Krok 5: Skompiluj ustalenia

TaskUpdate: activeForm "Kompilowanie ustaleń"

Znormalizuj wyniki subagentów do jednej listy ustaleń. Posortuj według wagi (CRITICAL → WARNING → OBSERVATION). Ogranicz do łącznie 10 — konsoliduj powiązane problemy (np. "6 files use the wrong case convention" → jedno ustalenie, nie sześć). Zastosuj format ustalenia, gramatykę wpływu i gramatykę opcji naprawy z **"Express the findings"** referencji.

**Jeden specyficzny dodatek harnessu, którego nie zawiera referencja:** każde ustalenie w zapisanym raporcie otrzymuje również pole `- **Decision**: PENDING` (wypełnia je narzędzie downstream do triage). Jest ono częścią kontraktu wyjściowego w Kroku 6 — nie pomijaj go.

## Krok 6: Zapisz raport

TaskUpdate: activeForm "Zapisywanie raportu"

Wyprowadź katalog zmiany z ustalonej ścieżki planu — jest to folder nadrzędny planu (`$(dirname "$PLAN")`), który wskazuje `context/changes/<change-id>` w katalogu głównym repozytorium albo `projects/<app>/context/changes/<change-id>` w monorepo. Zapisz raport do `<change-dir>/reviews/impl-review-ci.md`. Zaktualizuj także `<change-dir>/change.md` in-place: ustaw `status: impl_reviewed` oraz `updated: <today>` — commit CI w Kroku 7 przenosi to z powrotem do gałęzi PR obok przeglądu.

**Katalog jest wyprowadzany z planu, a nie wybierany: raport trafia do folderu `reviews/` obok `change.md` planu (`<…>/context/changes/<change-id>/reviews/impl-review-ci.md`).** NIE zapisuj raportu w `.claude-pr/`, `.github/`, katalogu głównym repozytorium ani w żadnym innym miejscu — nawet jeśli domyślny prompt Claude Code Action sugeruje inne. Narzędzie downstream do triage odczytuje dane wyłącznie z tej wyprowadzonej ścieżki. Formatowanie musi odpowiadać otaczającym przeglądom: zgodne z oxfmt (projekt uruchamia `oxfmt --check .` w CI).

### Kontrakt wyjściowy (kluczowy)

Plik **musi** zaczynać się od znacznika komentarza HTML `<!-- IMPL-REVIEW-REPORT -->` w pierwszym wierszu oraz **musi** zawierać `- **Decision**: PENDING` dla każdego ustalenia. Narzędzia downstream odczytują ten format, aby przekierować raport do workflow triage. Nie pomijaj żadnego z nich.

### Szablon

```markdown
<!-- IMPL-REVIEW-REPORT -->
# Implementation Review: [Plan Title]

- **Plan**: `context/changes/<change-id>/plan.md`
- **Scope**: Full plan (CI review on PR #<N>)
- **Reviewed phases**: <comma-separated phase numbers>
- **Date**: YYYY-MM-DD
- **CI run**: <GitHub Actions workflow run URL>
- **Verdict**: [APPROVED | NEEDS ATTENTION | REJECTED]
- **Findings**: [N critical] [N warnings] [N observations]

## Verdicts

| Dimension | Verdict |
|-----------|---------|
| Plan Adherence | PASS / WARNING / FAIL |
| Scope Discipline | PASS / WARNING / FAIL |
| Safety & Quality | PASS / WARNING / FAIL |
| Architecture | PASS / WARNING / FAIL |
| Pattern Consistency | PASS / WARNING / FAIL |
| Test Coverage | PASS / WARNING / FAIL |
| Success Criteria | PASS / WARNING / FAIL |

## Findings

### F1 — [Short title]

- **Severity**: ❌ CRITICAL
- **Impact**: 🔎 MEDIUM — real tradeoff; pause to reason through it
- **Dimension**: Safety & Quality
- **Location**: src/auth/handler.ts:42
- **Detail**: SQL query built with string concatenation. Plan specified parameterized queries in Phase 2; actual implementation uses template literals.
- **Fix**: Replace the template literal with a parameterized query using db.query($1, [value]).
  - Strength: Matches the pattern in src/users/query.ts; removes injection class entirely.
  - Tradeoff: Minor — one call site, a few-line change.
  - Confidence: HIGH — identical pattern exists elsewhere in this repo.
  - Blind spot: None significant.
- **Decision**: PENDING

### F2 — [Next finding]
…

<!-- End of report -->
```

`Reviewed phases` wymienia tylko fazy, których wiersze w `## Progress` planu niosą SHA commita obecnego w `git log origin/<base-ref>..HEAD`, albo których zmiany diff PR widocznie implementuje; gdy takich nie ma, wpisz `none`. Nigdy nie wpisuj domyślnie wszystkich faz — `/10x-archive` liczy to pole jako pokrycie przeglądem.

Sekcje z zerową liczbą ustaleń mogą zostać pominięte na liście Findings, ale tabela Verdicts zawsze pokazuje wszystkie siedem wymiarów.

### Ikony wagi/wpływu (w parze ze słowami)

- Waga: ❌ CRITICAL · ⚠️ WARNING · 👁 OBSERVATION
- Wpływ: 🏃 LOW · 🔎 MEDIUM · 🔬 HIGH

Nigdy nie używaj samej ikony bez etykiety — zmusza to czytelnika do zapamiętywania znaczenia każdego symbolu.

## Krok 7: Zatwierdź raport

TaskUpdate: activeForm "Zatwierdzanie przeglądu"

```bash
# Derive every path from the plan's own directory so this works at the
# repo root AND in a monorepo where context/changes/ lives under a
# workspace subtree (e.g. projects/<app>/context/changes/<id>/plan.md).
CHANGE_DIR=$(dirname "$PLAN")            # …/context/changes/<change-id>
CHANGE_ID=$(basename "$CHANGE_DIR")      # <change-id>
REVIEW_PATH="${CHANGE_DIR}/reviews/impl-review-ci.md"
CHANGE_MD="${CHANGE_DIR}/change.md"
mkdir -p "${CHANGE_DIR}/reviews"

git config user.name "claude[bot]"
git config user.email "41898282+claude[bot]@users.noreply.github.com"

git add "$REVIEW_PATH" "$CHANGE_MD"
git commit -m "chore(review): impl-review for ${CHANGE_ID}

CI-generated implementation review.
Pull this branch and triage locally."

# Shelfmark: no [skip ci]. The push uses the workflow's GITHUB_TOKEN,
# and GitHub never starts workflows for GITHUB_TOKEN pushes, so the bot
# commit cannot retrigger this workflow. The workflow's guard also skips
# the Claude step whenever HEAD is authored by claude[bot].

# Retry once on concurrent-push race (common on active PRs)
git push || { git pull --rebase && git push; } || PUSH_FAILED=1

if [ "$PUSH_FAILED" = "1" ]; then
  # Don't lose the work — inline the report in a PR comment as a fallback.
  gh pr comment "$PR_NUMBER" --body "$(printf '⚠️ impl-review generated but push failed (branch moved). Report content below:\n\n<details><summary>Click to expand</summary>\n\n\`\`\`markdown\n%s\n\`\`\`\n\n</details>' "$(cat "$REVIEW_PATH")")"
  exit 0
fi
```

**Nie dodawaj `[skip ci]` (Shelfmark).** Pętli zapobiega to, że push idzie tokenem `GITHUB_TOKEN` (GitHub nie uruchamia po nim workflow), oraz guard workflow, który pomija krok Claude, gdy HEAD jest od `claude[bot]`. `[skip ci]` na HEAD blokowałby zdarzenia etykiet (`impl-review-override`).

**Idempotencja: nie stosuj amend, zawsze twórz nowy commit.** Jeśli ten workflow zostanie uruchomiony ponownie (np. przez ponowne uruchomienie z UI GHA lub późniejszy push go wywoła), plik przeglądu pod tą samą ścieżką zostanie nadpisany, a nowy commit zapisze nowy przegląd. Poprzednie przeglądy pozostają w historii git — to ślad audytowy. Amend by go usunął.

## Krok 8: Opublikuj komentarze przeglądu inline

TaskUpdate: activeForm "Publikowanie przeglądu inline"

Ustalenia, których `Location` jest konkretnym `file:line` **i** których wiersz znajduje się w diffie PR, stają się komentarzami przeglądu inline — zakotwiczonymi przy dokładnym wierszu w zakładce "Files changed". Ustalenia bez kotwicy wiersza (lub których wiersz znajduje się poza diffem) są odkładane do podsumowującego komentarza w kolejnym kroku.

**Użyj narzędzia MCP, nie `gh api`.** claude-code-action v1 udostępnia `mcp__github_inline_comment__create_inline_comment` przez wbudowany serwer MCP. Opakowuje ono endpoint dla pojedynczego komentarza (`POST /pulls/:n/comments`), który zgłasza **głośno** błędy nieprawidłowych pozycji wierszy — w przeciwieństwie do wsadowego endpointu przeglądu, który po cichu odrzuca nieprawidłowe wpisy i pozostawia pustą powłokę przeglądu. Publikuj każde ustalenie jako osobne wywołanie narzędzia, z `confirmed: true`, aby klasyfikator Haiku akcji nie buforował, a następnie filtrował (jest to właściwe dla deterministycznego recenzenta CI — każde emitowane przez nas ustalenie jest już decyzją triage).

### Ustal URL pliku przeglądu

Treści komentarzy inline zawierają link do zatwierdzonego raportu dla pełnego uzasadnienia Strength / Tradeoff / Confidence / Blind spot. Zbuduj `REVIEW_URL` względem właśnie wypchniętego commita:

```bash
SHA=$(git rev-parse HEAD)
REVIEW_URL="${GITHUB_SERVER_URL:-https://github.com}/${GITHUB_REPOSITORY}/blob/${SHA}/${REVIEW_PATH}"
```

### Klasyfikuj ustalenia: inline vs. tylko podsumowanie

GitHub Reviews API odrzuca komentarze do wierszy poza diffem, więc zweryfikuj to przed publikacją. Dla każdego ustalenia przeanalizuj `Location`; jeśli to `file:line`, sprawdź, czy `line` znajduje się wewnątrz hunka `+` dla `file`:

```bash
line_in_diff() {
  local file="$1" line="$2"
  git diff --unified=0 "origin/${BASE}...HEAD" -- "$file" \
    | awk -v target="$line" '
        /^@@/ {
          match($0, /\+([0-9]+)(,([0-9]+))?/, m);
          start = m[1]; count = (m[3] == "" ? 1 : m[3]);
          if (target >= start && target < start + count) { found = 1; exit }
        }
        END { exit !found }
      '
}
```

Kieruj każde ustalenie:

- **kwalifikujące się inline** → `Location` ma `file:line`, `file` jest w diffie PR, a `line_in_diff` zwraca true.
- **tylko podsumowanie** → wszystko inne: brak kotwicy wiersza, plik nie jest w diffie lub wiersz znajduje się poza hunkami diffu (częste dla MISSING TEST, MISSING IMPL i ustaleń na poziomie wymiaru).

Śledź liczniki: `N_INLINE`, `N_SUMMARY_ONLY`. Krok 9 używa obu w nagłówku podsumowania.

### Skomponuj treść każdego komentarza inline

Utrzymuj treści inline łatwe do przeskanowania — recenzenci czytają je szybko podczas przewijania diffu. Jednowierszowy znacznik wagi, tytuł, szczegół, jednowierszowe podsumowanie naprawy, link do pełnego raportu. Zakończ **niewidocznym znacznikiem**, aby następne uruchomienie mogło znaleźć i usunąć ten komentarz podczas zastępowania:

```markdown
❌ **CRITICAL** · Safety & Quality · **F1 — SQL query built with string concatenation**

Plan specified parameterized queries in Phase 2; actual implementation uses template literals.

**Fix:** Replace the template literal with a parameterized query using `db.query($1, [value])`.

_See [full report](<REVIEW_URL>) for reasoning (Strength / Tradeoff / Confidence / Blind spot)._

<!-- impl-review-ci:marker -->
```

Znacznik musi być obecny w **każdej** treści komentarza inline — w ten sposób podsekcja "Clean up prior run" poniżej identyfikuje artefakty do usunięcia. Pełna gramatyka opcji Fix (Strength / Tradeoff / Confidence / Blind spot) pozostaje w zatwierdzonym pliku raportu — umieszczanie jej inline zaśmieca widok diffu i powiela treść.

### Opublikuj każde ustalenie przez narzędzie MCP

**Przed** pierwszym wywołaniem MCP zapisz znacznik czasu UTC — poniższa podsekcja czyszczenia używa go do rozróżnienia komentarzy z "poprzednich uruchomień" od tych, które zaraz utworzysz:

```bash
NOW_ISO=$(date -u +%Y-%m-%dT%H:%M:%SZ)
```

Dla każdego ustalenia kwalifikującego się inline wywołaj `mcp__github_inline_comment__create_inline_comment` **raz**. Wywołanie jest synchroniczne — publikuje natychmiast po ustawieniu `confirmed: true`. Śledź powodzenia i niepowodzenia dla każdego ustalenia:

```
mcp__github_inline_comment__create_inline_comment({
  path: "<finding.file>",
  line: <finding.line>,
  side: "RIGHT",            // comment on the new version; LEFT is only for deleted lines
  confirmed: true,           // skip the Haiku classifier buffer — we're deterministic
  body: "<composed body incl. marker>"
})
```

Dlaczego `confirmed: true`: domyślnie akcja buforuje niepotwierdzone wywołania, przepuszcza je przez klasyfikator Haiku ("is this a real review or a test?") i publikuje tylko prawdziwe w post-stepie. Jest to przydatne dla konwersacyjnych przepływów przeglądu, gdzie Claude prowadzi eksplorację. W tej umiejętności każde ustalenie jest już decyzją triage — chcemy, aby były publikowane w niezmienionej postaci, synchronicznie, by krok sprawdzania werdyktu workflow widział stan końcowy.

**Śledź wyniki:**

- Każde udane wywołanie → zwiększ `N_INLINE_POSTED`.
- Każde niepowodzenie → zmniejsz planowaną liczbę, zaloguj file:line i dodaj ustalenie do listy tylko-podsumowanie, aby nadal dotarło do recenzentów. Nie próbuj ponownie — komunikat błędu narzędzia MCP wyjaśnia dlaczego (wiersz przesunął się poza diff, plik został przemianowany, przejściowy problem API), a ponowne próby rzadko pomagają.
- Jeśli **każde** wywołanie inline nie powiedzie się: ustaw `INLINE_POST_FAILED=1`. Ścieżka awaryjna Kroku 9 pokazuje wszystkie ustalenia w podsumowującym komentarzu z widocznym ostrzeżeniem.

Po powrocie wszystkich wywołań dla pojedynczych ustaleń ustaw `N_INLINE = N_INLINE_POSTED` i przeklasyfikuj ustalenia z nieudanym inline jako tylko-podsumowanie. Krok 9 używa końcowych liczników.

**Nie używaj `gh api POST /pulls/:n/reviews` z tablicą `comments[]`.** Ten endpoint po cichu odrzuca komentarze, których wiersz nie znajduje się w prawidłowej pozycji hunka diffu — kończysz z powłoką przeglądu i zerową liczbą zakotwiczonych komentarzy. Narzędzie MCP używa endpointu dla pojedynczego komentarza, który głośno zgłasza błąd, dzięki czemu wiemy, które ustalenie się nie powiodło i dlaczego.

**Pomiń całą podsekcję, jeśli `N_INLINE == 0`** — brak ustaleń kwalifikujących się inline, nic do publikacji. Krok 9 renderuje wszystko jako tylko-podsumowanie.

### Usuń komentarze inline z poprzedniego uruchomienia

Dopiero gdy przynajmniej jedno wywołanie MCP **powiedzie się** w tym uruchomieniu — usuń wcześniejsze komentarze inline bota zidentyfikowane przez znacznik. Kolejność publikuj-nowe-potem-usuwaj-stare jest celowa: jeśli publikacja wszystkich nowych nie powiedzie się, wcześniejsze komentarze pozostają widoczne, aby recenzenci nie zostali z niczym.

Zapisz `NOW_ISO` **przed** pierwszym wywołaniem MCP, aby niezawodnie poprzedzał każdy komentarz utworzony w tym uruchomieniu; użyj go poniżej, aby wyłączyć te komentarze z listy usuwania.

```bash
# NOW_ISO was captured earlier, before the first create_inline_comment call:
#   NOW_ISO=$(date -u +%Y-%m-%dT%H:%M:%SZ)

if [ "$N_INLINE_POSTED" -gt 0 ]; then
  gh api --paginate "repos/{owner}/{repo}/pulls/${PR_NUMBER}/comments" \
    --jq ".[] | select(.body | contains(\"<!-- impl-review-ci:marker -->\")) | select(.created_at < \"${NOW_ISO}\") | .id" \
    | while read -r COMMENT_ID; do
        gh api --method DELETE \
          "repos/{owner}/{repo}/pulls/comments/${COMMENT_ID}" 2>/dev/null || true
      done
fi
```

`|| true` przy usuwaniu jest celowe — przejściowe niepowodzenie API dla jednego komentarza nie powinno przerwać czyszczenia pozostałych. Pozostałe zaległości zostaną usunięte przy następnym uruchomieniu.

**Dlaczego `created_at < NOW_ISO` zamiast przechwytywania identyfikatorów komentarzy nowego przeglądu?** To prostsze i odporne na wyścigi, w których POST się powiódł, ale format odpowiedzi API się zmienia. Każdy komentarz utworzony przed "teraz" z definicji pochodzi z poprzedniego uruchomienia.

### Niepowodzenie nie jest krytyczne

Niepowodzenia MCP dla poszczególnych ustaleń (wiersz się przesunął, plik przemianowano, przejściowy problem API) są już obsłużone powyżej — nieudane ustalenie przechodzi do tylko-podsumowanie. Jedynym pozostałym katastrofalnym przypadkiem jest niepowodzenie **wszystkich** wywołań inline, które ustawia `INLINE_POST_FAILED=1`:

- Jeśli `INLINE_POST_FAILED=1`, Krok 9 uwzględnia **wszystkie** ustalenia w podsumowującym komentarzu z ostrzeżeniem na górze: `⚠️ Inline review failed to post; all findings shown below.`
- W przeciwnym razie Krok 9 wymienia tylko ustalenia wyłącznie podsumowujące (te inline znajdują się przy diffie).

Nigdy nie kończ workflow kodem niezerowym z powodu błędu publikacji inline — raport jest zatwierdzony, podsumowanie zostanie opublikowane, recenzenci nadal mają wszystko. Głośne ostrzeżenie jest lepsze niż czerwony workflow.

## Krok 9: Opublikuj podsumowujący komentarz PR

TaskUpdate: activeForm "Publikowanie komentarza podsumowującego"

Zatwierdzony plik zawiera pełne szczegóły; komentarze inline zakotwiczają ustalenia do konkretnych wierszy; to podsumowanie jest łatwym do przeskanowania punktem wejścia na osi czasu PR — werdykt, tabela wymiarów i wszelkie ustalenia, których nie można było opublikować inline.

### Skomponuj i opublikuj

Sekcje treści (w kolejności): baner bramki REJECTED (warunkowo), nagłówek werdyktu, linki do planu i pliku przeglądu, liczniki inline/podsumowania, uwaga awaryjna (warunkowo), tabela wymiarów, lista ustaleń (warunkowo), wiersz końcowy, **znacznik**:

```bash
# REJECTED verdict → prepend a visible gate banner. The workflow step
# after claude-code-action reads the verdict from the committed report
# and fails the check; the banner tells reviewers what to do.
if [ "$OVERALL_VERDICT" = "REJECTED" ]; then
  REJECTION_BANNER=$'> ⛔ **This check will fail** because the verdict is `REJECTED`.\n> Add the `impl-review-override` label to the PR to bypass after reviewing the findings.\n\n'
else
  REJECTION_BANNER=""
fi

if [ "$INLINE_POST_FAILED" = "1" ]; then
  # Inline post failed — show all findings in the summary as fallback.
  FINDINGS_FOR_SUMMARY_MARKDOWN="$ALL_FINDINGS_MARKDOWN"
  FINDINGS_SECTION_HEADER="### All findings"
  FAILURE_NOTE=$'⚠️ Inline review failed to post; all findings shown below.\n\n'
  INLINE_LINE="**Inline comments:** failed to post — see findings below"
else
  # Inline succeeded — summary shows only findings that couldn't be anchored.
  FINDINGS_FOR_SUMMARY_MARKDOWN="$SUMMARY_ONLY_FINDINGS_MARKDOWN"
  FINDINGS_SECTION_HEADER="### Findings without a line anchor"
  FAILURE_NOTE=""
  INLINE_LINE="**Inline comments:** ${N_INLINE} posted on changed lines · ${N_SUMMARY_ONLY} without line anchor (below)"
fi

# Capture cutoff for prior-comment cleanup below.
NOW_ISO=$(date -u +%Y-%m-%dT%H:%M:%SZ)

gh pr comment "$PR_NUMBER" --body "$(cat <<EOF
## 🔍 Implementation Review (CI)

${REJECTION_BANNER}**Verdict:** \`${OVERALL_VERDICT}\` — ${N_CRITICAL} critical, ${N_WARNINGS} warnings, ${N_OBSERVATIONS} observations
**Plan:** \`${PLAN}\`
**Review file:** [${REVIEW_PATH}](${REVIEW_URL})
${INLINE_LINE}

${FAILURE_NOTE}| Dimension | |
|---|---|
| Plan Adherence | ${V_PLAN} |
| Scope Discipline | ${V_SCOPE} |
| Safety & Quality | ${V_SAFETY} |
| Architecture | ${V_ARCH} |
| Pattern Consistency | ${V_PATTERN} |
| Test Coverage | ${V_TESTS} |
| Success Criteria | ${V_SUCCESS} |

${FINDINGS_SECTION_HEADER}

${FINDINGS_FOR_SUMMARY_MARKDOWN}

---

Pull this branch to see the full review at \`${REVIEW_PATH}\` and triage findings locally.

<!-- impl-review-ci:marker -->
EOF
)" && SUMMARY_POSTED=1
```

Renderuj każde ustalenie wymienione w podsumowaniu jako jeden zwarty wiersz (bez szczegółów opcji Fix — znajdują się w pliku):

```
- **F3** `WARNING` · Test Coverage · `src/api/routes.ts` — new `/status` endpoint has no test. Plan listed `pnpm test -- src/api/routes.test.ts` in Success Criteria but that file wasn't modified.
- **F5** `CRITICAL` · Plan Adherence · _(no file)_ — migration in Phase 3 is missing from the diff entirely.
```

Ogranicz listę podsumowania do 5. Jeśli jest ich więcej, dopisz `…and N more in the full report.`

### Usuń podsumowujący komentarz z poprzedniego uruchomienia

Ten sam wzorzec publikuj-nowe-potem-usuwaj-stare jak przy czyszczeniu inline. Usuwaj wcześniejsze podsumowania tylko wtedy, gdy nowe zostało pomyślnie opublikowane:

```bash
if [ "$SUMMARY_POSTED" = "1" ]; then
  gh api --paginate "repos/{owner}/{repo}/issues/${PR_NUMBER}/comments" \
    --jq ".[] | select(.body | contains(\"<!-- impl-review-ci:marker -->\")) | select(.created_at < \"${NOW_ISO}\") | .id" \
    | while read -r COMMENT_ID; do
        gh api --method DELETE \
          "repos/{owner}/{repo}/issues/comments/${COMMENT_ID}" 2>/dev/null || true
      done
fi
```

Zwróć uwagę na różnicę endpointów: komentarze podsumowujące w zakładce Conversation znajdują się pod `/issues/:n/comments` (komentarze podsumowujące PR są pod maską GitHub issues), podczas gdy komentarze przeglądu inline w Kroku 8 znajdują się pod `/pulls/:n/comments`.

### Przypadki brzegowe

- `N_SUMMARY_ONLY == 0` i `INLINE_POST_FAILED == 0` → całkowicie pomiń sekcję "Findings without a line anchor". Wystarczy werdykt + tabela wymiarów + liczba inline; bez pustej listy.
- `N_INLINE == 0` i `N_SUMMARY_ONLY == 0` → brak jakichkolwiek ustaleń. Nadal opublikuj podsumowanie (tabela werdyktów potwierdza PASS we wszystkich wymiarach); pomiń sekcję ustaleń.

Oznacz zadanie przeglądu CI jako `completed`. Gotowe — krok verdict-check workflow (po claude-code-action) odczytuje werdykt z zatwierdzonego raportu i powoduje niepowodzenie kontroli, gdy jest `REJECTED`, respektując etykietę `impl-review-override`.

## Uwagi operacyjne

- **Znacznik raportu jest kluczowy.** Komentarz pierwszego wiersza `<!-- IMPL-REVIEW-REPORT -->` i pola `Decision: PENDING` dla każdego ustalenia stanowią kontrakt wyjściowy z narzędziami downstream. Nie zmieniaj ich formatu.
- **Nigdy nie edytuj kodu źródłowego.** Wszystkie zmiany kodu przechodzą przez osobny krok implementacji po triage. Umiejętność przeglądu czyta, analizuje i zapisuje raport — nic więcej.
- **Przegląd inline ma charakter doradczy, nigdy blokujący.** Narzędzie MCP tworzy komentarze przeglądu (przez `POST /pulls/:n/comments`), a nie formalne decyzje przeglądu PR — więc nie ma powierzchni `event: APPROVE` / `REQUEST_CHANGES`, o którą umiejętność mogłaby się potknąć. Jeżeli przyszła wersja kiedykolwiek wyśle formalny przegląd, musi użyć `event: COMMENT`: umiejętność nie ma mandatu do zatwierdzania, a request-changes jest decyzją ludzkiego zarządzania. Bramka REJECTED — która *jest* blokująca — znajduje się w kroku verdict-check workflow, a nie w żadnym zgłoszeniu przeglądu po stronie Claude.
- **Werdykt `REJECTED` jest sygnałem bramkującym.** Krok workflow po przeglądzie analizuje werdykt z zatwierdzonego pliku raportu (`- **Verdict**: REJECTED`) i kończy się kodem niezerowym, chyba że PR ma etykietę `impl-review-override`. To **jedyny** sposób, w jaki ta umiejętność powoduje niepowodzenie kontroli — sama umiejętność kończy się kodem 0 nawet wtedy, gdy werdykt to REJECTED, ponieważ bramka znajduje się w workflow, a nie w turze Claude. To rozdzielenie jest istotne: niepowodzenie po stronie Claude pominęłoby także komentarz PR i zdezorientowało recenzentów; bramka po stronie workflow kończy się poprawnie po opublikowaniu wszystkich artefaktów.
- **Znacznik deduplikacji jest kluczowy.** Każda treść komentarza inline oraz komentarz podsumowujący kończą się `<!-- impl-review-ci:marker -->`. Następne uruchomienie używa tego znacznika do znajdowania i usuwania wcześniejszych artefaktów, zapobiegając ich kumulowaniu podczas ponownych uruchomień. Czyszczenie stosuje kolejność publikuj-nowe-potem-usuwaj-stare: jeśli nowa publikacja się nie powiedzie, poprzedni artefakt pozostaje widoczny, aby recenzenci nigdy nie widzieli zerowego pokrycia.
- **Nie wypisuj sekretów.** Subagenci są tylko do odczytu/grep/bash, ale mimo to: nigdy nie zrzucaj zmiennych środowiskowych do raportu ani komentarza. Jeśli ustalenie odwołuje się do ujawnionego sekretu (zakodowany na stałe token, poświadczenie w kodzie), ukryj rzeczywistą wartość — napisz `<REDACTED token matching pattern X>`, a nie dosłowny ciąg.
- **Koszt uruchamiania testów.** Agent 3 uruchamia polecenia testowe z planu. W przypadku dużych zestawów jest to czasochłonne. Jeśli projekt chce pominąć wykonywanie określonych zestawów, plan powinien pomijać te polecenia z Automated Verification — ta umiejętność uruchamia tylko to, co deklaruje plan, więc plan jest powierzchnią kontroli.
- **Niestandardowy format planu.** Jeśli plik planu istnieje, ale nie stosuje oczekiwanej anatomii (brak Success Criteria, brak Changes Required itd.), wykonaj przegląd na podstawie tego, co da się wyodrębnić, i odnotuj braki strukturalne w raporcie. Nie odmawiaj uruchomienia — częściowy sygnał jest lepszy niż brak sygnału.
- **Budżety tokenów subagentów.** Trzej subagenci pracują równolegle, więc główny kontekst pozostaje szczupły — zawiera tylko tekst planu, podsumowanie diffu i końcowy raport ustaleń każdego agenta. Jeśli raport subagenta staje się zbyt obszerny, poproś go o zwrócenie tylko ustaleń o najwyższej wadze, które zidentyfikował, a nie pełnego śladu dochodzenia.