import { useLocale, useTranslations } from "next-intl";
import type { CSSProperties } from "react";

import { accountStyles, type AccountType } from "@/components/accounts/account-style";
import { Money } from "@/components/money";
import { intlLocale, isLocale } from "@/i18n/locales";
import { formatMoney } from "@/lib/money";
import { cn } from "@/lib/utils";

import { AccountSparkline, type SparklinePoint } from "./account-sparkline";

export type AccountSummary = {
  id: string;
  name: string;
  type: AccountType;
  currency: string;
  minor_units: number;
  balance: number;
  /** End-of-day balances of the last 30 days, oldest first. */
  days?: SparklinePoint[];
};

/** Change over the series (last − first), or null when there are fewer than two points. */
export function trendChange(days: SparklinePoint[] | undefined): number | null {
  return days && days.length >= 2 ? days[days.length - 1].balance - days[0].balance : null;
}

/** Account balances as tinted cards: a swipeable row on phones, a grid on wider screens. */
export function AccountStrip({ accounts }: { accounts: AccountSummary[] }) {
  const t = useTranslations("accounts");
  const d = useTranslations("dashboard");
  const rawLocale = useLocale();
  const locale = intlLocale(isLocale(rawLocale) ? rawLocale : "en");
  return (
    <ul className="-mx-4 flex snap-x gap-3 overflow-x-auto px-4 pb-1 md:mx-0 md:grid md:grid-cols-3 md:overflow-visible md:px-0 xl:grid-cols-4">
      {accounts.map((a) => {
        const change = trendChange(a.days);
        const tone = change === null || change === 0 ? "text-muted-foreground" : change > 0 ? "text-income" : "text-expense";
        const label =
          change === null || change === 0
            ? d("trendFlat")
            : d(change > 0 ? "trendUp" : "trendDown", { change: formatMoney(Math.abs(change), a.currency, a.minor_units, locale) });
        return (
        <li
          key={a.id}
          className="grid w-40 shrink-0 snap-start grid-cols-1 gap-4.5 rounded-[22px] bg-[color-mix(in_oklab,var(--tile)_16%,var(--card))] p-4 md:w-auto"
          style={{ "--tile": accountStyles[a.type].color } as CSSProperties}
        >
          <div className="flex items-center justify-between gap-2 text-xs text-[color-mix(in_oklab,var(--tile)_55%,var(--foreground))]">
            <span className="font-bold tracking-wide">{a.currency}</span>
            <span className="truncate font-semibold">{t(`types.${a.type}`)}</span>
          </div>
          <div className="grid gap-0.5">
            <p className="truncate text-sm font-semibold">{a.name}</p>
            <p className="font-heading text-[19px] font-bold">
              <Money
                amount={a.balance}
                currency={a.currency}
                minorUnits={a.minor_units}
                className={a.balance < 0 ? "text-expense" : undefined}
              />
            </p>
          </div>
          {change !== null && (
            <div className={cn("grid gap-1", tone)} data-testid="account-trend" data-tone={tone}>
              <AccountSparkline points={a.days ?? []} label={label} className="h-8 w-full" />
              <p className="truncate text-xs font-semibold">
                {change === 0 ? (
                  <Money amount={0} currency={a.currency} minorUnits={a.minor_units} />
                ) : (
                  <Money amount={change} currency={a.currency} minorUnits={a.minor_units} tone={change > 0 ? "income" : "expense"} signed />
                )}
                {" · "}
                {d("trendDays")}
              </p>
            </div>
          )}
        </li>
        );
      })}
    </ul>
  );
}
