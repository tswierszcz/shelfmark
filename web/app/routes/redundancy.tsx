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
import { Nav } from "../components/nav";
import { Button } from "../components/button";
import { Badge } from "../components/badge";
import { LibraryStatusBadge } from "../components/library-status-badge";

const SETTLED_STATUSES = new Set(["categorized", "needs_manual", "failed"]);
const POLL_INTERVAL_MS = 2000;
const POLL_MAX_DURATION_MS = 60_000;

// An empty progress map is settled: the upload writes its libraries before
// responding, so a batch with no libraries means every purl was already in
// the catalog and nothing was enqueued.
function isBatchSettled(progress: BatchProgress): boolean {
  return Object.keys(progress).every((status) => SETTLED_STATUSES.has(status));
}

type PollOutcome = "settled" | "timeout" | "error";

// describeBatch turns the poll state into the batch line's suffix. "done"
// alone means every library was categorized; anything that needs review or
// failed is named, so an incomplete result never reads as plain success.
function describeBatch(
  polling: boolean,
  outcome: PollOutcome | null,
  progress: BatchProgress | null,
): { text: string; problem: boolean } {
  if (polling || outcome === null) {
    return { text: " — processing…", problem: false };
  }
  if (outcome === "error") {
    return { text: " — status unavailable", problem: true };
  }
  if (outcome === "timeout") {
    return { text: " — still processing", problem: false };
  }

  if (progress && Object.keys(progress).length === 0) {
    return { text: " — done: no new libraries to categorize", problem: false };
  }

  const needReview = progress?.needs_manual ?? 0;
  const failed = progress?.failed ?? 0;
  const issues = [
    needReview > 0 ? `${needReview} ${needReview === 1 ? "needs" : "need"} review` : null,
    failed > 0 ? `${failed} failed` : null,
  ].filter((part) => part !== null);
  if (issues.length === 0) {
    return { text: " — done", problem: false };
  }
  return { text: ` — done: ${issues.join(", ")}`, problem: true };
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
  const [pollOutcome, setPollOutcome] = useState<PollOutcome | null>(null);

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
      setProjects(result ?? []);
      return result ?? [];
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
      setReport(result ?? []);
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
          onSettled();
          return;
        }

        pollTimer.current = setTimeout(poll, POLL_INTERVAL_MS);
      } catch {
        if (pollToken.current !== token) {
          return;
        }
        setPolling(false);
        setPollOutcome("error");
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
      <Nav />
      <h1 className="text-xl font-semibold font-serif">Redundancy report</h1>

      {isArchitect && (
        <section className="mt-6 rounded-lg border border-rule bg-bg-soft p-4">
          <h2 className="text-lg font-semibold font-serif">Upload project SBOM</h2>
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
                className="border border-rule bg-bg-soft text-ink text-sm rounded px-3 py-2"
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

            <Button type="submit" disabled={uploading || !file || !projectName.trim()}>
              {uploading ? "Uploading…" : "Upload"}
            </Button>
          </form>

          {uploadError && <p className="mt-2 text-destructive text-sm">{uploadError}</p>}

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
      )}

      <section className="mt-6 rounded-lg border border-rule bg-bg-soft p-4">
        <h2 className="text-lg font-semibold font-serif">Report</h2>

        <div className="mt-3 space-y-1">
          <label htmlFor="project-picker" className="block text-sm font-medium">
            Project
          </label>
          <select
            id="project-picker"
            value={selectedProjectId}
            onChange={(event) => handleSelectProject(event.target.value)}
            className="border border-rule bg-bg-soft text-ink text-sm rounded px-3 py-2"
          >
            <option value="">Select a project…</option>
            {projects.map((project) => (
              <option key={project.id} value={project.id}>
                {project.name}
              </option>
            ))}
          </select>
        </div>

        {projectsError && <p className="mt-2 text-destructive text-sm">{projectsError}</p>}
        {reportError && <p className="mt-2 text-destructive text-sm">{reportError}</p>}

        {selectedProjectId && (
          <div className="mt-4 overflow-x-auto">
            <table className="w-full text-left text-sm">
              <thead>
                <tr className="border-b border-rule">
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
                    <tr key={library.id} className="border-b border-rule/50">
                      <td className="py-2 pr-4 font-mono">{library.purl}</td>
                      <td className="py-2 pr-4">{library.is_direct ? "Yes" : "No"}</td>
                      <td className="py-2 pr-4">{library.github_repo ?? "—"}</td>
                      <td className="py-2 pr-4">
                        {library.categories.length === 0 ? (
                          <LibraryStatusBadge status={library.status} />
                        ) : (
                          <ul className="space-y-1">
                            {library.categories.map((category) => (
                              <li
                                key={category.category_id}
                                className="flex flex-wrap items-center gap-2"
                              >
                                <span>{category.category_name}</span>
                                {category.strategic && <Badge tone="accent">Strategic</Badge>}
                                {category.retire_candidate && (
                                  <Badge tone="destructive">Retire candidate</Badge>
                                )}
                              </li>
                            ))}
                          </ul>
                        )}
                      </td>
                      <td className="py-2 pr-4">
                        {retireCandidate ? (
                          <span className="text-destructive">Retire?</span>
                        ) : (
                          "—"
                        )}
                      </td>
                    </tr>
                  );
                })}
                {report.length === 0 && (
                  <tr>
                    <td colSpan={5} className="py-4 text-center text-ink-faint">
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
