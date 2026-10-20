---
change_id: redundancy-view-brand-tokens
title: Introduce Shelfmark brand design tokens, applied to the redundancy report view
status: archived
created: 2026-10-19
updated: 2026-10-20
archived_at: 2026-10-20T20:22:32Z
---

## Notes

Wprowadzenie design-system tokenów marki Shelfmark (shelfmark.online) do aplikacji oraz zastosowanie ich na widoku context/web/app/routes/redundancy.tsx (najważniejszy ekran, 15 trafień w skanie literałów Tailwind). Repo nie ma obecnie żadnego systemu projektowego (tylko --font-sans w @theme, brak shadcn/radix/cva, brak katalogu komponentów) — ta zmiana go wprowadza w zakresie ograniczonym do tego, czego potrzebuje ten widok.

Źródło tokenów (wydobyte z żywej strony shelfmark.online):
- Kolory: --bg: #14110f, --bg-soft: #1b1714, --ink: #ece7df, --ink-dim: #a39b90, --ink-faint: #6d6459, --accent: #c9a227, --rule: #2e2823
- Fonty: Fraunces (serif, 400/600) na nagłówki, Inter (400/500) na treść — Google Fonts
- Logo SVG "grzbiety książek": rects #c9a227 i #ece7df (opacity .35-.85)
- Styl: ciemny, edytorialny, pill-shaped CTA (radius 999px), uppercase letter-spaced etykiety, cienkie linie-dzielniki (1px solid var(--rule)), brak kart/shadowów

Kolejne widoki (catalog.tsx, organization.tsx, login.tsx, dashboard.tsx) dostaną własne, osobne zmiany po tej, dziedziczące te same tokeny.

Design-system wariant (10x-ui): Brak systemu projektowego — wprowadzamy go. Ograniczony do bloku tokenów + komponentów używanych przez redundancy.tsx (nie cała biblioteka). Przegrywa z czymkolwiek repo już ma (nic nie ma).
