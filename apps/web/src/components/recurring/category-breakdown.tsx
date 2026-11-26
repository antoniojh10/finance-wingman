"use client";

import { useTranslations } from "next-intl";
import { useRouter } from "next/navigation";
import { type CSSProperties, useState } from "react";

import { CategoryPill } from "@/components/categories/category-pill";
import { Money } from "@/components/money";
import { type RecurringFilters, recurringFiltersHref, uncategorizedFilter } from "@/lib/recurring-filters";
import { cn } from "@/lib/utils";

export type RecurringCategoryTotal = {
  category_id: string | null;
  category_name: string | null;
  category_color: string | null;
  /** Active expenses of the category. */
  count: number;
  /** Monthly amount of the active expenses, in minor units. */
  expense: number;
};

export type BreakdownCurrency = {
  currency: string;
  minor_units: number;
  expense: number;
  categories: RecurringCategoryTotal[];
};

/** Whole percent of the total; a small non-zero share reads "<1%" instead of 0%. */
function percentLabel(part: number, total: number): string {
  if (total <= 0 || part <= 0) return "0%";
  const percent = Math.round((part / total) * 100);
  return percent === 0 ? "<1%" : `${percent}%`;
}

const mutedColor = "var(--muted-foreground)";

/** Filter value of a breakdown row: the category id, or `none` for uncategorized. */
const filterValue = (c: RecurringCategoryTotal) => c.category_id ?? uncategorizedFilter;

/**
 * "Where it goes": the monthly cost of active expenses split by category, as a
 * stacked bar and a list. A row filters the subscriptions list by that
 * category; pressing it again clears the filter.
 */
export function CategoryBreakdown({
  currencies,
  filters,
  className,
}: {
  currencies: BreakdownCurrency[];
  filters: RecurringFilters;
  className?: string;
}) {
  const t = useTranslations("subscriptions");
  const router = useRouter();
  const [selected, setSelected] = useState(currencies[0]?.currency);

  if (currencies.length === 0) {
    return null;
  }

  const current = currencies.find((c) => c.currency === selected) ?? currencies[0];
  // Biggest first, uncategorized last (the API already sends this order).
  const categories = [...current.categories].sort(
    (a, b) => Number(a.category_id === null) - Number(b.category_id === null) || b.expense - a.expense,
  );
  const total = categories.reduce((sum, c) => sum + c.expense, 0);

  function toggle(c: RecurringCategoryTotal) {
    const value = filterValue(c);
    router.replace(recurringFiltersHref({ ...filters, category: filters.category === value ? undefined : value }), {
      scroll: false,
    });
  }

  return (
    <section
      aria-label={t("breakdownTitle")}
      data-testid="category-breakdown"
      className={cn("grid min-w-0 content-start gap-3.5 rounded-[22px] bg-card p-4.5 ring-1 ring-foreground/5", className)}
    >
      <div className="flex flex-wrap items-baseline justify-between gap-x-2 gap-y-1">
        <h3 className="font-heading text-[17px] font-bold">{t("breakdownTitle")}</h3>
        <p className="text-[12.5px] text-muted-foreground">{t("breakdownHint")}</p>
      </div>
      {currencies.length > 1 && (
        <div role="group" aria-label={t("breakdownCurrency")} className="flex flex-wrap gap-1.5">
          {currencies.map((c) => (
            <button
              key={c.currency}
              type="button"
              aria-pressed={c.currency === current.currency}
              onClick={() => setSelected(c.currency)}
              className={cn(
                "min-h-8 rounded-[10px] border px-2.5 text-[13px] font-bold transition-colors",
                c.currency === current.currency
                  ? "border-primary bg-primary text-primary-foreground"
                  : "border-border text-muted-foreground hover:text-foreground",
              )}
            >
              {c.currency}
            </button>
          ))}
        </div>
      )}
      {categories.length === 0 ? (
        <p className="py-3 text-sm text-muted-foreground">{t("breakdownEmpty", { currency: current.currency })}</p>
      ) : (
        <>
          <div aria-hidden className="flex h-3.5 gap-0.5 overflow-hidden rounded-full bg-muted" data-testid="breakdown-bar">
            {categories.map((c) => (
              <span
                key={filterValue(c)}
                data-testid="breakdown-segment"
                className="block h-full min-w-1 bg-[var(--c)]"
                style={{ "--c": c.category_color ?? mutedColor, flex: c.expense } as CSSProperties}
              />
            ))}
          </div>
          <ul className="grid gap-0.5">
            {categories.map((c) => {
              const pressed = filters.category === filterValue(c);
              return (
                <li key={filterValue(c)}>
                  <button
                    type="button"
                    aria-pressed={pressed}
                    data-testid="breakdown-row"
                    onClick={() => toggle(c)}
                    style={{ "--c": c.category_color ?? mutedColor } as CSSProperties}
                    className={cn(
                      "grid min-h-11 w-full grid-cols-[minmax(0,1fr)_auto_auto] items-center gap-x-3 rounded-xl px-2 py-1.5 text-left transition-colors hover:bg-muted",
                      pressed && "bg-[color-mix(in_oklab,var(--c)_14%,var(--card))] hover:bg-[color-mix(in_oklab,var(--c)_14%,var(--card))]",
                    )}
                  >
                    <span className="flex min-w-0 flex-wrap items-center gap-x-1.5">
                      <CategoryPill name={c.category_name ?? t("uncategorized")} color={c.category_color} />
                      <span className="text-[12.5px] text-muted-foreground">{t("itemCount", { count: c.count })}</span>
                    </span>
                    <Money
                      amount={c.expense}
                      currency={current.currency}
                      minorUnits={current.minor_units}
                      className="text-right text-[15px] font-bold whitespace-nowrap"
                    />
                    <span className="w-[3.2em] text-right text-[13px] text-muted-foreground tabular-nums" data-testid="breakdown-percent">
                      {percentLabel(c.expense, total)}
                    </span>
                  </button>
                </li>
              );
            })}
          </ul>
        </>
      )}
    </section>
  );
}
