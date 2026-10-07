"use client";

import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { createWorkspace } from "@/app/actions/workspaces";
import { Field } from "@/components/field";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { useFormAction } from "@/hooks/use-form-action";

/** Creates a workspace and switches to it. */
export function CreateWorkspaceForm({ onCreated, submitLabel }: { onCreated?: () => void; submitLabel?: string }) {
  const t = useTranslations();
  const router = useRouter();
  const { state, onSubmit, pending } = useFormAction(createWorkspace, () => {
    toast.success(t("workspaces.created"));
    onCreated?.();
    router.refresh();
  });

  return (
    <form onSubmit={onSubmit} className="grid gap-4" noValidate>
      <Field id="workspace-name" label={t("workspaces.name")} error={state.fieldErrors?.name}>
        <Input
          id="workspace-name"
          name="name"
          maxLength={100}
          required
          placeholder={t("workspaces.namePlaceholder")}
          aria-invalid={Boolean(state.fieldErrors?.name)}
          className="bg-background"
        />
      </Field>
      {state.message && !state.ok && !state.fieldErrors && (
        <p className="text-sm text-destructive" role="alert">
          {state.message}
        </p>
      )}
      <Button type="submit" size="lg" disabled={pending}>
        {pending ? t("common.saving") : (submitLabel ?? t("workspaces.create"))}
      </Button>
    </form>
  );
}

export function CreateWorkspaceDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (open: boolean) => void }) {
  const t = useTranslations("workspaces");
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{t("createTitle")}</DialogTitle>
          <DialogDescription>{t("createDescription")}</DialogDescription>
        </DialogHeader>
        {open && <CreateWorkspaceForm onCreated={() => onOpenChange(false)} />}
      </DialogContent>
    </Dialog>
  );
}
