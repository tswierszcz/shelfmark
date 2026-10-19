import { getBatchProgress, type SkippedRow } from "./catalog";

export type Project = {
  id: string;
  name: string;
  created_by: string;
  created_at: string;
};

export type ProjectReportCategory = {
  category_id: string;
  category_name: string;
  strategic: boolean;
  retire_candidate: boolean;
};

export type ProjectReportLibrary = {
  id: string;
  purl: string;
  name: string;
  ecosystem: string;
  github_repo: string | null;
  status: string;
  is_direct: boolean;
  categories: ProjectReportCategory[];
};

export type ProjectUploadResult = {
  project_id: string;
  project_name: string;
  batch_id: string;
  created: number;
  skipped: SkippedRow[];
};

async function errorMessage(res: Response, fallback: string): Promise<string> {
  try {
    const body = (await res.json()) as { error?: string };
    return body.error ?? fallback;
  } catch {
    return fallback;
  }
}

export async function uploadProjectSbom(
  projectName: string,
  file: File,
): Promise<ProjectUploadResult> {
  const form = new FormData();
  form.set("project_name", projectName);
  form.set("file", file);

  const res = await fetch("/api/projects/upload", {
    method: "POST",
    body: form,
  });

  if (!res.ok) {
    throw new Error(await errorMessage(res, "Upload failed"));
  }

  return (await res.json()) as ProjectUploadResult;
}

export async function listProjects(): Promise<Project[]> {
  const res = await fetch("/api/projects");

  if (!res.ok) {
    throw new Error(await errorMessage(res, "Failed to list projects"));
  }

  return (await res.json()) as Project[];
}

export async function getProjectReport(
  projectId: string,
): Promise<ProjectReportLibrary[]> {
  const res = await fetch(`/api/projects/${encodeURIComponent(projectId)}/report`);

  if (!res.ok) {
    throw new Error(await errorMessage(res, "Failed to load project report"));
  }

  return (await res.json()) as ProjectReportLibrary[];
}

export { getBatchProgress };
