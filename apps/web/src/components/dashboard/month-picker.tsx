import { ChevronLeftIcon, ChevronRightIcon } from "lucide-react";
import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";

import { buttonVariants } from "@/components/ui/button";
import { formatMonth, shiftMonth } from "@/lib/dates";
import { intlLocale, isLocale } from "@/i18n/locales";

const arrow = buttonVariants({ variant: "secondary", size: "icon-lg", className: "bg-card hover:bg-accent" });

/**
 * Previous/next month navigation through the ?month= query parameter,
 * keeping the other parameters in `keep` (e.g. an owner filter).
 */
export function MonthPicker({
  month,
  basePath = "/",
  caption,
  keep = {},
}: {
  month: string;
  basePath?: string;
  caption?: string;
  keep?: Record<string, string | undefined>;
}) {
  const t = useTranslations("dashboard");
  const locale = useLocale();
  const label = formatMonth(month, intlLocale(isLocale(locale) ? locale : "en"));
  const href = (m: string) => {
    const params = new URLSearchParams({ month: m });
    for (const [key, value] of Object.entries(keep)) {
      if (value) {
        params.set(key, value);
      }
    }
    return `${basePath}?${params}`;
  };

  return (
    <div className="flex w-full items-center justify-between gap-2 md:w-auto">
      <Link href={href(shiftMonth(month, -1))} aria-label={t("previousMonth")} className={arrow}>
        <ChevronLeftIcon className="size-5" />
      </Link>
      <div className="min-w-40 text-center">
        <p className="font-heading text-[22px] leading-tight font-bold tracking-tight capitalize">{label}</p>
        {caption && <p className="text-[13px] text-muted-foreground">{caption}</p>}
      </div>
      <Link href={href(shiftMonth(month, 1))} aria-label={t("nextMonth")} className={arrow}>
        <ChevronRightIcon className="size-5" />
      </Link>
    </div>
  );
}
