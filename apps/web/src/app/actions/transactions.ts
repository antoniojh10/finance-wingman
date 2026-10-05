"use server";

import { revalidatePath } from "next/cache";
import { getTranslations } from "next-intl/server";

import { failure, success } from "@/lib/action-errors";
import { ApiError, unwrap } from "@/lib/api/client";
import { buildTransactionBody, text, type FormState } from "@/lib/forms";
import { authedApi } from "@/lib/session";

export async function saveTransaction(_: FormState, formData: FormData): Promise<FormState> {
  const t = await getTranslations("errors");
  const api = await authedApi();
  const id = text(formData, "id");

  let createdId: string | undefined;
  try {
    // Archived accounts are included so old transactions remain editable.
    const accounts = unwrap(await api.GET("/api/v1/accounts", { params: { query: { include_archived: true } } })).items;
    const built = buildTransactionBody(formData, accounts);
    if ("errors" in built) {
      const fieldErrors: Record<string, string> = {};
      for (const e of built.errors) {
        fieldErrors[e.field] = e.error === "required" ? t("required") : t("invalidAmount", { decimals: e.decimals ?? 2 });
      }
      return { ok: false, message: t("validation"), fieldErrors };
    }

    if (id) {
      unwrap(await api.PUT("/api/v1/transactions/{id}", { params: { path: { id } }, body: built.body }));
    } else {
      createdId = unwrap(await api.POST("/api/v1/transactions", { body: built.body })).id;
    }
  } catch (error) {
    return failure(error);
  }
  revalidatePath("/", "layout");
  return { ...success(), transactionId: createdId };
}

/** The active subscription a transaction looks like it pays, if any. */
export type RecurringMatchResult = { recurringId: string; name: string; period: string | null } | null;

/**
 * Looks for an active recurring item matching a (just created) transaction.
 * Failures are swallowed: the suggestion is optional and must never get in
 * the way of saving.
 */
export async function findRecurringMatch(transactionId: string): Promise<RecurringMatchResult> {
  try {
    const api = await authedApi();
    const match = unwrap(
      await api.GET("/api/v1/transactions/{id}/recurring-match", { params: { path: { id: transactionId } } }),
    );
    return match.item ? { recurringId: match.item.id, name: match.item.name, period: match.period_due_on } : null;
  } catch {
    return null;
  }
}

/** Links a transaction to a recurring item; `period` defaults to the closest due date. */
export async function linkTransactionRecurring(
  transactionId: string,
  recurringId: string,
  period?: string,
): Promise<FormState> {
  const api = await authedApi();
  try {
    unwrap(
      await api.PUT("/api/v1/transactions/{id}/recurring", {
        params: { path: { id: transactionId } },
        body: { recurring_id: recurringId, period: period || undefined },
      }),
    );
  } catch (error) {
    // The API explains why a link is refused (other account or type, bad period).
    if (error instanceof ApiError && (error.status === 422 || error.status === 409)) {
      return { ok: false, message: error.message };
    }
    return failure(error);
  }
  revalidatePath("/", "layout");
  return success();
}

export async function unlinkTransactionRecurring(transactionId: string): Promise<FormState> {
  const api = await authedApi();
  try {
    const result = await api.DELETE("/api/v1/transactions/{id}/recurring", { params: { path: { id: transactionId } } });
    if (!result.response.ok) {
      throw new ApiError(result.response.status, result.error);
    }
  } catch (error) {
    return failure(error);
  }
  revalidatePath("/", "layout");
  return success();
}

export async function deleteTransaction(id: string): Promise<FormState> {
  const api = await authedApi();
  const result = await api.DELETE("/api/v1/transactions/{id}", { params: { path: { id } } });
  if (!result.response.ok) {
    return failure(new ApiError(result.response.status, result.error));
  }
  revalidatePath("/", "layout");
  return success();
}
