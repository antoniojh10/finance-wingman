import { LogOutIcon } from "lucide-react";
import { useTranslations } from "next-intl";

import { logout } from "@/app/actions/auth";
import { NavLinks } from "@/components/nav-links";
import { Button } from "@/components/ui/button";

export function AppShell({ userName, children }: { userName: string; children: React.ReactNode }) {
  const t = useTranslations();

  return (
    <div className="min-h-dvh md:grid md:grid-cols-[15rem_1fr]">
      <aside className="sticky top-0 hidden h-dvh flex-col border-r px-3 py-5 md:flex">
        <p className="px-3 pb-6 text-base font-semibold tracking-tight">{t("common.appName")}</p>
        <nav className="grid gap-1" aria-label="Main">
          <NavLinks variant="sidebar" />
        </nav>
        <div className="mt-auto grid gap-2 px-3">
          <p className="truncate text-sm text-muted-foreground">{userName}</p>
          <form action={logout}>
            <Button type="submit" variant="ghost" size="sm" className="-ml-2.5">
              <LogOutIcon />
              {t("nav.signOut")}
            </Button>
          </form>
        </div>
      </aside>

      <div className="flex min-w-0 flex-col">
        <header className="flex items-center justify-between border-b px-4 py-3 md:hidden">
          <p className="font-semibold tracking-tight">{t("common.appName")}</p>
        </header>
        <main className="mx-auto w-full max-w-5xl flex-1 px-4 pt-5 pb-24 md:px-8 md:pt-8 md:pb-10">{children}</main>
      </div>

      <nav
        aria-label="Main"
        className="fixed inset-x-0 bottom-0 z-40 flex border-t bg-background/95 pb-[env(safe-area-inset-bottom)] backdrop-blur md:hidden"
      >
        <NavLinks variant="tabs" />
      </nav>
    </div>
  );
}
