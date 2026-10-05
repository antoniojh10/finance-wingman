"use client";

import { EllipsisVerticalIcon, type LucideIcon } from "lucide-react";
import { useTranslations } from "next-intl";

import { Button } from "@/components/ui/button";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";

export type RowAction = {
  label: string;
  icon: LucideIcon;
  onSelect: () => void;
  destructive?: boolean;
  disabled?: boolean;
};

/** A compact "more" menu for the actions of a list row. */
export function RowActions({ actions, className }: { actions: RowAction[]; className?: string }) {
  const t = useTranslations("common");
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Button variant="ghost" size="icon" aria-label={t("actions")} className={className}>
            <EllipsisVerticalIcon />
          </Button>
        }
      />
      <DropdownMenuContent align="end" className="w-auto min-w-40 rounded-xl p-1.5">
        {actions.map(({ label, icon: Icon, onSelect, destructive, disabled }) => (
          <DropdownMenuItem
            key={label}
            onClick={onSelect}
            disabled={disabled}
            variant={destructive ? "destructive" : "default"}
            className="gap-2.5 rounded-lg px-2.5 py-2.5 font-medium"
          >
            <Icon />
            {label}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
