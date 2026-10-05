import { useTranslations } from "next-intl";

import { Money } from "@/components/money";
import { yearlyFromMonthly } from "@/lib/recurring";
import { cn } from "@/lib/utils";

export type RecurringCurrencyTotals = {
  currency: string;
  minor_units: number;
  expense: number;
  expense_count: number;
  income: number;
  income_count: number;
  net: number;
};

// Each currency's card gets its own color, in order.
const cardStyles = ["bg-hero text-hero-foreground", "bg-lime text-lime-foreground", "bg-primary text-primary-foreground"];

/** Committed cost of active subscriptions per currency: monthly and yearly (monthly x 12). */
export function RecurringSummary({ currencies }: { currencies: RecurringCurrencyTotals[] }) {
  const t = useTranslations("subscriptions");

  if (currencies.length === 0) {
    return null;
  }

  return (
    <section aria-label={t("committed")} className="grid grid-cols-1 gap-3">
      <h2 className="text-[13px] font-bold tracking-wider text-muted-foreground uppercase">{t("committed")}</h2>
      <div className="grid grid-cols-1 gap-3 lg:grid-cols-2">
        {currencies.map((c, index) => {
          const rows = [
            { key: "expenses", amount: c.expense },
            { key: "income", amount: c.income },
            { key: "net", amount: c.net },
          ] as const;
          return (
            <div
              key={c.currency}
              role="group"
              aria-label={c.currency}
              className={cn("grid gap-3 rounded-[22px] px-4.5 py-4", cardStyles[index % cardStyles.length])}
            >
              <div className="flex items-center justify-between gap-3">
                <h3 className="font-sans text-[13px] font-bold tracking-wider">{c.currency}</h3>
                <p className="text-[13px] opacity-80">{t("activeCount", { count: c.expense_count + c.income_count })}</p>
              </div>
              <div className="grid grid-cols-[1fr_auto_auto] items-baseline gap-x-4 gap-y-1.5">
                <span />
                <span className="text-right text-xs font-bold opacity-80">{t("perMonth")}</span>
                <span className="text-right text-xs font-bold opacity-80">{t("perYear")}</span>
                {rows.map(({ key, amount }) => (
                  <div key={key} className="contents" data-testid={`summary-${c.currency}-${key}`}>
                    <span className="text-sm font-semibold">{t(key)}</span>
                    <Money
                      amount={amount}
                      currency={c.currency}
                      minorUnits={c.minor_units}
                      className="text-right text-base font-bold whitespace-nowrap"
                    />
                    <Money
                      amount={yearlyFromMonthly(amount)}
                      currency={c.currency}
                      minorUnits={c.minor_units}
                      className="text-right text-base font-bold whitespace-nowrap opacity-80"
                    />
                  </div>
                ))}
              </div>
            </div>
          );
        })}
      </div>
      <p className="text-xs text-muted-foreground">{t("summaryHint")}</p>
    </section>
  );
}
