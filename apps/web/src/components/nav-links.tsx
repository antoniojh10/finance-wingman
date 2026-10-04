"use client";

import { ArrowLeftRightIcon, LayoutDashboardIcon, SettingsIcon, TagsIcon, WalletIcon } from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useTranslations } from "next-intl";

import { cn } from "@/lib/utils";

export const navItems = [
  { href: "/", key: "dashboard", icon: LayoutDashboardIcon },
  { href: "/transactions", key: "transactions", icon: ArrowLeftRightIcon },
  { href: "/accounts", key: "accounts", icon: WalletIcon },
  { href: "/categories", key: "categories", icon: TagsIcon },
  { href: "/settings", key: "settings", icon: SettingsIcon },
] as const;

export function isActive(pathname: string, href: string): boolean {
  return href === "/" ? pathname === "/" : pathname === href || pathname.startsWith(`${href}/`);
}

/** Sidebar links on desktop, bottom tab bar on mobile. */
export function NavLinks({ variant }: { variant: "sidebar" | "tabs" }) {
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
              variant === "sidebar"
                ? "flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors hover:bg-muted"
                : "flex flex-1 flex-col items-center gap-1 py-2 text-[11px] font-medium",
              active ? "text-foreground" : "text-muted-foreground",
              variant === "sidebar" && active && "bg-muted",
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
