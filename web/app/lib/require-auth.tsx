import { useEffect, type ReactNode } from "react";
import { useNavigate } from "react-router";

import { useAuth } from "./auth";

export function RequireAuth({
  children,
  role,
}: {
  children: ReactNode;
  role?: "admin" | "architect" | "viewer";
}) {
  const { me, loading } = useAuth();
  const navigate = useNavigate();

  const wrongRole = !loading && !!me && role !== undefined && me.role !== role;

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
    return null;
  }

  if (!me || wrongRole) {
    return null;
  }

  return <>{children}</>;
}
