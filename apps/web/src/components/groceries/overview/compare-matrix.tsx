import { useLocale, useTranslations } from "next-intl";

import { Badge } from "@/components/ui/badge";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { intlLocale, isLocale } from "@/i18n/locales";
import { formatDate } from "@/lib/dates";
import { minorUnitsOf } from "@/lib/groceries";
import { cellTone, formatUnitPrice, overviewHref, sortCompareRows, type CompareRow, type Comparison, type OverviewQuery } from "@/lib/groceries-insights";
import { cn } from "@/lib/utils";

import { Segmented } from "./segmented";

const STICKY = "sticky left-0 z-10 bg-card shadow-[1px_0_0_var(--border)]";

/**
 * Product x store matrix of the cheapest latest unit price. The best cell of
 * a row is highlighted, the most expensive muted, stores without a price show
 * "No data" and old prices are tagged. Scrolls sideways on phones with the
 * product column pinned.
 */
export function CompareMatrix({ data, query }: { data: Comparison | undefined; query: OverviewQuery }) {
  const t = useTranslations("groceries.overview");
  const appLocale = useLocale();
  const locale = intlLocale(isLocale(appLocale) ? appLocale : "en");
  const minorUnits = data ? minorUnitsOf(data.currency) : 2;

  const unitLabel = (row: CompareRow) => {
    const short = row.compare_unit === "unit" ? t("unitName", { count: row.display_multiplier }) : t(`unitShort.${row.compare_unit}`);
    return row.display_multiplier > 1 ? `${row.display_multiplier} ${short}` : short;
  };

  const detail = (cell: CompareRow["cells"][string]) =>
    [
      [cell.brand, cell.pack_label].filter(Boolean).join(" · "),
      formatDate(cell.observed_on, locale),
      cell.source === "receipt" ? t("sourceReceipt") : t("sourcePriceCheck"),
    ]
      .filter(Boolean)
      .join(" · ");

  return (
    <section className="grid min-w-0 grid-cols-[minmax(0,1fr)] gap-4 rounded-3xl bg-card p-5 ring-1 ring-foreground/5" aria-labelledby="compare-title" data-testid="groceries-compare">
      <div className="grid gap-1">
        <h2 id="compare-title" className="font-heading text-lg font-bold">
          {t("compareTitle")}
        </h2>
        <p className="text-sm text-muted-foreground">{query.level === "product" ? t("compareHelpProduct") : t("compareHelpBrand")}</p>
      </div>
      <Segmented
        label={t("compareLevel")}
        className="justify-self-start max-md:w-full"
        options={[
          { key: "product", label: t("levelProduct"), href: overviewHref(query, { level: "product" }), active: query.level === "product" },
          { key: "brand", label: t("levelBrand"), href: overviewHref(query, { level: "brand" }), active: query.level === "brand" },
        ]}
      />
      {!data || data.rows.length === 0 || data.stores.length === 0 ? (
        <p className="py-8 text-center text-sm text-muted-foreground" data-testid="compare-empty">
          {query.level === "brand" ? t("compareEmptyBrand") : t("compareEmpty")}
        </p>
      ) : (
        <div data-testid="compare-matrix">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className={cn(STICKY, "min-w-32")}>{query.level === "brand" ? t("productBrand") : t("product")}</TableHead>
                {data.stores.map((store) => (
                  <TableHead key={store.id} className="min-w-28 text-right">
                    {store.name}
                  </TableHead>
                ))}
                <TableHead className="min-w-20 text-right">{t("gap")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {sortCompareRows(data.rows).map((row) => (
                <TableRow key={`${row.product_id}-${row.brand ?? ""}`} data-testid="compare-row">
                  <TableHead scope="row" className={cn(STICKY, "max-w-40 py-3 whitespace-normal text-foreground")}>
                    <span className="block font-bold">{row.product_name}</span>
                    <span className="block text-[12px] font-normal text-muted-foreground">
                      {row.brand ? `${row.brand} · ` : ""}
                      {t("perUnit", { unit: unitLabel(row) })}
                    </span>
                  </TableHead>
                  {data.stores.map((store) => {
                    const cell = row.cells[store.id];
                    if (!cell) {
                      return (
                        <TableCell key={store.id} className="text-right text-[13px] text-muted-foreground/70 italic" data-tone="empty">
                          {t("noData")}
                        </TableCell>
                      );
                    }
                    const tone = cellTone(row, store.id);
                    return (
                      <TableCell
                        key={store.id}
                        title={detail(cell)}
                        data-tone={tone}
                        data-stale={cell.stale || undefined}
                        className={cn(
                          "text-right tabular-nums",
                          tone === "best" && "bg-income-soft font-bold text-income",
                          tone === "worst" && "text-muted-foreground",
                        )}
                      >
                        <span className="block whitespace-nowrap">{formatUnitPrice(cell.display_unit_price, data.currency, minorUnits, locale)}</span>
                        {cell.stale && (
                          <Badge variant="outline" className="mt-0.5 h-5 px-1.5 text-[11px] text-amber-800 dark:text-amber-300">
                            {t("stale")}
                          </Badge>
                        )}
                        <span className="sr-only">{detail(cell)}</span>
                      </TableCell>
                    );
                  })}
                  <TableCell className="text-right text-[13px] whitespace-nowrap text-muted-foreground tabular-nums">
                    {row.max_diff_pct === null ? "—" : `+${Math.round(row.max_diff_pct)}%`}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          <p className="mt-3 text-[13px] text-muted-foreground">{t("compareLegend")}</p>
        </div>
      )}
    </section>
  );
}
