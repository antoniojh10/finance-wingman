import type { Metadata } from "next";
import Link from "next/link";
import { getTranslations } from "next-intl/server";

import { AuthHeading, AuthLayout } from "@/components/auth-layout";
import { buttonVariants } from "@/components/ui/button";

import { VerifyLinkForm } from "./verify-link-form";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("auth");
  return { title: t("verifyTitle") };
}

export default async function VerifyPage({ searchParams }: { searchParams: Promise<{ token?: string }> }) {
  const t = await getTranslations();
  const { token } = await searchParams;

  return (
    <AuthLayout appName={t("common.appName")}>
      <AuthHeading title={t("auth.verifyTitle")} description={token ? t("auth.verifyDescription") : t("auth.linkInvalid")} />
      {token ? (
        <VerifyLinkForm token={token} />
      ) : (
        <Link href="/login" className={buttonVariants({ variant: "outline", size: "lg", className: "w-full" })}>
          {t("auth.backToLogin")}
        </Link>
      )}
    </AuthLayout>
  );
}
