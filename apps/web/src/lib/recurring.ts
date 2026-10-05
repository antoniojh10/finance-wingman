import { optionalText, text } from "@/lib/forms";
import { parseAmount } from "@/lib/money";

export type RecurringType = "expense" | "income";
export type IntervalUnit = "week" | "month" | "year";
export type RecurringStatus = "active" | "paused" | "cancelled";

export type PaymentStatus = "paid" | "pending" | "overdue";

/** The yearly equivalent of a monthly committed amount (monthly x 12). */
export function yearlyFromMonthly(monthly: number): number {
  return monthly * 12;
}

type AccountRef = { id: string; minor_units: number };

export type RecurringFieldError = {
  field: string;
  error: "required" | "invalidAmount" | "invalidNumber";
  decimals?: number;
};

export type RecurringFields = {
  name: string;
  amount: number;
  category_id?: string;
  interval_unit: IntervalUnit;
  interval_count: number;
  start_on: string;
  total_payments?: number;
  notes: string;
};

/** Parses a whole number >= 1; returns null when invalid. */
function positiveInt(value: string): number | null {
  if (!/^\d{1,9}$/.test(value)) {
    return null;
  }
  const n = Number(value);
  return n >= 1 ? n : null;
}

/**
 * Validates the fields shared by creating and editing a recurring item,
 * converting the decimal amount with the account's minor units.
 */
export function buildRecurringFields(
  formData: FormData,
  account: AccountRef | undefined,
): { fields: RecurringFields } | { errors: RecurringFieldError[] } {
  const errors: RecurringFieldError[] = [];

  const name = text(formData, "name");
  if (!name) {
    errors.push({ field: "name", error: "required" });
  }

  let amount = 0;
  if (account) {
    const parsed = parseAmount(text(formData, "amount"), account.minor_units);
    if (parsed === null) {
      errors.push({ field: "amount", error: "invalidAmount", decimals: account.minor_units });
    } else {
      amount = parsed;
    }
  }

  const unitValue = text(formData, "interval_unit");
  const interval_unit: IntervalUnit = unitValue === "week" || unitValue === "year" ? unitValue : "month";

  const interval_count = positiveInt(text(formData, "interval_count"));
  if (interval_count === null) {
    errors.push({ field: "interval_count", error: "invalidNumber" });
  }

  const start_on = text(formData, "start_on");
  if (!/^\d{4}-\d{2}-\d{2}$/.test(start_on)) {
    errors.push({ field: "start_on", error: "required" });
  }

  let total_payments: number | undefined;
  const totalValue = text(formData, "total_payments");
  if (totalValue) {
    const parsed = positiveInt(totalValue);
    if (parsed === null) {
      errors.push({ field: "total_payments", error: "invalidNumber" });
    } else {
      total_payments = parsed;
    }
  }

  if (errors.length > 0) {
    return { errors };
  }
  return {
    fields: {
      name,
      amount,
      category_id: optionalText(formData, "category_id"),
      interval_unit,
      interval_count: interval_count ?? 1,
      start_on,
      total_payments,
      notes: text(formData, "notes"),
    },
  };
}
