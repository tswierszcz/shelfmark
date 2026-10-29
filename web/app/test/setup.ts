import { cleanup } from "@testing-library/react";
import { afterEach, beforeEach, vi } from "vitest";

import { takeUnmatchedRequests } from "./fetch-stub";

let consoleErrors: string[] = [];
let allowedConsoleErrors: RegExp[] = [];

// allowConsoleError lets the current test log console.error calls matching
// `matcher` without failing; any other console.error still fails it. Use it
// only in a test that expects the error (and say why there).
export function allowConsoleError(matcher: RegExp): void {
  allowedConsoleErrors.push(matcher);
}

beforeEach(() => {
  consoleErrors = [];
  allowedConsoleErrors = [];
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
  const unexpected = consoleErrors.filter(
    (message) => !allowedConsoleErrors.some((matcher) => matcher.test(message)),
  );
  if (unexpected.length > 0) {
    throw new Error(`console.error was called:\n  ${unexpected.join("\n  ")}`);
  }
});
