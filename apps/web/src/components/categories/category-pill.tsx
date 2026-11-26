import type { CSSProperties } from "react";

import { cn } from "@/lib/utils";

/**
 * A category name as a rounded pill with a color dot. The tint and ink are
 * mixed with the theme's card and foreground colors (like `ColorTile`), so any
 * stored color stays readable in light and dark mode. Categories without a
 * color fall back to the muted foreground.
 */
export function CategoryPill({ name, color, className }: { name: string; color?: string | null; className?: string }) {
  return (
    <span
      data-testid="category-pill"
      className={cn(
        "inline-flex max-w-full items-center gap-1.5 rounded-full bg-[color-mix(in_oklab,var(--tile)_16%,var(--card))] py-0.5 pr-2.5 pl-2 text-[12.5px] leading-snug font-semibold text-[color-mix(in_oklab,var(--tile)_58%,var(--foreground))]",
        className,
      )}
      style={{ "--tile": color ?? "var(--muted-foreground)" } as CSSProperties}
    >
      <span aria-hidden className="size-[7px] shrink-0 rounded-full bg-[var(--tile)]" />
      <span className="truncate">{name}</span>
    </span>
  );
}
