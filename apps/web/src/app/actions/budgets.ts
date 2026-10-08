"use server";

import { revalidatePath } from "next/cache";
import { getTranslations } from "next-intl/server";

import { failure, success } from "@/lib/action-errors";
import { unwrap } from "@/lib/api/client";
import { buildBudgetItems } from "@/lib/budgets";
import { parseMonth } from "@/lib/dates";
import { text, type FormState } from "@/lib/forms";
import { authedApi } from "@/lib/session";

/**
 * Saves the budgets edited for one month. Only the fields that changed are
 * sent: an amount sets the budget from that month on (until a later month
 * sets another), an empty field clears it from that month on. The amounts
 * are decimals in each currency; earlier months are never touched.
 */
export async function saveBudgets(_: FormState, formData: FormData): Promise<FormState> {
  const t = await getTranslations("errors");
  const api = await authedApi();
  const month = parseMonth(text(formData, "month"), "");

  try {
    if (!month) {
      return { ok: false, message: t("validation") };
    }
    const accounts = unwrap(await api.GET("/api/v1/accounts", { params: { query: { include_archived: true } } })).items;
    const minorUnits = new Map(accounts.map((a) => [a.currency, a.minor_units]));

    const { items, errors } = buildBudgetItems(formData, (currency) => minorUnits.get(currency));
    if (Object.keys(errors).length > 0) {
      const fieldErrors = Object.fromEntries(Object.keys(errors).map((field) => [field, t("invalidBudgetAmount")]));
      return { ok: false, message: t("validation"), fieldErrors };
    }
    if (items.length > 0) {
      unwrap(await api.PUT("/api/v1/budgets/{month}", { params: { path: { month } }, body: { items } }));
    }
  } catch (error) {
    return failure(error);
  }
  revalidatePath("/", "layout");
  return success();
}
