import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";

import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { intlLocale, isLocale } from "@/i18n/locales";
import { formatMoney } from "@/lib/money";
import {
  buildComparison,
  deltaTone,
  formatShortMonth,
  heatShare,
  monthTransactionsHref,
  type ComparisonRow,
  type MonthlyCurrency,
} from "@/lib/reports";
import { cn } from "@/lib/utils";

const STICKY = "sticky left-0 z-10 bg-card";

/**
 * Category x month grid of the expenses of one currency, with each
 * category's average over the closed months and how the last closed month
 * compares. Every amount links to that month's expenses. It uses the same
 * filters as the chart and is also its accessible, tabular view.
 */
export function ComparisonTable({
  data,
  months,
  includeCurrent,
  owner,
}: {
  data: MonthlyCurrency;
  months: number;
  includeCurrent: boolean;
  owner?: string;
}) {
  const t = useTranslations("reports");
  const common = useTranslations("common");
  const appLocale = useLocale();
  const locale = intlLocale(isLocale(appLocale) ? appLocale : "en");

  const grid = buildComparison(data, { months, includeCurrent, labels: { uncategorized: common("uncategorized"), other: common("other") } });
  const money = (amount: number) => formatMoney(Math.round(amount), data.currency, data.minor_units, locale);
  const percent = new Intl.NumberFormat(locale, { style: "percent", maximumFractionDigits: 0, signDisplay: "exceptZero" });
  const lastClosed = grid.lastClosedIndex >= 0 ? formatShortMonth(grid.months[grid.lastClosedIndex], locale) : null;

  const deltaLabel = lastClosed ? t("tableDelta", { month: lastClosed }) : t("tableDeltaNone");

  // Average and Δ are rendered twice: right after the category on phones and
  // after the months from md up. The unused copy is display:none, so it is
  // hidden from assistive tech too.
  const summaryCells = (row: ComparisonRow, tone: ReturnType<typeof deltaTone>, visibility: string, suffix: string) => (
    <>
      <TableCell className={cn("text-right tabular-nums", visibility)} data-testid={`comparison-average${suffix}`}>
        {row.average > 0 ? money(row.average) : "—"}
      </TableCell>
      <TableCell
        className={cn("text-right tabular-nums", visibility, tone === "up" && "text-expense", tone === "down" && "text-income")}
        data-testid={`comparison-delta${suffix}`}
        data-tone={tone ?? undefined}
      >
        {row.delta === null ? "—" : percent.format(row.delta)}
      </TableCell>
    </>
  );

  const renderRow = (row: ComparisonRow, isTotal: boolean) => {
    const tone = deltaTone(row.delta);
    return (
      <TableRow key={row.key ?? "total"} data-testid={isTotal ? "comparison-total" : `comparison-row-${row.key}`} className={cn(isTotal && "border-t-2 font-semibold")}>
        <TableHead scope="row" className={cn(STICKY, "max-w-40 truncate text-foreground")}>
          <span className="flex items-center gap-2">
            {!isTotal && <i aria-hidden className="size-2.5 shrink-0 rounded-[3px]" style={{ background: row.color ?? "var(--muted-foreground)" }} />}
            <span className="truncate">{isTotal ? t("tableTotal") : row.name}</span>
          </span>
        </TableHead>
        {summaryCells(row, tone, "md:hidden", "-mobile")}
        {row.values.map((value, i) => {
          const share = isTotal ? 0 : heatShare(row, i, grid.partialIndex);
          return (
            <TableCell
              key={grid.months[i]}
              className="text-right tabular-nums"
              style={share > 0 ? { background: `color-mix(in oklab, var(--primary) ${Math.round(share * 26)}%, transparent)` } : undefined}
            >
              {value > 0 ? (
                <Link
                  href={monthTransactionsHref(grid.months[i], row.categoryId, owner)}
                  className="rounded-sm underline-offset-2 hover:underline focus-visible:outline-2 focus-visible:outline-ring"
                >
                  {money(value)}
                </Link>
              ) : (
                "—"
              )}
            </TableCell>
          );
        })}
        {summaryCells(row, tone, "hidden md:table-cell", "")}
      </TableRow>
    );
  };

  return (
    <div data-testid="reports-table">
      <p className="mb-3 text-[13px] text-muted-foreground">{t("tableHint")}</p>
      <Table>
        <caption className="sr-only">{t("tableCaption")}</caption>
        <TableHeader>
          <TableRow>
            <TableHead className={STICKY}>{t("tableCategory")}</TableHead>
            <TableHead className="text-right md:hidden">{t("tableAverage")}</TableHead>
            <TableHead className="text-right md:hidden">{deltaLabel}</TableHead>
            {grid.months.map((month, i) => (
              <TableHead key={month} className="text-right" data-partial={i === grid.partialIndex ? "true" : undefined}>
                {formatShortMonth(month, locale)}
                {i === grid.partialIndex && (
                  <>
                    <span aria-hidden>*</span>
                    <span className="sr-only"> ({t("inProgress")})</span>
                  </>
                )}
              </TableHead>
            ))}
            <TableHead className="hidden text-right md:table-cell">{t("tableAverage")}</TableHead>
            <TableHead className="hidden text-right md:table-cell">{deltaLabel}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {grid.rows.map((row) => renderRow(row, false))}
          {renderRow(grid.total, true)}
        </TableBody>
      </Table>
      {grid.partialIndex >= 0 && <p className="mt-2 text-xs text-muted-foreground">* {t("tablePartialNote")}</p>}
    </div>
  );
}
