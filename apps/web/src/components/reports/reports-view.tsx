import { useTranslations } from "next-intl";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import type { OwnerOption } from "@/lib/owners";
import { buildReport, type MonthlyCurrency, type ReportsQuery } from "@/lib/reports";

import { MonthlyChart } from "./monthly-chart";
import { ReportsFilters } from "./reports-filters";

/** The reports page body: filters, then the chart of the selected currency or an empty state. */
export function ReportsView({
  currencies,
  query,
  owners,
  userId,
}: {
  /** Per-currency data from GET /api/v1/summary/monthly. */
  currencies: MonthlyCurrency[];
  query: ReportsQuery;
  owners: OwnerOption[];
  userId: string;
}) {
  const t = useTranslations("reports");
  const common = useTranslations("common");

  // The chosen currency, or the first one when it is missing from the data.
  const selected = currencies.find((c) => c.currency === query.currency) ?? currencies[0];
  const hasExpenses =
    selected !== undefined &&
    buildReport(selected, {
      months: query.months,
      includeCurrent: query.includeCurrent,
      labels: { uncategorized: common("uncategorized"), other: common("other") },
    }).series.length > 0;

  return (
    <div className="grid gap-5">
      <ReportsFilters query={query} currencies={currencies.map((c) => c.currency)} activeCurrency={selected?.currency} owners={owners} userId={userId} />
      {query.owner && <p className="text-[13px] text-muted-foreground">{t("ownerHint")}</p>}
      <Card>
        <CardHeader>
          <CardTitle>
            <h2>{t("chartTitle")}</h2>
          </CardTitle>
        </CardHeader>
        <CardContent>
          {selected && hasExpenses ? (
            <MonthlyChart key={selected.currency} data={selected} months={query.months} includeCurrent={query.includeCurrent} scale={query.scale} />
          ) : (
            <div className="py-10 text-center" data-testid="reports-empty">
              <p className="font-heading text-lg font-bold">{t("emptyTitle")}</p>
              <p className="text-sm text-muted-foreground">{t("emptyDescription")}</p>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
