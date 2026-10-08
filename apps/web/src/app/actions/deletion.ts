"use server";

import { revalidatePath } from "next/cache";
import { getTranslations } from "next-intl/server";

import { failure, success } from "@/lib/action-errors";
import { ApiError } from "@/lib/api/client";
import { text, type FormState } from "@/lib/forms";
import { authedApi } from "@/lib/session";

/**
 * Schedules the workspace for deletion; the user confirms by typing its
 * name. It is deleted after a 7-day grace period unless an owner cancels.
 */
export async function scheduleWorkspaceDeletion(_: FormState, formData: FormData): Promise<FormState> {
  const t = await getTranslations();
  const id = text(formData, "id");
  const name = text(formData, "name");
  if (!name) {
    return { ok: false, message: t("errors.validation"), fieldErrors: { name: t("errors.required") } };
  }
  const api = await authedApi();
  const result = await api.POST("/api/v1/workspaces/{id}/deletion", { params: { path: { id } }, body: { name } });
  if (result.response.status === 422) {
    return { ok: false, message: t("errors.validation"), fieldErrors: { name: t("workspaces.deleteNameMismatch") } };
  }
  if (!result.response.ok) {
    return failure(new ApiError(result.response.status, result.error), { conflict: t("workspaces.deletionAlreadyScheduled") });
  }
  revalidatePath("/", "layout");
  return success();
}

/** Cancels the workspace's scheduled deletion. */
export async function cancelWorkspaceDeletion(workspaceId: string): Promise<FormState> {
  const api = await authedApi();
  const result = await api.DELETE("/api/v1/workspaces/{id}/deletion", { params: { path: { id: workspaceId } } });
  if (!result.response.ok) {
    return failure(new ApiError(result.response.status, result.error));
  }
  revalidatePath("/", "layout");
  return success();
}
