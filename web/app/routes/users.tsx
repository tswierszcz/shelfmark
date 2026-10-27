import { useEffect, useState, type FormEvent } from "react";

import { createUser, listUsers, type User } from "../lib/users";
import { RequireAuth } from "../lib/require-auth";
import { Button } from "../components/button";
import { Nav } from "../components/nav";

function UsersContent() {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [role, setRole] = useState<User["role"]>("viewer");
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);

  const [users, setUsers] = useState<User[]>([]);
  const [usersError, setUsersError] = useState<string | null>(null);

  async function refreshUsers() {
    try {
      setUsersError(null);
      const result = await listUsers();
      setUsers(result ?? []);
    } catch {
      setUsersError("Failed to load users.");
    }
  }

  useEffect(() => {
    refreshUsers();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function handleCreate(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();

    setCreateError(null);
    setCreating(true);
    try {
      await createUser(email, password, role);
      setEmail("");
      setPassword("");
      setRole("viewer");
      await refreshUsers();
    } catch (err) {
      setCreateError(err instanceof Error ? err.message : "Failed to create user.");
    } finally {
      setCreating(false);
    }
  }

  return (
    <main className="mx-auto max-w-4xl p-6">
      <Nav />
      <h1 className="font-serif text-xl font-semibold">Users</h1>

      <section className="mt-6 rounded-lg border border-rule bg-bg-soft p-4">
        <h2 className="font-serif text-lg font-medium">Add user</h2>
        <form onSubmit={handleCreate} className="mt-3 flex flex-wrap items-end gap-3">
          <div className="space-y-1">
            <label htmlFor="email" className="block text-sm font-medium">
              Email
            </label>
            <input
              id="email"
              type="email"
              required
              value={email}
              onChange={(event) => setEmail(event.target.value)}
              className="rounded border border-rule bg-bg-soft px-3 py-2 text-ink"
            />
          </div>

          <div className="space-y-1">
            <label htmlFor="password" className="block text-sm font-medium">
              Password
            </label>
            <input
              id="password"
              type="password"
              required
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              className="rounded border border-rule bg-bg-soft px-3 py-2 text-ink"
            />
          </div>

          <div className="space-y-1">
            <label htmlFor="role" className="block text-sm font-medium">
              Role
            </label>
            <select
              id="role"
              value={role}
              onChange={(event) => setRole(event.target.value as User["role"])}
              className="rounded border border-rule bg-bg-soft px-3 py-2 text-ink"
            >
              <option value="admin">admin</option>
              <option value="architect">architect</option>
              <option value="viewer">viewer</option>
            </select>
          </div>

          <Button type="submit" disabled={creating}>
            {creating ? "Adding…" : "Add user"}
          </Button>
        </form>

        {createError && <p className="mt-2 text-sm text-destructive">{createError}</p>}
      </section>

      <section className="mt-6 rounded-lg border border-rule bg-bg-soft p-4">
        <h2 className="font-serif text-lg font-medium">Existing users</h2>

        {usersError && <p className="mt-2 text-sm text-destructive">{usersError}</p>}

        <div className="mt-4 overflow-x-auto">
          <table className="w-full text-left text-sm">
            <thead>
              <tr className="border-b border-rule">
                <th className="py-2 pr-4">Email</th>
                <th className="py-2 pr-4">Role</th>
              </tr>
            </thead>
            <tbody>
              {users.map((user) => (
                <tr key={user.email} className="border-b border-rule">
                  <td className="py-2 pr-4">{user.email}</td>
                  <td className="py-2 pr-4">{user.role}</td>
                </tr>
              ))}
              {users.length === 0 && (
                <tr>
                  <td colSpan={2} className="py-4 text-center text-ink-faint">
                    No users yet.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </section>
    </main>
  );
}

export default function Users() {
  return (
    <RequireAuth role="admin">
      <UsersContent />
    </RequireAuth>
  );
}
