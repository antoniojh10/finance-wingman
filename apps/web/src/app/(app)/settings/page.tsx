import { LogOutIcon } from "lucide-react";
import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";

import { logout } from "@/app/actions/auth";
import { CopyButton } from "@/components/copy-button";
import { PageHeader } from "@/components/page-header";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { apiBaseUrl } from "@/lib/api/client";
import { getCurrentUser } from "@/lib/session";

import { ProfileForm } from "./profile-form";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("settings");
  return { title: t("title") };
}

export default async function SettingsPage() {
  const t = await getTranslations();
  const user = await getCurrentUser();
  // The public API URL may differ from the internal one used by the server.
  const mcpUrl = `${(process.env.API_PUBLIC_URL ?? apiBaseUrl()).replace(/\/$/, "")}/mcp`;

  return (
    <>
      <PageHeader title={t("settings.title")} />
      <div className="grid gap-4">
        <Card>
          <CardHeader>
            <CardTitle>{t("settings.profile")}</CardTitle>
          </CardHeader>
          <CardContent>
            <ProfileForm name={user.name} email={user.email} locale={user.locale} />
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>{t("settings.assistants")}</CardTitle>
            <CardDescription>{t("settings.assistantsDescription")}</CardDescription>
          </CardHeader>
          <CardContent className="grid gap-5">
            <div className="grid gap-1.5">
              <p className="text-sm font-medium">{t("settings.mcpUrl")}</p>
              <div className="flex flex-wrap items-center gap-2">
                <code className="min-w-0 flex-1 truncate rounded-lg bg-muted px-3 py-2 text-sm" data-testid="mcp-url">
                  {mcpUrl}
                </code>
                <CopyButton value={mcpUrl} />
              </div>
            </div>
            <div className="grid gap-4 sm:grid-cols-2">
              <div>
                <p className="text-sm font-medium">{t("settings.claudeTitle")}</p>
                <p className="text-sm text-muted-foreground">{t("settings.claudeSteps")}</p>
              </div>
              <div>
                <p className="text-sm font-medium">{t("settings.chatgptTitle")}</p>
                <p className="text-sm text-muted-foreground">{t("settings.chatgptSteps")}</p>
              </div>
            </div>
            <p className="text-xs text-muted-foreground">{t("settings.assistantsHint")}</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>{t("settings.session")}</CardTitle>
            <CardDescription>{t("settings.signOutDescription")}</CardDescription>
          </CardHeader>
          <CardContent>
            <form action={logout}>
              <Button type="submit" variant="outline">
                <LogOutIcon />
                {t("nav.signOut")}
              </Button>
            </form>
          </CardContent>
        </Card>
      </div>
    </>
  );
}
