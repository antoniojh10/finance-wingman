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
      unwrap(await api.POST("/api/v1/transactions", { body: built.body }));
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
