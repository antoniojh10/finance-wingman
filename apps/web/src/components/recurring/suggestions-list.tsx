"use client";

import { useTranslations } from "next-intl";
import { useState, useTransition } from "react";
import { toast } from "sonner";

import { acceptSuggestion, dismissSuggestion } from "@/app/actions/recurring";
import { Field } from "@/components/field";
import { Money } from "@/components/money";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { useFormAction } from "@/hooks/use-form-action";
import { toDecimalString } from "@/lib/money";
import type { IntervalUnit, RecurringType } from "@/lib/recurring";

export type SuggestionRow = {
  key: string;
  name: string;
  type: RecurringType;
  account_name: string;
  category_name: string | null;
  currency: string;
  minor_units: number;
  amount: number;
  interval_unit: IntervalUnit;
  interval_count: number;
  transaction_ids: string[];
};

/**
 * Recurring patterns detected in the transactions. Each can be added (the
 * matching transactions get linked) or dismissed. Renders no list when there
 * are no suggestions.
 */
export function SuggestionsList({ suggestions }: { suggestions: SuggestionRow[] }) {
  const t = useTranslations("subscriptions");
  // The dialog lives here, not in the row: once accepted the suggestion
  // disappears from the props and its row unmounts before the toast and close.
  const [accepting, setAccepting] = useState<SuggestionRow | null>(null);
  const [open, setOpen] = useState(false);
  const [pending, startTransition] = useTransition();

  function dismiss(key: string) {
    startTransition(async () => {
      const result = await dismissSuggestion(key);
      if (result.ok) {
        toast.success(t("suggestions.dismissedToast"));
      } else {
        toast.error(result.message);
      }
    });
  }

  return (
    <>
      {suggestions.length > 0 && (
        <section className="grid grid-cols-1 gap-3" aria-labelledby="suggestions-heading" data-testid="suggestions">
          <div className="px-1">
            <h2 id="suggestions-heading" className="text-[19px] font-bold tracking-tight">
              {t("suggestions.title")}
            </h2>
            <p className="text-[13px] text-muted-foreground">{t("suggestions.description")}</p>
          </div>
          <ul className="divide-y divide-border/70 rounded-3xl bg-card px-4 ring-1 ring-foreground/5">
            {suggestions.map((s) => (
              <SuggestionItem
                key={s.key}
                suggestion={s}
                disabled={pending}
                onAdd={() => {
                  setAccepting(s);
                  setOpen(true);
                }}
                onDismiss={() => dismiss(s.key)}
              />
            ))}
          </ul>
        </section>
      )}
      {accepting && <AcceptSuggestionDialog suggestion={accepting} open={open} onOpenChange={setOpen} />}
    </>
  );
}

function SuggestionItem({
  suggestion: s,
  disabled,
  onAdd,
  onDismiss,
}: {
  suggestion: SuggestionRow;
  disabled: boolean;
  onAdd: () => void;
  onDismiss: () => void;
}) {
  const t = useTranslations("subscriptions");
  const details = [
    t(`frequency.${s.interval_unit}`, { count: s.interval_count }),
    s.account_name,
    s.category_name,
    t("suggestions.matches", { count: s.transaction_ids.length }),
  ].filter(Boolean);

  return (
    <li className="flex flex-wrap items-center gap-x-3 gap-y-2 py-3" data-testid="suggestion-row">
      <div className="min-w-0 flex-1 basis-48">
        <p className="truncate text-[15.5px] font-semibold">{s.name}</p>
        <p className="text-[12.5px] text-muted-foreground">{details.join(" · ")}</p>
      </div>
      <Money
        amount={s.amount}
        currency={s.currency}
        minorUnits={s.minor_units}
        tone={s.type}
        signed
        className="text-[15.5px] font-bold whitespace-nowrap"
      />
      <div className="flex gap-2">
        <Button size="sm" onClick={onAdd} disabled={disabled}>
          {t("suggestions.add")}
        </Button>
        <Button size="sm" variant="ghost" onClick={onDismiss} disabled={disabled}>
          {t("suggestions.dismiss")}
        </Button>
      </div>
    </li>
  );
}

function AcceptSuggestionDialog({
  suggestion,
  open,
  onOpenChange,
}: {
  suggestion: SuggestionRow;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const t = useTranslations("subscriptions");
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="block max-h-[90dvh] overflow-x-hidden overflow-y-auto sm:max-w-md">
        <DialogHeader className="mb-5">
          <DialogTitle>{t("suggestions.acceptTitle", { name: suggestion.name })}</DialogTitle>
          <DialogDescription>{t("suggestions.acceptDescription")}</DialogDescription>
        </DialogHeader>
        {open && (
          <AcceptSuggestionForm
            suggestion={suggestion}
            onAccepted={() => {
              toast.success(t("suggestions.acceptedToast"));
              onOpenChange(false);
            }}
            onCancel={() => onOpenChange(false)}
          />
        )}
      </DialogContent>
    </Dialog>
  );
}

function AcceptSuggestionForm({
  suggestion: suggestionProp,
  onAccepted,
  onCancel,
}: {
  suggestion: SuggestionRow;
  onAccepted: () => void;
  onCancel: () => void;
}) {
  const t = useTranslations();
  // Snapshot: the suggestion leaves the props before the dialog closes.
  const [suggestion] = useState(suggestionProp);
  const { state, onSubmit, pending } = useFormAction(acceptSuggestion, onAccepted);
  const errors = state.fieldErrors ?? {};

  return (
    <form onSubmit={onSubmit} className="grid min-w-0 grid-cols-[minmax(0,1fr)] gap-5" noValidate>
      <input type="hidden" name="key" value={suggestion.key} />
      <Field id="suggestion-name" label={t("subscriptions.name")} error={errors.name}>
        <Input
          id="suggestion-name"
          name="name"
          autoComplete="off"
          autoFocus
          defaultValue={suggestion.name}
          aria-invalid={Boolean(errors.name)}
        />
      </Field>
      <Field
        id="suggestion-amount"
        label={`${t("subscriptions.amount")} (${suggestion.currency})`}
        hint={t("subscriptions.amountHint")}
        error={errors.amount}
      >
        <Input
          id="suggestion-amount"
          name="amount"
          inputMode="decimal"
          autoComplete="off"
          defaultValue={toDecimalString(suggestion.amount, suggestion.minor_units)}
          aria-invalid={Boolean(errors.amount)}
        />
      </Field>

      {state.message && !state.ok && (
        <p className="text-sm text-destructive" role="alert">
          {state.message}
        </p>
      )}

      <div className="flex flex-col-reverse gap-2 pt-1 sm:flex-row sm:justify-end">
        <Button type="button" variant="ghost" size="lg" onClick={onCancel}>
          {t("common.cancel")}
        </Button>
        <Button type="submit" size="lg" disabled={pending} className="sm:min-w-36">
          {pending ? t("common.saving") : t("subscriptions.suggestions.accept")}
        </Button>
      </div>
    </form>
  );
}
