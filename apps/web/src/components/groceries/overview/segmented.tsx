import Link from "next/link";

import { cn } from "@/lib/utils";

export type SegmentOption = { key: string; label: string; href: string; active: boolean };

/** A pill switch of links, so every choice lives in the URL. */
export function Segmented({ label, options, className }: { label: string; options: SegmentOption[]; className?: string }) {
  return (
    <nav aria-label={label} className={cn("flex shrink-0 gap-1 rounded-2xl bg-muted p-1", className)}>
      {options.map((option) => (
        <Link
          key={option.key}
          href={option.href}
          scroll={false}
          aria-current={option.active ? "page" : undefined}
          className={cn(
            "flex min-h-11 flex-1 items-center justify-center rounded-xl px-3.5 text-sm font-bold whitespace-nowrap text-muted-foreground transition-colors hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none",
            option.active && "bg-card text-foreground shadow-sm",
          )}
        >
          {option.label}
        </Link>
      ))}
    </nav>
  );
}
