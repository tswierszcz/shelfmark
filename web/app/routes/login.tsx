import { useState, type FormEvent } from "react";
import { useNavigate } from "react-router";

import { useAuth } from "../lib/auth";
import { Button } from "../components/button";
import { Logo } from "../components/logo";

export default function Login() {
  const { login } = useAuth();
  const navigate = useNavigate();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState(false);
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(false);
    setSubmitting(true);

    try {
      await login(email, password);
      navigate("/dashboard");
    } catch {
      setError(true);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <main className="flex min-h-screen items-center justify-center p-4">
      <form
        onSubmit={handleSubmit}
        className="w-full max-w-sm space-y-4 rounded-lg border border-rule bg-bg-soft p-6"
      >
        <Logo className="mb-2" />
        <h1 className="font-serif text-xl font-semibold">Log in</h1>

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
            className="w-full rounded border border-rule bg-bg-soft px-3 py-2 text-ink"
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
            className="w-full rounded border border-rule bg-bg-soft px-3 py-2 text-ink"
          />
        </div>

        {error && (
          <p className="text-sm text-destructive">Invalid email or password.</p>
        )}

        <Button type="submit" disabled={submitting} className="w-full">
          {submitting ? "Logging in…" : "Log in"}
        </Button>
      </form>
    </main>
  );
}
