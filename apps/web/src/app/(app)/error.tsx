"use client";

import { useTranslations } from "next-intl";

import { Button } from "@/components/ui/button";

export default function ErrorPage({ reset }: { error: Error; reset: () => void }) {
  const t = useTranslations("errors");
  return (
    <div className="grid place-items-center gap-4 py-20 text-center">
      <h1 className="text-xl font-semibold">{t("pageTitle")}</h1>
      <p className="text-sm text-muted-foreground">{t("generic")}</p>
      <Button onClick={reset}>{t("retry")}</Button>
    </div>
  );
}
