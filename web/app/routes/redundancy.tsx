import { useEffect, useRef, useState, type FormEvent } from "react";

import type { BatchProgress } from "../lib/catalog";
import {
  getBatchProgress,
  getProjectReport,
  listProjects,
  uploadProjectSbom,
  type Project,
  type ProjectReportLibrary,
} from "../lib/projects";
import { useAuth } from "../lib/auth";
import { RequireAuth } from "../lib/require-auth";

const SETTLED_STATUSES = new Set(["categorized", "needs_manual", "failed"]);
const POLL_INTERVAL_MS = 2000;
const POLL_MAX_DURATION_MS = 60_000;

function isBatchSettled(progress: BatchProgress): boolean {
  const statuses = Object.keys(progress);
  if (statuses.length === 0) {
    return false;
  }
  return statuses.every((status) => SETTLED_STATUSES.has(status));
}

function RedundancyContent() {
  const { me } = useAuth();
  const isArchitect = me?.role === "architect";

  const [projectName, setProjectName] = useState("");
  const [file, setFile] = useState<File | null>(null);
  const [uploading, setUploading] = useState(false);
  const [uploadError, setUploadError] = useState<string | null>(null);

  const [batchId, setBatchId] = useState<string | null>(null);
  const [batchProgress, setBatchProgress] = useState<BatchProgress | null>(null);
  const [polling, setPolling] = useState(false);

  const [projects, setProjects] = useState<Project[]>([]);
  const [projectsError, setProjectsError] = useState<string | null>(null);

  const [selectedProjectId, setSelectedProjectId] = useState("");
  const [report, setReport] = useState<ProjectReportLibrary[]>([]);
  const [reportError, setReportError] = useState<string | null>(null);

  const pollTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const pollToken = useRef(0);

  async function refreshProjects() {
    try {
      setProjectsError(null);
      const result = await listProjects();
      setProjects(result);
      return result;
    } catch {
      setProjectsError("Failed to load projects.");
      return [];
    }
  }

  async function refreshReport(projectId: string) {
    if (!projectId) {
      setReport([]);
      return;
    }
    try {
      setReportError(null);
      const result = await getProjectReport(projectId);
      setReport(result);
    } catch (err) {
      setReportError(err instanceof Error ? err.message : "Failed to load report.");
    }
  }

  useEffect(() => {
    refreshProjects();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    return () => {
      if (pollTimer.current) {
        clearTimeout(pollTimer.current);
      }
    };
  }, []);

  function startPolling(id: string, onSettled: () => void) {
    if (pollTimer.current) {
      clearTimeout(pollTimer.current);
      pollTimer.current = null;
    }

    const token = ++pollToken.current;
    const startedAt = Date.now();
    setPolling(true);

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
          onSettled();
          return;
        }

        pollTimer.current = setTimeout(poll, POLL_INTERVAL_MS);
      } catch {
        if (pollToken.current !== token) {
          return;
        }
        setPolling(false);
        onSettled();
      }
    }

    poll();
  }

  async function handleUpload(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!file || !projectName.trim()) {
      return;
    }

    setUploadError(null);
    setUploading(true);
    try {
      const result = await uploadProjectSbom(projectName.trim(), file);
      setBatchId(result.batch_id);
      setBatchProgress(null);
      startPolling(result.batch_id, async () => {
        await refreshProjects();
        setSelectedProjectId(result.project_id);
        await refreshReport(result.project_id);
      });
    } catch (err) {
      setUploadError(err instanceof Error ? err.message : "Upload failed.");
    } finally {
      setUploading(false);
    }
  }

  function handleSelectProject(projectId: string) {
    setSelectedProjectId(projectId);
    refreshReport(projectId);
  }

  return (
    <main className="mx-auto max-w-4xl p-6">
      <h1 className="text-xl font-semibold">Redundancy report</h1>

      {isArchitect && (
        <section className="mt-6 rounded-lg border border-gray-200 p-4 dark:border-gray-800">
          <h2 className="text-lg font-medium">Upload project SBOM</h2>
          <form onSubmit={handleUpload} className="mt-3 flex flex-wrap items-end gap-3">
            <div className="space-y-1">
              <label htmlFor="project-name" className="block text-sm font-medium">
                Project name
              </label>
              <input
                id="project-name"
                type="text"
                required
                value={projectName}
                onChange={(event) => setProjectName(event.target.value)}
                placeholder="my-service"
                className="rounded border border-gray-300 px-3 py-2 dark:border-gray-700 dark:bg-gray-900"
              />
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

            <button
              type="submit"
              disabled={uploading || !file || !projectName.trim()}
              className="rounded bg-gray-900 px-3 py-2 text-white disabled:opacity-50 dark:bg-gray-100 dark:text-gray-900"
            >
              {uploading ? "Uploading…" : "Upload"}
            </button>
          </form>

          {uploadError && (
            <p className="mt-2 text-sm text-red-600 dark:text-red-400">{uploadError}</p>
          )}

          {batchId && (
            <div className="mt-4 text-sm">
              <p>
                Batch <span className="font-mono">{batchId}</span>
                {polling ? " — processing…" : " — done"}
              </p>
              {batchProgress && (
                <ul className="mt-1 flex flex-wrap gap-3">
                  {Object.entries(batchProgress).map(([status, count]) => (
                    <li key={status} className="rounded border border-gray-200 px-2 py-1 dark:border-gray-800">
                      {status}: {count}
                    </li>
                  ))}
                </ul>
              )}
            </div>
          )}
        </section>
      )}

      <section className="mt-6 rounded-lg border border-gray-200 p-4 dark:border-gray-800">
        <h2 className="text-lg font-medium">Report</h2>

        <div className="mt-3 space-y-1">
          <label htmlFor="project-picker" className="block text-sm font-medium">
            Project
          </label>
          <select
            id="project-picker"
            value={selectedProjectId}
            onChange={(event) => handleSelectProject(event.target.value)}
            className="rounded border border-gray-300 px-3 py-2 dark:border-gray-700 dark:bg-gray-900"
          >
            <option value="">Select a project…</option>
            {projects.map((project) => (
              <option key={project.id} value={project.id}>
                {project.name}
              </option>
            ))}
          </select>
        </div>

        {projectsError && (
          <p className="mt-2 text-sm text-red-600 dark:text-red-400">{projectsError}</p>
        )}
        {reportError && (
          <p className="mt-2 text-sm text-red-600 dark:text-red-400">{reportError}</p>
        )}

        {selectedProjectId && (
          <div className="mt-4 overflow-x-auto">
            <table className="w-full text-left text-sm">
              <thead>
                <tr className="border-b border-gray-200 dark:border-gray-800">
                  <th className="py-2 pr-4">Purl</th>
                  <th className="py-2 pr-4">Direct</th>
                  <th className="py-2 pr-4">GitHub repo</th>
                  <th className="py-2 pr-4">Categories</th>
                  <th className="py-2 pr-4">Retire?</th>
                </tr>
              </thead>
              <tbody>
                {report.map((library) => {
                  const retireCandidate = library.categories.some(
                    (category) => category.retire_candidate,
                  );

                  return (
                    <tr key={library.id} className="border-b border-gray-100 dark:border-gray-900">
                      <td className="py-2 pr-4 font-mono">{library.purl}</td>
                      <td className="py-2 pr-4">{library.is_direct ? "Yes" : "No"}</td>
                      <td className="py-2 pr-4">{library.github_repo ?? "—"}</td>
                      <td className="py-2 pr-4">
                        {library.categories.length === 0 ? (
                          "—"
                        ) : (
                          <ul className="space-y-1">
                            {library.categories.map((category) => (
                              <li
                                key={category.category_id}
                                className="flex flex-wrap items-center gap-2"
                              >
                                <span>{category.category_name}</span>
                                {category.strategic && (
                                  <span className="rounded bg-amber-100 px-2 py-0.5 text-xs text-amber-900 dark:bg-amber-900 dark:text-amber-100">
                                    Strategic
                                  </span>
                                )}
                                {category.retire_candidate && (
                                  <span className="rounded bg-red-100 px-2 py-0.5 text-xs text-red-900 dark:bg-red-900 dark:text-red-100">
                                    Retire candidate
                                  </span>
                                )}
                              </li>
                            ))}
                          </ul>
                        )}
                      </td>
                      <td className="py-2 pr-4">
                        {retireCandidate ? (
                          <span className="text-red-600 dark:text-red-400">Retire?</span>
                        ) : (
                          "—"
                        )}
                      </td>
                    </tr>
                  );
                })}
                {report.length === 0 && (
                  <tr>
                    <td colSpan={5} className="py-4 text-center text-gray-500">
                      No libraries recorded for this project.
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </main>
  );
}

export default function Redundancy() {
  return (
    <RequireAuth role={["architect", "viewer"]}>
      <RedundancyContent />
    </RequireAuth>
  );
}
