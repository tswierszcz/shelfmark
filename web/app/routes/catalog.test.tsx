// Risk: /catalog misreports a batch or swallows a failed action (test-plan
// #5). An upload whose purls are all known must settle at once with the
// "no new libraries" wording, a failed status poll must not read as "done",
// and a failed "Add category" must tell the admin.
import { fireEvent, screen, within } from "@testing-library/react";
import { describe, expect, test } from "vitest";

import type { StubResponse } from "../test/fetch-stub";
import { renderRoute } from "../test/render-route";
import Catalog from "./catalog";

const LIBRARY = {
  id: "lib-1",
  purl: "pkg:npm/left-pad@1.3.0",
  name: "left-pad",
  ecosystem: "npm",
  github_repo: null,
  status: "categorized",
  upload_batch_id: "batch-0",
  categories: [],
};

const BASE_FETCH = {
  "GET /api/catalog/categories": { body: [{ id: "cat-1", name: "String utils" }] },
  "GET /api/catalog/libraries": { body: [LIBRARY] },
  "POST /api/catalog/upload": { body: { batch_id: "batch-1", created: 0, skipped: [] } },
};

async function uploadFile() {
  const input = screen.getByLabelText("File");
  fireEvent.change(input, {
    target: { files: [new File(["{}"], "sbom.json", { type: "application/json" })] },
  });
  fireEvent.submit(input.closest("form")!);
}

describe("/catalog batch line", () => {
  test("an empty batch settles on the first poll as 'no new libraries'", async () => {
    renderRoute(Catalog, {
      path: "/catalog",
      role: "admin",
      fetch: { ...BASE_FETCH, "GET /api/catalog/batches/batch-1": { body: {} } },
    });
    await screen.findByText(LIBRARY.purl);

    await uploadFile();

    const line = await screen.findByText(/— done: no new libraries to categorize/);
    expect(line.className).not.toContain("text-destructive");
  });

  test("a failed status poll reads as unavailable, not done", async () => {
    renderRoute(Catalog, {
      path: "/catalog",
      role: "admin",
      fetch: { ...BASE_FETCH, "GET /api/catalog/batches/batch-1": { status: 500 } },
    });
    await screen.findByText(LIBRARY.purl);

    await uploadFile();

    const line = await screen.findByText(/— status unavailable/);
    expect(line.className).toContain("text-destructive");
    expect(screen.queryByText(/— done/)).toBeNull();
  });
});

describe("/catalog add category", () => {
  test("a failed attach shows an error", async () => {
    renderRoute(Catalog, {
      path: "/catalog",
      role: "admin",
      fetch: {
        ...BASE_FETCH,
        "POST /api/catalog/libraries/lib-1/categories": {
          status: 500,
          body: { error: "boom" },
        },
      },
    });
    const row = (await screen.findByText(LIBRARY.purl)).closest("tr")!;

    // Wait for the taxonomy so the row's picker has the option.
    await within(row).findByRole("option", { name: "String utils" });
    fireEvent.change(within(row).getByRole("combobox"), { target: { value: "cat-1" } });
    fireEvent.click(within(row).getByRole("button", { name: "Add" }));

    expect(await screen.findByText("Failed to attach category")).toBeTruthy();
    expect(within(row).getByRole("button", { name: "Add" })).toBeTruthy();
  });
});

// Risk: /catalog lies about its list state or crashes on a nil slice
// (test-plan #5).
const EMPTY_ROW = "No libraries match these filters.";

function renderCatalogList(libraries: StubResponse, extra: Record<string, StubResponse> = {}) {
  renderRoute(Catalog, {
    path: "/catalog",
    role: "admin",
    fetch: {
      "GET /api/catalog/categories": { body: [{ id: "cat-1", name: "String utils" }] },
      "GET /api/catalog/libraries": libraries,
      ...extra,
    },
  });
}

describe("/catalog list states", () => {
  test("pending shows Loading… and no empty row", async () => {
    renderCatalogList("pending");

    await screen.findByRole("heading", { name: "Libraries" });
    expect(screen.getByText("Loading…")).toBeTruthy();
    expect(screen.queryByText(EMPTY_ROW)).toBeNull();
  });

  test("[] shows the empty row", async () => {
    renderCatalogList({ body: [] });

    expect(await screen.findByText(EMPTY_ROW)).toBeTruthy();
    expect(screen.queryByText("Loading…")).toBeNull();
  });

  test("a top-level null body reads as empty, no crash", async () => {
    renderCatalogList({ body: null });

    expect(await screen.findByText(EMPTY_ROW)).toBeTruthy();
  });

  test("500 shows the error and no empty row", async () => {
    renderCatalogList({ status: 500, body: { error: "boom" } });

    expect(await screen.findByText("Failed to load libraries.")).toBeTruthy();
    expect(screen.queryByText(EMPTY_ROW)).toBeNull();
    expect(screen.queryByText("Loading…")).toBeNull();
  });

  test("a library with null categories still renders", async () => {
    renderCatalogList({ body: [{ ...LIBRARY, categories: null }] });

    const row = (await screen.findByText(LIBRARY.purl)).closest("tr")!;
    expect(within(row).getByRole("button", { name: "Add" })).toBeTruthy();
  });

  test("data renders rows", async () => {
    renderCatalogList({
      body: [LIBRARY, { ...LIBRARY, id: "lib-2", purl: "pkg:npm/is-odd@3.0.1" }],
    });

    expect(await screen.findByText(LIBRARY.purl)).toBeTruthy();
    expect(screen.getByText("pkg:npm/is-odd@3.0.1")).toBeTruthy();
    expect(screen.queryByText(EMPTY_ROW)).toBeNull();
  });

  test("a filter refetch keeps rows visible with no Loading… row", async () => {
    renderCatalogList(
      { body: [LIBRARY] },
      { "GET /api/catalog/libraries?status=failed": "pending" },
    );
    await screen.findByText(LIBRARY.purl);

    fireEvent.change(screen.getByLabelText("Status"), { target: { value: "failed" } });

    expect(screen.getByText(LIBRARY.purl)).toBeTruthy();
    expect(screen.queryByText("Loading…")).toBeNull();
  });

  test("a failed filter refetch clears the old rows", async () => {
    renderCatalogList(
      { body: [LIBRARY] },
      { "GET /api/catalog/libraries?status=failed": { status: 500 } },
    );
    await screen.findByText(LIBRARY.purl);

    fireEvent.change(screen.getByLabelText("Status"), { target: { value: "failed" } });

    expect(await screen.findByText("Failed to load libraries.")).toBeTruthy();
    expect(screen.queryByText(LIBRARY.purl)).toBeNull();
    expect(screen.queryByText(EMPTY_ROW)).toBeNull();
  });
});
