import { render } from "@testing-library/react";
import type { ComponentType } from "react";
import { createRoutesStub } from "react-router";

import { AuthProvider, type Me } from "../lib/auth";
import { stubFetch, type StubResponse } from "./fetch-stub";

type RenderRouteOptions = {
  // Path the route is mounted at and the test starts on.
  path?: string;
  // Signed-in role answered by /api/me; null answers 401 (no session).
  // Override "/api/me" in `fetch` for anything else (pending, 500).
  role: Me["role"] | null;
  fetch?: Record<string, StubResponse>;
};

// renderRoute renders a route component inside the real AuthProvider and a
// router stub, with fetch answered from a table of raw responses. /login and
// /dashboard render identifiable headings so redirects can be asserted.
export function renderRoute(
  Component: ComponentType,
  { path = "/page", role, fetch = {} }: RenderRouteOptions,
) {
  stubFetch({
    "/api/me":
      role === null
        ? { status: 401 }
        : { body: { email: `${role}@example.test`, role } },
    ...fetch,
  });

  const Stub = createRoutesStub([
    { path, Component },
    { path: "/login", Component: () => <h1>Login page</h1> },
    { path: "/dashboard", Component: () => <h1>Dashboard page</h1> },
  ]);

  return render(
    <AuthProvider>
      <Stub initialEntries={[path]} />
    </AuthProvider>,
  );
}
