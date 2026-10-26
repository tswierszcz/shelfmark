<!-- IMPL-REVIEW-REPORT -->
# Implementation Review: Testing pipeline input hardening

- **Plan**: context/changes/testing-pipeline-input-hardening/plan.md
- **Scope**: Full plan
- **Reviewed phases**: 1, 2, 3
- **Date**: 2026-10-26
- **Verdict**: NEEDS ATTENTION
- **Findings**: 0 critical, 3 warnings, 6 observations

## Verdicts

| Dimension | Verdict |
|-----------|---------|
| Plan Adherence | PASS |
| Scope Discipline | WARNING |
| Safety & Quality | WARNING |
| Architecture | PASS |
| Pattern Consistency | WARNING |
| Success Criteria | PASS |

Success criteria re-run 2026-10-26: `go build ./...` OK; TestUploadHandler_OversizedFileReturns400, TestProjectUploadHandler_OversizedFileReturns400, TestSanitizeForPrompt, TestQueue_BatchSettles_CountsReconcileAndFailureReasonPersists PASS; `go test ./internal/catalog/...` ok (171s); `go test ./internal/db/...` ok. Manual 1.5 confirmed by user; 3.6 verified in browser against a throwaway local instance (batch of 9 → `{"failed": 9}`, sum 9, all terminal; every row carried `error_detail` = OpenRouter 401).

## Findings

### F1 — error_detail leaks to Architect/Viewer via organization libraries endpoint

