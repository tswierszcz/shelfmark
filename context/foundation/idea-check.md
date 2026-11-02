# Shelfmark – ocena pomysłu (10x-idea-check)

Data oceny: 2026-10-10

## Rekomendacja

**Przejdź do sesji planistycznej (`/10x-shape`).**

Doświadczenie (20 lat w backendzie i integracjach, 10 lat w architekturze, rok pracy z agentami przy dużych projektach) i około 13 tygodni do terminu styczniowego wystarczają, żeby zacząć odkrywać wymagania. Zakres można ustalić w sesji planistycznej. Nie ma przeszkody, którą trzeba by sprawdzić wcześniej.

Największe ryzyko: pełna wizja ma skalę produktu startupowego. Obejmuje dwie płaszczyzny z podpisanym snapshotem, trzy widoki, ważenie CVE i koszt konsolidacji. Od jakości tagów capabilities zależy wartość całego produktu, bo bez dobrych tagów wykrywanie redundancji nie zadziała. To trzeba zweryfikować najwcześniej.

## Pomysł (wersja pierwotna)

Shelfmark to nakładka nad danymi o zależnościach, które organizacja już wytwarza (SBOM-y, wyniki skanerów CVE). Nie generuje SBOM-ów i nie skanuje podatności. Klasyfikuje biblioteki według tego, co robią: tagi z kuratorowanej taksonomii capabilities, sygnały zdrowia upstream i rola w projekcie (direct/transitive, runtime/test). Z tych danych wylicza redundancję funkcjonalną, wskazuje martwe biblioteki używane w produkcji, priorytet CVE ważony zasięgiem i szacowany koszt konsolidacji.

Produkt ma trzy widoki: Capabilities, Biblioteka i Projekt. Pozwala też ustalać rekomendacje (standard / do wycofania) z uzasadnieniem i historią decyzji.

Architektura ma dwie płaszczyzny: centralną bazę wiedzy publikowaną jako podpisany snapshot oraz instancję u klienta. Dane klienta nie opuszczają jego infrastruktury.

Odbiorcy: platform engineering i architekci w organizacjach z kilkudziesięcioma repozytoriami.

## Decyzje uczestnika

- Projekt od zera.
- Termin docelowy: 10 stycznia 2027 (termin 3).
- Dane wejściowe (SBOM-y) generowane istniejącymi narzędziami.
- Capabilities początkowo czytane z GitHuba, w drugim etapie tagowanie przez LLM.

## Sugestie (niezatwierdzone, do omówienia w `/10x-shape`)

- **Pierwszy działający kamień milowy:** import SBOM-ów (CycloneDX) z kilku repozytoriów jednego ekosystemu (np. Maven) i ręcznie przygotowana taksonomia dla ok. 100 bibliotek. Widok Capabilities pokazuje redundancję funkcjonalną.
- **MVP:** do tego sygnały zdrowia z GitHuba, flaga „martwa biblioteka używana bezpośrednio w runtime” oraz rekomendacje (standard / do wycofania) z uzasadnieniem i historią decyzji.
- **Później:** tagowanie przez LLM, centralna baza z podpisanym snapshotem, priorytet CVE, koszt konsolidacji.

## Czas

Liczba godzin tygodniowo nie została podana. Przy założeniu 8–10 h tygodniowo daje to około 100–130 h do 10 stycznia 2027. To szacunek dostępnego czasu, a nie wycena pracy. Wystarczy na MVP z testami i dokumentacją, ale nie na pełną wizję.

Termin styczniowy nie daje szansy na wyróżnienie. Zakres oceny wyznacza pierwsze zgłoszenie, więc ewentualne bloki Architect i Champion trzeba zgłosić razem z Builderem w tym samym terminie.

## Dopasowanie do kursu (Builder)

Prawdopodobna ścieżka do wymagań:

- **Kontrola dostępu:** logowanie z rolami (architekt podejmuje decyzje, zespół przegląda).
- **CRUD:** projekty, importy SBOM, decyzje i rekomendacje.
- **Logika biznesowa:** reguły redundancji i ocena „martwych” bibliotek.
- **Dokumenty kontekstowe:** PRD i dokumenty z sesji planistycznej.
- **Test z perspektywy użytkownika:** E2E przepływu import → rekomendacja.

Instalacja on-premise nie wyklucza projektu. Publiczny URL jest mile widziany, ale nieobowiązkowy. Demo można postawić na publicznych SBOM-ach.

Podział na dwie płaszczyzny może pasować do ścieżki Architect, ale jej wymagania będą przedstawione w module 3–4. Warto wtedy zapytać mentorów.

Ostateczną decyzję o certyfikacji podejmują osoby oceniające kurs.

## Do rozstrzygnięcia w sesji planistycznej

- Który ekosystem na start.
- Źródło i rozmiar początkowej taksonomii.
- Czy centralna płaszczyzna i snapshot wchodzą do MVP.
- Model ról i uprawnień.

## Opis pomysłu do `/10x-shape`

> Shelfmark: nakładka nad SBOM-ami organizacji, klasyfikująca biblioteki według capabilities (kuratorowana taksonomia), zdrowia upstream i roli w projekcie (direct/transitive, runtime/test), żeby wykrywać redundancję funkcjonalną i martwe zależności oraz zarządzać rekomendacjami (standard/do wycofania) z historią decyzji. Odbiorcy: platform engineering i architekci w organizacjach z dziesiątkami repozytoriów. Dane klienta zostają u klienta. Projekt od zera. SBOM-y generowane istniejącymi narzędziami. Capabilities początkowo z GitHuba, później przez LLM.
> Do rozstrzygnięcia: jeden ekosystem na start, źródło i rozmiar początkowej taksonomii, czy centralna płaszczyzna i snapshot wchodzą do MVP, model ról.
