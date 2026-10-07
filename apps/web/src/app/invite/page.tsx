import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";

import { AuthHeading, AuthLayout } from "@/components/auth-layout";
import { createApiClient } from "@/lib/api/client";

import { AcceptInvitationForm, InvalidInvitation } from "./accept-invitation-form";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("invite");
  return { title: t("title") };
}

export default async function InvitePage({ searchParams }: { searchParams: Promise<{ token?: string }> }) {
  const t = await getTranslations();
  const { token } = await searchParams;
  const preview = token
    ? (await createApiClient().POST("/api/v1/invitations/preview", { body: { token } })).data
    : undefined;

  if (!token || !preview) {
    return (
      <AuthLayout appName={t("common.appName")}>
        <AuthHeading title={t("invite.title")} />
        <InvalidInvitation message={t("invite.invalid")} />
      </AuthLayout>
    );
  }

  const values = {
    inviter: preview.invited_by ?? "",
    email: preview.email,
    workspace: preview.workspace_name,
    role: t(`workspaces.roles.${preview.role}`).toLowerCase(),
  };
  return (
    <AuthLayout appName={t("common.appName")}>
      <AuthHeading
        title={t("invite.title")}
        description={t(preview.invited_by ? "invite.description" : "invite.descriptionUnknown", values)}
      />
      <p className="mb-4 text-sm text-muted-foreground">{t("invite.signInNote", { email: preview.email })}</p>
      <AcceptInvitationForm token={token} />
    </AuthLayout>
  );
}
