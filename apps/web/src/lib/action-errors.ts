import "server-only";

import { redirect } from "next/navigation";
import { getTranslations } from "next-intl/server";

import { ApiError } from "@/lib/api/client";
import type { FormState } from "@/lib/forms";

/**
 * Converts a failed API call into form state. Expired sessions redirect to
 * sign-in; conflicts use the caller's message when provided.
 */
export async function failure(error: unknown, options: { conflict?: string } = {}): Promise<FormState> {
  const t = await getTranslations("errors");
  if (!(error instanceof ApiError)) {
    console.error(error);
    return { ok: false, message: t("generic") };
  }
  switch (error.status) {
    case 401:
      redirect("/auth/signout");
    case 403:
      return { ok: false, message: t("forbidden") };
    case 404:
      return { ok: false, message: t("notFound") };
    case 409:
      return { ok: false, message: options.conflict ?? error.message };
    case 422:
      return { ok: false, message: t("validation"), fieldErrors: error.fieldErrors };
    default:
      console.error(error);
      return { ok: false, message: t("generic") };
  }
}

export function success(): FormState {
  return { ok: true, nonce: Date.now() };
}
