import {
  createContext,
  createElement,
  useCallback,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from "react";

export type Me = {
  email: string;
  role: "admin" | "architect" | "viewer";
};

export async function login(email: string, password: string): Promise<Me> {
  const res = await fetch("/api/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email, password }),
  });

  if (!res.ok) {
    throw new Error("Login failed");
  }

  return (await res.json()) as Me;
}

export async function logout(): Promise<void> {
  await fetch("/api/logout", { method: "POST" });
}

export async function me(): Promise<Me | null> {
  const res = await fetch("/api/me");

  if (res.status === 401) {
    return null;
  }

  if (!res.ok) {
    throw new Error("Failed to fetch current user");
  }

  return (await res.json()) as Me;
}

type AuthContextValue = {
  me: Me | null;
  loading: boolean;
  login: (email: string, password: string) => Promise<Me>;
  logout: () => Promise<void>;
};

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [currentUser, setCurrentUser] = useState<Me | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let active = true;

    me()
      .then((result) => {
        if (active) {
          setCurrentUser(result);
        }
      })
      .finally(() => {
        if (active) {
          setLoading(false);
        }
      });

    return () => {
      active = false;
    };
  }, []);

  const handleLogin = useCallback(async (email: string, password: string) => {
    const result = await login(email, password);
    setCurrentUser(result);
    return result;
  }, []);

  const handleLogout = useCallback(async () => {
    await logout();
    setCurrentUser(null);
  }, []);

  return createElement(
    AuthContext.Provider,
    {
      value: {
        me: currentUser,
        loading,
        login: handleLogin,
        logout: handleLogout,
      },
    },
    children,
  );
}

export function useAuth(): AuthContextValue {
  const context = useContext(AuthContext);

  if (!context) {
    throw new Error("useAuth must be used within an AuthProvider");
  }

  return context;
}
