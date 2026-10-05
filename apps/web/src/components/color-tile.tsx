import type { CSSProperties } from "react";

import { cn } from "@/lib/utils";

/**
 * A rounded tile tinted from a data color (a category or account type).
 * The tint and ink are mixed with the theme's card and foreground colors, so
 * any stored color stays readable in light and dark mode.
 */
export function ColorTile({
  color,
  label,
  children,
  className,
}: {
  color?: string | null;
  /** Its first letter is shown when there are no children. */
  label?: string;
  children?: React.ReactNode;
  className?: string;
}) {
  return (
    <span
      aria-hidden
      className={cn(
        "flex size-10.5 shrink-0 items-center justify-center rounded-[14px] bg-[color-mix(in_oklab,var(--tile)_18%,var(--card))] font-heading text-base font-bold text-[color-mix(in_oklab,var(--tile)_55%,var(--foreground))] [&_svg:not([class*='size-'])]:size-5",
        className,
      )}
      style={{ "--tile": color ?? "var(--muted-foreground)" } as CSSProperties}
    >
      {children ?? (label?.trim().charAt(0).toUpperCase() || "?")}
    </span>
  );
}
