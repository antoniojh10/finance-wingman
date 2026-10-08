"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useLocale, useTranslations } from "next-intl";
import { useState } from "react";
import { toast } from "sonner";

import { saveBudgets } from "@/app/actions/budgets";
import { ColorTile } from "@/components/color-tile";
import { Money } from "@/components/money";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { useFormAction } from "@/hooks/use-form-action";
import { intlLocale, isLocale } from "@/i18n/locales";
import { amountField, initialField, type EditCurrency, type EditRow } from "@/lib/budgets";
import { formatMonth } from "@/lib/dates";
import { formatAmountInput, toDecimalString } from "@/lib/money";

const key = (currency: string, categoryId: string) => `${currency}:${categoryId}`;

function asInput(amount: number | null, minorUnits: number): string {
  return amount === null ? "" : formatAmountInput(toDecimalString(amount, minorUnits));
}

/**
 * Edit mode of the budgets page: one amount per expense category and
 * currency for the month. An amount applies from this month on until a later
 * month sets another; emptying a field removes the budget from this month on.
 * Suggestions only fill the fields: nothing is saved until the form is.
 */
export function BudgetEditor({
  month,
  currencies,
  doneHref,
}: {
  month: string;
  currencies: EditCurrency[];
  /** Where to go after saving, and from "Cancel". */
  doneHref: string;
}) {
  const t = useTranslations("budgets");
  const tc = useTranslations("common");
  const router = useRouter();
  const currentLocale = useLocale();
  const locale = intlLocale(isLocale(currentLocale) ? currentLocale : "en");

  const [values, setValues] = useState<Record<string, string>>(() =>
    Object.fromEntries(currencies.flatMap((c) => c.rows.map((r) => [key(c.currency, r.categoryId), asInput(r.amount, c.minorUnits)]))),
  );
  const { state, onSubmit, pending } = useFormAction(saveBudgets, () => {
    toast.success(t("saved"));
    router.push(doneHref);
  });
  const errors = state.fieldErrors ?? {};

  const suggestionCount = currencies.reduce((sum, c) => sum + c.rows.filter((r) => r.suggested !== null).length, 0);

  function use(currency: EditCurrency, row: EditRow) {
    if (row.suggested !== null) {
      setValues((current) => ({ ...current, [key(currency.currency, row.categoryId)]: asInput(row.suggested, currency.minorUnits) }));
    }
  }

  function useAll() {
    setValues((current) => {
      const next = { ...current };
      for (const c of currencies) {
        for (const row of c.rows) {
          if (row.suggested !== null) {
            next[key(c.currency, row.categoryId)] = asInput(row.suggested, c.minorUnits);
          }
        }
      }
      return next;
    });
  }

  if (currencies.length === 0) {
    return <p className="py-6 text-center text-sm text-muted-foreground">{t("noCategories")}</p>;
  }

  return (
    <form onSubmit={onSubmit} className="grid gap-6" noValidate>
      <input type="hidden" name="month" value={month} />
      <p className="text-sm text-muted-foreground">{t("editHint", { month: formatMonth(month, locale) })}</p>

      {suggestionCount > 0 && (
        <div className="flex flex-wrap items-center gap-3">
          <Button type="button" variant="secondary" onClick={useAll}>
            {t("applyAllSuggestions")}
          </Button>
          <p className="text-[13px] text-muted-foreground">{t("suggestionsHint")}</p>
        </div>
      )}

      {currencies.map((currency) => (
        <Card key={currency.currency} data-testid={`budget-edit-${currency.currency}`}>
          <CardHeader>
            <CardTitle>
              <h2>{currency.currency}</h2>
            </CardTitle>
          </CardHeader>
          <CardContent>
            <ul className="divide-y">
              {currency.rows.map((row) => {
                const field = amountField(currency.currency, row.categoryId);
                const inherited = row.amount !== null && row.amountMonth !== null && row.amountMonth !== month;
                return (
                  <li key={row.categoryId} className="flex flex-wrap items-center gap-x-3 gap-y-2 py-3">
                    <ColorTile color={row.color} label={row.name} className="size-10 rounded-[13px]" />
                    <div className="min-w-0 flex-1 basis-40">
                      <label htmlFor={field} className="block truncate text-[15px] font-semibold">
                        {row.name}
                      </label>
                      {inherited && row.amountMonth && (
                        <p className="text-[12.5px] text-muted-foreground">{t("inheritedFrom", { month: formatMonth(row.amountMonth, locale) })}</p>
                      )}
                      {row.suggested !== null && (
                        <p className="text-[12.5px] text-muted-foreground">
                          {t("suggested")}{" "}
                          <Money amount={row.suggested} currency={currency.currency} minorUnits={currency.minorUnits} />
                          {" · "}
                          <button
                            type="button"
                            onClick={() => use(currency, row)}
                            className="font-semibold text-primary underline-offset-2 hover:underline"
                            aria-label={t("useSuggestionFor", { name: row.name })}
                          >
                            {t("useSuggestion")}
                          </button>
                        </p>
                      )}
                    </div>
                    <input type="hidden" name={initialField(currency.currency, row.categoryId)} value={asInput(row.amount, currency.minorUnits)} />
                    <div className="w-full sm:w-44">
                      <Input
                        id={field}
                        name={field}
                        inputMode="decimal"
                        autoComplete="off"
                        placeholder={t("noBudget")}
                        value={values[key(currency.currency, row.categoryId)] ?? ""}
                        onChange={(event) =>
                          setValues((current) => ({
                            ...current,
                            [key(currency.currency, row.categoryId)]: formatAmountInput(event.target.value),
                          }))
                        }
                        aria-invalid={Boolean(errors[field])}
                        aria-describedby={errors[field] ? `${field}-error` : undefined}
                        className="text-right tabular-nums"
                      />
                      {errors[field] && (
                        <p id={`${field}-error`} className="mt-1 text-xs text-destructive" role="alert">
                          {errors[field]}
                        </p>
                      )}
                    </div>
                  </li>
                );
              })}
            </ul>
          </CardContent>
        </Card>
      ))}

      {state.message && !state.ok && (
        <p className="text-sm text-destructive" role="alert">
          {state.message}
        </p>
      )}

      <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
        <Button variant="ghost" size="lg" nativeButton={false} render={<Link href={doneHref} />}>
          {tc("cancel")}
        </Button>
        <Button type="submit" size="lg" disabled={pending} className="sm:min-w-36">
          {pending ? tc("saving") : tc("save")}
        </Button>
      </div>
    </form>
  );
}
