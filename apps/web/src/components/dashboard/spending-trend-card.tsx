import Link from "next/link";
import { useTranslations } from "next-intl";

import { MonthlyChart } from "@/components/reports/monthly-chart";
import { Card, CardAction, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { buildReport, type MonthlyCurrency } from "@/lib/reports";

/** The months the card shows, the one in progress included. */
export const TREND_MONTHS = 3;

/**
 * The last three months of spending by category on the dashboard: the
 * reports chart in its compact form, for the first currency that has
 * expenses. The full report is one link away.
 */
export function SpendingTrendCard({ currencies, href }: { currencies: MonthlyCurrency[]; href: string }) {
  const t = useTranslations("dashboard");
  const common = useTranslations("common");
  const labels = { uncategorized: common("uncategorized"), other: common("other") };
  const selected = currencies.find((c) => buildReport(c, { months: TREND_MONTHS, includeCurrent: true, labels }).series.length > 0);

  return (
    <Card data-testid="spending-trend-card">
      <CardHeader>
        <CardTitle>
          <h2>{t("spendingTrend")}</h2>
        </CardTitle>
        <CardAction>
          <Link href={href} className="flex min-h-9 items-center text-sm font-bold text-primary">
            {t("seeReport")}
          </Link>
        </CardAction>
      </CardHeader>
      <CardContent>
        {selected ? (
          <MonthlyChart key={selected.currency} data={selected} months={TREND_MONTHS} includeCurrent scale="amount" compact />
        ) : (
          <p className="py-2 text-sm text-muted-foreground" data-testid="spending-trend-empty">
            {t("spendingTrendEmpty")}
          </p>
        )}
      </CardContent>
    </Card>
  );
}
