import type { ComponentProps } from "react";

const VARIANT_CLASSES: Record<"primary" | "secondary", string> = {
  primary: "rounded-full bg-accent text-bg font-medium px-4 py-2 disabled:opacity-50",
  secondary: "rounded-full border border-rule text-ink px-4 py-2",
};

export function Button({
  variant = "primary",
  className,
  ...props
}: { variant?: "primary" | "secondary" } & ComponentProps<"button">) {
  const classes = `${VARIANT_CLASSES[variant]}${className ? ` ${className}` : ""}`;
  return <button className={classes} {...props} />;
}
