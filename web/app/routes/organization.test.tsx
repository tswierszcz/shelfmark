// Risk: /organization lies about its list state or crashes on a nil slice
// (test-plan #5). The table must show "Loading…" while in flight, the empty
// row only once loaded empty, the error (not the empty row) on failure, and
// survive `null` at the top level or in a library's categories/strategic.
// Only an architect may mark a category strategic.
import { screen, within } from "@testing-library/react";
import { describe, expect, test } from "vitest";

import { renderRoute } from "../test/render-route";
import type { Me } from "../lib/auth";
import type { StubResponse } from "../test/fetch-stub";
import Organization from "./organization";

const EMPTY_ROW = "No libraries match these filters.";

const LIBRARY = {
  id: "lib-1",
  purl: "pkg:npm/left-pad@1.3.0",
  name: "left-pad",
  ecosystem: "npm",
  github_repo: null,
  status: "categorized",
  upload_batch_id: "batch-0",
  is_direct: true,
  categories: ["String utils"],
  strategic: [],
};

function renderOrganization(libraries: StubResponse, role: Me["role"] = "viewer") {
  renderRoute(Organization, {
    path: "/organization",
    role,
    fetch: {
      "GET /api/catalog/categories": { body: [{ id: "cat-1", name: "String utils" }] },
      "GET /api/organization/libraries": libraries,
    },
  });
}

describe("/organization list states", () => {
  test("pending shows Loading… and no empty row", async () => {
    renderOrganization("pending");

    await screen.findByRole("heading", { name: "Libraries" });
    expect(screen.getByText("Loading…")).toBeTruthy();
    expect(screen.queryByText(EMPTY_ROW)).toBeNull();
  });

  test("[] shows the empty row", async () => {
    renderOrganization({ body: [] });

    expect(await screen.findByText(EMPTY_ROW)).toBeTruthy();
    expect(screen.queryByText("Loading…")).toBeNull();
  });

  test("a top-level null body reads as empty, no crash", async () => {
    renderOrganization({ body: null });

    expect(await screen.findByText(EMPTY_ROW)).toBeTruthy();
  });

  test("500 shows the error and no empty row", async () => {
    renderOrganization({ status: 500, body: { error: "boom" } });

    expect(await screen.findByText("Failed to load libraries.")).toBeTruthy();
    expect(screen.queryByText(EMPTY_ROW)).toBeNull();
    expect(screen.queryByText("Loading…")).toBeNull();
  });

  test("a library with null categories and strategic still renders", async () => {
    renderOrganization({ body: [{ ...LIBRARY, categories: null, strategic: null }] });

    const row = (await screen.findByText(LIBRARY.purl)).closest("tr")!;
    // github_repo and categories both render "—".
    expect(within(row).getAllByText("—")).toHaveLength(2);
  });

  test("a library with categories but null strategic still renders", async () => {
    renderOrganization({ body: [{ ...LIBRARY, strategic: null }] });

    const row = (await screen.findByText(LIBRARY.purl)).closest("tr")!;
    expect(within(row).getByText("String utils")).toBeTruthy();
    expect(within(row).queryByText("Strategic")).toBeNull();
  });

  test("data renders rows", async () => {
    renderOrganization({
      body: [
        LIBRARY,
        { ...LIBRARY, id: "lib-2", purl: "pkg:npm/is-odd@3.0.1", categories: [] },
      ],
    });

    expect(await screen.findByText(LIBRARY.purl)).toBeTruthy();
    expect(screen.getByText("pkg:npm/is-odd@3.0.1")).toBeTruthy();
    expect(screen.queryByText(EMPTY_ROW)).toBeNull();
  });
});

describe("/organization strategic marking by role", () => {
  test("a viewer gets no Mark strategic button", async () => {
    renderOrganization({ body: [LIBRARY] }, "viewer");

    const row = (await screen.findByText(LIBRARY.purl)).closest("tr")!;
    // Wait for the taxonomy, which is what would enable the button.
    await screen.findByRole("option", { name: "String utils" });
    expect(within(row).getByText("String utils")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Mark strategic" })).toBeNull();
  });

  test("an architect gets a Mark strategic button", async () => {
    renderOrganization({ body: [LIBRARY] }, "architect");

    expect(await screen.findByRole("button", { name: "Mark strategic" })).toBeTruthy();
  });
});
