"use client";

import { useLocale, useTranslations } from "next-intl";
import { useState } from "react";
import { toast } from "sonner";

import { registerRecurringPayment } from "@/app/actions/recurring";
import { Field } from "@/components/field";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { useFormAction } from "@/hooks/use-form-action";
import { formatDate } from "@/lib/dates";
import { toDecimalString } from "@/lib/money";

/** What a payment is registered for: one period of a recurring item. */
export type PaymentTarget = {
  id: string;
  name: string;
  /** Estimated amount, in minor units. */
  amount: number;
  currency: string;
  minor_units: number;
  /** Due date being paid. */
  period: string;
};

/**
 * Confirm dialog to register a payment, prefilled with the estimated amount,
 * today and the current period. Amount and date are editable.
 */
export function RegisterPaymentDialog({
  target,
  defaultDate,
  trigger,
  open: controlledOpen,
  onOpenChange,
}: {
  target: PaymentTarget;
  /** Today's date, the default payment date. */
  defaultDate: string;
  trigger?: React.ReactElement;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
}) {
  const t = useTranslations("subscriptions");
  const [uncontrolledOpen, setUncontrolledOpen] = useState(false);
  const open = controlledOpen ?? uncontrolledOpen;
  const setOpen = onOpenChange ?? setUncontrolledOpen;

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      {trigger && <DialogTrigger render={trigger} />}
      <DialogContent className="block max-h-[90dvh] overflow-x-hidden overflow-y-auto sm:max-w-md">
        <DialogHeader className="mb-5">
          <DialogTitle>{t("registerTitle", { name: target.name })}</DialogTitle>
          <DialogDescription>{t("registerDescription")}</DialogDescription>
        </DialogHeader>
        {open && (
          <RegisterPaymentForm
            target={target}
            defaultDate={defaultDate}
            onRegistered={() => {
              toast.success(t("registeredToast"));
              setOpen(false);
            }}
            onCancel={() => setOpen(false)}
          />
        )}
      </DialogContent>
    </Dialog>
  );
}

export function RegisterPaymentForm({
  target: targetProp,
  defaultDate,
  onRegistered,
  onCancel,
}: {
  target: PaymentTarget;
  defaultDate: string;
  onRegistered: () => void;
  onCancel: () => void;
}) {
  const t = useTranslations();
  const locale = useLocale();
  // Keep the values the form opened with: the revalidated item arrives before
  // the dialog closes and changing defaultValue on mounted inputs makes Base UI warn.
  const [target] = useState(targetProp);
  const { state, onSubmit, pending } = useFormAction(registerRecurringPayment, onRegistered);
  const errors = state.fieldErrors ?? {};

  return (
    <form onSubmit={onSubmit} className="grid min-w-0 grid-cols-[minmax(0,1fr)] gap-5" noValidate>
      <input type="hidden" name="id" value={target.id} />
      <input type="hidden" name="period" value={target.period} />
      <p className="text-[13px] text-muted-foreground">
        {t("subscriptions.registerPeriod", { date: formatDate(target.period, locale) })}
      </p>
      <Field id="payment-amount" label={`${t("subscriptions.amount")} (${target.currency})`} error={errors.amount}>
        <Input
          id="payment-amount"
          name="amount"
          inputMode="decimal"
          autoComplete="off"
          autoFocus
          defaultValue={toDecimalString(target.amount, target.minor_units)}
          aria-invalid={Boolean(errors.amount)}
        />
      </Field>
      <Field id="payment-date" label={t("subscriptions.paidOn")} error={errors.date}>
        <Input id="payment-date" name="date" type="date" required defaultValue={defaultDate} />
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
          {pending ? t("common.saving") : t("subscriptions.register")}
        </Button>
      </div>
    </form>
  );
}
