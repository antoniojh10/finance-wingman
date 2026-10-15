"use server";

import { revalidatePath } from "next/cache";
import { getTranslations } from "next-intl/server";

import { failure, success } from "@/lib/action-errors";
import { ApiError, unwrap } from "@/lib/api/client";
import { text, type FormState } from "@/lib/forms";
import { buildAssignment } from "@/lib/groceries";
import { authedApi } from "@/lib/session";

/**
 * Assigns one receipt line to a product variant, picked from the catalog or
 * created on the spot. The API also remembers the store's wording of the
 * line, so the next receipt with the same text is matched automatically.
 */
export async function assignReceiptLine(_: FormState, formData: FormData): Promise<FormState> {
  const t = await getTranslations("errors");
  const receiptId = text(formData, "receipt_id");
  const built = buildAssignment(formData);
  if ("errors" in built) {
    const fieldErrors = Object.fromEntries(
      Object.entries(built.errors).map(([field, error]) => [field, error === "required" ? t("required") : t("invalidPositiveNumber")]),
    );
    return { ok: false, message: t("validation"), fieldErrors };
  }

  try {
    const api = await authedApi();
    unwrap(
      await api.POST("/api/v1/groceries/receipts/{id}/assign", {
        params: { path: { id: receiptId } },
        body: { assignments: [built.body] },
      }),
    );
  } catch (error) {
    return failure(error);
  }
  revalidatePath("/groceries", "layout");
  return success();
}

/** Clears the product of a receipt line. The store alias is kept. */
export async function unassignReceiptLine(receiptId: string, lineId: string): Promise<FormState> {
  try {
    const api = await authedApi();
    unwrap(
      await api.POST("/api/v1/groceries/receipts/{id}/assign", {
        params: { path: { id: receiptId } },
        body: { assignments: [{ line_id: lineId, variant_id: null }] },
      }),
    );
  } catch (error) {
    return failure(error);
  }
  revalidatePath("/groceries", "layout");
  return success();
}

/** Deletes a receipt. The expense it was linked to is kept. */
export async function deleteReceipt(id: string): Promise<FormState> {
  const api = await authedApi();
  const result = await api.DELETE("/api/v1/groceries/receipts/{id}", { params: { path: { id } } });
  if (!result.response.ok) {
    return failure(new ApiError(result.response.status, result.error));
  }
  revalidatePath("/groceries", "layout");
  return success();
}
