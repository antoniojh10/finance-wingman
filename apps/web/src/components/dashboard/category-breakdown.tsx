import { useLocale, useTranslations } from "next-intl";

import { Money } from "@/components/money";
import { intlLocale, isLocale } from "@/i18n/locales";

export type CategoryTotal = {
  category_id: string | null;
  category_name?: string | null;
  category_color?: string | null;
  total: number;
  transaction_count: number;
};

const MAX_ROWS = 6;

type Row = { key: string; name: string; color: string | null; total: number; count: number };

/** Groups totals into at most MAX_ROWS rows, folding the rest into "Other". */
export function toRows(totals: CategoryTotal[], labels: { uncategorized: string; other: string }): Row[] {
  const sorted = [...totals].sort((a, b) => b.total - a.total);
  const rows: Row[] = sorted.slice(0, MAX_ROWS).map((t) => ({
    key: t.category_id ?? "uncategorized",
    name: t.category_name ?? labels.uncategorized,
    color: t.category_color ?? null,
    total: t.total,
    count: t.transaction_count,
  }));
  const rest = sorted.slice(MAX_ROWS);
  if (rest.length > 0) {
    rows[MAX_ROWS - 1] = {
      key: "other",
      name: labels.other,
      color: null,
      total: rest.reduce((sum, t) => sum + t.total, rows[MAX_ROWS - 1].total),
      count: rest.reduce((sum, t) => sum + t.transaction_count, rows[MAX_ROWS - 1].count),
    };
  }
  return rows;
}

/**
 * Ranked horizontal bars of spending per category for one currency. A single
 * series, so every bar uses the same color; the category color is only an
 * identity dot next to its name.
 */
export function CategoryBreakdown({
  totals,
  currency,
  minorUnits,
}: {
  totals: CategoryTotal[];
  currency: string;
  minorUnits: number;
}) {
  const t = useTranslations();
  const locale = useLocale();
  const percent = new Intl.NumberFormat(intlLocale(isLocale(locale) ? locale : "en"), {
    style: "percent",
    maximumFractionDigits: 0,
  });

  if (totals.length === 0) {
    return <p className="text-sm text-muted-foreground">{t("dashboard.noSpending")}</p>;
  }

  const rows = toRows(totals, { uncategorized: t("common.uncategorized"), other: t("common.other") });
  const sum = rows.reduce((acc, r) => acc + r.total, 0);
  const max = Math.max(...rows.map((r) => r.total));

  return (
    <ul className="grid gap-3">
      {rows.map((row) => (
        <li key={row.key} className="group grid gap-1.5 rounded-md">
          <div className="flex items-baseline justify-between gap-3 text-sm">
            <span className="flex min-w-0 items-center gap-2">
              <span
                aria-hidden
                className="size-2.5 shrink-0 rounded-full border border-foreground/10"
                style={{ backgroundColor: row.color ?? "var(--muted-foreground)" }}
              />
              <span className="truncate">{row.name}</span>
              <span className="hidden text-xs text-muted-foreground group-hover:inline">
                {t("dashboard.share", { percent: percent.format(row.total / sum) })} ·{" "}
                {t("dashboard.transactionCount", { count: row.count })}
              </span>
            </span>
            <Money amount={row.total} currency={currency} minorUnits={minorUnits} className="shrink-0 font-medium" />
          </div>
          <div className="h-2 rounded-full bg-muted" aria-hidden>
            <div
              className="h-full rounded-full bg-chart-1 transition-[width]"
              style={{ width: `${Math.max((row.total / max) * 100, 1)}%` }}
            />
          </div>
        </li>
      ))}
    </ul>
  );
}
