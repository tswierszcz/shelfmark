---
project: "Shelfmark"
version: 1
status: draft
created: 2026-10-15
updated: 2026-10-16
prd_version: 1
main_goal: speed
top_blocker: time
milestone_id: first-usable-redundancy-report
milestone_seq: 1
milestone_status: open
---

# Roadmap: Shelfmark

> Derived from context/foundation/prd.md (v1) + auto-researched codebase baseline.
> Edit-in-place; the next milestone continues this file. Archive only when an independent roadmap replaces it.
> Slices below are listed in dependency order. The "At a glance" table is the index.

## Milestone

**M-1: First usable redundancy report** — Status: open

- **Intent:** Prove the core value loop — a seeded, categorized library catalog plus one real project SBOM produces a redundancy report an architect can act on.
- **Source materials:** `context/foundation/prd.md` (v1)
- **Done when:** every F-NN and S-NN below is `done`.
- **Scope anchors:** FR-001–FR-008, US-01, Access Control section.

## Vision recap

In organizations with dozens to hundreds of repositories, managing dependencies — version bumps, CVE tracking, upstream support, health research — consumes enormous ongoing effort, because the number of libraries in use is itself the attack surface and nobody answers "what is redundant," only "what is vulnerable." Shelfmark classifies libraries by capability and confronts that classification against an organization's own SBOMs to surface functional redundancy.

## North star

