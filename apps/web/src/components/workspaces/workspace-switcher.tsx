"use client";

import { CheckIcon, ChevronsUpDownIcon, PlusIcon, SettingsIcon } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useState, useTransition } from "react";
import { toast } from "sonner";

import { switchWorkspace } from "@/app/actions/workspaces";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { cn } from "@/lib/utils";

import { CreateWorkspaceDialog } from "./create-workspace";

export type WorkspaceOption = { id: string; name: string };

/** Shows the current workspace and switches to another one. */
export function WorkspaceSwitcher({
  current,
  workspaces,
  className,
}: {
  current?: WorkspaceOption;
  workspaces: WorkspaceOption[];
  className?: string;
}) {
  const t = useTranslations("workspaces");
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [creating, setCreating] = useState(false);

  function select(workspace: WorkspaceOption) {
    if (workspace.id === current?.id) {
      return;
    }
    startTransition(async () => {
      const result = await switchWorkspace(workspace.id);
      if (result.ok) {
        toast.success(t("switched", { name: workspace.name }));
        router.refresh();
      } else {
        toast.error(result.message);
      }
    });
  }

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger
          render={
            <Button
              variant="outline"
              aria-label={t("switch")}
              disabled={pending}
              className={cn("h-auto min-w-0 justify-between gap-2 rounded-xl px-3 py-2", className)}
            >
              <span className="grid min-w-0 text-left">
                <span className="text-[11px] font-medium text-muted-foreground">{t("label")}</span>
                <span className="truncate text-sm font-semibold">{current?.name ?? "—"}</span>
              </span>
              <ChevronsUpDownIcon className="shrink-0 text-muted-foreground" />
            </Button>
          }
        />
        <DropdownMenuContent className="w-auto min-w-56 rounded-xl p-1.5">
          <DropdownMenuGroup>
            <DropdownMenuLabel>{t("switch")}</DropdownMenuLabel>
            {workspaces.map((workspace) => (
              <DropdownMenuItem
                key={workspace.id}
                onClick={() => select(workspace)}
                className="gap-2.5 rounded-lg px-2.5 py-2 font-medium"
              >
                <CheckIcon className={workspace.id === current?.id ? "" : "invisible"} aria-hidden />
                <span className="truncate">{workspace.name}</span>
              </DropdownMenuItem>
            ))}
          </DropdownMenuGroup>
          <DropdownMenuSeparator />
          <DropdownMenuItem onClick={() => setCreating(true)} className="gap-2.5 rounded-lg px-2.5 py-2 font-medium">
            <PlusIcon />
            {t("create")}
          </DropdownMenuItem>
          {current && (
            <DropdownMenuItem
              render={<Link href="/settings/workspace" />}
              className="gap-2.5 rounded-lg px-2.5 py-2 font-medium"
            >
              <SettingsIcon />
              {t("manage")}
            </DropdownMenuItem>
          )}
        </DropdownMenuContent>
      </DropdownMenu>
      <CreateWorkspaceDialog open={creating} onOpenChange={setCreating} />
    </>
  );
}
