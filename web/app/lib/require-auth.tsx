import { useEffect, type ReactNode } from "react";
import { useNavigate } from "react-router";

import { useAuth } from "./auth";

type Role = "admin" | "architect" | "viewer";

export function RequireAuth({
  children,
  role,
}: {
  children: ReactNode;
  role?: Role | Role[];
}) {
  const { me, loading } = useAuth();
  const navigate = useNavigate();

  const wrongRole =
    !loading &&
    !!me &&
    role !== undefined &&
    (Array.isArray(role) ? !role.includes(me.role) : me.role !== role);

  useEffect(() => {
    if (!loading && !me) {
      navigate("/login");
      return;
    }
    if (wrongRole) {
      navigate("/dashboard");
    }
  }, [loading, me, navigate, wrongRole]);

  if (loading) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-bg text-ink-faint text-sm">
        Loading…
      </div>
    );
  }

  if (!me || wrongRole) {
    return null;
  }

  return <>{children}</>;
}
