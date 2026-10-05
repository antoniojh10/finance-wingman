import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { getTranslations } from "next-intl/server";

import { AuthHeading, AuthLayout } from "@/components/auth-layout";
import { getSessionToken } from "@/lib/session";

import { LoginForm } from "./login-form";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("auth");
  return { title: t("title") };
}

export default async function LoginPage({ searchParams }: { searchParams: Promise<{ expired?: string }> }) {
  if (await getSessionToken()) {
    redirect("/");
  }
  const t = await getTranslations();
  const { expired } = await searchParams;

  return (
    <AuthLayout appName={t("common.appName")}>
      <AuthHeading title={t("auth.title")} description={t("auth.subtitle")} />
      {expired && (
        <p className="rounded-2xl bg-card px-4 py-3 text-sm ring-1 ring-foreground/5" role="status">
          {t("auth.sessionExpired")}
        </p>
      )}
      <LoginForm />
    </AuthLayout>
  );
}
