import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";

import { BudgetBar } from "@/components/budgets/budget-bar";
import { BudgetStateBadge } from "@/components/budgets/budget-state-badge";
import { ColorTile } from "@/components/color-tile";
import { Money } from "@/components/money";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { intlLocale, isLocale } from "@/i18n/locales";
import { formatMonth } from "@/lib/dates";
import { budgetState, type BudgetCurrencyData, type BudgetLineData } from "@/lib/budgets";

/** A labelled figure of a budget line. */
function Figure({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex items-baseline gap-1.5">
      <dt className="text-muted-foreground">{label}</dt>
      <dd className="font-semibold">{children}</dd>
    </div>
  );
}

function BudgetRow({
  line,
  month,
  currency,
  minorUnits,
}: {
  line: BudgetLineData;
  month: string;
  currency: string;
  minorUnits: number;
}) {
  const t = useTranslations("budgets");
  const currentLocale = useLocale();
  const locale = intlLocale(isLocale(currentLocale) ? currentLocale : "en");
  const name = line.category_name ?? t("uncategorized");
  const money = (amount: number) => <Money amount={amount} currency={currency} minorUnits={minorUnits} />;
  const inherited = line.amount !== null && line.amount_month !== null && line.amount_month !== month;

  return (
    <li className="grid gap-2 py-3" data-testid="budget-row">
      <div className="flex items-center gap-3">
        <ColorTile color={line.category_color} label={name} className="size-10 rounded-[13px]" />
        <div className="min-w-0 flex-1">
          <p className="truncate text-[15px] font-semibold">{name}</p>
          {line.amount === null ? (
            <p className="text-[12.5px] text-muted-foreground">{t("noBudget")}</p>
          ) : (
            <p className="text-[12.5px] text-muted-foreground">
              {t("budget")} <Money amount={line.amount} currency={currency} minorUnits={minorUnits} />
              {inherited && line.amount_month && (
                <span data-testid="inherited-hint"> · {t("inheritedFrom", { month: formatMonth(line.amount_month, locale) })}</span>
              )}
            </p>
          )}
        </div>
        {line.amount !== null && line.state !== "none" && <BudgetStateBadge state={line.state} />}
      </div>
      {line.amount !== null && line.state !== "none" && (
        <BudgetBar amount={line.amount} spent={line.spent} committed={line.committed} state={line.state} />
      )}
      <dl className="flex flex-wrap gap-x-4 gap-y-0.5 text-[13px]">
        <Figure label={t("spent")}>{money(line.spent)}</Figure>
        {(line.committed > 0 || line.amount !== null) && <Figure label={t("committed")}>{money(line.committed)}</Figure>}
        {line.remaining !== null &&
          (line.remaining < 0 ? (
            <Figure label={t("overBy")}>{money(-line.remaining)}</Figure>
          ) : (
            <Figure label={t("remaining")}>{money(line.remaining)}</Figure>
          ))}
      </dl>
    </li>
  );
}

/** Totals of a currency, with an overall bar when something is budgeted. */
function CurrencySummary({ data }: { data: BudgetCurrencyData }) {
  const t = useTranslations("budgets");
  const state = budgetState(data.budgeted, data.spent + data.committed);
  const money = (amount: number) => <Money amount={amount} currency={data.currency} minorUnits={data.minor_units} />;

  return (
    <div className="grid gap-3" data-testid="budget-summary">
      {data.budgeted > 0 && state !== "none" && (
        <BudgetBar amount={data.budgeted} spent={data.spent} committed={data.committed} state={state} className="h-3.5" />
      )}
      <dl className="grid grid-cols-2 gap-x-4 gap-y-2 text-sm sm:grid-cols-4">
        <div>
          <dt className="text-muted-foreground">{t("budgeted")}</dt>
          <dd className="text-base font-bold">{money(data.budgeted)}</dd>
        </div>
        <div>
          <dt className="text-muted-foreground">{t("spent")}</dt>
          <dd className="text-base font-bold">{money(data.spent)}</dd>
        </div>
        <div>
          <dt className="text-muted-foreground">{t("committed")}</dt>
          <dd className="text-base font-bold">{money(data.committed)}</dd>
        </div>
        <div>
          <dt className="text-muted-foreground">{data.remaining < 0 ? t("overBy") : t("remaining")}</dt>
          <dd className="text-base font-bold">{money(Math.abs(data.remaining))}</dd>
        </div>
      </dl>
    </div>
  );
}

/**
 * Budget status of a month: one card per currency with the totals, a row per
 * category and the spending that has no category (which has no budget).
 */
export function BudgetList({
  currencies,
  month,
  editHref,
}: {
  currencies: BudgetCurrencyData[];
  month: string;
  /** Link to the edit mode, offered when there is nothing to show. */
  editHref: string;
}) {
  const t = useTranslations("budgets");

  if (currencies.length === 0) {
    return (
      <Card className="mx-auto max-w-md text-center">
        <CardHeader>
          <CardTitle>
            <h2>{t("emptyTitle")}</h2>
          </CardTitle>
          <p className="text-sm text-muted-foreground">{t("emptyDescription")}</p>
        </CardHeader>
        <CardContent>
          <Button nativeButton={false} render={<Link href={editHref} />}>
            {t("setBudgets")}
          </Button>
        </CardContent>
      </Card>
    );
  }

  return (
    <div className="grid gap-6">
      {currencies.map((data) => {
        const rows = [...data.categories, ...(data.uncategorized ? [data.uncategorized] : [])];
        return (
          <Card key={data.currency} data-testid={`budget-${data.currency}`}>
            <CardHeader>
              <CardTitle>
                <h2>{data.currency}</h2>
              </CardTitle>
            </CardHeader>
            <CardContent className="grid gap-4">
              <CurrencySummary data={data} />
              <ul className="divide-y">
                {rows.map((line) => (
                  <BudgetRow
                    key={line.category_id ?? "uncategorized"}
                    line={line}
                    month={month}
                    currency={data.currency}
                    minorUnits={data.minor_units}
                  />
                ))}
              </ul>
            </CardContent>
          </Card>
        );
      })}
    </div>
  );
}
