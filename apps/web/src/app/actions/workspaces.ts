"use server";

import { revalidatePath } from "next/cache";
import { redirect } from "next/navigation";
import { getLocale, getTranslations } from "next-intl/server";

import { failure, success } from "@/lib/action-errors";
import { ApiError, createApiClient, unwrap, type ApiClient } from "@/lib/api/client";
import { emailPattern, text, type FormState } from "@/lib/forms";
import { authedApi, setSession } from "@/lib/session";
import { isLocale } from "@/i18n/locales";

async function switchTo(api: ApiClient, workspaceId: string) {
  unwrap(await api.PUT("/api/v1/auth/session/workspace", { body: { workspace_id: workspaceId } }));
}

export async function switchWorkspace(workspaceId: string): Promise<FormState> {
  const api = await authedApi();
  try {
    await switchTo(api, workspaceId);
  } catch (error) {
    return failure(error);
  }
  revalidatePath("/", "layout");
  return success();
}

/** Creates a workspace owned by the user and switches the session to it. */
export async function createWorkspace(_: FormState, formData: FormData): Promise<FormState> {
  const t = await getTranslations();
  const name = text(formData, "name");
  if (!name) {
    return { ok: false, message: t("errors.validation"), fieldErrors: { name: t("errors.required") } };
  }
  const api = await authedApi();
  try {
    const workspace = unwrap(await api.POST("/api/v1/workspaces", { body: { name } }));
    await switchTo(api, workspace.id);
  } catch (error) {
    return failure(error);
  }
  revalidatePath("/", "layout");
  return success();
}

export async function renameWorkspace(_: FormState, formData: FormData): Promise<FormState> {
  const t = await getTranslations();
  const id = text(formData, "id");
  const name = text(formData, "name");
  if (!name) {
    return { ok: false, message: t("errors.validation"), fieldErrors: { name: t("errors.required") } };
  }
  const api = await authedApi();
  try {
    unwrap(await api.PATCH("/api/v1/workspaces/{id}", { params: { path: { id } }, body: { name } }));
  } catch (error) {
    return failure(error);
  }
  revalidatePath("/", "layout");
  return success();
}

export async function inviteMember(_: FormState, formData: FormData): Promise<FormState> {
  const t = await getTranslations();
  const id = text(formData, "workspace_id");
  const email = text(formData, "email").toLowerCase();
  const role = text(formData, "role") === "owner" ? "owner" : "member";
  if (!emailPattern.test(email)) {
    return { ok: false, message: t("errors.validation"), fieldErrors: { email: t("auth.invalidEmail") } };
  }
  const locale = await getLocale();
  const api = await authedApi();
  try {
    unwrap(
      await api.POST("/api/v1/workspaces/{id}/invitations", {
        params: { path: { id } },
        body: { email, role, locale: isLocale(locale) ? locale : undefined },
      }),
    );
  } catch (error) {
    return failure(error, { conflict: t("workspaces.inviteConflict") });
  }
  revalidatePath("/settings/workspace");
  return success();
}

export async function revokeInvitation(workspaceId: string, invitationId: string): Promise<FormState> {
  const api = await authedApi();
  const result = await api.DELETE("/api/v1/workspaces/{id}/invitations/{invitation_id}", {
    params: { path: { id: workspaceId, invitation_id: invitationId } },
  });
  if (!result.response.ok) {
    return failure(new ApiError(result.response.status, result.error));
  }
  revalidatePath("/settings/workspace");
  return success();
}

export async function setMemberRole(workspaceId: string, userId: string, role: "owner" | "member"): Promise<FormState> {
  const t = await getTranslations();
  const api = await authedApi();
  const result = await api.PATCH("/api/v1/workspaces/{id}/members/{user_id}", {
    params: { path: { id: workspaceId, user_id: userId } },
    body: { role },
  });
  if (!result.response.ok) {
    return failure(new ApiError(result.response.status, result.error), { conflict: t("workspaces.lastOwner") });
  }
  revalidatePath("/", "layout");
  return success();
}

export async function removeMember(workspaceId: string, userId: string): Promise<FormState> {
  const t = await getTranslations();
  const api = await authedApi();
  const result = await api.DELETE("/api/v1/workspaces/{id}/members/{user_id}", {
    params: { path: { id: workspaceId, user_id: userId } },
  });
  if (!result.response.ok) {
    return failure(new ApiError(result.response.status, result.error), { conflict: t("workspaces.lastOwner") });
  }
  revalidatePath("/settings/workspace");
  return success();
}

/** Leaves the workspace and moves the session to another one, if any. */
export async function leaveWorkspace(workspaceId: string, userId: string): Promise<FormState> {
  const result = await removeMember(workspaceId, userId);
  if (!result.ok) {
    return result;
  }
  const api = await authedApi();
  try {
    const { items } = unwrap(await api.GET("/api/v1/workspaces"));
    if (items.length > 0) {
      await switchTo(api, items[0].id);
    }
  } catch (error) {
    return failure(error);
  }
  revalidatePath("/", "layout");
  redirect("/");
}

export type AcceptInvitationState = { error?: string };

/** Accepts an invitation and signs in as the invitee (replacing any session). */
export async function acceptInvitation(_: AcceptInvitationState, formData: FormData): Promise<AcceptInvitationState> {
  const t = await getTranslations("invite");
  const token = text(formData, "token");
  if (!token) {
    return { error: t("invalid") };
  }
  const { data, response } = await createApiClient().POST("/api/v1/invitations/accept", { body: { token } });
  if (!response.ok || !data?.token) {
    return { error: t("invalid") };
  }
  await setSession(data.token, data.expires_at);
  redirect("/");
}
