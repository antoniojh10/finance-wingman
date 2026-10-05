"use client";

import { useTranslations } from "next-intl";
import { useState } from "react";
import { toast } from "sonner";

import { saveCategory } from "@/app/actions/categories";
import { Field } from "@/components/field";
import { NativeSelect } from "@/components/native-select";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { useFormAction } from "@/hooks/use-form-action";
import { cn } from "@/lib/utils";

// A small palette of distinguishable colors to tag categories.
export const categoryColors = ["#6d4aff", "#ff6b4a", "#ffb020", "#2f8cff", "#14b8a6", "#f05aa6", "#12a150", "#9aa0b8"];

export type EditableCategory = {
  id: string;
  name: string;
  kind: "expense" | "income";
  color?: string | null;
};

export function CategoryDialog({
  category,
  defaultKind = "expense",
  trigger,
  open: controlledOpen,
  onOpenChange,
}: {
  category?: EditableCategory;
  defaultKind?: "expense" | "income";
  /** Omit when the dialog is opened through `open` (e.g. from a menu). */
  trigger?: React.ReactElement;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
}) {
  const t = useTranslations();
  const [uncontrolledOpen, setUncontrolledOpen] = useState(false);
  const open = controlledOpen ?? uncontrolledOpen;
  const setOpen = onOpenChange ?? setUncontrolledOpen;

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      {trigger && <DialogTrigger render={trigger} />}
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{t(category ? "categories.editTitle" : "categories.addTitle")}</DialogTitle>
        </DialogHeader>
        {open && (
          <CategoryForm
            category={category}
            defaultKind={defaultKind}
            onSaved={() => {
              toast.success(t("categories.saved"));
              setOpen(false);
            }}
            onCancel={() => setOpen(false)}
          />
        )}
      </DialogContent>
    </Dialog>
  );
}

export function CategoryForm({
  category,
  defaultKind,
  onSaved,
  onCancel,
}: {
  category?: EditableCategory;
  defaultKind: "expense" | "income";
  onSaved: () => void;
  onCancel: () => void;
}) {
  const t = useTranslations();
  const { state, onSubmit, pending } = useFormAction(saveCategory, onSaved);
  const [color, setColor] = useState(category?.color ?? categoryColors[0]);
  const errors = state.fieldErrors ?? {};

  return (
    <form onSubmit={onSubmit} className="grid gap-4" noValidate>
      {category && <input type="hidden" name="id" value={category.id} />}
      <input type="hidden" name="color" value={color} />
      <Field id="name" label={t("categories.name")} error={errors.name}>
        <Input
          id="name"
          name="name"
          maxLength={100}
          required
          autoFocus
          placeholder={t("categories.namePlaceholder")}
          defaultValue={category?.name}
          aria-invalid={Boolean(errors.name)}
        />
      </Field>
      <Field id="kind" label={t("categories.kind")} hint={category ? t("categories.kindLocked") : undefined}>
        <NativeSelect id="kind" name="kind" defaultValue={category?.kind ?? defaultKind} disabled={Boolean(category)}>
          <option value="expense">{t("categories.kinds.expense")}</option>
          <option value="income">{t("categories.kinds.income")}</option>
        </NativeSelect>
      </Field>
      <fieldset className="grid gap-1.5">
        <legend className="mb-1.5 text-sm font-medium">{t("categories.color")}</legend>
        <div className="flex flex-wrap gap-2" role="radiogroup" aria-label={t("categories.color")}>
          {categoryColors.map((c) => (
            <button
              key={c}
              type="button"
              role="radio"
              aria-checked={color === c}
              aria-label={c}
              onClick={() => setColor(c)}
              className={cn(
                "size-8 rounded-full border-2 transition-transform",
                color === c ? "scale-110 border-foreground" : "border-transparent",
              )}
              style={{ backgroundColor: c }}
            />
          ))}
        </div>
        {errors.color && <p className="text-xs text-destructive">{errors.color}</p>}
      </fieldset>
      {state.message && !state.ok && (
        <p className="text-sm text-destructive" role="alert">
          {state.message}
        </p>
      )}
      <DialogFooter>
        <Button type="button" variant="outline" onClick={onCancel}>
          {t("common.cancel")}
        </Button>
        <Button type="submit" disabled={pending}>
          {pending ? t("common.saving") : t("common.save")}
        </Button>
      </DialogFooter>
    </form>
  );
}
