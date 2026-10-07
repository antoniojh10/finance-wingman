import { useTranslations } from "next-intl";

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";

import { CreateWorkspaceForm } from "./create-workspace";

/** Shown instead of the app when the session has no workspace. */
export function NoWorkspace() {
  const t = useTranslations("workspaces");
  return (
    <Card className="mx-auto max-w-md">
      <CardHeader>
        <CardTitle>
          <h1>{t("noneTitle")}</h1>
        </CardTitle>
        <CardDescription>{t("noneDescription")}</CardDescription>
      </CardHeader>
      <CardContent>
        <CreateWorkspaceForm />
      </CardContent>
    </Card>
  );
}
