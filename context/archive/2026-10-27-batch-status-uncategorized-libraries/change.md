---
change_id: batch-status-uncategorized-libraries
title: Surface failed and uncategorized libraries in batch status and project report
status: archived
created: 2026-10-27
updated: 2026-10-27
archived_at: 2026-10-27T18:21:54Z
---

## Notes

Silent failure found while writing the sbom-upload-report-smoke E2E (test-plan risk #1, "reports success on an incomplete result"): when categorization yields no categories for every uploaded library, /redundancy shows "Batch <id> — done" and the report says "No libraries recorded for this project." although the libraries were stored. Batch status must reflect libraries that failed or need review; the project report must list every uploaded library, showing uncategorized ones as such.
