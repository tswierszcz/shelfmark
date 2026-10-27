export type Category = {
  id: string;
  name: string;
};

export type Library = {
  id: string;
  purl: string;
  name: string;
  ecosystem: string;
  github_repo: string | null;
  status: string;
  upload_batch_id: string;
  categories: string[];
};

export type LibraryFilter = {
  status?: string;
  category?: string;
  ecosystem?: string;
};

export type SkippedRow = {
  Index: number;
  Reason: string;
};

export type UploadResult = {
  batch_id: string;
  created: number;
  skipped: SkippedRow[];
};

export type BatchProgress = Record<string, number>;

async function errorMessage(res: Response, fallback: string): Promise<string> {
  try {
    const body = (await res.json()) as { error?: string };
    return body.error ?? fallback;
  } catch {
    return fallback;
  }
}

export async function uploadCatalogFile(
  file: File,
  kind: "sbom" | "csv",
): Promise<UploadResult> {
  const form = new FormData();
  form.set("kind", kind);
  form.set("file", file);

  const res = await fetch("/api/catalog/upload", {
    method: "POST",
    body: form,
  });

  if (!res.ok) {
    throw new Error(await errorMessage(res, "Upload failed"));
  }

  return (await res.json()) as UploadResult;
}

export async function getBatchProgress(batchId: string): Promise<BatchProgress> {
  const res = await fetch(`/api/catalog/batches/${encodeURIComponent(batchId)}`);

  if (!res.ok) {
    throw new Error("Failed to fetch batch progress");
  }

  return (await res.json()) as BatchProgress;
}

export async function listLibraries(filter: LibraryFilter = {}): Promise<Library[]> {
  const params = new URLSearchParams();
  if (filter.status) params.set("status", filter.status);
  if (filter.category) params.set("category", filter.category);
  if (filter.ecosystem) params.set("ecosystem", filter.ecosystem);

  const query = params.toString();
  const res = await fetch(`/api/catalog/libraries${query ? `?${query}` : ""}`);

  if (!res.ok) {
    throw new Error("Failed to list libraries");
  }

  return (await res.json()) as Library[];
}

export async function listTaxonomy(): Promise<Category[]> {
  const res = await fetch("/api/catalog/categories");

  if (!res.ok) {
    throw new Error("Failed to list categories");
  }

  return (await res.json()) as Category[];
}

export async function attachCategory(
  libraryId: string,
  categoryId: string,
): Promise<void> {
  const res = await fetch(
    `/api/catalog/libraries/${encodeURIComponent(libraryId)}/categories`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ category_id: categoryId }),
    },
  );

  if (!res.ok) {
    throw new Error("Failed to attach category");
  }
}
