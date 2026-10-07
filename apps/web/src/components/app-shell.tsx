import { LogOutIcon, PlusIcon } from "lucide-react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { Suspense } from "react";

import { logout } from "@/app/actions/auth";
import { Avatar, Brand, BrandMark } from "@/components/brand";
import { AddTransactionLink, SidebarLinks, TabLinks } from "@/components/nav-links";
import { Button, buttonVariants } from "@/components/ui/button";
import { WorkspaceSwitcher, type WorkspaceOption } from "@/components/workspaces/workspace-switcher";

export function AppShell({
  userName,
  workspace,
  workspaces,
  children,
}: {
  userName: string;
  /** The session's workspace; undefined when the user has none. */
  workspace?: WorkspaceOption;
  workspaces: WorkspaceOption[];
  children: React.ReactNode;
}) {
  const t = useTranslations();

  return (
    <div className="min-h-dvh md:grid md:grid-cols-[16rem_1fr]">
      <aside className="sticky top-0 hidden h-dvh flex-col gap-6 border-r bg-sidebar px-4 py-6 md:flex">
        <Link href="/" className="px-2">
          <Brand name={t("common.appName")} />
        </Link>
        <WorkspaceSwitcher current={workspace} workspaces={workspaces} className="w-full" />
        <Suspense>
          <AddTransactionLink className={buttonVariants({ size: "lg", className: "w-full" })}>
            <PlusIcon />
            {t("transactions.add")}
          </AddTransactionLink>
        </Suspense>
        <nav className="grid gap-1" aria-label="Main">
          <SidebarLinks />
        </nav>
        <div className="mt-auto flex items-center gap-3 px-1">
          <Avatar name={userName} className="size-10" />
          <div className="min-w-0 flex-1">
            <p className="truncate text-sm font-semibold">{userName}</p>
            <form action={logout}>
              <Button type="submit" variant="link" size="sm" className="h-auto px-0 text-muted-foreground">
                <LogOutIcon />
                {t("nav.signOut")}
              </Button>
            </form>
          </div>
        </div>
      </aside>

      <div className="flex min-w-0 flex-col">
        <header className="flex items-center gap-3 px-5 pt-5 pb-1 md:hidden">
          {/* Only the logo mark fits next to the workspace switcher. */}
          <Link href="/" aria-label={t("common.appName")} className="shrink-0">
            <BrandMark />
          </Link>
          <WorkspaceSwitcher current={workspace} workspaces={workspaces} className="min-w-0 flex-1 shrink" />
          <Link href="/settings" aria-label={t("nav.settings")} className="shrink-0 rounded-full">
            <Avatar name={userName} />
          </Link>
        </header>
        <main className="mx-auto w-full max-w-5xl flex-1 px-4 pt-4 pb-32 md:px-8 md:pt-10 md:pb-12">{children}</main>
      </div>

      <nav
        aria-label="Main"
        className="fixed inset-x-0 bottom-0 z-40 grid grid-cols-5 items-center border-t bg-card/95 px-2 pt-2.5 pb-[max(1rem,env(safe-area-inset-bottom))] backdrop-blur md:hidden"
      >
        <Suspense>
          <TabLinks />
        </Suspense>
      </nav>
    </div>
  );
}
