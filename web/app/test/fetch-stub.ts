import { vi } from "vitest";

// A stubbed answer for one request. An object is sent as a real Response
// whose body is the raw JSON of `body` (so `body: null` arrives as the
// literal `null`, exactly like a Go nil slice); omit `body` for an empty
// body. "pending" never settles (loading state); "network-error" rejects
// the way fetch does when the server is unreachable.
export type StubResponse =
  | { status?: number; body?: unknown }
  | "pending"
  | "network-error";

// Status for a request no stub matched. It is answered, never thrown: a throw
// would land in the route's own catch and could make an error-state test pass
// because of a typo in a stubbed URL. setup.ts fails the test instead.
export const UNMATCHED_STATUS = 599;

let unmatched: string[] = [];

// stubFetch replaces global fetch for the current test. Keys are
// "METHOD /path?query", "METHOD /path", "/path?query" or "/path", tried in
// that order, so a key without a method or query matches any.
export function stubFetch(routes: Record<string, StubResponse>): void {
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const raw = input instanceof Request ? input.url : input.toString();
      const url = new URL(raw, "http://localhost");
      const method = (
        init?.method ?? (input instanceof Request ? input.method : "GET")
      ).toUpperCase();

      const keys = [
        `${method} ${url.pathname}${url.search}`,
        `${method} ${url.pathname}`,
        `${url.pathname}${url.search}`,
        url.pathname,
      ];
      const key = keys.find((candidate) => candidate in routes);
      if (key === undefined) {
        unmatched.push(`${method} ${url.pathname}${url.search}`);
        return new Response(null, { status: UNMATCHED_STATUS });
      }

      const stub = routes[key];
      if (stub === "pending") {
        return new Promise<Response>(() => {});
      }
      if (stub === "network-error") {
        throw new TypeError("Failed to fetch");
      }
      const body = "body" in stub ? JSON.stringify(stub.body) : null;
      return new Response(body, {
        status: stub.status ?? 200,
        headers: body === null ? undefined : { "Content-Type": "application/json" },
      });
    }),
  );
}

// takeUnmatchedRequests returns and clears the requests no stub matched.
export function takeUnmatchedRequests(): string[] {
  const result = unmatched;
  unmatched = [];
  return result;
}
