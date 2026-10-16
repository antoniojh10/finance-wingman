import { useLocale, useTranslations } from "next-intl";

import { Money } from "@/components/money";
import { Badge } from "@/components/ui/badge";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { intlLocale, isLocale } from "@/i18n/locales";
import { minorUnitsOf, formatQuantity } from "@/lib/groceries";
import { breakdownBars, overviewHref, type Breakdown, type OverviewQuery } from "@/lib/groceries-insights";

import { useGroupLabel } from "../use-group-label";
import { Segmented } from "./segmented";

const FALLBACK_COLOR = "var(--muted-foreground)";

/** Spend of the month by product group (non-food tagged) or by store. */
export function BreakdownCard({ data, query }: { data: Breakdown; query: OverviewQuery }) {
  const t = useTranslations("groceries.overview");
  const minorUnits = minorUnitsOf(data.currency);
  const rows = breakdownBars(data, query.by);
  const groupName = useGroupLabel();
  const appLocale = useLocale();
  const percent = new Intl.NumberFormat(intlLocale(isLocale(appLocale) ? appLocale : "en"), { maximumFractionDigits: 0 });

  return (
    <section className="grid content-start gap-4 rounded-3xl bg-card p-5 ring-1 ring-foreground/5" aria-labelledby="breakdown-title" data-testid="groceries-breakdown">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 id="breakdown-title" className="font-heading text-lg font-bold">
          {query.by === "group" ? t("byGroup") : t("byStore")}
        </h2>
        <Segmented
          label={t("groupBy")}
          options={[
            { key: "group", label: t("groupByGroup"), href: overviewHref(query, { by: "group" }), active: query.by === "group" },
            { key: "store", label: t("groupByStore"), href: overviewHref(query, { by: "store" }), active: query.by === "store" },
          ]}
        />
      </div>
      <ul className="grid gap-3.5">
        {rows.map((row) => (
          <li key={row.key} className="grid gap-1.5" data-testid="breakdown-row">
            <div className="flex items-baseline justify-between gap-3 text-sm">
              <span className="flex min-w-0 items-center gap-2">
                <span className="truncate font-bold">{row.label === null ? t("unassigned") : row.defaultKey ? groupName({ name: row.label, default_key: row.defaultKey }) : row.label}</span>
                {row.isFood === false && (
                  <Badge variant="secondary" className="h-5 px-2 text-[11px]">
                    {t("nonFood")}
                  </Badge>
                )}
                {row.receiptCount !== undefined && <span className="shrink-0 text-[13px] text-muted-foreground">{t("receipts", { count: row.receiptCount })}</span>}
              </span>
              <span className="flex shrink-0 items-baseline gap-2">
                <Money amount={row.total} currency={data.currency} minorUnits={minorUnits} className="font-bold" />
                <span className="w-10 text-right text-[13px] text-muted-foreground tabular-nums">{percent.format(row.share)}%</span>
              </span>
            </div>
            <div className="h-2.5 rounded-full bg-muted" aria-hidden>
              <div
                className="h-full rounded-full"
                style={{
                  width: `${Math.max(row.width * 100, 1.5)}%`,
                  background: row.unassigned ? "repeating-linear-gradient(45deg, var(--muted-foreground) 0 4px, transparent 4px 8px)" : (row.color ?? FALLBACK_COLOR),
                  opacity: row.isFood === false ? 0.65 : 1,
                }}
              />
            </div>
          </li>
        ))}
      </ul>
    </section>
  );
}

/** The ten products with the most spend of the month. */
export function TopProducts({ data }: { data: Breakdown }) {
  const t = useTranslations("groceries.overview");
  const unit = useTranslations("groceries.overview.unitShort");
  const minorUnits = minorUnitsOf(data.currency);

  return (
    <section className="grid content-start gap-3 rounded-3xl bg-card p-5 ring-1 ring-foreground/5" aria-labelledby="top-products-title" data-testid="groceries-top-products">
      <h2 id="top-products-title" className="font-heading text-lg font-bold">
        {t("topProducts")}
      </h2>
      {data.top_products.length === 0 ? (
        <p className="py-6 text-center text-sm text-muted-foreground">{t("topProductsEmpty")}</p>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t("product")}</TableHead>
              <TableHead className="text-right">{t("quantity")}</TableHead>
              <TableHead className="text-right">{t("spend")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {data.top_products.map((product) => (
              <TableRow key={product.product_id}>
                <TableCell className="max-w-40 truncate font-bold">{product.name}</TableCell>
                <TableCell className="text-right whitespace-nowrap text-muted-foreground tabular-nums">
                  {formatQuantity(product.quantity)} {unit(product.compare_unit)}
                </TableCell>
                <TableCell className="text-right whitespace-nowrap">
                  <Money amount={product.spend_minor} currency={data.currency} minorUnits={minorUnits} className="font-bold" />
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
    </section>
  );
}
