"use client";

import { useEffect, useState } from "react";

import { getBudgetImpact, type BudgetImpact } from "@/app/actions/budgets";
import { parseAmount } from "@/lib/money";

const DEBOUNCE_MS = 300;

/**
 * Budget impact of the expense being typed: whether it would leave its
 * category near or over budget. Waits for the inputs to settle, ignores
 * answers to older inputs and is null while there is nothing to report
 * (disabled, incomplete input, no budget or a failed lookup).
 */
export function useBudgetImpact({
  enabled,
  categoryId,
  currency,
  minorUnits,
  amount,
  date,
}: {
  enabled: boolean;
  categoryId: string;
  currency: string;
  minorUnits: number;
  /** The amount as typed. */
  amount: string;
  date: string;
}): BudgetImpact | null {
  const minor = parseAmount(amount, minorUnits);
  const key = enabled && categoryId && currency && date && minor ? `${categoryId}|${currency}|${date}|${minor}` : null;
  const [result, setResult] = useState<{ key: string; impact: BudgetImpact | null } | null>(null);

  useEffect(() => {
    if (!key || !minor) {
      return;
    }
    let cancelled = false;
    const timer = setTimeout(async () => {
      const impact = await getBudgetImpact({ categoryId, currency, date, amount: minor });
      if (!cancelled) {
        setResult({ key, impact });
      }
    }, DEBOUNCE_MS);
    return () => {
      cancelled = true;
      clearTimeout(timer);
    };
  }, [key, minor, categoryId, currency, date]);

  return key && result?.key === key ? result.impact : null;
}
