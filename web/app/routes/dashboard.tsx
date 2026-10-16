import { useNavigate } from "react-router";

import { useAuth } from "../lib/auth";
import { RequireAuth } from "../lib/require-auth";

function DashboardContent() {
  const { me, logout } = useAuth();
  const navigate = useNavigate();

  async function handleLogout() {
    await logout();
    navigate("/login");
  }

  return (
    <main className="mx-auto max-w-lg p-6">
      <h1 className="text-xl font-semibold">Dashboard</h1>
      <p className="mt-4">
        Logged in as <span className="font-medium">{me?.email}</span> (
        {me?.role})
      </p>
      <button
        type="button"
        onClick={handleLogout}
        className="mt-6 rounded bg-gray-900 px-3 py-2 text-white dark:bg-gray-100 dark:text-gray-900"
      >
        Log out
      </button>
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
