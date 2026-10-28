# Klasyfikacja problemów z `redundancy-flow-analysis/research.md`

Źródło: `context/changes/redundancy-flow-analysis/research.md` (sekcja „Dług techniczny” T1–T12, „Zasięg oddziaływania”, „Open Questions”), priory: `context/map/repo-map.md` (strefy ryzyka 1–2, rozjazdy struktury).

**KANDYDAT** = naprawa zmieniłaby strukturę kodu. **WEJŚCIE** = nie jest kandydatem (test, dokumentacja, komentarz); służy do oceny kosztu i wykonalności.

| Id | Problem | Źródło | Klasyfikacja | Uzasadnienie |
|---|---|---|---|---|
| K1 | Zakres „wyboru strategicznego” zdefiniowany w dwóch miejscach: limit per (kategoria, ekosystem) vs raport i „kategorie przejrzane” per kategoria | T1; V2 | KANDYDAT (z zastrzeżeniem domenowym) | jedno źródło reguły zakresu zmienia strukturę; czy „przejrzana” ma być per ekosystem — decyzja produktowa |
| K2 | Jeden konkretny `Store` dla pięciu możliwości; te same tabele odpytywane z `store.go` i `project.go`; raport czyta `strategic_marks` z pominięciem metod znaczników | T5; V2–V4, V8; repo-map strefa 2 | KANDYDAT | podział warstwy danych / repozytoria per możliwość |
| K3 | Kontrakt HTTP Go ↔ TS trzymany konwencją: ręczne typy TS, `SkippedRow` bez tagów, zduplikowany klucz `is_direct` | T4; V11, V13 | KANDYDAT | jawne DTO / tagi / test kontraktu zmieniają strukturę szwu |
| K4 | Status paczki liczy tylko nowe purle; ponowne kolejkowanie bibliotek z wcześniejszej paczki | T3; V5 | KANDYDAT | zmiana modelu członkostwa w paczce (batch ↔ biblioteka) |
| K5 | Zduplikowana logika: pętla komponentów SBOM (2 kopie), zbiór statusów + `isBatchSettled` (2), `errorMessage` (4) | T7; V10, V14, V15 | KANDYDAT | ekstrakcja wspólnych funkcji |
| K6 | Granice plików w pakiecie `catalog`: `handlers.go` z handlerami czterech możliwości; `project_handlers.go` importuje pomocniki HTTP z `handlers.go` | repo-map „Realne powiązania”; V9 | KANDYDAT | wydzielenie wspólnych pomocników HTTP / podział plików lub pakietów |
| K7 | Kroki uploadu bez wspólnej transakcji (częściowy stan po błędzie) | T11 | KANDYDAT | jednostka pracy obejmująca zapis bibliotek i członkostwa |
| K8 | Dwa znaczenia `is_direct`: katalogowe (`EXISTS` po dowolnym projekcie) i per projekt | T10; V13 | KANDYDAT (mały) | rozdzielenie pojęć / nazw w modelu odczytu |
| W1 | Brak testu agregacji między projektami (ryzyko #4) | T2; V6, V7 | WEJŚCIE | brakujący test — osłona dla K1, K2, K4 |
| W2 | E2E poza CI, brak testów frontendu | T12 | WEJŚCIE | osłona dla K3, K5 |
| W3 | Komentarz parsera przeczy kodowi | T6 | WEJŚCIE | poprawka komentarza (lub decyzja, która strona ma rację) |
| W4 | `created` = liczba rozwiązanych | T8; V12 | WEJŚCIE | nazwa pola w kontrakcie; wchodzi w K3 jako szczegół |
| W5 | Nieaktualne komentarze `Enqueuer` / `noopEnqueuer` | T9 | WEJŚCIE | sprzątanie |
| W6 | Brak CI uruchamiającego e2e, `docs/reference/contract-surfaces.md` nie istnieje | Open Questions | WEJŚCIE | luka w dokumentacji / osłonach |
