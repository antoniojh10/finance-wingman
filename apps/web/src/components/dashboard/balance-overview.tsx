"use client";

import { ArrowDownLeftIcon, ArrowUpRightIcon } from "lucide-react";
import { useTranslations } from "next-intl";
import { useState } from "react";

import { Money } from "@/components/money";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { cn } from "@/lib/utils";

import { CategoryBreakdown, type CategoryTotal } from "./category-breakdown";

export type CurrencySummary = {
  currency: string;
  minor_units: number;
  income: number;
  expense: number;
  net: number;
  balance: number;
  expenses: CategoryTotal[];
};

/**
 * The month at a glance for one currency at a time (currencies are never
 * combined): balance, income and expenses, then spending by category.
 */
export function BalanceOverview({ currencies }: { currencies: CurrencySummary[] }) {
  const t = useTranslations();
  const [selected, setSelected] = useState(currencies[0]?.currency);
  const summary = currencies.find((c) => c.currency === selected) ?? currencies[0];
  if (!summary) {
    return null;
  }
  const money = (amount: number, className?: string) => (
    <Money amount={amount} currency={summary.currency} minorUnits={summary.minor_units} className={className} />
  );
  const positiveNet = summary.net > 0;

  return (
    <div className="grid gap-5 lg:grid-cols-2 lg:items-start">
      <section
        aria-label={t("dashboard.balance")}
        className="relative grid gap-4 overflow-hidden rounded-[28px] bg-hero px-5 pt-5.5 pb-5 text-hero-foreground"
      >
        <span aria-hidden className="absolute -top-11 -right-9 size-30 rounded-full bg-primary dark:bg-[#5b3df5]" />
        <span aria-hidden className="absolute -top-15 right-7.5 size-21 rounded-full bg-lime opacity-85 mix-blend-screen" />
        <div className="relative grid gap-1">
          <p className="text-sm font-semibold text-hero-muted">{t("dashboard.balance")}</p>
          <p className="font-heading text-[40px] leading-tight font-bold tracking-tight tabular-nums">{money(summary.balance)}</p>
          <p className="text-[13px] text-hero-muted">{t("dashboard.balanceHint")}</p>
        </div>
        {currencies.length > 1 && (
          <div role="group" aria-label={t("accounts.currency")} className="relative flex flex-wrap gap-2">
            {currencies.map((c) => (
              <button
                key={c.currency}
                type="button"
                aria-pressed={c.currency === summary.currency}
                onClick={() => setSelected(c.currency)}
                className={cn(
                  "h-9 min-w-16 rounded-full px-3.5 text-[13px] font-bold tracking-wide transition-colors",
                  c.currency === summary.currency ? "bg-lime text-lime-foreground" : "bg-white/10 text-hero-foreground hover:bg-white/15",
                )}
              >
                {c.currency}
              </button>
            ))}
          </div>
        )}
        <div className="relative grid grid-cols-2 gap-2.5">
          <Stat label={t("dashboard.income")} icon={<ArrowDownLeftIcon />} iconClass="bg-lime" valueClass="text-lime">
            {money(summary.income)}
          </Stat>
          <Stat label={t("dashboard.expenses")} icon={<ArrowUpRightIcon />} iconClass="bg-[#ff8f75]" valueClass="text-[#ffb3a3]">
            {money(summary.expense)}
          </Stat>
        </div>
        <div className="relative flex items-center justify-between border-t border-white/12 pt-3.5">
          <span className="text-sm text-hero-muted">{t("dashboard.net")}</span>
          <span className="text-[17px] font-bold"><Money
              amount={summary.net}
              currency={summary.currency}
              minorUnits={summary.minor_units}
              tone={positiveNet ? "income" : "neutral"}
              signed={positiveNet}
              className={positiveNet ? "text-lime" : undefined}
            /></span>
        </div>
      </section>

      <Card>
        <CardHeader>
          <CardTitle>
            <h2>{t("dashboard.spendingByCategory")}</h2>
          </CardTitle>
        </CardHeader>
        <CardContent>
          <CategoryBreakdown totals={summary.expenses} currency={summary.currency} minorUnits={summary.minor_units} />
        </CardContent>
      </Card>
    </div>
  );
}

function Stat({
  label,
  icon,
  iconClass,
  valueClass,
  children,
}: {
  label: string;
  icon: React.ReactNode;
  iconClass: string;
  valueClass: string;
  children: React.ReactNode;
}) {
  return (
    <div className="grid gap-1.5 rounded-[18px] bg-white/8 px-3.5 py-3">
      <p className="flex items-center gap-1.5 text-[13px] text-hero-muted">
        <span className={cn("flex size-5.5 items-center justify-center rounded-lg text-[#16133a] [&_svg]:size-3.5", iconClass)}>{icon}</span>
        {label}
      </p>
      <p className={cn("text-[17px] font-bold tabular-nums", valueClass)}>{children}</p>
    </div>
  );
}
