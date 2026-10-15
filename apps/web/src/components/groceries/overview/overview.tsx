import Link from "next/link";
import { useTranslations } from "next-intl";

import { MonthPicker } from "@/components/dashboard/month-picker";
import { Money } from "@/components/money";
import { buttonVariants } from "@/components/ui/button";
import { minorUnitsOf } from "@/lib/groceries";
import { categoryGap, nonFoodShare, overviewHref, type Basket, type Breakdown, type Comparison, type OverviewQuery } from "@/lib/groceries-insights";
import { cn } from "@/lib/utils";

import { BasketCard } from "./basket-card";
import { BreakdownCard, TopProducts } from "./breakdown-card";
import { CompareMatrix } from "./compare-matrix";
import { Segmented } from "./segmented";

function Kpi({ label, children, hint, testId }: { label: string; children: React.ReactNode; hint?: React.ReactNode; testId: string }) {
  return (
    <div className="grid content-start gap-1 rounded-3xl bg-card px-5 py-4 ring-1 ring-foreground/5" data-testid={testId}>
      <p className="text-[13px] font-bold text-muted-foreground">{label}</p>
      <p className="font-heading text-2xl leading-tight font-bold tracking-tight">{children}</p>
      {hint && <div className="text-[13px] text-muted-foreground">{hint}</div>}
    </div>
  );
}

/** The four figures on top of the month: receipts vs category, count, non-food and coverage. */
export function OverviewKpis({ data, month }: { data: Breakdown; month: string }) {
  const t = useTranslations("groceries.overview");
  const minorUnits = minorUnitsOf(data.currency);
  const money = (amount: number, className?: string) => <Money amount={amount} currency={data.currency} minorUnits={minorUnits} className={className} />;
  const gap = categoryGap(data);
  const percent = (value: number) => `${Math.round(value)}%`;

  return (
    <div className="grid grid-cols-2 gap-3 lg:grid-cols-4" data-testid="groceries-kpis">
      <Kpi
        label={t("kpiReceipts")}
        testId="kpi-receipts"
        hint={
          gap === null ? (
            t("noCategory")
          ) : (
            <>
              {t("categoryTotal")} {money(data.groceries_category_minor ?? 0)} ·
              {gap !== 0 && (
                <span className={cn("ml-1 font-bold", gap > 0 ? "text-expense" : "text-income")}>
                  {gap > 0 ? "+" : "−"}
                  {money(Math.abs(gap))}
                </span>
              )}
            </>
          )
        }
      >
        {money(data.total_minor)}
      </Kpi>
      <Kpi label={t("kpiCount")} testId="kpi-count" hint={<>{t("average")} {money(data.average_receipt_minor)}</>}>
        {data.receipt_count}
      </Kpi>
      <Kpi label={t("kpiNonFood")} testId="kpi-non-food" hint={t("ofLines", { percent: percent(nonFoodShare(data)) })}>
        {money(data.non_food_minor)}
      </Kpi>
      <Kpi
        label={t("kpiCoverage")}
        testId="kpi-coverage"
        hint={
          data.unassigned_minor > 0 ? (
            <>
              {t("unassignedTotal")} {money(data.unassigned_minor)} ·{" "}
              <Link href={`/groceries/receipts?month=${month}`} className="font-bold text-primary underline-offset-2 hover:underline">
                {t("reviewReceipts")}
              </Link>
            </>
          ) : (
            t("allAssigned")
          )
        }
      >
        {percent(data.coverage_pct)}
      </Kpi>
    </div>
  );
}

/**
 * The Summary tab: month and currency filters, the month's spend, the store
 * comparison matrix and the basket. Every choice is a link, so the page is a
 * pure function of the URL and the three insight responses.
 */
export function GroceriesOverview({
  query,
  month,
  currencies,
  currency,
  breakdown,
  comparison,
  basket,
  basketMonths,
}: {
  query: OverviewQuery;
  /** The month on screen (YYYY-MM). */
  month: string;
  /** Currencies with any data. */
  currencies: string[];
  /** The currency on screen. */
  currency: string | undefined;
  breakdown: Breakdown | undefined;
  comparison: Comparison | undefined;
  basket: Basket | undefined;
  basketMonths: number;
}) {
  const t = useTranslations("groceries.overview");

  if (!currency) {
    return (
      <div className="grid justify-items-center gap-4 rounded-3xl bg-card px-6 py-12 text-center ring-1 ring-foreground/5" data-testid="overview-empty">
        <p className="max-w-md text-sm text-muted-foreground">{t("empty")}</p>
        <Link href="/groceries/receipts" className={buttonVariants({ variant: "secondary" })}>
          {t("goToReceipts")}
        </Link>
      </div>
    );
  }

  const active = { ...query, month, currency };

  return (
    <div className="grid grid-cols-[minmax(0,1fr)] gap-8">
      <div className="flex flex-wrap items-center gap-x-4 gap-y-3 max-md:justify-between">
        <MonthPicker month={month} basePath="/groceries" keep={{ currency, by: query.by === "group" ? undefined : query.by, level: query.level === "product" ? undefined : query.level, basket: query.basket === "comparable" ? undefined : query.basket }} />
        {currencies.length > 1 && (
          <Segmented
            label={t("currency")}
            className="max-md:w-full"
            options={currencies.map((code) => ({ key: code, label: code, href: overviewHref(active, { currency: code }), active: code === currency }))}
          />
        )}
      </div>

      <section className="grid grid-cols-[minmax(0,1fr)] gap-4" aria-label={t("spendTitle")}>
        {breakdown && breakdown.receipt_count > 0 ? (
          <>
            <OverviewKpis data={breakdown} month={month} />
            <div className="grid gap-4 lg:grid-cols-2 [&>*]:min-w-0">
              <BreakdownCard data={breakdown} query={active} />
              <TopProducts data={breakdown} />
            </div>
          </>
        ) : (
          <div className="grid justify-items-center gap-3 rounded-3xl bg-card px-6 py-10 text-center ring-1 ring-foreground/5" data-testid="spend-empty">
            <p className="max-w-md text-sm text-muted-foreground">{t("emptyMonth")}</p>
            <Link href="/groceries/receipts?month=all" className={buttonVariants({ variant: "secondary" })}>
              {t("goToReceipts")}
            </Link>
          </div>
        )}
      </section>

      <CompareMatrix data={comparison} query={active} />
      <BasketCard data={basket} months={basketMonths} query={active} />
    </div>
  );
}
