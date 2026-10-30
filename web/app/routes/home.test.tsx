// Risk: the public landing (test-plan #5, landing-polish) — an anonymous
// visitor must see the landing at once with the shelfmark.online contact
// address and no "In development" badge, and a signed-in user of any role
// must be sent on to /dashboard. A failing /api/me must not redirect.
import { act, screen } from "@testing-library/react";
import { describe, expect, test } from "vitest";

import { renderRoute } from "../test/render-route";
import { allowConsoleError } from "../test/setup";
import Home from "./home";

const HEADING = /Scanners tell you what.s vulnerable/;

// settle lets the /api/me answer and any redirect effect run, so a test that
// expects to stay on the landing does not pass just by asserting too early.
async function settle() {
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
}

describe("Home", () => {
  test("shows the landing to a visitor without a session", async () => {
    renderRoute(Home, { path: "/", role: null });
    await settle();

    expect(screen.getByRole("heading", { level: 1, name: HEADING })).toBeTruthy();
    expect(screen.getByRole("link", { name: "Zaloguj" }).getAttribute("href")).toBe("/login");
    expect(screen.getByRole("link", { name: "Get in touch" }).getAttribute("href")).toBe(
      "mailto:hello@shelfmark.online?subject=Shelfmark",
    );
    expect(
      screen.getByRole("link", { name: "hello@shelfmark.online" }).getAttribute("href"),
    ).toBe("mailto:hello@shelfmark.online");
    expect(screen.queryByText(/shelfmark\.dev/)).toBeNull();
    expect(screen.queryByText("In development")).toBeNull();
  });

  test("shows the landing at once while /api/me is in flight", () => {
    renderRoute(Home, { path: "/", role: null, fetch: { "/api/me": "pending" } });

    expect(screen.getByRole("heading", { level: 1, name: HEADING })).toBeTruthy();
    expect(screen.queryByText("Loading…")).toBeNull();
  });

  test.each(["admin", "architect", "viewer"] as const)(
    "sends a signed-in %s to /dashboard",
    async (role) => {
      renderRoute(Home, { path: "/", role });

      expect(await screen.findByRole("heading", { name: "Dashboard page" })).toBeTruthy();
      expect(screen.queryByRole("heading", { level: 1, name: HEADING })).toBeNull();
    },
  );

  test("stays on the landing when /api/me fails", async () => {
    // The provider logs the failed /api/me; it reads as "no session".
    allowConsoleError(/Failed to load the current user/);
    renderRoute(Home, {
      path: "/",
      role: null,
      fetch: { "/api/me": { status: 500, body: { error: "boom" } } },
    });
    await settle();

    expect(screen.getByRole("heading", { level: 1, name: HEADING })).toBeTruthy();
    expect(screen.queryByRole("heading", { name: "Dashboard page" })).toBeNull();
  });
});
