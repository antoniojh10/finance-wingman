"use client";

import { Trash2Icon, UndoIcon } from "lucide-react";
import { useRouter } from "next/navigation";
import { useFormatter, useTranslations } from "next-intl";
import { useState, useTransition } from "react";
import { toast } from "sonner";

import { cancelAccountDeletion, scheduleAccountDeletion } from "@/app/actions/deletion";
import { Field } from "@/components/field";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { useFormAction } from "@/hooks/use-form-action";
import { deletionDate } from "@/lib/deletion";
import { cn } from "@/lib/utils";

export type AccountDeletionImpact = {
  id: string;
  name: string;
  role: "owner" | "member";
  members: number;
  outcome: "leave" | "delete" | "blocked";
};

export type DeleteAccountProps = {
  email: string;
  deletion: { scheduled_for?: string; workspaces: AccountDeletionImpact[] };
};

/**
 * Deleting the user's account: what happens to each workspace, the
 * confirmation (typing their email), and cancelling a scheduled deletion.
 */
export function DeleteAccount({ email, deletion }: DeleteAccountProps) {
  const t = useTranslations("account");
  const router = useRouter();
  const [open, setOpen] = useState(false);
  // Held here, not in the dialog: scheduling re-renders the page with the
  // scheduled state, which unmounts the dialog before it could report it.
  const form = useFormAction(scheduleAccountDeletion, () => {
    toast.success(t("scheduled"));
    setOpen(false);
    router.refresh();
  });

  if (deletion.scheduled_for) {
    return <ScheduledDeletion scheduledFor={deletion.scheduled_for} />;
  }
  const blocked = deletion.workspaces.some((w) => w.outcome === "blocked");

  return (
    <Card>
      <CardHeader>
        <CardTitle>
          <h2>{t("title")}</h2>
        </CardTitle>
        <CardDescription>{t("description")}</CardDescription>
      </CardHeader>
      <CardContent className="grid gap-3">
        <WorkspaceImpacts workspaces={deletion.workspaces} />
        <p className="text-sm text-muted-foreground">{t("backups")}</p>
        {blocked && (
          <p className="text-sm text-destructive" role="alert">
            {t("blockedSummary")}
          </p>
        )}
        <Button
          variant="destructive"
          size="lg"
          className="w-full border-2 border-destructive/25"
          disabled={blocked}
          onClick={() => setOpen(true)}
        >
          <Trash2Icon />
          {t("delete")}
        </Button>
      </CardContent>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t("confirmTitle")}</DialogTitle>
            <DialogDescription>{t("description")}</DialogDescription>
          </DialogHeader>
          {open && <ConfirmForm email={email} workspaces={deletion.workspaces} form={form} />}
        </DialogContent>
      </Dialog>
    </Card>
  );
}

const outcomeStyles: Record<AccountDeletionImpact["outcome"], string> = {
  leave: "bg-muted/50",
  delete: "bg-destructive/10",
  blocked: "bg-destructive/10 ring-1 ring-destructive/40",
};

function WorkspaceImpacts({ workspaces }: { workspaces: AccountDeletionImpact[] }) {
  const t = useTranslations();
  if (workspaces.length === 0) {
    return null;
  }
  const messages = { leave: t("account.leave"), delete: t("account.deleteWorkspace"), blocked: t("account.blocked") };
  return (
    <ul className="grid gap-2" aria-label={t("account.workspaces")}>
      {workspaces.map((w) => (
        <li key={w.id} data-testid="deletion-impact" className={cn("grid gap-1 rounded-2xl px-3 py-2.5", outcomeStyles[w.outcome])}>
          <span className="flex items-center gap-2">
            <span className="min-w-0 flex-1 truncate text-sm font-semibold">{w.name}</span>
            <Badge variant={w.role === "owner" ? "default" : "secondary"}>{t(`workspaces.roles.${w.role}`)}</Badge>
          </span>
          <span className={cn("text-xs", w.outcome === "leave" ? "text-muted-foreground" : "text-destructive")}>{messages[w.outcome]}</span>
        </li>
      ))}
    </ul>
  );
}

function ConfirmForm({
  email,
  workspaces,
  form,
}: {
  email: string;
  workspaces: AccountDeletionImpact[];
  form: ReturnType<typeof useFormAction>;
}) {
  const t = useTranslations("account");
  const format = useFormatter();
  const [typed, setTyped] = useState("");
  // Fixed when the dialog opens, so it doesn't change on re-renders.
  const [scheduledFor] = useState(() => deletionDate(Date.now()));
  const { state, onSubmit, pending } = form;
  const deleted = workspaces.filter((w) => w.outcome === "delete");

  return (
    <form onSubmit={onSubmit} className="grid gap-4" noValidate>
      <p className="text-sm">{t("confirmDescription", { date: format.dateTime(scheduledFor, { dateStyle: "long" }) })}</p>
      {deleted.length > 0 && (
        <ul className="grid gap-1 text-sm text-destructive">
          {deleted.map((w) => (
            <li key={w.id}>
              <span className="font-semibold">{w.name}</span>: {t("deleteWorkspace")}
            </li>
          ))}
        </ul>
      )}
      <p className="text-sm text-muted-foreground">{t("backups")}</p>
      <Field id="delete-account-email" label={t("confirmLabel", { email })} error={state.fieldErrors?.email}>
        <Input
          id="delete-account-email"
          name="email"
          type="email"
          autoComplete="off"
          value={typed}
          onChange={(event) => setTyped(event.target.value)}
          aria-invalid={Boolean(state.fieldErrors?.email)}
          className="bg-background"
        />
      </Field>
      {state.message && !state.ok && !state.fieldErrors && (
        <p className="text-sm text-destructive" role="alert">
          {state.message}
        </p>
      )}
      <Button
        type="submit"
        variant="destructive"
        size="lg"
        disabled={pending || typed.trim().toLowerCase() !== email.toLowerCase()}
      >
        {t("confirm")}
      </Button>
    </form>
  );
}

function ScheduledDeletion({ scheduledFor }: { scheduledFor: string }) {
  const t = useTranslations("account");
  const format = useFormatter();
  const router = useRouter();
  const [pending, startTransition] = useTransition();

  function cancel() {
    startTransition(async () => {
      const result = await cancelAccountDeletion();
      if (result.ok) {
        toast.success(t("cancelled"));
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
        <CardDescription>{t("scheduledDescription", { date: format.dateTime(new Date(scheduledFor), { dateStyle: "long" }) })}</CardDescription>
      </CardHeader>
      <CardContent>
        <Button size="lg" variant="outline" className="w-full" onClick={cancel} disabled={pending}>
          <UndoIcon />
          {t("cancel")}
        </Button>
      </CardContent>
    </Card>
  );
}
