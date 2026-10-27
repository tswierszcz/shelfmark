# Settle an empty upload batch — Plan Brief

> Full plan: `context/changes/empty-batch-status/plan.md`

## What & Why

Re-uploading an SBOM whose libraries are all known creates an empty batch. `/redundancy` then polls for 60 s and claims "— still processing" although nothing runs. Found in the production smoke test of the previous change.

## Key Decisions Made

| Decision | Choice | Why (1 sentence) |
| --- | --- | --- |
| Wording | `— done: no new libraries to categorize` | Says why nothing was processed (user choice A) |
| Where | Frontend only | Libraries are written before the upload responds, so `{}` on the first poll is unambiguous |

## Phases at a Glance

| Phase | What it delivers | Key risk |
| --- | --- | --- |
| 1. Empty batch settles as done | Immediate, honest batch line; Go + E2E tests | Treating `{}` as settled must not hide a real batch (ruled out: rows exist before the response) |

**Estimated effort:** under 1 session.
