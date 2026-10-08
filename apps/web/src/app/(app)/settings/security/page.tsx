import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";

import { PageHeader } from "@/components/page-header";
import { DeleteAccount } from "@/components/security/delete-account";
import { SecuritySettings } from "@/components/security/security-settings";
import { authedApi, expectData, getCurrentUser } from "@/lib/session";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("security");
  return { title: t("title") };
}

export default async function SecuritySettingsPage() {
  const t = await getTranslations("security");
  const api = await authedApi();
  const [sessions, connections, deletion, user] = await Promise.all([
    api.GET("/api/v1/auth/sessions").then(expectData),
    api.GET("/api/v1/auth/connections").then(expectData),
    api.GET("/api/v1/auth/me/deletion").then(expectData),
    getCurrentUser(),
  ]);

  return (
    <>
      <PageHeader title={t("title")} />
      <div className="grid gap-5">
        <SecuritySettings sessions={sessions.items} connections={connections.items} />
        <DeleteAccount email={user.email} deletion={deletion} />
      </div>
    </>
  );
}
