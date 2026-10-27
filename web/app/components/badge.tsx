import type { ReactNode } from "react";

type BadgeTone = "accent" | "destructive" | "neutral";

const TONE_CLASSES: Record<BadgeTone, string> = {
  accent: "rounded bg-accent/15 text-accent px-2 py-0.5 text-xs",
  destructive: "rounded bg-destructive/15 text-destructive px-2 py-0.5 text-xs",
  neutral: "rounded border border-rule text-ink-dim px-2 py-0.5 text-xs",
};

export function Badge({ tone, children }: { tone: BadgeTone; children: ReactNode }) {
  return <span className={TONE_CLASSES[tone]}>{children}</span>;
}
