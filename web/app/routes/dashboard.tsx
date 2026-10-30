import { useEffect, useState } from "react";
import { Link } from "react-router";

import { useAuth, type Me } from "../lib/auth";
import { getDashboardStats, type DashboardStats } from "../lib/dashboard";
import { RequireAuth } from "../lib/require-auth";
import { Badge } from "../components/badge";
import { Nav } from "../components/nav";

const LINK_CLASSES = "text-accent underline underline-offset-2";

function Tile({ label, value, detail }: { label: string; value: number; detail?: string }) {
  return (
    <div className="rounded border border-rule p-4">
      <p className="text-xs uppercase tracking-[.09em] text-ink-faint">{label}</p>
      <p className="mt-1 font-serif text-2xl font-semibold">{value}</p>
      {detail && <p className="text-xs text-ink-dim">{detail}</p>}
    </div>
  );
}

// CallToAction is the one next step each role gets: admin fixes libraries
// the pipeline could not categorize, architect decides on redundant
// categories, viewer has nothing to act on.
function CallToAction({ role, stats }: { role: Me["role"]; stats: DashboardStats }) {
  const needsManual = stats.libraries.by_status?.needs_manual ?? 0;

  if (role === "admin" && needsManual > 0) {
    return (
      <p className="rounded border border-rule bg-bg-soft p-4 text-sm">
        <Link to="/catalog" className={LINK_CLASSES}>
          {needsManual} {needsManual === 1 ? "library needs" : "libraries need"} manual review
        </Link>
      </p>
    );
  }
  if (role === "architect" && stats.unreviewed_hotspots > 0) {
    return (
      <p className="rounded border border-rule bg-bg-soft p-4 text-sm">
        <Link to="/organization" className={LINK_CLASSES}>
          {stats.unreviewed_hotspots} redundant{" "}
          {stats.unreviewed_hotspots === 1 ? "category" : "categories"} without a strategic decision
        </Link>
      </p>
    );
  }
  return null;
}

function Statistics({ role, stats }: { role: Me["role"]; stats: DashboardStats }) {
  const byStatus = stats.libraries.by_status ?? {};
  const ecosystems = stats.libraries.by_ecosystem ?? [];
  const hotspots = stats.hotspots ?? [];
  const statuses = [
    { label: "Categorized", count: byStatus.categorized ?? 0 },
    { label: "Needs review", count: byStatus.needs_manual ?? 0 },
    { label: "Failed", count: byStatus.failed ?? 0 },
    { label: "Processing", count: (byStatus.pending ?? 0) + (byStatus.processing ?? 0) },
  ];

  return (
    <div className="mt-6 space-y-8">
      <CallToAction role={role} stats={stats} />

      <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
        <Tile label="Libraries" value={stats.libraries.total} />
        <Tile
          label="Categories in use"
          value={stats.categories.in_use}
          detail={`of ${stats.categories.total} in the taxonomy`}
        />
        <Tile label="Projects" value={stats.projects} />
        <Tile label="Strategic marks" value={stats.strategic_marks} />
      </div>

      <section>
        <h2 className="font-serif text-lg font-semibold">Catalog status</h2>
        <ul className="mt-2 divide-y divide-rule border-y border-rule text-sm">
          {statuses.map((status) => (
            <li key={status.label} className="flex justify-between py-2">
              <span className="text-ink-dim">{status.label}</span>
              <span>{status.count}</span>
            </li>
          ))}
        </ul>
      </section>

      <section>
        <h2 className="font-serif text-lg font-semibold">Redundancy hotspots</h2>
        <p className="mt-1 text-sm text-ink-faint">
          Categories with two or more libraries used directly by projects.
        </p>
        {hotspots.length === 0 ? (
          <p className="mt-2 text-sm text-ink-dim">No redundancy found yet.</p>
        ) : (
          <ul className="mt-2 divide-y divide-rule border-y border-rule text-sm">
            {hotspots.map((hotspot) => (
              <li key={hotspot.category_id} className="flex items-center justify-between gap-4 py-2">
                <span>{hotspot.category_name}</span>
                <span className="flex items-center gap-3">
                  <span className="text-ink-dim">{hotspot.library_count} libraries</span>
                  {hotspot.reviewed ? (
                    <Badge tone="accent">Reviewed</Badge>
                  ) : (
                    <Badge tone="neutral">No decision</Badge>
                  )}
                </span>
              </li>
            ))}
          </ul>
        )}
      </section>

      <section>
        <h2 className="font-serif text-lg font-semibold">Ecosystems</h2>
        <ul className="mt-2 divide-y divide-rule border-y border-rule text-sm">
          {ecosystems.map((ecosystem) => (
            <li key={ecosystem.ecosystem} className="flex justify-between py-2">
              <span className="text-ink-dim">{ecosystem.ecosystem}</span>
              <span>{ecosystem.count}</span>
            </li>
          ))}
        </ul>
      </section>
    </div>
  );
}

function DashboardContent() {
  const { me } = useAuth();
  const [stats, setStats] = useState<DashboardStats | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let active = true;

    getDashboardStats()
      .then((result) => {
        if (active) setStats(result);
      })
      .catch((err: unknown) => {
        if (active) {
          setError(err instanceof Error ? err.message : "Failed to load dashboard statistics");
        }
      });

    return () => {
      active = false;
    };
  }, []);

  return (
    <main className="mx-auto max-w-4xl p-6">
      <Nav />
      <h1 className="font-serif text-xl font-semibold">Dashboard</h1>
      <p className="mt-1 text-sm text-ink-faint">
        Logged in as <span className="text-ink-dim">{me?.email}</span> ({me?.role})
      </p>

      {error ? (
        <p className="mt-6 text-sm text-destructive">{error}</p>
      ) : !stats ? (
        <p className="mt-6 text-sm text-ink-faint">Loading statistics…</p>
      ) : stats.libraries.total === 0 ? (
        <p className="mt-6 text-sm text-ink-dim">
          {me?.role === "admin" ? (
            <Link to="/catalog" className={LINK_CLASSES}>
              Upload a catalog to get started
            </Link>
          ) : (
            "The catalog is empty."
          )}
        </p>
      ) : (
        me && <Statistics role={me.role} stats={stats} />
      )}
    </main>
  );
}

export default function Dashboard() {
  return (
    <RequireAuth>
      <DashboardContent />
    </RequireAuth>
  );
}
