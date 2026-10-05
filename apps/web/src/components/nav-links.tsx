"use client";

import { ArrowLeftRightIcon, HouseIcon, PlusIcon, RepeatIcon, SettingsIcon, TagIcon, WalletIcon } from "lucide-react";
import Link from "next/link";
import { usePathname, useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";

import { newTransactionHref } from "@/lib/query";
import { cn } from "@/lib/utils";

export const navItems = [
  { href: "/", key: "dashboard", icon: HouseIcon },
  { href: "/transactions", key: "transactions", icon: ArrowLeftRightIcon },
  { href: "/accounts", key: "accounts", icon: WalletIcon },
  { href: "/subscriptions", key: "subscriptions", icon: RepeatIcon },
  { href: "/categories", key: "categories", icon: TagIcon },
  { href: "/settings", key: "settings", icon: SettingsIcon },
] as const;

export function isActive(pathname: string, href: string): boolean {
  if (href === "/") {
    return pathname === "/";
  }
  if (href === "/transactions" && pathname === "/transactions/new") {
    return false;
  }
  return pathname === href || pathname.startsWith(`${href}/`);
}

/** The current location, used as the return target of the new transaction page. */
function useReturnPath(): string {
  const pathname = usePathname();
  const search = useSearchParams().toString();
  if (pathname === "/transactions/new") {
    return "/";
  }
  return search ? `${pathname}?${search}` : pathname;
}

/** Desktop sidebar links, including settings. */
export function SidebarLinks() {
  const pathname = usePathname();
  const t = useTranslations("nav");

  return (
    <>
      {navItems.map(({ href, key, icon: Icon }) => {
        const active = isActive(pathname, href);
        return (
          <Link
            key={href}
            href={href}
            aria-current={active ? "page" : undefined}
            className={cn(
              "flex items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-semibold transition-colors",
              active ? "bg-primary/12 text-primary" : "text-muted-foreground hover:bg-accent hover:text-foreground",
            )}
          >
            <Icon className="size-5 shrink-0" aria-hidden />
            <span>{t(key)}</span>
          </Link>
        );
      })}
    </>
  );
}

/** Primary "add transaction" action that remembers where the user came from. */
export function AddTransactionLink({ className, children }: { className?: string; children?: React.ReactNode }) {
  const t = useTranslations("transactions");
  const returnTo = useReturnPath();
  return (
    <Link href={newTransactionHref(returnTo)} aria-label={children ? undefined : t("add")} className={className}>
      {children ?? <PlusIcon className="size-6" strokeWidth={2.4} aria-hidden />}
    </Link>
  );
}

/** Mobile bottom tab bar: four sections (categories live in settings) around a floating add button. */
export function TabLinks() {
  const pathname = usePathname();
  const t = useTranslations("nav");
  const tabs = navItems.filter((item) => item.key !== "settings" && item.key !== "categories");

  const tab = ({ href, key, icon: Icon }: (typeof tabs)[number]) => {
    const active = isActive(pathname, href);
    return (
      <Link
        key={href}
        href={href}
        aria-current={active ? "page" : undefined}
        className={cn(
          "flex min-h-13 min-w-0 flex-col items-center justify-center gap-0.5 text-[11px] tracking-tight",
          active ? "font-bold text-primary" : "font-semibold text-muted-foreground",
        )}
      >
        <span className={cn("flex h-7.5 w-13 items-center justify-center rounded-full", active && "bg-primary/14")}>
          <Icon className="size-5.5" aria-hidden />
        </span>
        <span className="max-w-full truncate">{t(key)}</span>
      </Link>
    );
  };

  return (
    <>
      {tabs.slice(0, 2).map(tab)}
      <AddTransactionLink className="-mt-7 flex size-14.5 items-center justify-center justify-self-center rounded-[20px] bg-primary text-primary-foreground shadow-[0_12px_24px_-10px_rgb(22_19_58/0.55)] transition-transform active:scale-95" />
      {tabs.slice(2).map(tab)}
    </>
  );
}
