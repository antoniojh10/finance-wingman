import Link from "next/link";
import { useTranslations } from "next-intl";

import { SHARED, type OwnerOption } from "@/lib/owners";
import { cn } from "@/lib/utils";

/**
 * Chips to look at one member's accounts (or the shared ones) instead of the
 * whole workspace. They are links, so they work without JavaScript. Renders
 * nothing in single-member workspaces.
 */
export function OwnerFilter({
  owners,
  userId,
  value,
  hrefFor,
}: {
  owners: OwnerOption[];
  /** The signed-in user, listed first as "Mine". */
  userId: string;
  /** Active filter: a user id or "shared"; undefined for everyone. */
  value?: string;
  hrefFor: (owner: string | undefined) => string;
}) {
  const t = useTranslations("owners");
  if (owners.length < 2) {
    return null;
  }
  const others = owners.filter((o) => o.id !== userId);
  const options: { value: string | undefined; label: string }[] = [
    { value: undefined, label: t("everyone") },
    { value: userId, label: t("mine") },
    ...others.map((o) => ({ value: o.id, label: o.name })),
    { value: SHARED, label: t("shared") },
  ];

  return (
    <nav aria-label={t("filterLabel")} className="-mx-4 flex gap-2 overflow-x-auto px-4 pb-0.5 md:mx-0 md:px-0">
      {options.map((option) => {
        const active = value === option.value;
        return (
          <Link
            key={option.value ?? "all"}
            href={hrefFor(option.value)}
            aria-current={active ? "page" : undefined}
            className={cn(
              "flex h-10 shrink-0 items-center rounded-full border-[1.5px] px-4 text-sm font-bold transition-colors",
              active ? "border-foreground bg-foreground text-background" : "border-border bg-card hover:border-foreground/30",
            )}
          >
            {option.label}
          </Link>
        );
      })}
    </nav>
  );
}
