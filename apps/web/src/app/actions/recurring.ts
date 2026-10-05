"use server";

import { revalidatePath } from "next/cache";
import { getTranslations } from "next-intl/server";

import { failure, success } from "@/lib/action-errors";
import { ApiError, unwrap } from "@/lib/api/client";
import { text, type FormState } from "@/lib/forms";
import { parseAmount } from "@/lib/money";
import { buildRecurringFields, type RecurringStatus } from "@/lib/recurring";
import { authedApi } from "@/lib/session";

export async function saveRecurring(_: FormState, formData: FormData): Promise<FormState> {
  const t = await getTranslations("errors");
  const api = await authedApi();
  const id = text(formData, "id");

  try {
    // The amount is entered as a decimal in the account's currency. Editing
    // keeps the account the item was created with.
    let account: { id: string; minor_units: number } | undefined;
    if (id) {
      account = unwrap(await api.GET("/api/v1/recurring/{id}", { params: { path: { id } } }));
    } else {
      const accounts = unwrap(await api.GET("/api/v1/accounts")).items;
      account = accounts.find((a) => a.id === text(formData, "account_id"));
    }

    const built = buildRecurringFields(formData, account);
    const fieldErrors: Record<string, string> = {};
    if ("errors" in built) {
      for (const e of built.errors) {
        fieldErrors[e.field] =
          e.error === "required"
            ? t("required")
            : e.error === "invalidNumber"
              ? t("invalidWholeNumber")
              : t("invalidAmount", { decimals: e.decimals ?? 2 });
      }
    }
    if (!account) {
      fieldErrors.account_id = t("required");
    }
    if (!("fields" in built) || !account) {
      return { ok: false, message: t("validation"), fieldErrors };
    }
    const { fields } = built;

    if (id) {
      unwrap(
        await api.PATCH("/api/v1/recurring/{id}", {
          params: { path: { id } },
          body: {
            name: fields.name,
            amount: fields.amount,
            interval_unit: fields.interval_unit,
            interval_count: fields.interval_count,
            start_on: fields.start_on,
            notes: fields.notes,
            category_id: fields.category_id,
            clear_category: !fields.category_id,
            total_payments: fields.total_payments,
            clear_total_payments: fields.total_payments === undefined,
          },
        }),
      );
    } else {
      const type = text(formData, "type") === "income" ? "income" : "expense";
      unwrap(await api.POST("/api/v1/recurring", { body: { ...fields, type, account_id: account.id } }));
    }
  } catch (error) {
    // A name conflict (409) shows the API message as is.
    return failure(error);
  }
  revalidatePath("/", "layout");
  return success();
}

/**
 * Registers a payment of a recurring item: creates the transaction and links
 * it to the period. The amount is a decimal in the item's currency.
 */
export async function registerRecurringPayment(_: FormState, formData: FormData): Promise<FormState> {
  const t = await getTranslations("errors");
  const api = await authedApi();
  const id = text(formData, "id");
  const period = text(formData, "period");
  const date = text(formData, "date");

  try {
    const item = unwrap(await api.GET("/api/v1/recurring/{id}", { params: { path: { id } } }));
    const amount = parseAmount(text(formData, "amount"), item.minor_units);
    const fieldErrors: Record<string, string> = {};
    if (amount === null || amount < 1) {
      fieldErrors.amount = t("invalidAmount", { decimals: item.minor_units });
    }
    if (!/^\d{4}-\d{2}-\d{2}$/.test(date)) {
      fieldErrors.date = t("required");
    }
    if (amount === null || Object.keys(fieldErrors).length > 0) {
      return { ok: false, message: t("validation"), fieldErrors };
    }
    unwrap(
      await api.POST("/api/v1/recurring/{id}/payments", {
        params: { path: { id } },
        body: { amount, date, period: period || undefined },
      }),
    );
  } catch (error) {
    return failure(error);
  }
  revalidatePath("/", "layout");
  return success();
}

/** Pauses, resumes, cancels or reactivates a recurring item. */
export async function setRecurringStatus(id: string, status: RecurringStatus): Promise<FormState> {
  const api = await authedApi();
  try {
    unwrap(await api.PATCH("/api/v1/recurring/{id}", { params: { path: { id } }, body: { status } }));
  } catch (error) {
    return failure(error);
  }
  revalidatePath("/", "layout");
  return success();
}

/**
 * Accepts a detected suggestion: creates the recurring item and links the
 * matching transactions. The name and amount (a decimal in the suggestion's
 * currency) may be adjusted.
 */
export async function acceptSuggestion(_: FormState, formData: FormData): Promise<FormState> {
  const t = await getTranslations("errors");
  const api = await authedApi();
  const key = text(formData, "key");
  const name = text(formData, "name");

  try {
    const suggestion = unwrap(await api.GET("/api/v1/recurring/suggestions")).items.find((s) => s.key === key);
    if (!suggestion) {
      // The pattern changed or was already handled.
      return { ok: false, message: t("notFound") };
    }
    const amount = parseAmount(text(formData, "amount"), suggestion.minor_units);
    const fieldErrors: Record<string, string> = {};
    if (!name) {
      fieldErrors.name = t("required");
    }
    if (amount === null || amount < 1) {
      fieldErrors.amount = t("invalidAmount", { decimals: suggestion.minor_units });
    }
    if (amount === null || Object.keys(fieldErrors).length > 0) {
      return { ok: false, message: t("validation"), fieldErrors };
    }
    unwrap(await api.POST("/api/v1/recurring/suggestions/accept", { body: { key, name, amount } }));
  } catch (error) {
    // A name conflict (409) shows the API message as is.
    return failure(error);
  }
  revalidatePath("/", "layout");
  return success();
}

/** Dismisses a suggestion so it is not proposed again. */
export async function dismissSuggestion(key: string): Promise<FormState> {
  const api = await authedApi();
  // 204 has no body, so check the status instead of unwrapping data.
  const result = await api.POST("/api/v1/recurring/suggestions/dismiss", { body: { key } });
  if (!result.response.ok) {
    return failure(new ApiError(result.response.status, result.error));
  }
  revalidatePath("/", "layout");
  return success();
}
