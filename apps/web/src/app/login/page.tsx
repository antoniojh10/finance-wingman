import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { getTranslations } from "next-intl/server";

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
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
    <main className="flex min-h-dvh items-center justify-center px-4 py-10">
      <div className="w-full max-w-sm">
        <p className="mb-6 text-center text-lg font-semibold tracking-tight">{t("common.appName")}</p>
        <Card>
          <CardHeader>
            <CardTitle>{t("auth.title")}</CardTitle>
            <CardDescription>{t("auth.subtitle")}</CardDescription>
          </CardHeader>
          <CardContent className="grid gap-4">
            {expired && (
              <p className="rounded-lg bg-muted px-3 py-2 text-sm" role="status">
                {t("auth.sessionExpired")}
              </p>
            )}
            <LoginForm />
          </CardContent>
        </Card>
      </div>
    </main>
  );
}
