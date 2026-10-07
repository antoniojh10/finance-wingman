import { AppShell } from "@/components/app-shell";
import { NoWorkspace } from "@/components/workspaces/no-workspace";
import { getCurrentSession, getWorkspaces } from "@/lib/session";

export default async function AuthenticatedLayout({ children }: { children: React.ReactNode }) {
  const [session, workspaces] = await Promise.all([getCurrentSession(), getWorkspaces()]);
  const { user, workspace } = session;
  return (
    <AppShell userName={user.name || user.email} workspace={workspace} workspaces={workspaces}>
      {/* Every page shows workspace data, which needs a workspace. */}
      {workspace ? children : <NoWorkspace />}
    </AppShell>
  );
}
