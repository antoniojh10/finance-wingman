import { useLocale, useTranslations } from "next-intl";

import { ColorTile } from "@/components/color-tile";
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
 * Spending per category for one currency: a stacked bar showing each
 * category's share in its own color, then a ranked list with amounts.
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
    return <p className="py-6 text-center text-sm text-muted-foreground">{t("dashboard.noSpending")}</p>;
  }

  const rows = toRows(totals, { uncategorized: t("common.uncategorized"), other: t("common.other") });
  const sum = rows.reduce((acc, r) => acc + r.total, 0);

  return (
    <div className="grid gap-4">
      <div className="flex h-3.5 gap-[3px]" aria-hidden>
        {rows.map((row) => (
          <div
            key={row.key}
            className="min-w-1.5 rounded-full"
            style={{ flex: `${row.total} 1 0`, backgroundColor: row.color ?? "var(--muted-foreground)" }}
          />
        ))}
      </div>
      <ul className="grid gap-1">
        {rows.map((row) => (
          <li key={row.key} className="flex min-h-13 items-center gap-3">
            <ColorTile color={row.color} label={row.name} className="size-10 rounded-[13px]" />
            <div className="min-w-0 flex-1">
              <p className="truncate text-[15px] font-semibold">{row.name}</p>
              <p className="text-[12.5px] text-muted-foreground">
                {t("dashboard.share", { percent: percent.format(row.total / sum) })} ·{" "}
                {t("dashboard.transactionCount", { count: row.count })}
              </p>
            </div>
            <Money amount={row.total} currency={currency} minorUnits={minorUnits} className="shrink-0 text-[15px] font-bold" />
          </li>
        ))}
      </ul>
    </div>
  );
}
