// Risk: the /redundancy report lies about its state, keeps a previous
// project's rows, or crashes on a nil slice (test-plan #5). The report must
// show "Loading…" while in flight, "No libraries recorded for this project."
// only on a successful empty answer, the error (and no old rows, no empty
// row) on failure, and survive `null` at the top level or in categories.
// Only an architect may upload or delete a project.
import { fireEvent, screen, within } from "@testing-library/react";
import { describe, expect, test } from "vitest";

import { renderRoute } from "../test/render-route";
import type { Me } from "../lib/auth";
import type { StubResponse } from "../test/fetch-stub";
import Redundancy from "./redundancy";

const EMPTY_ROW = "No libraries recorded for this project.";

const PROJECTS = [
  { id: "p-1", name: "billing", created_by: "a@example.test", created_at: "2026-10-01T00:00:00Z" },
  { id: "p-2", name: "search", created_by: "a@example.test", created_at: "2026-10-02T00:00:00Z" },
];

const LIBRARY = {
  id: "lib-1",
  purl: "pkg:maven/com.google.code.gson/gson@2.10.1",
  name: "gson",
  ecosystem: "maven",
  github_repo: null,
  status: "categorized",
  is_direct: true,
  categories: [
    {
      category_id: "cat-1",
      category_name: "JSON",
      strategic: false,
      retire_candidate: true,
    },
  ],
};

function renderRedundancy(
  fetch: Record<string, StubResponse>,
  role: Me["role"] = "viewer",
) {
  renderRoute(Redundancy, {
    path: "/redundancy",
    role,
    fetch: { "GET /api/projects": { body: PROJECTS }, ...fetch },
  });
}

async function selectProject(id: string) {
  const picker = screen.getByLabelText("Project");
  await within(picker).findByRole("option", { name: PROJECTS.find((p) => p.id === id)!.name });
  fireEvent.change(picker, { target: { value: id } });
}

describe("/redundancy report states", () => {
  test("a slow answer for an earlier project does not land under the current one", async () => {
    let releaseFirst!: () => void;
    const firstReport = new Promise<void>((resolve) => {
      releaseFirst = resolve;
    });
    renderRedundancy({
      "GET /api/projects/p-1/report": { body: [LIBRARY], after: firstReport },
      "GET /api/projects/p-2/report": { body: [] },
    });
    await screen.findByRole("heading", { name: "Report" });

    await selectProject("p-1");
    await selectProject("p-2");
    expect(await screen.findByText(EMPTY_ROW)).toBeTruthy();

    releaseFirst();
    await new Promise((resolve) => setTimeout(resolve, 50));

    expect(screen.getByText(EMPTY_ROW)).toBeTruthy();
    expect(screen.queryByText(LIBRARY.purl)).toBeNull();
  });

  test("pending shows Loading… and no empty row", async () => {
    renderRedundancy({ "GET /api/projects/p-1/report": "pending" });
    await screen.findByRole("heading", { name: "Report" });

    await selectProject("p-1");

    expect(await screen.findByText("Loading…")).toBeTruthy();
    expect(screen.queryByText(EMPTY_ROW)).toBeNull();
  });

  test("[] shows the empty row", async () => {
    renderRedundancy({ "GET /api/projects/p-1/report": { body: [] } });
    await screen.findByRole("heading", { name: "Report" });

    await selectProject("p-1");

    expect(await screen.findByText(EMPTY_ROW)).toBeTruthy();
    expect(screen.queryByText("Loading…")).toBeNull();
  });

  test("a top-level null body reads as empty, no crash", async () => {
    renderRedundancy({ "GET /api/projects/p-1/report": { body: null } });
    await screen.findByRole("heading", { name: "Report" });

    await selectProject("p-1");

    expect(await screen.findByText(EMPTY_ROW)).toBeTruthy();
  });

  test("500 shows the error and no empty row", async () => {
    renderRedundancy({
      "GET /api/projects/p-1/report": { status: 500, body: { error: "boom" } },
    });
    await screen.findByRole("heading", { name: "Report" });

    await selectProject("p-1");

    expect(await screen.findByText("boom")).toBeTruthy();
    expect(screen.queryByText(EMPTY_ROW)).toBeNull();
    expect(screen.queryByText("Loading…")).toBeNull();
  });

  test("a library with null categories still renders", async () => {
    renderRedundancy({
      "GET /api/projects/p-1/report": { body: [{ ...LIBRARY, categories: null }] },
    });
    await screen.findByRole("heading", { name: "Report" });

    await selectProject("p-1");

    const row = (await screen.findByText(LIBRARY.purl)).closest("tr")!;
    expect(within(row).queryByText("Retire?")).toBeNull();
  });

  test("data renders rows", async () => {
    renderRedundancy({ "GET /api/projects/p-1/report": { body: [LIBRARY] } });
    await screen.findByRole("heading", { name: "Report" });

    await selectProject("p-1");

    const row = (await screen.findByText(LIBRARY.purl)).closest("tr")!;
    expect(within(row).getByText("JSON")).toBeTruthy();
    expect(within(row).getByText("Retire candidate")).toBeTruthy();
    expect(screen.queryByText(EMPTY_ROW)).toBeNull();
  });

  test("a failed report after a successful one drops the old rows", async () => {
    renderRedundancy({
      "GET /api/projects/p-1/report": { body: [LIBRARY] },
      "GET /api/projects/p-2/report": { status: 500, body: { error: "boom" } },
    });
    await screen.findByRole("heading", { name: "Report" });
    await selectProject("p-1");
    await screen.findByText(LIBRARY.purl);

    await selectProject("p-2");

    expect(await screen.findByText("boom")).toBeTruthy();
    expect(screen.queryByText(LIBRARY.purl)).toBeNull();
    expect(screen.queryByText(EMPTY_ROW)).toBeNull();
  });
});

describe("/redundancy actions by role", () => {
  test("a viewer gets no upload form and no Delete project", async () => {
    renderRedundancy({ "GET /api/projects/p-1/report": { body: [LIBRARY] } }, "viewer");
    await screen.findByRole("heading", { name: "Report" });

    await selectProject("p-1");
    await screen.findByText(LIBRARY.purl);

    expect(screen.queryByRole("heading", { name: "Upload project SBOM" })).toBeNull();
    expect(screen.queryByLabelText("Project name")).toBeNull();
    expect(screen.queryByRole("button", { name: "Delete project" })).toBeNull();
  });

  test("an architect gets the upload form and Delete project", async () => {
    renderRedundancy({ "GET /api/projects/p-1/report": { body: [LIBRARY] } }, "architect");
    await screen.findByRole("heading", { name: "Report" });

    expect(screen.getByRole("heading", { name: "Upload project SBOM" })).toBeTruthy();
    expect(screen.getByLabelText("Project name")).toBeTruthy();

    await selectProject("p-1");
    await screen.findByText(LIBRARY.purl);

    expect(screen.getByRole("button", { name: "Delete project" })).toBeTruthy();
  });
});
