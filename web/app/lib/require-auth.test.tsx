// Risk: role-gate (test-plan #3/#5, UI side) — a page gated by RequireAuth
// must show only "Loading…" until /api/me answers, send a visitor without a
// session to /login, send a signed-in user of the wrong role to /dashboard,
// and never mount the protected content for either.
import { screen } from "@testing-library/react";
import { describe, expect, test } from "vitest";

import { renderRoute } from "../test/render-route";
import { allowConsoleError } from "../test/setup";
import { RequireAuth } from "./require-auth";

function AdminPage() {
  return (
    <RequireAuth role="admin">
      <h1>Protected content</h1>
    </RequireAuth>
  );
}

function ArchitectOrViewerPage() {
  return (
    <RequireAuth role={["architect", "viewer"]}>
      <h1>Protected content</h1>
    </RequireAuth>
  );
}

describe("RequireAuth", () => {
  test("shows Loading… and no protected content while /api/me is in flight", () => {
    renderRoute(AdminPage, { role: "admin", fetch: { "/api/me": "pending" } });

    expect(screen.getByText("Loading…")).toBeTruthy();
    expect(screen.queryByText("Protected content")).toBeNull();
  });

  test("sends a visitor without a session to /login", async () => {
    renderRoute(AdminPage, { role: null });

    expect(await screen.findByRole("heading", { name: "Login page" })).toBeTruthy();
    expect(screen.queryByText("Protected content")).toBeNull();
  });

  test("sends a signed-in user of the wrong role to /dashboard", async () => {
    renderRoute(AdminPage, { role: "architect" });

    expect(await screen.findByRole("heading", { name: "Dashboard page" })).toBeTruthy();
    expect(screen.queryByText("Protected content")).toBeNull();
  });

  test("renders the content for the allowed role", async () => {
    renderRoute(AdminPage, { role: "admin" });

    expect(await screen.findByRole("heading", { name: "Protected content" })).toBeTruthy();
  });

  test.each(["architect", "viewer"] as const)(
    "a role list admits %s",
    async (role) => {
      renderRoute(ArchitectOrViewerPage, { role });

      expect(await screen.findByRole("heading", { name: "Protected content" })).toBeTruthy();
    },
  );

  test("a role list still rejects a role not in it", async () => {
    renderRoute(ArchitectOrViewerPage, { role: "admin" });

    expect(await screen.findByRole("heading", { name: "Dashboard page" })).toBeTruthy();
    expect(screen.queryByText("Protected content")).toBeNull();
  });
});

describe("AuthProvider", () => {
  // Characterization, not a spec: research.md open question 1 left open
  // whether a failing /api/me should show an error instead. Today it reads
  // as "logged out". Changing that is a product decision and an explicit
  // edit of this test.
  test("AuthProvider_ApiMe500_TreatedAsLoggedOut_Characterization", async () => {
    // The provider logs the failed /api/me instead of leaving it unhandled.
    allowConsoleError();
    renderRoute(AdminPage, {
      role: "admin",
      fetch: { "/api/me": { status: 500, body: { error: "boom" } } },
    });

    expect(await screen.findByRole("heading", { name: "Login page" })).toBeTruthy();
    expect(screen.queryByText("Protected content")).toBeNull();
  });
});
