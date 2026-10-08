"use server";

import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import { getLocale, getTranslations } from "next-intl/server";

import { createApiClient } from "@/lib/api/client";
import { clientIpHeaders } from "@/lib/client-ip";
import { emailPattern, text } from "@/lib/forms";
import { clearSession, getSessionToken, setSession } from "@/lib/session";
import { LOCALE_COOKIE, isLocale } from "@/i18n/locales";

export type LoginState = {
  step: "email" | "code";
  email?: string;
  error?: string;
};

async function rememberLocale(locale: string) {
  if (isLocale(locale)) {
    (await cookies()).set(LOCALE_COOKIE, locale, { path: "/", maxAge: 60 * 60 * 24 * 365, sameSite: "lax" });
  }
}

export async function requestLogin(_: LoginState, formData: FormData): Promise<LoginState> {
  const t = await getTranslations("auth");
  const email = text(formData, "email").toLowerCase();
  if (!emailPattern.test(email)) {
    return { step: "email", email, error: t("invalidEmail") };
  }
  const locale = await getLocale();
  const result = await createApiClient(undefined, await clientIpHeaders()).POST("/api/v1/auth/login", {
    body: { email, locale: isLocale(locale) ? locale : undefined },
  });
  if (result.response.status === 422) {
    return { step: "email", email, error: t("invalidEmail") };
  }
  if (!result.response.ok) {
    return { step: "email", email, error: t("sendFailed") };
  }
  return { step: "code", email };
}

export async function verifyCode(_: LoginState, formData: FormData): Promise<LoginState> {
  const t = await getTranslations("auth");
  const email = text(formData, "email");
  const code = text(formData, "code").replace(/\s/g, "");
  if (!/^\d{6}$/.test(code)) {
    return { step: "code", email, error: t("invalidCode") };
  }
  const { data, response } = await createApiClient(undefined, await clientIpHeaders()).POST("/api/v1/auth/verify", { body: { email, code } });
  if (!response.ok || !data?.token) {
    return { step: "code", email, error: response.status === 401 || response.status === 422 ? t("invalidCode") : t("sendFailed") };
  }
  await setSession(data.token, data.expires_at);
  await rememberLocale(data.user.locale);
  redirect("/");
}

export type VerifyLinkState = { error?: string };

export async function verifyLink(_: VerifyLinkState, formData: FormData): Promise<VerifyLinkState> {
  const t = await getTranslations("auth");
  const token = text(formData, "token");
  if (!token) {
    return { error: t("linkInvalid") };
  }
  const { data, response } = await createApiClient(undefined, await clientIpHeaders()).POST("/api/v1/auth/verify", { body: { token } });
  if (!response.ok || !data?.token) {
    return { error: t("linkInvalid") };
  }
  await setSession(data.token, data.expires_at);
  await rememberLocale(data.user.locale);
  redirect("/");
}

export async function logout(): Promise<void> {
  const token = await getSessionToken();
  if (token) {
    // Revoke the session server-side; the cookie is cleared regardless.
    await createApiClient(token).POST("/api/v1/auth/logout").catch(() => undefined);
  }
  await clearSession();
  redirect("/login");
}
