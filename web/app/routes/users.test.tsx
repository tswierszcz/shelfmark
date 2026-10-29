// Risk: a list route lies about its state (test-plan #5). /users must show
// "Loading…" while the request is in flight, "No users yet." only once it
// has loaded empty (including a Go nil slice sent as `null`), and the error
// message, not the empty row, when the request fails.
import { screen } from "@testing-library/react";
import { describe, expect, test } from "vitest";

import { renderRoute } from "../test/render-route";
import Users from "./users";

const EMPTY_ROW = "No users yet.";

function renderUsers(response: Parameters<typeof renderRoute>[1]["fetch"]) {
  renderRoute(Users, { path: "/users", role: "admin", fetch: response });
}

describe("/users list states", () => {
  test("pending shows Loading… and no empty row", async () => {
    renderUsers({ "GET /api/users": "pending" });

    await screen.findByRole("heading", { name: "Existing users" });
    expect(screen.getByText("Loading…")).toBeTruthy();
    expect(screen.queryByText(EMPTY_ROW)).toBeNull();
  });

  test("[] shows the empty row", async () => {
    renderUsers({ "GET /api/users": { body: [] } });

    expect(await screen.findByText(EMPTY_ROW)).toBeTruthy();
    expect(screen.queryByText("Loading…")).toBeNull();
  });

  test("a top-level null body reads as empty, no crash", async () => {
    renderUsers({ "GET /api/users": { body: null } });

    expect(await screen.findByText(EMPTY_ROW)).toBeTruthy();
  });

  test("500 shows the error and no empty row", async () => {
    renderUsers({ "GET /api/users": { status: 500, body: { error: "boom" } } });

    expect(await screen.findByText("Failed to load users.")).toBeTruthy();
    expect(screen.queryByText(EMPTY_ROW)).toBeNull();
    expect(screen.queryByText("Loading…")).toBeNull();
  });

  test("data renders rows", async () => {
    renderUsers({
      "GET /api/users": {
        body: [
          { email: "a@example.test", role: "admin" },
          { email: "v@example.test", role: "viewer" },
        ],
      },
    });

    expect(await screen.findByText("a@example.test")).toBeTruthy();
    expect(screen.getByText("v@example.test")).toBeTruthy();
    expect(screen.queryByText(EMPTY_ROW)).toBeNull();
  });
});
