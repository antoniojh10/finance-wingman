"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useTranslations } from "next-intl";

import { activeGroceriesTab, GROCERIES_TABS } from "@/lib/groceries";
import { cn } from "@/lib/utils";

/** Sub-navigation of the groceries section, driven by GROCERIES_TABS. */
export function GroceriesTabs() {
  const t = useTranslations("groceries");
  const active = activeGroceriesTab(usePathname());

  return (
    <nav aria-label={t("tabs.label")} className="mb-6 flex gap-1 overflow-x-auto rounded-2xl bg-muted p-1 md:w-fit">
      {GROCERIES_TABS.map((tab) => (
        <Link
          key={tab.key}
          href={tab.href}
          aria-current={active === tab.key ? "page" : undefined}
          className={cn(
            "flex min-h-11 min-w-0 flex-1 items-center justify-center rounded-xl px-5 text-sm font-bold whitespace-nowrap text-muted-foreground transition-colors hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none",
            active === tab.key && "bg-card text-foreground shadow-sm",
          )}
        >
          {t(`tabs.${tab.key}`)}
        </Link>
      ))}
    </nav>
  );
}
