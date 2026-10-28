---
change_id: impl-review-ci
title: Run /10x-impl-review-ci on pull requests in GitHub Actions
status: new
created: 2026-10-27
updated: 2026-10-27
archived_at: null
---

## Notes

Wpięcie /10x-impl-review-ci do GitHub Actions (krok 7 ścieżki wydania M1): workflow .github/workflows/impl-review.yml na bazie .claude/skills/10x-impl-review-ci/references/workflow-template.yml, dostosowany do Go+npm; vendoring skilla do repo (wyjątek w .gitignore); etykiety impl-review / impl-review-override; sekret ANTHROPIC_API_KEY ustawia użytkownik.

**Blocked 2026-10-27:** no paid Anthropic API key (billing issue), so no `ANTHROPIC_API_KEY` secret for claude-code-action. Parked before planning; resume with `/10x-plan impl-review-ci` once the key exists.
