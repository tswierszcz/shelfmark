---
change_id: empty-batch-status
title: Settle an empty upload batch as done instead of still processing
status: implementing
created: 2026-10-27
updated: 2026-10-27
archived_at: null
---

## Notes

Found in the production smoke test after batch-status-uncategorized-libraries (archived 2026-10-27): re-uploading `docs/samples/smoke-sbom.cdx.json`, whose purls all already exist, creates no libraries under the new batch (CreateLibraries is ON CONFLICT DO NOTHING). Batch progress is `{}`, the UI polls 60 s and shows "— still processing" although nothing was processing. User-approved wording: "— done: no new libraries to categorize".
