export type EcosystemCount = {
  ecosystem: string;
  count: number;
};

export type Hotspot = {
  category_id: string;
  category_name: string;
  library_count: number;
  reviewed: boolean;
};

export type DashboardStats = {
  libraries: {
    total: number;
    by_status: Record<string, number>;
    by_ecosystem: EcosystemCount[];
  };
  categories: {
    total: number;
    in_use: number;
  };
  projects: number;
  strategic_marks: number;
  hotspots: Hotspot[];
  unreviewed_hotspots: number;
};

async function errorMessage(res: Response, fallback: string): Promise<string> {
  try {
    const body = (await res.json()) as { error?: string };
    return body.error ?? fallback;
  } catch {
    return fallback;
  }
}

export async function getDashboardStats(): Promise<DashboardStats> {
  const res = await fetch("/api/dashboard");

  if (!res.ok) {
    throw new Error(await errorMessage(res, "Failed to load dashboard statistics"));
  }

  return (await res.json()) as DashboardStats;
}
