export type User = { email: string; role: "admin" | "architect" | "viewer" };

async function errorMessage(res: Response, fallback: string): Promise<string> {
  try {
    const body = (await res.json()) as { error?: string };
    return body.error ?? fallback;
  } catch {
    return fallback;
  }
}

export async function listUsers(): Promise<User[]> {
  const res = await fetch("/api/users");

  if (!res.ok) {
    throw new Error(await errorMessage(res, "Failed to list users"));
  }

  return (await res.json()) as User[];
}

export async function createUser(
  email: string,
  password: string,
  role: User["role"],
): Promise<void> {
  const res = await fetch("/api/users", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email, password, role }),
  });

  if (!res.ok) {
    throw new Error(await errorMessage(res, "Failed to create user"));
  }
}
