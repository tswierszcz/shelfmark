# Raport architektoniczny — moduł 4 (10xArchitect)

Autor: Tomasz Świerszcz · Data: 2026-11-02 · Wszystkie artefakty powstały na jednym repozytorium (Shelfmark).

## 1. Opisane projekty

| Repo | Stack | Skala (orientacyjnie) | Artefakty |
|---|---|---|---|
| **Shelfmark** (`tswierszcz/shelfmark`): klasyfikacja bibliotek OSS według capability i raport redundancji per SBOM projektu | Go (jeden moduł `backend/`), React Router SPA (`web/`), Postgres, Fly.io | 17 dni historii, 575 zmian, z czego 294 w kodzie; ok. 28 plików Go i 21 TS w grafie; jedna osoba, 80% commitów współtworzonych z AI | L2, L3, L4, L5 |

Źródła: L2 `context/map/repo-map.md` · L3 `context/archive/2026-10-27-redundancy-flow-analysis/research.md` · L4 `context/archive/2026-10-27-refactor-opportunities/{research,plan}.md` · L5 `context/domain/{domain-distillation,02-invariant-aggregate-refactor,03-anti-corruption-layer}.md`.


## 2. Mapa projektu (L2)

- **Strefa ryzyka 1: raport redundancji i status przetwarzania.** 11 z 25 commitów w tym obszarze to poprawki błędów. Status przetwarzania naprawiano dwa razy tego samego dnia, a błąd „`null` zamiast pustej listy” dwa razy w ciągu 5 dni. Ten obszar zależy od największej liczby innych części systemu.
- **Strefa ryzyka 2: wspólna warstwa danych.** Jeden pakiet Go obsługuje pięć różnych funkcji, a jeden plik (`store.go`) jest używany przez 8 innych miejsc. Jedyna twarda reguła biznesowa, limit bibliotek strategicznych, była naprawiana dwa razy; za drugim razem okazało się, że dwa jednoczesne kliknięcia mogły ją obejść.
- **Lokalne centra i punkty wejścia:** `routes.go` (lista wszystkich adresów API z rolami), `redundancy.tsx` (główny ekran), `project_handlers.go` → `project.go` (logika raportu).
- **Najważniejsze niewiadome** (pytania, na które mapa nie umiała odpowiedzieć):
  1. Jak działa cała ścieżka od wgrania SBOM projektu do gotowego raportu, krok po kroku? Mapa widzi, które pliki są ze sobą połączone, ale nie widzi kolejności kroków, bo część z nich dzieje się w tle, w kolejce. Na to pytanie odpowiada research z L3.
  2. Czy warto podzielić pakiet `catalog` na mniejsze części, skoro obsługuje pięć funkcji? Nie wiadomo, czy zysk przewyższy koszt.
  3. Czy serwer i strona zawsze tak samo rozumieją przesyłane dane? Serwer (Go) i strona (TypeScript) nie korzystają ze wspólnego opisu danych (np. Swagger/OpenAPI); oba opisy są pisane ręcznie i nic nie sprawdza, czy się zgadzają. Już dwa razy się nie zgadzały: dla konta bez projektów serwer wysyłał „nic” (`null`) zamiast pustej listy, a strona się przez to wysypywała.

## 3. Analiza ficzera (L3)

**Przepływ:** wgranie SBOM → przetwarzanie bibliotek → status → raport. Wybrałem go, bo to pierwsza strefa ryzyka z mapy i najważniejsza funkcja całej aplikacji.

**Jak działa:**
1. **Wejście:** architekt wgrywa plik SBOM projektu (listę jego zależności w formacie CycloneDX).
2. **Zapis:** serwer dopisuje do katalogu nieznane jeszcze biblioteki i w jednej transakcji zastępuje listę bibliotek projektu nową listą z pliku.
3. **Przetwarzanie w tle:** kolejka (5 równoległych wątków, do 3 prób) pobiera dane o bibliotekach z deps.dev i GitHuba, a model AI przypisuje im kategorię.
4. **Czekanie:** ekran co 2 sekundy pyta serwer, czy przetwarzanie się skończyło (najwyżej minutę), a potem pobiera raport.
5. **Wynik:** biblioteka jest **kandydatem do wycofania**, gdy projekt używa jej bezpośrednio, w jej kategorii wybrano już bibliotekę strategiczną, a ona sama nie jest strategiczna.

