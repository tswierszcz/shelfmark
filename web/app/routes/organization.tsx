import { useEffect, useState } from "react";

import { listTaxonomy, type Category } from "../lib/catalog";
import { useAuth } from "../lib/auth";
import {
  listOrganizationLibraries,
  markStrategic,
  unmarkStrategic,
  type OrganizationLibrary,
} from "../lib/organization";
import { RequireAuth } from "../lib/require-auth";

function OrganizationContent() {
  const { me } = useAuth();
  const isArchitect = me?.role === "architect";

  const [statusFilter, setStatusFilter] = useState("");
  const [categoryFilter, setCategoryFilter] = useState("");
  const [ecosystemFilter, setEcosystemFilter] = useState("");
  const [strategicFilter, setStrategicFilter] = useState("");

  const [libraries, setLibraries] = useState<OrganizationLibrary[]>([]);
  const [librariesError, setLibrariesError] = useState<string | null>(null);
  const [categories, setCategories] = useState<Category[]>([]);

  const [pendingKey, setPendingKey] = useState<string | null>(null);
  const [justifications, setJustifications] = useState<Record<string, string>>({});
  const [openForm, setOpenForm] = useState<string | null>(null);
  const [formError, setFormError] = useState<string | null>(null);

  async function refreshLibraries() {
    try {
      setLibrariesError(null);
      const filter = {
        status: statusFilter || undefined,
        category: categoryFilter || undefined,
        ecosystem: ecosystemFilter || undefined,
        strategic:
          strategicFilter === "" ? undefined : strategicFilter === "true",
      };
      const result = await listOrganizationLibraries(filter);
      setLibraries(result);
    } catch {
      setLibrariesError("Failed to load libraries.");
    }
  }

  useEffect(() => {
    refreshLibraries();
    listTaxonomy()
      .then(setCategories)
      .catch(() => setCategories([]));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    refreshLibraries();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [statusFilter, categoryFilter, ecosystemFilter, strategicFilter]);

  function categoryIdForName(name: string): string | undefined {
    return categories.find((category) => category.name === name)?.id;
  }

  function formKey(libraryId: string, categoryId: string): string {
    return `${libraryId}:${categoryId}`;
  }

  async function handleMark(libraryId: string, categoryId: string) {
    const key = formKey(libraryId, categoryId);
    const justification = (justifications[key] ?? "").trim();
    if (!justification) {
      setFormError("Justification is required.");
      return;
    }

    setFormError(null);
    setPendingKey(key);
    try {
      await markStrategic(libraryId, categoryId, justification);
      setJustifications((prev) => ({ ...prev, [key]: "" }));
      setOpenForm(null);
      await refreshLibraries();
    } catch (err) {
      setFormError(err instanceof Error ? err.message : "Failed to mark strategic.");
    } finally {
      setPendingKey(null);
    }
  }

  async function handleUnmark(libraryId: string, categoryId: string) {
    const key = formKey(libraryId, categoryId);
    const justification = (justifications[key] ?? "").trim();
    if (!justification) {
      setFormError("Justification is required.");
      return;
    }

    setFormError(null);
    setPendingKey(key);
    try {
      await unmarkStrategic(libraryId, categoryId, justification);
      setJustifications((prev) => ({ ...prev, [key]: "" }));
      setOpenForm(null);
      await refreshLibraries();
    } catch (err) {
      setFormError(err instanceof Error ? err.message : "Failed to unmark strategic.");
    } finally {
      setPendingKey(null);
    }
  }

  return (
    <main className="mx-auto max-w-4xl p-6">
      <h1 className="text-xl font-semibold">Organization</h1>

      <section className="mt-6 rounded-lg border border-gray-200 p-4 dark:border-gray-800">
        <h2 className="text-lg font-medium">Libraries</h2>

        <div className="mt-3 flex flex-wrap gap-3">
          <div className="space-y-1">
            <label htmlFor="status-filter" className="block text-sm font-medium">
              Status
            </label>
            <select
              id="status-filter"
              value={statusFilter}
              onChange={(event) => setStatusFilter(event.target.value)}
              className="rounded border border-gray-300 px-3 py-2 dark:border-gray-700 dark:bg-gray-900"
            >
              <option value="">All</option>
              <option value="pending">pending</option>
              <option value="processing">processing</option>
              <option value="categorized">categorized</option>
              <option value="needs_manual">needs_manual</option>
              <option value="failed">failed</option>
            </select>
          </div>

          <div className="space-y-1">
            <label htmlFor="category-filter" className="block text-sm font-medium">
              Category
            </label>
            <select
              id="category-filter"
              value={categoryFilter}
              onChange={(event) => setCategoryFilter(event.target.value)}
              className="rounded border border-gray-300 px-3 py-2 dark:border-gray-700 dark:bg-gray-900"
            >
              <option value="">All</option>
              {categories.map((category) => (
                <option key={category.id} value={category.name}>
                  {category.name}
                </option>
              ))}
            </select>
          </div>

          <div className="space-y-1">
            <label htmlFor="ecosystem-filter" className="block text-sm font-medium">
              Ecosystem
            </label>
            <input
              id="ecosystem-filter"
              type="text"
              value={ecosystemFilter}
              onChange={(event) => setEcosystemFilter(event.target.value)}
              placeholder="maven, npm…"
              className="rounded border border-gray-300 px-3 py-2 dark:border-gray-700 dark:bg-gray-900"
            />
          </div>

          <div className="space-y-1">
            <label htmlFor="strategic-filter" className="block text-sm font-medium">
              Strategic
            </label>
            <select
              id="strategic-filter"
              value={strategicFilter}
              onChange={(event) => setStrategicFilter(event.target.value)}
              className="rounded border border-gray-300 px-3 py-2 dark:border-gray-700 dark:bg-gray-900"
            >
              <option value="">All</option>
              <option value="true">Strategic only</option>
              <option value="false">Non-strategic only</option>
            </select>
          </div>
        </div>

        {librariesError && (
          <p className="mt-2 text-sm text-red-600 dark:text-red-400">{librariesError}</p>
        )}
        {formError && (
          <p className="mt-2 text-sm text-red-600 dark:text-red-400">{formError}</p>
        )}

        <div className="mt-4 overflow-x-auto">
          <table className="w-full text-left text-sm">
            <thead>
              <tr className="border-b border-gray-200 dark:border-gray-800">
                <th className="py-2 pr-4">Purl</th>
                <th className="py-2 pr-4">Status</th>
                <th className="py-2 pr-4">Direct</th>
                <th className="py-2 pr-4">GitHub repo</th>
                <th className="py-2 pr-4">Categories</th>
              </tr>
            </thead>
            <tbody>
              {libraries.map((library) => (
                <tr key={library.id} className="border-b border-gray-100 dark:border-gray-900">
                  <td className="py-2 pr-4 font-mono">{library.purl}</td>
                  <td className="py-2 pr-4">{library.status}</td>
                  <td className="py-2 pr-4">{library.is_direct ? "Yes" : "No"}</td>
                  <td className="py-2 pr-4">{library.github_repo ?? "—"}</td>
                  <td className="py-2 pr-4">
                    {library.categories.length === 0 ? (
                      "—"
                    ) : (
                      <ul className="space-y-2">
                        {library.categories.map((categoryName) => {
                          const categoryId = categoryIdForName(categoryName);
                          const mark = library.strategic.find(
                            (entry) => entry.category_name === categoryName,
                          );
                          const key = categoryId
                            ? formKey(library.id, categoryId)
                            : undefined;

                          return (
                            <li key={categoryName}>
                              <div className="flex flex-wrap items-center gap-2">
                                <span>{categoryName}</span>
                                {mark ? (
                                  <span
                                    className="rounded bg-amber-100 px-2 py-0.5 text-xs text-amber-900 dark:bg-amber-900 dark:text-amber-100"
                                    title={`${mark.justification} (by ${mark.marked_by})`}
                                  >
                                    Strategic
                                  </span>
                                ) : null}

                                {isArchitect && categoryId && key ? (
                                  <button
                                    type="button"
                                    onClick={() =>
                                      setOpenForm((prev) => (prev === key ? null : key))
                                    }
                                    className="rounded border border-gray-300 px-2 py-0.5 text-xs dark:border-gray-700"
                                  >
                                    {mark ? "Unmark" : "Mark strategic"}
                                  </button>
                                ) : null}
                              </div>

                              {isArchitect && categoryId && key && openForm === key ? (
                                <div className="mt-1 flex flex-wrap items-start gap-2">
                                  <textarea
                                    value={justifications[key] ?? ""}
                                    onChange={(event) =>
                                      setJustifications((prev) => ({
                                        ...prev,
                                        [key]: event.target.value,
                                      }))
                                    }
                                    placeholder="Justification"
                                    rows={2}
                                    className="w-64 rounded border border-gray-300 px-2 py-1 text-xs dark:border-gray-700 dark:bg-gray-900"
                                  />
                                  <button
                                    type="button"
                                    disabled={pendingKey === key}
                                    onClick={() =>
                                      mark
                                        ? handleUnmark(library.id, categoryId)
                                        : handleMark(library.id, categoryId)
                                    }
                                    className="rounded bg-gray-900 px-2 py-1 text-xs text-white disabled:opacity-50 dark:bg-gray-100 dark:text-gray-900"
                                  >
                                    {pendingKey === key
                                      ? "Submitting…"
                                      : mark
                                        ? "Confirm unmark"
                                        : "Confirm mark"}
                                  </button>
                                </div>
                              ) : null}
                            </li>
                          );
                        })}
                      </ul>
                    )}
                  </td>
                </tr>
              ))}
              {libraries.length === 0 && (
                <tr>
                  <td colSpan={5} className="py-4 text-center text-gray-500">
                    No libraries match these filters.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </section>
    </main>
  );
}

export default function Organization() {
  return (
    <RequireAuth role={["architect", "viewer"]}>
      <OrganizationContent />
    </RequireAuth>
  );
}
