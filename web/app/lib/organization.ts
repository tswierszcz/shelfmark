import type { Library } from "./catalog";

export type StrategicMark = {
  category_id: string;
  category_name: string;
  justification: string;
  marked_by: string;
};

export type OrganizationLibrary = Library & {
  is_direct: boolean;
  strategic: StrategicMark[];
};

export type OrganizationLibraryFilter = {
  status?: string;
  category?: string;
  ecosystem?: string;
  strategic?: boolean;
};

async function errorMessage(res: Response, fallback: string): Promise<string> {
  try {
    const body = (await res.json()) as { error?: string };
    return body.error ?? fallback;
  } catch {
    return fallback;
  }
}

export async function listOrganizationLibraries(
  filter: OrganizationLibraryFilter = {},
): Promise<OrganizationLibrary[]> {
  const params = new URLSearchParams();
  if (filter.status) params.set("status", filter.status);
  if (filter.category) params.set("category", filter.category);
  if (filter.ecosystem) params.set("ecosystem", filter.ecosystem);
  if (filter.strategic !== undefined) {
    params.set("strategic", filter.strategic ? "true" : "false");
  }

  const query = params.toString();
  const res = await fetch(`/api/organization/libraries${query ? `?${query}` : ""}`);

  if (!res.ok) {
    throw new Error(await errorMessage(res, "Failed to list libraries"));
  }

  return (await res.json()) as OrganizationLibrary[];
}

export async function markStrategic(
  libraryId: string,
  categoryId: string,
  justification: string,
): Promise<void> {
  const res = await fetch(
    `/api/organization/libraries/${encodeURIComponent(libraryId)}/categories/${encodeURIComponent(categoryId)}/strategic`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ justification }),
    },
  );

  if (!res.ok) {
    throw new Error(await errorMessage(res, "Failed to mark library strategic"));
  }
}

export async function unmarkStrategic(
  libraryId: string,
  categoryId: string,
  justification: string,
): Promise<void> {
  const res = await fetch(
    `/api/organization/libraries/${encodeURIComponent(libraryId)}/categories/${encodeURIComponent(categoryId)}/strategic`,
    {
      method: "DELETE",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ justification }),
    },
  );

  if (!res.ok) {
    throw new Error(await errorMessage(res, "Failed to unmark library strategic"));
  }
}