**Dług techniczny (top 3):**
1. **Raport nie rozróżnia ekosystemów, choć limit bibliotek strategicznych je rozróżnia.** Limit liczy się osobno dla Javy, Pythona itd., a to, czy kategoria jest „przejrzana”, już wspólnie dla wszystkich. **Potwierdzone ast-grepem:** z 7 zapytań SQL do tabeli znaczników strategicznych tylko jedno (limit) filtruje po ekosystemie. Dziś to nie szkodzi, bo system przyjmuje tylko Javę (świadome uproszczenie, zob. sekcja 6).
2. **Brak testów raportu dla kilku projektów naraz.** Każdy test sprawdzał jeden projekt (potwierdzone ast-grepem), a biblioteki i ich oznaczenia są wspólne dla wszystkich projektów. *Naprawione w L4:* faza 1 refaktoru dodała takie testy.
3. **Ukryte powiązanie przez bazę danych.** Raport czytał tabelę znaczników strategicznych własnym SQL-em, z pominięciem kodu, który nią zarządza, więc zmiana reguły w jednym miejscu nie trafiała do drugiego. *Częściowo naprawione w L4:* dostęp do tej tabeli jest teraz w jednym pliku; testy nadal wymagają prawdziwej bazy Postgres.

## 4. Plan refaktoryzacji (L4)

- **Wybrana opcja:** W research L4 AI ułożyło ranking trzech możliwych refaktorów (według tego, ile kosztuje zostawienie problemu, a ile jego naprawa). Wybrałem pierwszą pozycję: jeden kod odpowiedzialny za zarządzanie znacznikiem strategicznym.
- **Problem:** wiedza o tym, która biblioteka jest strategiczna, była rozrzucona po trzech miejscach w kodzie: w warstwie danych, w raporcie (który miał własne zapytania SQL i własną funkcję liczącą „kategorie przejrzane”) i w filtrze listy bibliotek. Te kopie zaczęły się już rozjeżdżać (problem z ekosystemem z sekcji 3).
- **Cel:** wszystko, co dotyczy znaczników strategicznych, trafia do jednego pliku, `strategic.go`. Tylko on czyta i zapisuje tabelę znaczników. Raport zamiast liczyć sam, pyta ten plik o trzy rzeczy: czy biblioteka jest strategiczna, czy jej kategoria jest przejrzana i czy jest kandydatem do wycofania.
- **Efekt:** przyszła zmiana reguły, np. decyzja o ekosystemie, dotknie jednego miejsca zamiast trzech.

Dwie pozostałe propozycje z rankingu, czyli automatyczne pilnowanie zgodności odpowiedzi API między backendem a frontendem oraz porządki w pakiecie `catalog`, zostały odłożone na później.


**Fazy:** najpierw zabezpieczenie testami, przeniesienie kodu ze zbiorczego `store.go` a na końcu właściwa zmiana. Każdy krok można było osobno sprawdzić i cofnąć.

| Faza | Co robi | Sprawdzenie automatyczne | Sprawdzenie ręczne |
|---|---|---|---|
| 1. Testy regresji | Testy, które zapisują, jak raport działa dziś, także dla kilku projektów naraz | testy przechodzą, a celowe zepsucie reguły robi je czerwonymi (czyli naprawdę czegoś pilnują) | czy scenariusze pokrywają ryzyko wielu projektów z test-planu |
| 2. Wyniesienie kodu  | Kod dedykowany znacznikom przeniesiony do `strategic.go`  bez zmiany ani jednej linijki | te same testy przechodzą przed i po | w diffie widać tylko przeniesione bloki |
| 3. Przepiecie raportu | Raport pyta `strategic.go` zamiast liczyć sam | testy z fazy 1 przechodzą bez zmian; zapytania do tabeli znaczników są tylko w `strategic.go` | raport w działającej aplikacji wygląda tak samo jak przed zmianą |

Plan został zrealizowany (commity `86dccdd`, `13225cf`, `9495cbe`), a impl-review dał werdykt APPROVED.

## 5. Domena wg DDD (L5)

