import { ChevronLeftIcon, ChevronRightIcon } from "lucide-react";
import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";

import { Button } from "@/components/ui/button";
import { formatMonth, shiftMonth } from "@/lib/dates";
import { intlLocale, isLocale } from "@/i18n/locales";

/** Previous/next month navigation through the ?month= query parameter. */
export function MonthPicker({ month, basePath = "/" }: { month: string; basePath?: string }) {
  const t = useTranslations("dashboard");
  const locale = useLocale();
  const label = formatMonth(month, intlLocale(isLocale(locale) ? locale : "en"));

  return (
    <div className="flex items-center gap-1">
      <Button
        nativeButton={false}
        render={<Link href={`${basePath}?month=${shiftMonth(month, -1)}`} aria-label={t("previousMonth")} />}
        variant="ghost"
        size="icon"
      >
        <ChevronLeftIcon />
      </Button>
      <span className="min-w-36 text-center text-sm font-medium capitalize">{label}</span>
      <Button
        nativeButton={false}
        render={<Link href={`${basePath}?month=${shiftMonth(month, 1)}`} aria-label={t("nextMonth")} />}
        variant="ghost"
        size="icon"
      >
        <ChevronRightIcon />
      </Button>
    </div>
  );
}
