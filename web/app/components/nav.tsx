import { useState } from "react";
import { Link, useLocation, useNavigate } from "react-router";

import { useAuth, type Me } from "../lib/auth";
import { Button } from "./button";
import { Logo } from "./logo";

const LINKS: { to: string; label: string; roles: Me["role"][] }[] = [
  { to: "/dashboard", label: "Dashboard", roles: ["admin", "architect", "viewer"] },
  { to: "/catalog", label: "Catalog", roles: ["admin"] },
  { to: "/organization", label: "Organization", roles: ["architect", "viewer"] },
  { to: "/redundancy", label: "Redundancy", roles: ["architect", "viewer"] },
];

export function Nav() {
  const { me, logout } = useAuth();
  const location = useLocation();
  const navigate = useNavigate();
  const [logoutError, setLogoutError] = useState(false);

  async function handleLogout() {
    setLogoutError(false);
    try {
      await logout();
      navigate("/login");
    } catch {
      setLogoutError(true);
    }
  }

  const links = LINKS.filter((link) => me && link.roles.includes(me.role));

  return (
    <nav className="mb-8 flex flex-wrap items-center justify-between gap-4 border-b border-rule pb-4">
      <Link to="/dashboard" aria-label="Dashboard">
        <Logo />
      </Link>

      <div className="flex flex-wrap items-center gap-1">
        {links.map((link) => {
          const active = location.pathname === link.to;
          return (
            <Link
              key={link.to}
              to={link.to}
              aria-current={active ? "page" : undefined}
              className={
                active
                  ? "rounded-full bg-accent px-3 py-1.5 text-sm font-medium text-bg"
                  : "rounded-full px-3 py-1.5 text-sm text-ink-dim hover:text-ink"
              }
            >
              {link.label}
            </Link>
          );
        })}
      </div>

      <div className="flex items-center gap-3 text-sm text-ink-dim">
        {logoutError && (
          <span className="text-destructive">Log out failed.</span>
        )}
        <span className="hidden sm:inline">{me?.email}</span>
        <Button
          type="button"
          variant="secondary"
          onClick={handleLogout}
          className="px-3 py-1.5 text-sm"
        >
          Log out
        </Button>
      </div>
    </nav>
  );
}
