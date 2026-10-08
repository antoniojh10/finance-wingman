"use server";

import { revalidatePath } from "next/cache";

import { failure, success } from "@/lib/action-errors";
import { ApiError } from "@/lib/api/client";
import type { FormState } from "@/lib/forms";
import { authedApi } from "@/lib/session";

const securityPath = "/settings/security";

/** Signs out one of the user's other sessions. */
export async function revokeSession(id: string): Promise<FormState> {
  const api = await authedApi();
  const result = await api.DELETE("/api/v1/auth/sessions/{id}", { params: { path: { id } } });
  if (!result.response.ok) {
    return failure(new ApiError(result.response.status, result.error));
  }
  revalidatePath(securityPath);
  return success();
}

/** Signs out every session of the user except this one. */
export async function revokeOtherSessions(): Promise<FormState> {
  const api = await authedApi();
  const result = await api.POST("/api/v1/auth/sessions/revoke-others");
  if (!result.response.ok) {
    return failure(new ApiError(result.response.status, result.error));
  }
  revalidatePath(securityPath);
  return success();
}

/** Disconnects an app (OAuth client): it loses access immediately. */
export async function disconnectApp(id: string): Promise<FormState> {
  const api = await authedApi();
  const result = await api.DELETE("/api/v1/auth/connections/{id}", { params: { path: { id } } });
  if (!result.response.ok) {
    return failure(new ApiError(result.response.status, result.error));
  }
  revalidatePath(securityPath);
  return success();
}
