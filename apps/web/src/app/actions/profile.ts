"use server";

import { cookies } from "next/headers";
import { revalidatePath } from "next/cache";

import { failure, success } from "@/lib/action-errors";
import { unwrap } from "@/lib/api/client";
import { text, type FormState } from "@/lib/forms";
import { authedApi } from "@/lib/session";
import { LOCALE_COOKIE, isLocale } from "@/i18n/locales";

export async function updateProfile(_: FormState, formData: FormData): Promise<FormState> {
  const locale = text(formData, "locale");
  const api = await authedApi();
  try {
    const user = unwrap(
      await api.PATCH("/api/v1/auth/me", {
        body: { name: text(formData, "name"), locale: isLocale(locale) ? locale : undefined },
      }),
    );
    (await cookies()).set(LOCALE_COOKIE, user.locale, { path: "/", maxAge: 60 * 60 * 24 * 365, sameSite: "lax" });
  } catch (error) {
    return failure(error);
  }
  revalidatePath("/", "layout");
  return success();
}
