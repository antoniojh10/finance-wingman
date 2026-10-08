import { TriangleAlertIcon } from "lucide-react";
import { useLocale, useTranslations } from "next-intl";

import type { BudgetImpact } from "@/app/actions/budgets";
import { intlLocale, isLocale } from "@/i18n/locales";
import { formatMoney } from "@/lib/money";

/**
 * Builds the text announcing that an expense leaves its category near or
 * over budget (null when it needs no warning). Shared by the form hint and
 * the toast after saving.
 */
export function useBudgetMessage() {
  const t = useTranslations("transactions");
  const current = useLocale();
  const locale = intlLocale(isLocale(current) ? current : "en");

  return (impact: BudgetImpact | null, category: string, currency: string, minorUnits: number): string | null => {
    if (!impact || !impact.hasBudget || !impact.warning) {
      return null;
    }
    if (impact.remaining < 0) {
      return t("budgetHintOver", { category, over: formatMoney(-impact.remaining, currency, minorUnits, locale) });
    }
    return t("budgetHintNear", { category, remaining: formatMoney(impact.remaining, currency, minorUnits, locale) });
  };
}

/** Non-blocking notice under the category: saving is never prevented. */
export function BudgetImpactHint({ message, over }: { message: string; over: boolean }) {
  return (
    <p
      role="status"
      data-testid="budget-hint"
      data-state={over ? "over" : "near"}
      className={
        over
          ? "flex items-start gap-2 rounded-2xl bg-destructive/10 px-4 py-3 text-sm font-medium text-destructive"
          : "flex items-start gap-2 rounded-2xl bg-amber-500/15 px-4 py-3 text-sm font-medium text-amber-700 dark:text-amber-400"
      }
    >
      <TriangleAlertIcon aria-hidden className="mt-0.5 size-4 shrink-0" />
      {message}
    </p>
  );
}
