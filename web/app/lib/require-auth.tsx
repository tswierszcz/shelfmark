import { useEffect, type ReactNode } from "react";
import { useNavigate } from "react-router";

import { useAuth } from "./auth";

export function RequireAuth({ children }: { children: ReactNode }) {
  const { me, loading } = useAuth();
  const navigate = useNavigate();

  useEffect(() => {
    if (!loading && !me) {
      navigate("/login");
    }
  }, [loading, me, navigate]);

  if (loading) {
    return null;
  }

  if (!me) {
    return null;
  }

  return <>{children}</>;
}
