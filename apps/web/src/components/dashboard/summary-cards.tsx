import { useTranslations } from "next-intl";

import { Money } from "@/components/money";
import { Card, CardContent, CardDescription, CardHeader } from "@/components/ui/card";

type CurrencyTotals = {
  currency: string;
  minor_units: number;
  income: number;
  expense: number;
  net: number;
  balance: number;
};

/** Headline figures for one currency. Currencies are never combined. */
export function SummaryCards({ summary }: { summary: CurrencyTotals }) {
  const t = useTranslations("dashboard");
  const money = (amount: number, tone: "neutral" | "income" = "neutral") => (
    <Money amount={amount} currency={summary.currency} minorUnits={summary.minor_units} tone={tone} />
  );

  const items = [
    { key: "income", value: money(summary.income, summary.income > 0 ? "income" : "neutral") },
    { key: "expenses", value: money(summary.expense) },
    { key: "net", value: money(summary.net, summary.net > 0 ? "income" : "neutral") },
    { key: "balance", value: money(summary.balance), hint: t("balanceHint") },
  ] as const;

  return (
    <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
      {items.map((item) => (
        <Card key={item.key} size="sm">
          <CardHeader>
            <CardDescription>
              {t(item.key)} · {summary.currency}
            </CardDescription>
            <p className="text-lg font-semibold tracking-tight break-words sm:text-2xl" title={"hint" in item ? item.hint : undefined}>
              {item.value}
            </p>
          </CardHeader>
          {"hint" in item && (
            <CardContent className="hidden text-xs text-muted-foreground sm:block">{item.hint}</CardContent>
          )}
        </Card>
      ))}
    </div>
  );
}