- **Severity**: ⚠️ WARNING
- **Impact**: 🔎 MEDIUM — real tradeoff; pause to reason through it
- **Dimension**: Safety & Quality
- **Location**: backend/internal/catalog/store.go:57, backend/internal/catalog/handlers.go:275 (ListOrganizationLibrariesHandler → libraryResponse embeds Library)
- **Detail**: `Library.ErrorDetail` is serialized wherever `Library` is, so `GET /api/organization/libraries` (Architect + Viewer, main.go:55) now returns raw wrapped error chains — upstream URLs, dial/DNS errors, upstream status codes. No secrets, but internal diagnostics reach non-admin roles. The plan only intended it as an admin/pipeline diagnostic.
- **Fix A ⭐ Recommended**: Nil out `ErrorDetail` in `ListOrganizationLibrariesHandler` before building the response (admin `/api/catalog/libraries` keeps it).
  - Strength: One-line, local to the only non-admin consumer; admin keeps the debugging value Phase 3 was built for.
  - Tradeoff: Relies on each future non-admin handler remembering to strip it.
  - Confidence: HIGH — verified the two handlers share `libraryResponse`.
  - Blind spot: Project report path builds its own struct (doesn't carry ErrorDetail today) — fine, but not enforced.
- **Fix B**: Tag `ErrorDetail` `json:"-"` on `Library` and add an explicit `error_detail` field only in an admin-specific response type.
  - Strength: Safe by default for every future handler.
  - Tradeoff: New DTO and more churn in handlers.go; admin UI contract changes shape slightly.
  - Confidence: MED — straightforward, but touches the shared response type.
  - Blind spot: Frontend currently ignores the field; not verified any consumer expects it.
- **Decision**: FIXED (Fix A) — ListOrganizationLibrariesHandler nils ErrorDetail; regression test TestErrorDetail_VisibleToAdminCatalogButNotOrganizationLibraries (break-check red→restored)

### F2 — Test Postgres helper leaks a Docker volume per test

- **Severity**: ⚠️ WARNING
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Safety & Quality
- **Location**: backend/internal/catalog/store_test.go:55
- **Detail**: Cleanup uses `docker rm -f <name>` without `-v`, so each integration test leaves an anonymous postgres data volume (~40MB). Today this filled `/` to 100% (722 volumes, 28.9GB) and made the whole suite fail with "postgres did not become ready". Reproduced again during this review.
- **Fix**: Use `docker rm -f -v` in the cleanup and add `--tmpfs /var/lib/postgresql/data` to `docker run` (no disk writes, faster startup).
- **Decision**: FIXED — `--tmpfs /var/lib/postgresql/data` + `docker rm -f -v` in all three Postgres test helpers (catalog/store_test.go, auth/handlers_test.go, db/migrate_test.go)

### F3 — TestSanitizeForPrompt table is driven by case names, assertions are weak

- **Severity**: ⚠️ WARNING
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Pattern Consistency
- **Location**: backend/internal/catalog/openrouter_test.go:31
- **Detail**: Assertions `switch` on `tt.name`, so renaming a case silently skips its checks. The injection case only checks `\n`, not `\r`/`\t`/U+0085, nor whitespace collapsing. Unlike the table-driven style in purl_test.go (explicit `want`).
- **Fix**: Give each case a `want string` with the exact expected output and add `\r\n\t` + multi-space cases.
- **Decision**: FIXED — table now uses exact `want`; added CR/LF/tab/NEL, whitespace-collapse, Cf and exact-200-rune cases

### F4 — GitHub topics still reach the Categorize prompt unsanitized

- **Severity**: ℹ️ OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Safety & Quality
- **Location**: backend/internal/catalog/openrouter.go:186
- **Detail**: Plan scoped sanitization to purl-derived fields; `githubTopics` (third-party controlled) and `taxonomy` go in via `%v`. Risk is low (GitHub restricts topics to `[a-z0-9-]`, ≤50 chars; output filtered against taxonomy at pipeline.go:71), but the plan's own scoping left this gap.
- **Fix**: Map `sanitizeForPrompt` over `githubTopics` and cap their count (e.g. 20).
- **Decision**: FIXED — sanitizeTopicsForPrompt (sanitize each topic, cap at 20) applied in Categorize; TestSanitizeTopicsForPrompt_SanitizesEachTopicAndCapsCount

### F5 — error_detail has no length bound

- **Severity**: ℹ️ OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Safety & Quality
- **Location**: backend/internal/catalog/queue.go:96
- **Detail**: Message embeds `lib.Purl` via `%q`; purl length is bounded only by the 10MB upload cap.
- **Fix**: Truncate `errMsg` to ~500 bytes before persisting.
- **Decision**: SKIPPED

### F6 — Non-oversize multipart parse errors are swallowed

- **Severity**: ℹ️ OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Safety & Quality
- **Location**: backend/internal/catalog/handlers.go:70, backend/internal/catalog/project_handlers.go:41
- **Detail**: Only `*http.MaxBytesError` is handled; any other `ParseMultipartForm` error falls through to the misleading `kind must be...` / missing-field 400 the comment says it prevents.
- **Fix**: Return 400 "invalid multipart form" for any other parse error.
- **Decision**: SKIPPED

### F7 — error_detail uses omitempty while sibling nullable fields serialize null

- **Severity**: ℹ️ OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Pattern Consistency
- **Location**: backend/internal/catalog/store.go:57
- **Detail**: Plan specified `omitempty`, but `github_repo` (same struct, project.go:36) emits `null`; API shape is inconsistent. Moot if F1 Fix B is chosen.
- **Fix**: Drop `omitempty` to match `github_repo` (or keep it and accept; plan asked for it).
- **Decision**: SKIPPED

### F8 — Format-category runes (bidi, zero-width) pass the sanitizer

- **Severity**: ℹ️ OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Safety & Quality
- **Location**: backend/internal/catalog/openrouter.go:131
- **Detail**: Only `unicode.IsControl` (Cc) is replaced; Cf runes such as U+202E or U+200B survive. `%q` quoting remains the real boundary, so impact is cosmetic.
- **Fix**: Also replace `unicode.Is(unicode.Cf, r)`.
- **Decision**: FIXED — sanitizeForPrompt also replaces unicode.Cf runes

### F9 — Undocumented adaptations beyond the plan

- **Severity**: ℹ️ OBSERVATION
- **Impact**: 🏃 LOW — quick decision; fix is obvious and narrowly scoped
- **Dimension**: Scope Discipline
- **Location**: handlers.go:70, project_handlers.go:41, queue_test.go:107, project.go:77
- **Detail**: Benign extras/drift: explicit `ParseMultipartForm` oversize checks (needed because `FormValue` swallows the error), a test-local `artifactRoutingFinder` instead of combining `fakeRepoFinder` + `countingErrFinder` (one queue shares one finder list), and `LibrariesByPurls` scanning `error_detail`. `ProjectReport` does not carry `error_detail` (fine, not in plan).
- **Fix**: Record these adaptations in change.md `## Notes`.
- **Decision**: FIXED — adaptations recorded in change.md ## Notes
