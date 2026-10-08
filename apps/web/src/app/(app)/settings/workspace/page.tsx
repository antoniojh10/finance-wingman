import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { getTranslations } from "next-intl/server";

import { PageHeader } from "@/components/page-header";
import { DeleteWorkspace } from "@/components/workspaces/delete-workspace";
import { WorkspaceSettings } from "@/components/workspaces/workspace-settings";
import { authedApi, expectData, getCurrentSession, getWorkspaces } from "@/lib/session";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("workspaces");
  return { title: t("title") };
}

export default async function WorkspaceSettingsPage() {
  const { user, workspace } = await getCurrentSession();
  if (!workspace) {
    redirect("/");
  }
  const api = await authedApi();
  const params = { params: { path: { id: workspace.id } } };
  const [members, invitations, workspaces] = await Promise.all([
    api.GET("/api/v1/workspaces/{id}/members", params).then(expectData),
    workspace.role === "owner" ? api.GET("/api/v1/workspaces/{id}/invitations", params).then(expectData) : { items: [] },
    getWorkspaces(),
  ]);
  const deletionScheduledFor = workspaces.find((w) => w.id === workspace.id)?.deletion_scheduled_for;

  return (
    <>
      <PageHeader title={workspace.name} />
      <div className="grid gap-5">
        <WorkspaceSettings workspace={workspace} userId={user.id} members={members.items} invitations={invitations.items} />
        <DeleteWorkspace workspace={{ ...workspace, deletion_scheduled_for: deletionScheduledFor }} />
      </div>
    </>
  );
}
