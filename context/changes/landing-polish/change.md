---
change_id: landing-polish
title: Landing page polish (contact address, badge, logged-in redirect)
status: implementing
created: 2026-10-30
updated: 2026-10-30
archived_at: null
---

## Notes

Decyzje użytkownika (2026-10-30), po ustaleniu, że shelfmark.online to już produkcyjna aplikacja z `home.tsx` (port z 2026-10-20, `home-landing-page`):

- Adres kontaktowy w „Get in touch” i stopce: `hello@shelfmark.online` (zamiast `hello@shelfmark.dev`).
- Usunąć plakietkę „In development”.
- Zalogowany użytkownik wchodzący na `/` jest przekierowywany na `/dashboard`; niezalogowany widzi landing z przyciskiem „Zaloguj”.
