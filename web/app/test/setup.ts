import { cleanup } from "@testing-library/react";
import { afterEach, beforeEach, vi } from "vitest";

import { takeUnmatchedRequests } from "./fetch-stub";

let consoleErrors: string[] = [];
let allowConsoleErrors = false;

// allowConsoleError lets the current test log console.error without failing.
// Use it only in a test that expects the error (and say why there).
export function allowConsoleError(): void {
  allowConsoleErrors = true;
}

beforeEach(() => {
  consoleErrors = [];
  allowConsoleErrors = false;
  // React reports act() warnings and render errors through console.error;
  // any of them means the test exercised something it did not assert on.
  vi.spyOn(console, "error").mockImplementation((...args: unknown[]) => {
    consoleErrors.push(args.map(String).join(" "));
  });
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();

  const unmatched = takeUnmatchedRequests();
  if (unmatched.length > 0) {
    throw new Error(`Unstubbed fetch request(s):\n  ${unmatched.join("\n  ")}`);
  }
  if (consoleErrors.length > 0 && !allowConsoleErrors) {
    throw new Error(`console.error was called:\n  ${consoleErrors.join("\n  ")}`);
  }
});
