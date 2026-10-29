import { useEffect, useRef, useState, type FormEvent } from "react";

import {
  attachCategory,
  getBatchProgress,
  listLibraries,
  listTaxonomy,
  uploadCatalogFile,
  type BatchProgress,
  type Category,
  type Library,
} from "../lib/catalog";
import {
  describeBatch,
  isBatchSettled,
  POLL_INTERVAL_MS,
  POLL_MAX_DURATION_MS,
  type PollOutcome,
} from "../lib/batch";
import { RequireAuth } from "../lib/require-auth";
import { Button } from "../components/button";
import { Nav } from "../components/nav";

function CatalogContent() {
  const [file, setFile] = useState<File | null>(null);
  const [kind, setKind] = useState<"sbom" | "csv">("sbom");
  const [uploading, setUploading] = useState(false);
  const [uploadError, setUploadError] = useState<string | null>(null);

  const [batchId, setBatchId] = useState<string | null>(null);
  const [batchProgress, setBatchProgress] = useState<BatchProgress | null>(null);
  const [polling, setPolling] = useState(false);
  const [pollOutcome, setPollOutcome] = useState<PollOutcome | null>(null);

  const [statusFilter, setStatusFilter] = useState("");
  const [categoryFilter, setCategoryFilter] = useState("");
  const [ecosystemFilter, setEcosystemFilter] = useState("");

  const [libraries, setLibraries] = useState<Library[]>([]);
  const [librariesError, setLibrariesError] = useState<string | null>(null);
  // True from the first render so the empty row never flashes before data.
  const [librariesLoading, setLibrariesLoading] = useState(true);
  const [categories, setCategories] = useState<Category[]>([]);

  const [selectedCategory, setSelectedCategory] = useState<Record<string, string>>({});
  const [attaching, setAttaching] = useState<string | null>(null);
  const [attachError, setAttachError] = useState<string | null>(null);

  const pollTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const pollToken = useRef(0);

  async function refreshLibraries() {
    setLibrariesLoading(true);
    try {
      setLibrariesError(null);
      const filter = {
        status: statusFilter || undefined,
        category: categoryFilter || undefined,
        ecosystem: ecosystemFilter || undefined,
      };
      const result = await listLibraries(filter);
      setLibraries(result ?? []);
    } catch {
      setLibraries([]);
      setLibrariesError("Failed to load libraries.");
    } finally {
      setLibrariesLoading(false);
    }
  }

  useEffect(() => {
    refreshLibraries();
    listTaxonomy()
      .then((result) => setCategories(result ?? []))
      .catch(() => setCategories([]));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    refreshLibraries();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [statusFilter, categoryFilter, ecosystemFilter]);

  useEffect(() => {
    return () => {
      if (pollTimer.current) {
        clearTimeout(pollTimer.current);
      }
    };
  }, []);

  function startPolling(id: string) {
    if (pollTimer.current) {
      clearTimeout(pollTimer.current);
      pollTimer.current = null;
    }

    const token = ++pollToken.current;
    const startedAt = Date.now();
    setPolling(true);
    setPollOutcome(null);

    async function poll() {
      try {
        const progress = await getBatchProgress(id);
        if (pollToken.current !== token) {
          return;
        }
        setBatchProgress(progress);

        const settled = isBatchSettled(progress);
        const timedOut = Date.now() - startedAt > POLL_MAX_DURATION_MS;

        if (settled || timedOut) {
          setPolling(false);
          setPollOutcome(settled ? "settled" : "timeout");
          refreshLibraries();
          return;
        }

        pollTimer.current = setTimeout(poll, POLL_INTERVAL_MS);
      } catch {
        if (pollToken.current !== token) {
          return;
        }
        // A transient poll failure shouldn't silently freeze the table on
        // whatever state it last showed — refresh once with whatever is
        // actually in the database before giving up.
        setPolling(false);
        setPollOutcome("error");
        refreshLibraries();
      }
    }

    poll();
  }

  async function handleUpload(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!file) {
      return;
    }

    setUploadError(null);
    setUploading(true);
    try {
      const result = await uploadCatalogFile(file, kind);
      setBatchId(result.batch_id);
      setBatchProgress(null);
      startPolling(result.batch_id);
      refreshLibraries();
    } catch (err) {
      setUploadError(err instanceof Error ? err.message : "Upload failed.");
    } finally {
      setUploading(false);
    }
  }

  async function handleAttach(libraryId: string) {
    const categoryId = selectedCategory[libraryId];
    if (!categoryId) {
      return;
    }

    setAttachError(null);
    setAttaching(libraryId);
    try {
      await attachCategory(libraryId, categoryId);
      await refreshLibraries();
    } catch (err) {
      setAttachError(err instanceof Error ? err.message : "Failed to add category.");
    } finally {
      setAttaching(null);
    }
  }

  return (
    <main className="mx-auto max-w-4xl p-6">
      <Nav />
      <h1 className="font-serif text-xl font-semibold">Catalog</h1>

      <section className="mt-6 rounded-lg border border-rule bg-bg-soft p-4">
        <h2 className="font-serif text-lg font-medium">Upload</h2>
        <form onSubmit={handleUpload} className="mt-3 flex flex-wrap items-end gap-3">
          <div className="space-y-1">
            <label htmlFor="kind" className="block text-sm font-medium">
              Kind
            </label>
            <select
              id="kind"
              value={kind}
              onChange={(event) => setKind(event.target.value as "sbom" | "csv")}
              className="rounded border border-rule bg-bg-soft px-3 py-2 text-ink"
            >
              <option value="sbom">SBOM</option>
              <option value="csv">CSV</option>
            </select>
          </div>

          <div className="space-y-1">
            <label htmlFor="file" className="block text-sm font-medium">
              File
            </label>
            <input
              id="file"
              type="file"
              required
              onChange={(event) => setFile(event.target.files?.[0] ?? null)}
              className="block text-sm"
            />
          </div>

          <Button type="submit" disabled={uploading || !file}>
            {uploading ? "Uploading…" : "Upload"}
          </Button>
        </form>

        {uploadError && (
          <p className="mt-2 text-sm text-destructive">{uploadError}</p>
        )}

        {batchId && (
          <div className="mt-4 text-sm">
            {(() => {
              const batch = describeBatch(polling, pollOutcome, batchProgress);
              return (
                <p className={batch.problem ? "text-destructive" : undefined}>
                  Batch <span className="font-mono">{batchId}</span>
                  {batch.text}
                </p>
              );
            })()}
            {batchProgress && (
              <ul className="mt-1 flex flex-wrap gap-3">
                {Object.entries(batchProgress).map(([status, count]) => (
                  <li key={status} className="rounded border border-rule px-2 py-1">
                    {status}: {count}
                  </li>
                ))}
              </ul>
            )}
          </div>
        )}
      </section>

      <section className="mt-6 rounded-lg border border-rule bg-bg-soft p-4">
        <h2 className="font-serif text-lg font-medium">Libraries</h2>

        <div className="mt-3 flex flex-wrap gap-3">
          <div className="space-y-1">
            <label htmlFor="status-filter" className="block text-sm font-medium">
              Status
            </label>
            <select
              id="status-filter"
              value={statusFilter}
              onChange={(event) => setStatusFilter(event.target.value)}
              className="rounded border border-rule bg-bg-soft px-3 py-2 text-ink"
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
              className="rounded border border-rule bg-bg-soft px-3 py-2 text-ink"
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
              className="rounded border border-rule bg-bg-soft px-3 py-2 text-ink"
            />
          </div>
        </div>

        {librariesError && (
          <p className="mt-2 text-sm text-destructive">{librariesError}</p>
        )}
        {attachError && (
          <p className="mt-2 text-sm text-destructive">{attachError}</p>
        )}

        <div className="mt-4 overflow-x-auto">
          <table className="w-full text-left text-sm">
            <thead>
              <tr className="border-b border-rule">
                <th className="py-2 pr-4">Purl</th>
                <th className="py-2 pr-4">Status</th>
                <th className="py-2 pr-4">Categories</th>
                <th className="py-2 pr-4">GitHub repo</th>
                <th className="py-2 pr-4">Add category</th>
              </tr>
            </thead>
            <tbody>
              {libraries.map((library) => {
                const libraryCategories = library.categories ?? [];
                return (
                  <tr key={library.id} className="border-b border-rule">
                    <td className="py-2 pr-4 font-mono">{library.purl}</td>
                    <td className="py-2 pr-4">{library.status}</td>
                    <td className="py-2 pr-4">
                      {libraryCategories.length > 0 ? libraryCategories.join(", ") : "—"}
                    </td>
                    <td className="py-2 pr-4">{library.github_repo ?? "—"}</td>
                    <td className="py-2 pr-4">
                      <div className="flex items-center gap-2">
                        <select
                          value={selectedCategory[library.id] ?? ""}
                          onChange={(event) =>
                            setSelectedCategory((prev) => ({
                              ...prev,
                              [library.id]: event.target.value,
                            }))
                          }
                          className="rounded border border-rule bg-bg-soft px-2 py-1 text-ink"
                        >
                          <option value="">Select…</option>
                          {categories.map((category) => (
                            <option key={category.id} value={category.id}>
                              {category.name}
                            </option>
                          ))}
                        </select>
                        <Button
                          type="button"
                          disabled={attaching === library.id || !selectedCategory[library.id]}
                          onClick={() => handleAttach(library.id)}
                          className="px-2 py-1 text-xs"
                        >
                          {attaching === library.id ? "Adding…" : "Add"}
                        </Button>
                      </div>
                    </td>
                  </tr>
                );
              })}
              {librariesLoading && libraries.length === 0 && (
                <tr>
                  <td colSpan={5} className="py-4 text-center text-ink-faint">
                    Loading…
                  </td>
                </tr>
              )}
              {!librariesLoading && !librariesError && libraries.length === 0 && (
                <tr>
                  <td colSpan={5} className="py-4 text-center text-ink-faint">
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

export default function Catalog() {
  return (
    <RequireAuth role="admin">
      <CatalogContent />
    </RequireAuth>
  );
}
