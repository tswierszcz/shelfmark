---
project: "Shelfmark"
version: 1
status: draft
created: 2026-10-15
context_type: greenfield
product_type: web-app
target_scale:
  users: small
  qps: low
  data_volume: small
timeline_budget:
  mvp_weeks: 12
  hard_deadline: "2027-01-10"
  after_hours_only: true
---

# Shelfmark — PRD

## Vision & Problem Statement

In organizations whose teams carry thousands to hundreds of thousands of dependencies, managing those dependencies — upgrading versions, tracking CVEs, chasing upstream support, researching project health metrics, and increasingly building "remediated" replacement builds — consumes enormous ongoing engineering effort. The cost recurs continuously: every version bump and every new CVE repeats the same manual, scattered research.

Insight: the number of libraries in use is itself the attack surface. CVE scanners and version-bump bots fix vulnerabilities one at a time, but they never reduce the set of dependencies an organization has to trust. Nobody answers "what is redundant" — only "what is vulnerable."

The solution requires an internal knowledge base that classifies libraries by capability (what each library actually does). An organization's own dependency data (SBOMs) is confronted against this classification base to surface functional redundancy. Building and seeding this classification base is the first necessary stage of the project — without it, analyzing an organization's own data would show nothing.

## User & Persona

Primary persona: an architect or platform engineer responsible for library standards, present across many organizations with dozens to hundreds of repositories. They feel this pain continuously — at every mass version bump and every CVE response.

## Success Criteria

### Primary
- MVP flow, part 1 (catalog seeding): Admin enters a purl list or SBOM; the system derives purls from the SBOM. For each purl, the system finds (or asks an LLM to find) the library's GitHub link, pulls tags from GitHub as categorization, then asks an LLM whether that categorization is correct — if not, the LLM adds its own categories.
- MVP flow, part 2 (organization analysis): Architect has a "My organization" view; they can mark which library in a category is strategic/standard. They repeatedly upload a project's SBOM and get a report of that project's libraries by category, flagging redundancy (two libraries in the same category within one project). An organization-level report shows redundancy across all uploaded projects so the architect can pick strategic libraries.

### Secondary
- Where possible, the system fetches current health metrics for a library from the web, as an enrichment on top of the categorization.

### Guardrails
- SBOM data uploaded to the system contains no private/customer data — only library coordinates and their roles.
- Direct vs. transitive role is shown when the SBOM input supports it (not yet confirmed whether the chosen SBOM source/format carries this distinction — see Open Questions).

## User Stories

### US-01: Architect sees library redundancy in a project

- **Given** a logged-in architect, a classification base already seeded (FR-001–FR-004), and the organization's strategic libraries marked per category/ecosystem (FR-007)
- **When** the architect uploads a project's SBOM
- **Then** they get a category report with direct dependencies flagged, each marked strategic/non-strategic, and non-strategic direct libraries clearly flagged as candidates to retire (FR-005, FR-008)

#### Acceptance Criteria
- A non-strategic direct library in a category is clearly flagged as a candidate to retire.

## Functional Requirements

### Catalog & categorization (Admin)
- FR-001: Admin can upload an SBOM and trigger the library categorization process. Priority: must-have
  > Socrates: No counter-argument; stands as written.
- FR-002: Admin can trigger the categorization process by providing a CSV list of purls, as an alternative to uploading an SBOM. Priority: must-have
  > Socrates: No counter-argument; stands as written.
- FR-003: Admin can view the list of categorized libraries and filter it. Priority: must-have
  > Socrates: No counter-argument; stands as written — needed to verify what the automated categorization produced.
- FR-004: Admin can add their own categories to a library. Priority: must-have
  > Socrates: No counter-argument; stands as written — needed when automated categorization (GitHub tags / LLM) is wrong.

### Organization analysis (Architect)
- FR-005: Architect can upload a project's SBOM and receive a report of all its libraries by category, with direct dependencies highlighted, optionally showing health metrics, and showing whether a library is the organization's marked strategic choice for its category. Priority: must-have
  > Socrates: No counter-argument; stands as written.
- FR-006: Architect can view the organization's library list, filterable by strategic/standard status, with direct dependencies emphasized. Priority: must-have
  > Socrates: No counter-argument; stands as written — needed to assess the organization as a whole, not just one project.
- FR-007: Architect can mark a direct library within a category as strategic, scoped per technology/ecosystem (e.g. one strategic choice for Maven/Java, a separate one for npm/JS, since no single library spans both). Normally one strategic library per category per ecosystem, but up to two are allowed. Direct libraries in a category/ecosystem not marked strategic are implicitly candidates to retire. Priority: must-have
  > Socrates: Counter-argument considered: what happens when no library is marked strategic in a category with multiple direct libraries? Resolution: no counter-argument against the FR itself; the category simply has no strategic library yet — the system does not force a choice.
- FR-008: User can view a single project and see which of its direct dependencies are non-strategic (i.e. candidates to retire for that project). Priority: must-have
  > Socrates: No counter-argument; stands as written — this is the concrete per-team action: what to fix in my project.

## Non-Functional Requirements

- Shelfmark classifies and reports on libraries from the Java/Maven ecosystem; Java/Maven support is must-have for the MVP.
- Python ecosystem support is a nice-to-have for the MVP — it may slip past the MVP without harming the primary flow.

## Business Logic

For each library, Shelfmark determines a capability classification from its GitHub repository data, with an LLM making the final categorization decision.

Input: a purl (package URL) identifying the library. Output: for each purl, one or more capability categories drawn from a curated taxonomy. The admin encounters this rule when seeding the classification base (FR-001, FR-002) and when reviewing or correcting its output (FR-003, FR-004).

Sourcing detail (cascading): 1) the system looks up the library's GitHub repository and pulls its topic tags as a first-pass categorization; 2) an LLM is asked whether that categorization is correct, and adds or corrects categories where it is not; 3) where neither source yields a confident category, the admin enters one manually (a direct database entry is sufficient for the MVP — no dedicated UI required for this fallback path).

## Access Control

Login required (email + password); accounts/roles provisioned via instance configuration. Three roles:

| Role | Capabilities |
|---|---|
| Admin | Manages users/roles (via configuration); populates and corrects the classification knowledge base (library catalog + capability categories); uploads project SBOMs |
| Architect | Uploads project SBOMs for analysis; sets and changes recommendations (standard / to retire) with justification |
| Viewer | Views reports and current recommendations; no edit capabilities |

Accounts are now managed through an in-app admin page backed by a database table, with the original `.env` variables retained only as the one-time bootstrap source for a fresh deployment.

## Non-Goals

- No ecosystems other than Java/Maven (and optionally Python) — npm, Go, Ruby, .NET and others are out of MVP scope.
- No SBOM generation and no CVE scanning — Shelfmark consumes SBOMs produced by existing tools and answers "what is redundant", not "what is vulnerable".
- No automatic standard enforcement (CI integration, PR blocking) — Shelfmark recommends strategic libraries; it does not block.
- No central knowledge base shared across organizations — the MVP is a single organization instance; a shared knowledge base, if ever built, is a future consideration.

## Open Questions

1. **Does the chosen SBOM format/source carry the direct-vs-transitive dependency role?** — Owner: user. Affects whether the direct/transitive guardrail can be met as stated.
