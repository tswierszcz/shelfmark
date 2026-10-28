# Sources: documents

- documents: 3 requirements, 21 narrative (2 active change folders with notes/research, 19 archived changes; 18 plans total), 4 reference, 5 engineering

## Domain documents
| path | kind | what it is about |
|---|---|---|
| `context/foundation/prd.md` | requirements | product requirements: vision, persona, success criteria, user story US-01, functional requirements FR-001–FR-008, business logic, access control, non-goals, open questions |
| `context/foundation/shape-notes.md` | requirements | discovery notes that fed the PRD (problem, users, scope decisions) |
| `context/foundation/archive/shape-notes-2026-10-15-2157.md` | requirements | earlier version of the shape notes |
| `context/foundation/roadmap.md` | narrative | milestone, foundations F-01–F-02, slices S-01–S-03, done list |
| `context/archive/2026-10-1*-*/`, `context/archive/2026-10-2*-*/` (19 folders) | narrative | per-change identity, plan, plan brief, reviews; the vocabulary used while building each slice and fix (catalog seeding, strategic marking, project report, user management, batch status, deploy, test rollouts) |
| `context/changes/bootstrap-verification/`, `context/changes/impl-review-ci/` | narrative | open but parked changes (scaffold verification, CI review setup) — little domain content |
| `CLAUDE.md`, `AGENTS.md` | reference | agent instructions; AGENTS.md has a one-line product description |
| `docs/runbook.md` | reference | operations runbook; names seed accounts, smoke flow (upload catalog, upload project SBOM, see report) |
| `docs/samples/smoke-catalog.csv`, `docs/samples/smoke-sbom.cdx.json` | reference | sample business data (library coordinates, an SBOM with a dependency graph) |
| `idea-check.md` | reference | pre-project idea assessment (Polish) |
| `context/foundation/test-plan.md` | engineering | risk register and test rollout; states business risks as rules (redundancy report correctness, strategic cap) |
| `context/foundation/lessons.md` | engineering | one team lesson (empty list contract) |
| `context/foundation/tech-stack.md`, `infrastructure.md`, `health-check.md`, `test-stack.md` | engineering | stack, platform and test tooling decisions |

Not listed as domain evidence (analyses of this codebase, priors for the synthesis only): `context/map/repo-map.md`, `context/changes/redundancy-flow-analysis/research.md`, `context/archive/2026-10-27-refactor-opportunities/research.md` and `candidates.md`.

## Product goals
- `context/foundation/prd.md:22` — managing dependencies "consumes enormous ongoing engineering effort"; the cost recurs at every version bump and CVE.
- `context/foundation/prd.md:24` — insight: "the number of libraries in use is itself the attack surface"; nobody answers "what is redundant".
- `context/foundation/prd.md:26` — solution: "an internal knowledge base that classifies libraries by capability", confronted with the organization's SBOMs "to surface functional redundancy"; seeding the base is the first necessary stage.
- `context/foundation/prd.md:30` — persona: "an architect or platform engineer responsible for library standards".
- `context/foundation/prd.md:35-36` — primary success criteria: MVP part 1 (catalog seeding by Admin, LLM-assisted categorization) and part 2 (organization analysis: mark strategic library per category, upload project SBOM, report flagging redundancy, organization-level report).
- `context/foundation/prd.md:39` — secondary: health metrics enrichment.
- `context/foundation/prd.md:42-43` — guardrails: no private data in SBOMs; direct vs transitive role shown when the SBOM supports it.
- `context/foundation/prd.md:105-111` — non-goals: only Java/Maven (optionally Python); no SBOM generation or CVE scanning; no automatic enforcement; single-organization instance.

## Change goals
Each archived `plan.md` / `plan-brief.md` states the goal of one change (e.g. strategic marking, project redundancy report, batch status fixes, refactor of strategic-mark ownership). These are not product goals and are not used to judge the Core.

## Limitations
- One PRD, one user story (US-01); no glossary document, no ADR directory, no user-facing help content. Most vocabulary over time lives in archived change plans.
- `idea-check.md` and shape notes are partly in Polish; PRD, roadmap, plans and code are in English.
