import { useLocale } from "next-intl";

import { formatMoney } from "@/lib/money";
import { intlLocale, isLocale } from "@/i18n/locales";
import { cn } from "@/lib/utils";

export type MoneyTone = "neutral" | "income" | "expense";

/** Displays an amount in minor units, formatted for the current locale. */
export function Money({
  amount,
  currency,
  minorUnits,
  tone = "neutral",
  signed = false,
  className,
}: {
  amount: number;
  currency: string;
  minorUnits: number;
  tone?: MoneyTone;
  /** Prefix + or − according to the tone (income/expense). */
  signed?: boolean;
  className?: string;
}) {
  const locale = useLocale();
  const formatted = formatMoney(Math.abs(amount), currency, minorUnits, intlLocale(isLocale(locale) ? locale : "en"));
  const sign = signed && tone !== "neutral" ? (tone === "income" ? "+" : "−") : amount < 0 ? "−" : "";
  return (
    <span
      className={cn(
        "tabular-nums",
        tone === "income" && "text-income",
        className,
      )}
    >
      {sign}
      {formatted}
    </span>
  );
}
