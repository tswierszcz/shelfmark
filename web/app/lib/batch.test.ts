// Risk: a batch line reads as plain success while the batch is incomplete,
// timed out, or its status could not be read (test-plan #5; the /catalog
// copy of this logic drifted once). One row per outcome, explicit want.
import { describe, expect, test } from "vitest";

import type { BatchProgress } from "./catalog";
import { describeBatch, isBatchSettled, type PollOutcome } from "./batch";

describe("isBatchSettled", () => {
  const cases: { name: string; progress: BatchProgress; want: boolean }[] = [
    { name: "empty batch (every purl already known)", progress: {}, want: true },
    { name: "all categorized", progress: { categorized: 3 }, want: true },
    { name: "settled mix", progress: { categorized: 1, needs_manual: 1, failed: 1 }, want: true },
    { name: "still pending", progress: { categorized: 1, pending: 2 }, want: false },
    { name: "still processing", progress: { processing: 1 }, want: false },
  ];

  for (const tc of cases) {
    test(tc.name, () => {
      expect(isBatchSettled(tc.progress)).toBe(tc.want);
    });
  }
});

describe("describeBatch", () => {
  const cases: {
    name: string;
    polling: boolean;
    outcome: PollOutcome | null;
    progress: BatchProgress | null;
    want: { text: string; problem: boolean };
  }[] = [
    {
      name: "polling",
      polling: true,
      outcome: null,
      progress: { pending: 2 },
      want: { text: " — processing…", problem: false },
    },
    {
      name: "not started",
      polling: false,
      outcome: null,
      progress: null,
      want: { text: " — processing…", problem: false },
    },
    {
      name: "poll error",
      polling: false,
      outcome: "error",
      progress: { pending: 2 },
      want: { text: " — status unavailable", problem: true },
    },
    {
      name: "timeout",
      polling: false,
      outcome: "timeout",
      progress: { pending: 2 },
      want: { text: " — still processing", problem: false },
    },
    {
      name: "empty batch",
      polling: false,
      outcome: "settled",
      progress: {},
      want: { text: " — done: no new libraries to categorize", problem: false },
    },
    {
      name: "all categorized",
      polling: false,
      outcome: "settled",
      progress: { categorized: 3 },
      want: { text: " — done", problem: false },
    },
    {
      name: "one needs review",
      polling: false,
      outcome: "settled",
      progress: { categorized: 2, needs_manual: 1 },
      want: { text: " — done: 1 needs review", problem: true },
    },
    {
      name: "several need review",
      polling: false,
      outcome: "settled",
      progress: { needs_manual: 2 },
      want: { text: " — done: 2 need review", problem: true },
    },
    {
      name: "failed",
      polling: false,
      outcome: "settled",
      progress: { categorized: 1, failed: 1 },
      want: { text: " — done: 1 failed", problem: true },
    },
    {
      name: "review and failed",
      polling: false,
      outcome: "settled",
      progress: { needs_manual: 1, failed: 2 },
      want: { text: " — done: 1 needs review, 2 failed", problem: true },
    },
  ];

  for (const tc of cases) {
    test(tc.name, () => {
      expect(describeBatch(tc.polling, tc.outcome, tc.progress)).toEqual(tc.want);
    });
  }
});
