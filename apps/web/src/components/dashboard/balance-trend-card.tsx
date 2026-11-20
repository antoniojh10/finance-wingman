"use client";

import { useLocale, useTranslations } from "next-intl";
import { useState } from "react";

import { intlLocale, isLocale } from "@/i18n/locales";
import { hasTrend, type DailyCurrency } from "@/lib/balance-trend";
import { formatMoney } from "@/lib/money";
import { cn } from "@/lib/utils";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

import { BalanceTrendChart } from "./balance-trend-chart";

/**
 * The balance day by day for the month in view, one currency at a time,
 * with the projection to the month end when the month is not over.
 */
export function BalanceTrendCard({ currencies }: { currencies: DailyCurrency[] }) {
  const t = useTranslations("balanceTrend");
  const accounts = useTranslations("accounts");
  const appLocale = useLocale();
  const locale = intlLocale(isLocale(appLocale) ? appLocale : "en");
  const available = currencies.filter(hasTrend);
  const [selected, setSelected] = useState(available[0]?.currency);
  const data = available.find((c) => c.currency === selected) ?? available[0];

  return (
    <Card data-testid="balance-trend-card">
      <CardHeader>
        <CardTitle>
          <h2>{t("title")}</h2>
        </CardTitle>
        {data && data.projection.length > 0 && (
          <p className="text-[13px] text-muted-foreground" data-testid="balance-trend-caption">
            {t("caption", { amount: formatMoney(data.projection_basis.average_daily_expense, data.currency, data.minor_units, locale) })}
          </p>
        )}
      </CardHeader>
      <CardContent className="grid gap-4">
        {available.length > 1 && (
          <div role="group" aria-label={accounts("currency")} className="flex flex-wrap gap-2">
            {available.map((c) => (
              <button
                key={c.currency}
                type="button"
                aria-pressed={c.currency === data?.currency}
                onClick={() => setSelected(c.currency)}
                className={cn(
                  "h-9 min-w-16 rounded-full px-3.5 text-[13px] font-bold tracking-wide transition-colors",
                  c.currency === data?.currency ? "bg-primary text-primary-foreground" : "bg-muted text-foreground hover:bg-muted/70",
                )}
              >
                {c.currency}
              </button>
            ))}
          </div>
        )}
        {data ? (
          <BalanceTrendChart key={data.currency} data={data} />
        ) : (
          <p className="py-2 text-sm text-muted-foreground" data-testid="balance-trend-empty">
            {t("empty")}
          </p>
        )}
      </CardContent>
    </Card>
  );
}
