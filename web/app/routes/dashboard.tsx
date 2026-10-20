import { useAuth } from "../lib/auth";
import { RequireAuth } from "../lib/require-auth";
import { Nav } from "../components/nav";

function DashboardContent() {
  const { me } = useAuth();

  return (
    <main className="mx-auto max-w-4xl p-6">
      <Nav />
      <h1 className="font-serif text-xl font-semibold">Dashboard</h1>
      <p className="mt-4">
        Logged in as <span className="font-medium">{me?.email}</span> (
        {me?.role})
      </p>
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
