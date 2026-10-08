"use client";

import { DownloadIcon, Trash2Icon, UndoIcon } from "lucide-react";
import { useRouter } from "next/navigation";
import { useFormatter, useTranslations } from "next-intl";
import { useState, useTransition } from "react";
import { toast } from "sonner";

import { cancelWorkspaceDeletion, scheduleWorkspaceDeletion } from "@/app/actions/deletion";
import { Field } from "@/components/field";
import { Button, buttonVariants } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { useFormAction } from "@/hooks/use-form-action";
import { deletionDate } from "@/lib/deletion";

export type DeleteWorkspaceProps = {
  workspace: { id: string; name: string; role: "owner" | "member"; deletion_scheduled_for?: string };
};

/**
 * Deleting the workspace: owners schedule it (typing its name to confirm)
 * or cancel a scheduled deletion; members only see that it is scheduled.
 */
export function DeleteWorkspace({ workspace }: DeleteWorkspaceProps) {
  const t = useTranslations("workspaces");
  const router = useRouter();
  const [open, setOpen] = useState(false);
  // Held here, not in the dialog: scheduling re-renders the page with the
  // scheduled state, which unmounts the dialog before it could report it.
  const form = useFormAction(scheduleWorkspaceDeletion, () => {
    toast.success(t("deletionScheduled"));
    setOpen(false);
    router.refresh();
  });
  const isOwner = workspace.role === "owner";
  if (workspace.deletion_scheduled_for) {
    return <ScheduledDeletion workspace={workspace} scheduledFor={workspace.deletion_scheduled_for} canCancel={isOwner} />;
  }
  return isOwner ? <ScheduleDeletion workspace={workspace} form={form} open={open} onOpenChange={setOpen} /> : null;
}

type ScheduleForm = ReturnType<typeof useFormAction>;

function ScheduledDeletion({
  workspace,
  scheduledFor,
  canCancel,
}: {
  workspace: { id: string; name: string };
  scheduledFor: string;
  canCancel: boolean;
}) {
  const t = useTranslations("workspaces");
  const format = useFormatter();
  const router = useRouter();
  const [pending, startTransition] = useTransition();

  function cancel() {
    startTransition(async () => {
      const result = await cancelWorkspaceDeletion(workspace.id);
      if (result.ok) {
        toast.success(t("deletionCancelled"));
        router.refresh();
      } else {
        toast.error(result.message);
      }
    });
  }

  return (
    <Card className="ring-2 ring-destructive/40">
      <CardHeader>
        <CardTitle>
          <h2>{t("scheduledTitle")}</h2>
        </CardTitle>
        <CardDescription>
          {t("scheduledDescription", { name: workspace.name, date: format.dateTime(new Date(scheduledFor), { dateStyle: "long" }) })}{" "}
          {canCancel ? t("scheduledOwnerHint") : t("scheduledMemberHint")}
        </CardDescription>
      </CardHeader>
      {canCancel && (
        <CardContent>
          <Button size="lg" variant="outline" className="w-full" onClick={cancel} disabled={pending}>
            <UndoIcon />
            {t("cancelDeletion")}
          </Button>
        </CardContent>
      )}
    </Card>
  );
}

function ScheduleDeletion({
  workspace,
  form,
  open,
  onOpenChange: setOpen,
}: {
  workspace: { id: string; name: string };
  form: ScheduleForm;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const t = useTranslations();

  return (
    <Card>
      <CardHeader>
        <CardTitle>
          <h2>{t("workspaces.deleteTitle")}</h2>
        </CardTitle>
        <CardDescription>{t("workspaces.deleteDescription", { name: workspace.name })}</CardDescription>
      </CardHeader>
      <CardContent className="grid gap-3">
        <p className="text-sm">{t("workspaces.deleteExport")}</p>
        <div className="grid gap-2 sm:grid-cols-2">
          <a href="/export?format=json" download className={buttonVariants({ variant: "outline", size: "lg" })}>
            <DownloadIcon />
            {t("settings.exportJson")}
          </a>
          <a href="/export?format=csv" download className={buttonVariants({ variant: "outline", size: "lg" })}>
            <DownloadIcon />
            {t("settings.exportCsv")}
          </a>
        </div>
        <p className="text-sm text-muted-foreground">{t("workspaces.deleteBackups")}</p>
        <Button variant="destructive" size="lg" className="w-full border-2 border-destructive/25" onClick={() => setOpen(true)}>
          <Trash2Icon />
          {t("workspaces.delete")}
        </Button>
      </CardContent>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t("workspaces.deleteConfirmTitle", { name: workspace.name })}</DialogTitle>
            <DialogDescription>{t("workspaces.deleteDescription", { name: workspace.name })}</DialogDescription>
          </DialogHeader>
          {open && <ConfirmDeletionForm workspace={workspace} form={form} />}
        </DialogContent>
      </Dialog>
    </Card>
  );
}

function ConfirmDeletionForm({ workspace, form }: { workspace: { id: string; name: string }; form: ScheduleForm }) {
  const t = useTranslations();
  const format = useFormatter();
  const [typed, setTyped] = useState("");
  const { state, onSubmit, pending } = form;
  // Fixed when the dialog opens, so it doesn't change on re-renders.
  const [scheduledFor] = useState(() => deletionDate(Date.now()));
  const date = format.dateTime(scheduledFor, { dateStyle: "long" });

  return (
    <form onSubmit={onSubmit} className="grid gap-4" noValidate>
      <p className="text-sm">{t("workspaces.deleteConfirmDescription", { name: workspace.name, date })}</p>
      <p className="text-sm text-muted-foreground">{t("workspaces.deleteBackups")}</p>
      <input type="hidden" name="id" value={workspace.id} />
      <Field
        id="delete-workspace-name"
        label={t("workspaces.deleteConfirmLabel", { name: workspace.name })}
        error={state.fieldErrors?.name}
      >
        <Input
          id="delete-workspace-name"
          name="name"
          autoComplete="off"
          value={typed}
          onChange={(event) => setTyped(event.target.value)}
          aria-invalid={Boolean(state.fieldErrors?.name)}
          className="bg-background"
        />
      </Field>
      {state.message && !state.ok && !state.fieldErrors && (
        <p className="text-sm text-destructive" role="alert">
          {state.message}
        </p>
      )}
      <Button type="submit" variant="destructive" size="lg" disabled={pending || typed.trim() !== workspace.name}>
        {t("workspaces.deleteConfirm")}
      </Button>
    </form>
  );
}
