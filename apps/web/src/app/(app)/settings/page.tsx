import { ChevronRightIcon, LogOutIcon, ShieldIcon, TagIcon, UsersIcon } from "lucide-react";
import type { Metadata } from "next";
import Link from "next/link";
import { getTranslations } from "next-intl/server";

import { logout } from "@/app/actions/auth";
import { Avatar } from "@/components/brand";
import { CopyButton } from "@/components/copy-button";
import { PageHeader } from "@/components/page-header";
import { ThemePicker } from "@/components/theme-picker";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { apiBaseUrl } from "@/lib/api/client";
import { getCurrentSession } from "@/lib/session";

import { ExportDataCard } from "./export-data-card";
import { ProfileForm } from "./profile-form";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("settings");
  return { title: t("title") };
}

export default async function SettingsPage() {
  const t = await getTranslations();
  const { user, workspace } = await getCurrentSession();
  // The public API URL may differ from the internal one used by the server.
  const mcpUrl = `${(process.env.API_PUBLIC_URL ?? apiBaseUrl()).replace(/\/$/, "")}/mcp`;
  const steps = [
    { title: t("settings.claudeTitle"), text: t("settings.claudeSteps"), color: "bg-[#ff8f75]" },
    { title: t("settings.chatgptTitle"), text: t("settings.chatgptSteps"), color: "bg-[#7fb6ff]" },
  ];

  return (
    <>
      <PageHeader title={t("settings.title")} />
      <div className="grid grid-cols-1 gap-5 lg:grid-cols-2 lg:items-start">
        <Card>
          <CardHeader className="flex items-center gap-3.5">
            <Avatar name={user.name || user.email} className="size-14 text-2xl" />
            <CardTitle>
              <h2>{t("settings.profile")}</h2>
            </CardTitle>
          </CardHeader>
          <CardContent>
            <ProfileForm name={user.name} email={user.email} locale={user.locale} />
          </CardContent>
        </Card>

        <div className="grid min-w-0 grid-cols-1 gap-5">
          <section className="relative grid gap-4 overflow-hidden rounded-3xl bg-hero px-5 py-5.5 text-hero-foreground">
            <span aria-hidden className="absolute -top-7.5 -right-7.5 size-25 rounded-full bg-lime" />
            <span aria-hidden className="absolute -top-11 right-12.5 size-17.5 rounded-full bg-primary dark:bg-[#5b3df5]" />
            <div className="relative grid gap-1.5 pr-24">
              <h2 className="text-[21px] font-bold">{t("settings.assistants")}</h2>
              <p className="text-sm leading-relaxed text-hero-muted">{t("settings.assistantsDescription")}</p>
            </div>
            <div className="grid min-w-0 gap-1.5">
              <p className="text-[13px] font-bold text-hero-muted">{t("settings.mcpUrl")}</p>
              <div className="flex items-center gap-2 rounded-2xl bg-white/8 py-1.5 pr-1.5 pl-3.5">
                <code className="min-w-0 flex-1 truncate font-mono text-[13px]" data-testid="mcp-url">
                  {mcpUrl}
                </code>
                <CopyButton value={mcpUrl} className="border-0 bg-lime text-lime-foreground hover:bg-lime/85" />
              </div>
            </div>
            <ol className="grid gap-2.5">
              {steps.map((step, index) => (
                <li key={step.title} className="flex gap-3 rounded-2xl bg-white/6 p-3.5">
                  <span
                    className={`flex size-7.5 shrink-0 items-center justify-center rounded-[10px] text-sm font-bold text-[#16133a] ${step.color}`}
                  >
                    {index + 1}
                  </span>
                  <div className="grid gap-1">
                    <h3 className="font-sans text-[15px] font-bold">{step.title}</h3>
                    <p className="text-[13.5px] leading-relaxed text-hero-muted">{step.text}</p>
                  </div>
                </li>
              ))}
            </ol>
            <p className="text-[13px] leading-relaxed text-hero-muted">{t("settings.assistantsHint")}</p>
          </section>

          {workspace && (
            <Link
              href="/settings/workspace"
              className="flex min-h-17.5 items-center gap-3.5 rounded-3xl bg-card px-5 py-3 ring-1 ring-foreground/5 transition-colors hover:bg-accent"
            >
              <UsersIcon className="size-5 shrink-0 text-primary" aria-hidden />
              <span className="grid min-w-0 flex-1">
                <span className="font-heading text-base font-bold">{t("workspaces.manage")}</span>
                <span className="text-sm text-muted-foreground">{t("workspaces.settingsDescription", { name: workspace.name })}</span>
              </span>
              <ChevronRightIcon className="size-5 shrink-0 text-muted-foreground" aria-hidden />
            </Link>
          )}

          <Link
            href="/settings/security"
            className="flex min-h-17.5 items-center gap-3.5 rounded-3xl bg-card px-5 py-3 ring-1 ring-foreground/5 transition-colors hover:bg-accent"
          >
            <ShieldIcon className="size-5 shrink-0 text-primary" aria-hidden />
            <span className="grid min-w-0 flex-1">
              <span className="font-heading text-base font-bold">{t("security.title")}</span>
              <span className="text-sm text-muted-foreground">{t("security.settingsDescription")}</span>
            </span>
            <ChevronRightIcon className="size-5 shrink-0 text-muted-foreground" aria-hidden />
          </Link>

          <Link
            href="/categories"
            className="flex min-h-17.5 items-center gap-3.5 rounded-3xl bg-card px-5 py-3 ring-1 ring-foreground/5 transition-colors hover:bg-accent"
          >
            <TagIcon className="size-5 shrink-0 text-primary" aria-hidden />
            <span className="grid min-w-0 flex-1">
              <span className="font-heading text-base font-bold">{t("nav.categories")}</span>
              <span className="text-sm text-muted-foreground">{t("settings.categoriesDescription")}</span>
            </span>
            <ChevronRightIcon className="size-5 shrink-0 text-muted-foreground" aria-hidden />
          </Link>

          {workspace && <ExportDataCard />}

          <Card>
            <CardHeader>
              <CardTitle>
                <h2>{t("settings.appearance")}</h2>
              </CardTitle>
              <CardDescription>{t("settings.appearanceDescription")}</CardDescription>
            </CardHeader>
            <CardContent>
              <ThemePicker />
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>
                <h2>{t("settings.session")}</h2>
              </CardTitle>
              <CardDescription>{t("settings.signOutDescription")}</CardDescription>
            </CardHeader>
            <CardContent>
              <form action={logout}>
                <Button type="submit" variant="destructive" size="lg" className="w-full border-2 border-destructive/25">
                  <LogOutIcon />
                  {t("nav.signOut")}
                </Button>
              </form>
            </CardContent>
          </Card>
        </div>
      </div>
    </>
  );
}