**S-03: Architect sees library redundancy in a project** — the smallest end-to-end flow whose successful delivery proves the core product hypothesis (that capability classification plus an organization's own dependency data can surface actionable redundancy) — placed as early as its Prerequisites allow, since everything else only matters if this works.

## At a glance

| ID   | Change ID                     | Outcome (user can …)                                                          | Prerequisites | PRD refs                       | Status   |
| ---- | ------------------------------ | ------------------------------------------------------------------------------ | -------------- | ------------------------------- | -------- |
| F-01 | minimal-auth-role-scaffold     | (foundation) users log in and actions are gated by role                       | —              | Access Control section          | done     |
| F-02 | data-layer-connectivity        | (foundation) the app can persist and migrate structured data                  | —              | FR-001, FR-002                  | done     |
| S-01 | seed-classification-catalog    | Admin can seed and categorize the library catalog                             | F-01, F-02     | FR-001, FR-002, FR-003, FR-004  | in-progress |
| S-02 | mark-strategic-libraries       | Architect can view the org's libraries and mark a direct library as strategic | S-01, F-01     | FR-006, FR-007                  | proposed |
| S-03 | project-redundancy-report      | Architect can upload a project's SBOM and see a redundancy report             | S-01, S-02, F-01 | FR-005, FR-008, US-01          | proposed |

## Streams

Navigation aid — groups items that share a Prerequisites chain. Canonical ordering still lives in the dependency graph below; this table is the proposed reading order across parallel tracks.

| Stream | Theme                        | Chain                          | Note                                                                  |
| ------ | ----------------------------- | ------------------------------- | ---------------------------------------------------------------------- |
| A      | Access & redundancy pipeline  | `F-01` → `S-01` → `S-02` → `S-03` | Main critical path to the north star; speed goal keeps it linear.    |
| B      | Data layer enablement         | `F-02`                          | Joins Stream A at `S-01`, which is the first consumer of persistence. |

## Baseline

What's already in place in the codebase as of `2026-10-15` (auto-researched + user-confirmed).
Foundations below assume these are present and do NOT re-scaffold them.

- **Frontend:** partial — a scaffolded SPA exists with routing wired up (`web/app/routes.ts`, `web/app/routes/home.tsx`), but only a template demo page; no project-specific views yet.
- **Backend / API:** partial — a server exists (`backend/cmd/shelfmark/main.go`) serving a health check and the built frontend; no application routes yet.
- **Data:** absent — no database driver, no migration tooling, no schema files.
- **Auth:** absent — no login or role-gating code.
- **Deploy / infra:** partial — a container build definition exists (`Dockerfile`); no deployment config and no CI workflow yet.
- **Observability:** absent — only ad hoc logging; no structured logging, tracing, or metrics.

## Foundations

### F-01: Minimal auth & role scaffold

- **Outcome:** (foundation) users can log in and every subsequent action can be checked against one of three roles.
- **Change ID:** minimal-auth-role-scaffold
- **PRD refs:** Access Control section
- **Unlocks:** S-01 (admin-only catalog actions), S-02 (architect-only marking action), S-03 (role-gated views)
- **Prerequisites:** —
- **Parallel with:** F-02
- **Blockers:** —
- **Unknowns:** —
- **Risk:** Sequenced first because every downstream slice gates an action by role (admin vs. architect vs. viewer); retrofitting checks into already-working slices is more error-prone than building the gate once up front.
- **Status:** done

### F-02: Data layer connectivity

- **Outcome:** (foundation) the application can connect to a structured data store and run migrations, so each slice can persist its own tables instead of improvising in-memory state.
- **Change ID:** data-layer-connectivity
- **PRD refs:** FR-001, FR-002
- **Unlocks:** S-01 (first slice that needs to persist a seeded catalog)
- **Prerequisites:** —
- **Parallel with:** F-01
- **Blockers:** —
- **Unknowns:** —
- **Risk:** Chosen as its own foundation (not a hidden step inside S-01) because the user's main investment choice for this milestone is the data layer — every slice from here on persists something, and getting connectivity and migrations wrong once is cheaper than fixing it per slice.
- **Status:** done

## Slices

### S-01: Admin seeds and categorizes the library catalog

- **Outcome:** Admin can populate the library catalog (by purl list or SBOM) and have each library automatically categorized by capability, with manual review and correction available.
- **Change ID:** seed-classification-catalog
- **PRD refs:** FR-001, FR-002, FR-003, FR-004
- **Prerequisites:** F-01, F-02
- **Parallel with:** —
- **Blockers:** —
- **Unknowns:**
  - Does the team already have access/credentials for the code-hosting lookup and the categorization service the classification mechanism depends on? — Owner: user. Block: no (can be arranged alongside early implementation, not before planning starts).
- **Risk:** Must come first — nothing downstream has any data to show without a seeded, categorized catalog. This is also where the categorization mechanism (the most novel part of the product) gets proven out in isolation, before anything else depends on it.
- **Status:** in-progress

### S-02: Architect marks strategic libraries

- **Outcome:** Architect can view the organization's library list and mark a direct library within a category as strategic (scoped per ecosystem).
- **Change ID:** mark-strategic-libraries
- **PRD refs:** FR-006, FR-007
- **Prerequisites:** S-01, F-01
- **Parallel with:** —
- **Blockers:** —
- **Unknowns:** —
- **Risk:** Strategic marking must exist before the redundancy report (S-03) can distinguish strategic from non-strategic libraries; building the report first would mean faking this state.
- **Status:** proposed

### S-03: Architect sees library redundancy in a project

- **Outcome:** Architect can upload a project's SBOM and see a category report with direct dependencies flagged, strategic/non-strategic status shown, and non-strategic direct libraries flagged as candidates to retire.
- **Change ID:** project-redundancy-report
- **PRD refs:** FR-005, FR-008, US-01
- **Prerequisites:** S-01, S-02, F-01
- **Parallel with:** —
- **Blockers:** —
- **Unknowns:**
  - Does the chosen SBOM format/source carry the direct-vs-transitive dependency role? — Owner: user. Block: no (the guardrail already allows showing the distinction only when the input supports it; this slice can ship without it).
- **Risk:** This is the north star and the riskiest integration point — categorized catalog, strategic marks, and a real uploaded SBOM all come together here — sequenced last deliberately, since everything before it is validated by the time this runs.
- **Status:** proposed

## Backlog Handoff

| Roadmap ID | Change ID                  | Suggested issue title                                   | Ready for `/10x-plan` | Notes                                      |
| ---------- | --------------------------- | --------------------------------------------------------- | ---------------------- | ------------------------------------------- |
| F-01       | minimal-auth-role-scaffold  | Minimal auth & role scaffold (admin/architect/viewer)      | yes                    | Run `/10x-plan minimal-auth-role-scaffold`  |
| F-02       | data-layer-connectivity     | Data layer connectivity & migration tooling               | yes                    | Run `/10x-plan data-layer-connectivity`     |
| S-01       | seed-classification-catalog | Admin: seed and categorize the library catalog            | no                     | Prerequisites F-01, F-02 not yet done       |
| S-02       | mark-strategic-libraries    | Architect: mark strategic libraries per category          | no                     | Blocked on S-01                             |
| S-03       | project-redundancy-report   | Architect: project redundancy report (north star)         | no                     | Blocked on S-01, S-02                       |

This table is the clean handoff to Jira/Linear or any MCP-backed backlog.

## Open Roadmap Questions

(None cross-cutting at this time — the one open PRD question is tracked as a per-slice Unknown on S-03.)

## Parked

- **Health metrics enrichment** — Why parked: PRD Secondary success criterion, explicitly "nice to have"; deferred given the time constraint on this milestone.
- **Python ecosystem support** — Why parked: PRD NFR marks it nice-to-have; Java/Maven is the must-have ecosystem for this milestone.
- **SBOM generation and CVE scanning** — Why parked: PRD Non-Goal; Shelfmark consumes SBOMs from existing tools and answers "what is redundant," not "what is vulnerable."
- **Automatic standard enforcement (CI/PR blocking)** — Why parked: PRD Non-Goal; Shelfmark recommends, it does not block.
- **Central knowledge base shared across organizations** — Why parked: PRD Non-Goal; this milestone is a single-organization instance.

## Milestone History

(Empty — this is the first milestone.)

## Done

- **F-01: (foundation) users log in and actions are gated by role** — Archived 2026-10-16 → `context/archive/2026-10-16-minimal-auth-role-scaffold/`. Lesson: —.
- **F-02: (foundation) the app can persist and migrate structured data** — Archived 2026-10-16 → `context/archive/2026-10-16-data-layer-connectivity/`. Lesson: —.
