// Risk: the dashboard (test-plan #5, dashboard-stats) — every role must see
// the statistics with distinct loading/error/empty/data states, only the
// matching role must get its call to action, and a malformed body with null
// arrays must not crash the page.
import { screen, within } from "@testing-library/react";
import { describe, expect, test } from "vitest";

import type { DashboardStats } from "../lib/dashboard";
import { renderRoute } from "../test/render-route";
import Dashboard from "./dashboard";

function statsBody(overrides: Partial<DashboardStats> = {}): DashboardStats {
  return {
    libraries: {
      total: 10,
      by_status: { pending: 1, processing: 1, categorized: 5, needs_manual: 2, failed: 1 },
      by_ecosystem: [{ ecosystem: "maven", count: 10 }],
    },
    categories: { total: 14, in_use: 6 },
    projects: 4,
    strategic_marks: 3,
    hotspots: [
      { category_id: "c1", category_name: "logging", library_count: 3, reviewed: false },
      { category_id: "c2", category_name: "json-processing", library_count: 2, reviewed: true },
    ],
    unreviewed_hotspots: 2,
    ...overrides,
  };
}

const NEEDS_REVIEW = /libraries need manual review/;
const NO_DECISION = /redundant categories without a strategic decision/;

describe("Dashboard", () => {
  test("shows a loading state while /api/dashboard is in flight", async () => {
    renderRoute(Dashboard, {
      path: "/dashboard",
      role: "viewer",
      fetch: { "/api/dashboard": "pending" },
    });

    expect(await screen.findByText("Loading statistics…")).toBeTruthy();
  });

  test("shows the error and no numbers when /api/dashboard fails", async () => {
    renderRoute(Dashboard, {
      path: "/dashboard",
      role: "architect",
      fetch: { "/api/dashboard": { status: 500, body: { error: "failed to load dashboard statistics" } } },
    });

    expect(await screen.findByText("failed to load dashboard statistics")).toBeTruthy();
    expect(screen.queryByText("Libraries")).toBeNull();
    expect(screen.queryByText("Loading statistics…")).toBeNull();
  });

  test("offers the admin a catalog upload when the catalog is empty", async () => {
    renderRoute(Dashboard, {
      path: "/dashboard",
      role: "admin",
      fetch: { "/api/dashboard": { body: statsBody({ libraries: { total: 0, by_status: {}, by_ecosystem: [] } }) } },
    });

    const link = await screen.findByRole("link", { name: "Upload a catalog to get started" });
    expect(link.getAttribute("href")).toBe("/catalog");
  });

  test.each(["architect", "viewer"] as const)("tells a %s the catalog is empty", async (role) => {
    renderRoute(Dashboard, {
      path: "/dashboard",
      role,
      fetch: { "/api/dashboard": { body: statsBody({ libraries: { total: 0, by_status: {}, by_ecosystem: [] } }) } },
    });

    expect(await screen.findByText("The catalog is empty.")).toBeTruthy();
    expect(screen.queryByRole("link", { name: /catalog/i })).toBeNull();
  });

  test("shows the statistics with hotspots in payload order", async () => {
    renderRoute(Dashboard, {
      path: "/dashboard",
      role: "viewer",
      fetch: { "/api/dashboard": { body: statsBody() } },
    });

    expect(await screen.findByText("Categories in use")).toBeTruthy();
    expect(screen.getByText("of 14 in the taxonomy")).toBeTruthy();
    expect(screen.getByText("Processing").nextSibling?.textContent).toBe("2");

    const hotspots = screen.getByRole("heading", { name: "Redundancy hotspots" }).parentElement!;
    const rows = within(hotspots).getAllByRole("listitem");
    expect(rows.map((row) => row.textContent)).toEqual([
      "logging3 librariesNo decision",
      "json-processing2 librariesReviewed",
    ]);
    expect(screen.getByRole("heading", { level: 1, name: "Dashboard" })).toBeTruthy();
  });

  test("admin sees the manual-review link only when libraries need review", async () => {
    renderRoute(Dashboard, {
      path: "/dashboard",
      role: "admin",
      fetch: { "/api/dashboard": { body: statsBody() } },
    });

    const link = await screen.findByRole("link", { name: "2 libraries need manual review" });
    expect(link.getAttribute("href")).toBe("/catalog");
    expect(screen.queryByRole("link", { name: NO_DECISION })).toBeNull();
  });

  test("admin sees no manual-review link when nothing needs review", async () => {
    const body = statsBody();
    body.libraries.by_status.needs_manual = 0;
    renderRoute(Dashboard, { path: "/dashboard", role: "admin", fetch: { "/api/dashboard": { body } } });

    expect(await screen.findByText("Categories in use")).toBeTruthy();
    expect(screen.queryByRole("link", { name: NEEDS_REVIEW })).toBeNull();
  });

  test("architect sees the no-decision link only when hotspots are unreviewed", async () => {
    renderRoute(Dashboard, {
      path: "/dashboard",
      role: "architect",
      fetch: { "/api/dashboard": { body: statsBody() } },
    });

    const link = await screen.findByRole("link", {
      name: "2 redundant categories without a strategic decision",
    });
    expect(link.getAttribute("href")).toBe("/organization");
    expect(screen.queryByRole("link", { name: NEEDS_REVIEW })).toBeNull();
    expect(screen.queryByRole("link", { name: "Catalog" })).toBeNull();
  });

  test("architect sees no no-decision link when every hotspot is reviewed", async () => {
    renderRoute(Dashboard, {
      path: "/dashboard",
      role: "architect",
      fetch: { "/api/dashboard": { body: statsBody({ unreviewed_hotspots: 0 }) } },
    });

    expect(await screen.findByText("Categories in use")).toBeTruthy();
    expect(screen.queryByRole("link", { name: NO_DECISION })).toBeNull();
  });

  test("viewer sees no call to action", async () => {
    renderRoute(Dashboard, {
      path: "/dashboard",
      role: "viewer",
      fetch: { "/api/dashboard": { body: statsBody() } },
    });

    expect(await screen.findByText("Categories in use")).toBeTruthy();
    expect(screen.queryByRole("link", { name: NEEDS_REVIEW })).toBeNull();
    expect(screen.queryByRole("link", { name: NO_DECISION })).toBeNull();
  });

  test("renders null hotspots and ecosystems without crashing", async () => {
    const body = statsBody() as unknown as Record<string, unknown>;
    body.hotspots = null;
    (body.libraries as Record<string, unknown>).by_ecosystem = null;
    renderRoute(Dashboard, { path: "/dashboard", role: "viewer", fetch: { "/api/dashboard": { body } } });

    expect(await screen.findByText("No redundancy found yet.")).toBeTruthy();
    expect(screen.getByRole("heading", { name: "Ecosystems" })).toBeTruthy();
  });
});