**Język domeny (najważniejsze pojęcia):**
- *znacznik strategiczny (standard)*: architekt wskazuje bibliotekę jako standard w danej kategorii;
- *kategoria przejrzana*: kategoria, w której wybrano już co najmniej jeden standard;
- *kandydat do wycofania*: biblioteka używana bezpośrednio, w przejrzanej kategorii, która nie jest standardem;
- *raport projektu*: zestawienie bibliotek projektu z tymi oznaczeniami;
- *purl*: standardowy identyfikator pakietu z wersją (np. `pkg:maven/com.google.code.gson/gson@2.10`), używany w plikach SBOM i w usługach takich jak deps.dev; w Shelfmarku to tożsamość biblioteki.

Z 24 pojęć z dokumentów 4 nie istnieją w kodzie: miara „redundancji”, raport dla całej organizacji, jawna decyzja „do wycofania” i metryki kondycji bibliotek. Będą potrzebne w drugim etapie (M2), gdy baza pakietów zostanie zapełniona.

**Najważniejsze rozjazdy między dokumentami a kodem:**
- **Standard w dowolnej kategorii:** serwer nie sprawdza, czy biblioteka należy do kategorii, pilnuje tego tylko interfejs. Jedno wywołanie API może zmienić oznaczenia „do wycofania” we wszystkich projektach.
- **Ekosystem:** standard ma być wybierany osobno dla każdego ekosystemu, a „przejrzana” jest liczona wspólnie (ten sam problem co w sekcji 3).
- **Brak raportu organizacji:** obiecany w wymaganiach, nie ma go w kodzie; planowany na M2.

**Najważniejsza reguła i jej strażnik:** wybór standardu ma trzy warunki: najwyżej 2 standardy w kategorii, tylko biblioteka z tej kategorii i zawsze z uzasadnieniem. AI uznało drugi warunek za najgroźniejszy, bo dotyczy rdzenia produktu, a pilnuje go tylko interfejs. Propozycja: jeden obiekt `CategoryStrategy`, który przy każdym oznaczeniu sprawdza po stronie serwera wszystkie trzy warunki naraz.

**Warstwa ochronna (ACL):** najbardziej rozlana po kodzie jest biblioteka `packageurl-go` razem z założeniem, że każdy pakiet to Maven: siedzi w 5 plikach Go i w bazie danych, a ten sam purl jest odczytywany dwa razy. Dodanie Pythona wymagałoby dziś zmian we wszystkich tych miejscach. Propozycja: jeden plik `packageref.go` jako jedyne miejsce, które zna tę bibliotekę. Sprawdzian: wyszukanie `packageurl-go` ma zwrócić tylko ten plik; dziś zwraca 2 pliki (sprawdzone 2026-11-02).

## 6. Decyzje architekta mające istotny wpływ na całość projektu

**1. Jeden ekosystem (Java) w pierwszym etapie.** To moja decyzja jako Product Ownera. AI wskazało, że raport i limit standardów różnie traktują ekosystem, a kod zakłada, że każdy pakiet to Maven. Uznałem to za świadome uproszczenie, a nie błąd do naprawy teraz. Celem pierwszego etapu (M1) jest jak najszybciej pozwolić organizacji wgrać swoje pakiety i zobaczyć cały ich zbiór; dopiero na tej podstawie ma sens przeglądanie kategorii i dodawanie kolejnych ekosystemów. Uproszczenie wprowadziłem, gdy dostałem statystyki liczby bibliotek w pierwszej firmie, w której to wdrożę. Kod, który miał wszędzie wspierać wiele ekosystemów, robi to tylko częściowo. Nie przeszkadza to w działaniu i nie ma potrzeby usuwania tego wsparcia z już napisanego modułu.

**2. Warstwa ochronna (ACL) dopiero razem z Pythonem.** Analiza AI jest trafna, także w zakresie ograniczenia związanego z kwalifikatorami w purl. Dziś to świadome uproszczenie, które stanie się długiem technicznym, gdy będę wdrażał Pythona. Zanim to nastąpi, muszę pokazać skuteczność rozwiązania na samej Javie. Sama biblioteka `packageurl-go` jest prosta i realizuje standard PURL; gdyby przestała być wspierana, jej przepisanie nie będzie problemem.
