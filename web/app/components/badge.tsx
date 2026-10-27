import type { ReactNode } from "react";

const TONE_CLASSES: Record<"accent" | "destructive", string> = {
  accent: "rounded bg-accent/15 text-accent px-2 py-0.5 text-xs",
  destructive: "rounded bg-destructive/15 text-destructive px-2 py-0.5 text-xs",
};

export function Badge({ tone, children }: { tone: "accent" | "destructive"; children: ReactNode }) {
  return <span className={TONE_CLASSES[tone]}>{children}</span>;
}
