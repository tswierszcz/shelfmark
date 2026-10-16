<!-- BEGIN @przeprogramowani/10x-cli -->

## Zestaw narzędzi AI 10xDevs - Moduł 2, Lekcja 2

Przekształć jeden element roadmapy w pierwszy cykl implementacji za pomocą **łańcucha planowania zmian**:

```
/10x-roadmap -> /10x-new -> /10x-plan -> /10x-plan-review -> /10x-implement
```

`/10x-new`, `/10x-plan`, `/10x-plan-review` i `/10x-implement` są głównym tematem lekcji. `/10x-frame` i `/10x-research` nie są tutaj wymaganymi rytuałami; są ścieżkami eskalacji wprowadzanymi w następnej lekcji.

### Router zadań - od czego zacząć

| Umiejętność | Użyj jej, gdy |
| --- | --- |
| **Konfiguracja zmiany (główny temat lekcji)** | |
| `/10x-new <change-id>` | Wybrano element roadmapy i potrzebny jest stabilny folder zmiany. Tworzy `context/changes/<change-id>/change.md`, aby planowanie, implementacja, postęp, commity i późniejszy przegląd miały wspólną tożsamość. Użyj PO wybraniu elementu roadmapy, PRZED `/10x-plan`. |
| **Planowanie (główny temat lekcji)** | |
| `/10x-plan <change-id>` | Masz folder zmiany i potrzebujesz planu implementacji możliwego do przeglądu. Odczytuje kontekst roadmapy, dokumenty podstawowe, dowody z codebase oraz wszelkie istniejące notatki dotyczące zmiany; zapisuje `plan.md` i `plan-brief.md` z fazami, kontraktami plików, kryteriami sukcesu oraz `## Progress`. |
| **Gotowość planu (główny temat lekcji)** | |
| `/10x-plan-review <change-id>` | Masz `plan.md` i potrzebujesz lekkiej kontroli gotowości przed rozpoczęciem kodowania. Użyj jej, aby wychwycić brakujący stan końcowy, słabe kontrakty, nieprawidłowo sformatowany postęp, dryf zakresu lub martwe punkty, zanim rozpoczną się zmiany w kodzie. |
| **Implementacja (główny temat lekcji)** | |
| `/10x-implement <change-id> phase <n>` | Masz zatwierdzony plan i chcesz wykonać jedną fazę z weryfikacją, ręczną bramką, rytuałem commitu oraz zapisaniem SHA w `## Progress`. |
| **Zamknięcie cyklu życia** | |
| `/10x-archive <change-id>` | Zmiana jest scalona lub celowo zamknięta. Przenieś ją z aktywnego `context/changes/` do stanu archiwalnego. |

### Jak łańcuch przekazuje pracę

- `/10x-new` tworzy trwałą tożsamość zmiany.
- `/10x-plan` przekształca tę tożsamość w kontrakt implementacyjny.
- `/10x-plan-review` sprawdza plan, zanim agent zmodyfikuje kod.
- `/10x-implement` wykonuje jedną zaplanowaną fazę, weryfikuje, prosi o ręczne potwierdzenie, gdy jest potrzebne, wykonuje commit i rejestruje postęp.

### Granice lekcji

- Plan jest domyślnym routerem po wyborze elementu roadmapy. Zacznij od `/10x-plan`, chyba że problem jest niejasny lub blokują Cię dowody zewnętrzne.
- Nie uruchamiaj `/10x-frame + /10x-research` jako ceremonii dla każdej zmiany.
- Nie przekształcaj tej lekcji w pełną, kompleksową budowę produktu. Punkt kontrolny z zaplanowanym i częściowo lub w pełni zaimplementowanym strumieniem jest prawidłowy.
- Przegląd kodu zaimplementowanego diffu należy do Lekcji 3 poprzez `/10x-impl-review`.
- Zamknięcie cyklu życia przez `/10x-archive` po scaleniu lub celowym zamknięciu zmiany.

### Ścieżki używane w tej lekcji

- `context/foundation/roadmap.md` - nadrzędna roadmapa
- `context/changes/<change-id>/change.md` - tożsamość zmiany
- `context/changes/<change-id>/plan.md` - kontrakt implementacyjny
- `context/changes/<change-id>/plan-brief.md` - skompresowane przekazanie
- `context/foundation/lessons.md` - powtarzające się reguły i pułapki
- `docs/reference/contract-surfaces.md` - rejestr nazw kluczowych dla działania systemu

Umiejętności nie mogą zapisywać do `context/archive/`. Zarchiwizowane zmiany są niezmienne; jeśli rozstrzygnięta ścieżka docelowa zaczyna się od `context/archive/`, przerwij z komunikatem: "This change is archived. Open a new change with `/10x-new` instead."

<!-- END @przeprogramowani/10x-cli -->
