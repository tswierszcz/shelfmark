---
change_id: impl-review-ci
title: Run /10x-impl-review-ci on pull requests in GitHub Actions
status: implementing
created: 2026-10-27
updated: 2026-11-02
archived_at: null
---

## Notes

Wpięcie /10x-impl-review-ci do GitHub Actions (krok 7 ścieżki wydania M1): workflow .github/workflows/impl-review.yml na bazie .claude/skills/10x-impl-review-ci/references/workflow-template.yml, dostosowany do Go+npm; vendoring skilla do repo (wyjątek w .gitignore); etykiety impl-review / impl-review-override; sekret ANTHROPIC_API_KEY ustawia użytkownik.

**Blocked 2026-10-27 → unblocked 2026-11-01:** no paid Anthropic API key (billing issue) until 2026-11-01; planning resumed once the key existed.

**Open 2026-11-02 (Progress 2.6):** smoke run works end to end (PR #20, CI verdict APPROVED), but the per-review cost on the Anthropic console is too high. Next: try OpenRouter with a GLM model for the review and compare cost and quality before closing 2.6.
