import type { BatchProgress } from "./catalog";

export const SETTLED_STATUSES = new Set(["categorized", "needs_manual", "failed"]);
export const POLL_INTERVAL_MS = 2000;
export const POLL_MAX_DURATION_MS = 60_000;

// An empty progress map is settled: the upload writes its libraries before
// responding, so a batch with no libraries means every purl was already in
// the catalog and nothing was enqueued.
export function isBatchSettled(progress: BatchProgress): boolean {
  return Object.keys(progress).every((status) => SETTLED_STATUSES.has(status));
}

export type PollOutcome = "settled" | "timeout" | "error";

// describeBatch turns the poll state into the batch line's suffix. "done"
// alone means every library was categorized; anything that needs review or
// failed is named, so an incomplete result never reads as plain success.
export function describeBatch(
  polling: boolean,
  outcome: PollOutcome | null,
  progress: BatchProgress | null,
): { text: string; problem: boolean } {
  if (polling || outcome === null) {
    return { text: " — processing…", problem: false };
  }
  if (outcome === "error") {
    return { text: " — status unavailable", problem: true };
  }
  if (outcome === "timeout") {
    return { text: " — still processing", problem: false };
  }

  if (progress && Object.keys(progress).length === 0) {
    return { text: " — done: no new libraries to categorize", problem: false };
  }

  const needReview = progress?.needs_manual ?? 0;
  const failed = progress?.failed ?? 0;
  const issues = [
    needReview > 0 ? `${needReview} ${needReview === 1 ? "needs" : "need"} review` : null,
    failed > 0 ? `${failed} failed` : null,
  ].filter((part) => part !== null);
  if (issues.length === 0) {
    return { text: " — done", problem: false };
  }
  return { text: ` — done: ${issues.join(", ")}`, problem: true };
}
