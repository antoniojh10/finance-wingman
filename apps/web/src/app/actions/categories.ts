"use server";

import { revalidatePath } from "next/cache";
import { getTranslations } from "next-intl/server";

import { failure, success } from "@/lib/action-errors";
import { ApiError, unwrap } from "@/lib/api/client";
import { text, type FormState } from "@/lib/forms";
import { authedApi } from "@/lib/session";

const colorPattern = /^#[0-9a-fA-F]{6}$/;

export async function saveCategory(_: FormState, formData: FormData): Promise<FormState> {
  const t = await getTranslations();
  const api = await authedApi();
  const id = text(formData, "id");
  const name = text(formData, "name");
  const color = text(formData, "color");

  const fieldErrors: Record<string, string> = {};
  if (!name) {
    fieldErrors.name = t("errors.required");
  }
  if (color && !colorPattern.test(color)) {
    fieldErrors.color = t("errors.invalidColor");
  }
  if (Object.keys(fieldErrors).length > 0) {
    return { ok: false, message: t("errors.validation"), fieldErrors };
  }

  try {
    if (id) {
      unwrap(await api.PATCH("/api/v1/categories/{id}", { params: { path: { id } }, body: { name, color } }));
    } else {
      const kind = text(formData, "kind") === "income" ? "income" : "expense";
      unwrap(await api.POST("/api/v1/categories", { body: { name, kind, color: color || undefined } }));
    }
  } catch (error) {
    return failure(error, { conflict: t("categories.nameTaken") });
  }
  revalidatePath("/", "layout");
  return success();
}

export async function setCategoryArchived(id: string, archived: boolean): Promise<FormState> {
  const t = await getTranslations();
  const api = await authedApi();
  try {
    unwrap(await api.PATCH("/api/v1/categories/{id}", { params: { path: { id } }, body: { archived } }));
  } catch (error) {
    return failure(error, { conflict: t("categories.nameTaken") });
  }
  revalidatePath("/", "layout");
  return success();
}

export async function deleteCategory(id: string): Promise<FormState> {
  const api = await authedApi();
  const result = await api.DELETE("/api/v1/categories/{id}", { params: { path: { id } } });
  if (!result.response.ok) {
    return failure(new ApiError(result.response.status, result.error));
  }
  revalidatePath("/", "layout");
  return success();
}
