---
artifact: domain-glossary
created: 2026-10-28
source: context/domain/domain-distillation.md
---

# Glossary: Shelfmark

Use these words in code, tests, commits and conversations. When a name in the code differs from the term, the term wins in new code; renaming old code is a separate decision.

## Classification catalog

| term | means | name in code | don't call it |
|---|---|---|---|
| Library | One package version known to the catalog, identified by its full purl. | `Library`, table `libraries` | artifact (a version-independent package is not modelled) |
| purl | The package URL that identifies a library, version included. | `Purl`, `ParsePurl` | coordinates, id |
| Ecosystem | The technology a library belongs to; today always Maven. | `Ecosystem` | language, platform |
| Category | One capability from the curated taxonomy that a library can be attached to. | `Category`, table `categories` | tag, topic (topics are GitHub input only), capability (thesis word) |
| Taxonomy | The closed, seeded set of categories. | `ListTaxonomy`, `/api/catalog/categories` | category list you can edit (there is no taxonomy management) |
| Classification base | The catalog of libraries with their categories, built by Admin. | package `catalog`, `/api/catalog/*` | organization (that is the architect's view of the same list) |
| Categorization | Finding a library's GitHub repo and asking an LLM which taxonomy categories fit. | `ProcessLibrary`, `Categorizer`, `Queue` | classification run, enrichment |
| Category source | Who attached a category: `github`, `llm` or `manual`. | `library_categories.source` | origin |
| Library status | Where a library is in categorization: `pending`, `processing`, `categorized`, `needs_manual`, `failed`. | `libraries.status` | state; "reviewed" (see Analysis) |
| needs_manual | Categorization ran but found no confident taxonomy category; a person must attach one. | `needs_manual` | "Needs review" in new code (UI label only), reviewed |
| Batch | The libraries one upload added to the catalog for the first time (meaning under review, see Q-03). | `upload_batch_id`, `BatchProgress` | the upload's libraries, project batch |
| Health metrics | Planned enrichment with a library's health signals; not built. | missing in code | — |

## Organization analysis

| term | means | name in code | don't call it |
|---|---|---|---|
| Project | A named set of libraries from one SBOM upload, matched by name without case. | `Project`, table `projects` | application, repo |
| SBOM | The CycloneDX file a project or catalog upload is read from. | `ParseProjectSBOM`, `ParseCycloneDXSBOM` | BOM file |
| Direct / transitive | Whether a library is a direct dependency of this project; set per project. | `project_libraries.is_direct` | `Library.is_direct` (that means "direct in any project") |
| Strategic mark | An architect's decision that a library is the standard choice in a category, with a justification. | `StrategicMark`, `MarkStrategic`, table `strategic_marks` | standard, strategic pick, designation, recommendation |
| Justification | The required reason recorded with every mark and unmark. | `strategic_marks.justification` | comment, note |
| Strategic cap | At most two strategic libraries per category and ecosystem. | `count >= 2` in `MarkStrategic`, `ErrStrategicCapReached` | limit of one |
| Reviewed category | A category in which at least one library holds an active strategic mark (any ecosystem today, see Q-01). | `isReviewed` | needs review (that is a library status) |
| Candidate to retire | A direct library in a reviewed category that is not itself the strategic choice there; computed, never stored. | `isRetireCandidate`, `RetireCandidate` | redundant library, to retire (no explicit retire decision exists) |
| Redundancy | Two or more libraries of the same category in one project (PRD measure). | missing in code | candidate to retire (a different, decision-based notion) |
| Redundancy report | A project's libraries by category with direct, strategic and retire-candidate flags. | `ProjectReport`, `/api/projects/{id}/report`, SPA route `redundancy` | project report vs redundancy report as two things (they are one) |
| Recommendation | PRD's "standard / to retire" decision; only the strategic mark exists, retire is computed. | missing in code (as an explicit decision) | strategic mark (only half of it) |
| Organization-level report | Redundancy across all uploaded projects (PRD); not built. | missing in code | — |
| My organization view | The architect's list of all catalog libraries with strategic marks and global direct flag. | `ListOrganizationLibrariesHandler`, `organization.tsx` | catalog (that is Admin's page of the same libraries) |
