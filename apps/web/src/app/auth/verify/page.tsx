import type { Metadata } from "next";
import Link from "next/link";
import { getTranslations } from "next-intl/server";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";

import { VerifyLinkForm } from "./verify-link-form";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("auth");
  return { title: t("verifyTitle") };
}

export default async function VerifyPage({ searchParams }: { searchParams: Promise<{ token?: string }> }) {
  const t = await getTranslations("auth");
  const { token } = await searchParams;

  return (
    <main className="flex min-h-dvh items-center justify-center px-4 py-10">
      <Card className="w-full max-w-sm">
        <CardHeader>
          <CardTitle>{t("verifyTitle")}</CardTitle>
          <CardDescription>{token ? t("verifyDescription") : t("linkInvalid")}</CardDescription>
        </CardHeader>
        <CardContent>
          {token ? (
            <VerifyLinkForm token={token} />
          ) : (
            <Button nativeButton={false} render={<Link href="/login" />} variant="outline" className="w-full">
              {t("backToLogin")}
            </Button>
          )}
        </CardContent>
      </Card>
    </main>
  );
}
