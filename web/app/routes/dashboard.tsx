import { Link, useNavigate } from "react-router";

import { useAuth } from "../lib/auth";
import { RequireAuth } from "../lib/require-auth";
import { Button } from "../components/button";
import { Logo } from "../components/logo";

function DashboardContent() {
  const { me, logout } = useAuth();
  const navigate = useNavigate();

  async function handleLogout() {
    await logout();
    navigate("/login");
  }

  return (
    <main className="mx-auto max-w-lg p-6">
      <Logo className="mb-6" />
      <h1 className="font-serif text-xl font-semibold">Dashboard</h1>
      <p className="mt-4">
        Logged in as <span className="font-medium">{me?.email}</span> (
        {me?.role})
      </p>
      {me?.role === "admin" && (
        <p className="mt-2">
          <Link to="/catalog" className="text-accent underline">
            Go to catalog
          </Link>
        </p>
      )}
      <Button type="button" onClick={handleLogout} className="mt-6">
        Log out
      </Button>
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
