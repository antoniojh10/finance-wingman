import { DownloadIcon } from "lucide-react";
import { useTranslations } from "next-intl";

import { buttonVariants } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";

/** Downloads of the workspace data, served by the /export route handler. */
export function ExportDataCard() {
  const t = useTranslations("settings");
  return (
    <Card>
      <CardHeader>
        <CardTitle>
          <h2>{t("exportTitle")}</h2>
        </CardTitle>
        <CardDescription>{t("exportDescription")}</CardDescription>
      </CardHeader>
      <CardContent className="grid gap-2 sm:grid-cols-2">
        <a href="/export?format=json" download className={buttonVariants({ variant: "outline", size: "lg" })}>
          <DownloadIcon />
          {t("exportJson")}
        </a>
        <a href="/export?format=csv" download className={buttonVariants({ variant: "outline", size: "lg" })}>
          <DownloadIcon />
          {t("exportCsv")}
        </a>
      </CardContent>
    </Card>
  );
}
