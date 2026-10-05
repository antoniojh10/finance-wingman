"use client";

import { useTranslations } from "next-intl";
import { useState } from "react";
import { toast } from "sonner";

import { saveRecurring } from "@/app/actions/recurring";
import { ChipRadioGroup } from "@/components/chip-radio";
import { ColorTile } from "@/components/color-tile";
import { Field } from "@/components/field";
import { NativeSelect } from "@/components/native-select";
import type { AccountOption, CategoryOption } from "@/components/transactions/types";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { useFormAction } from "@/hooks/use-form-action";
import { toDecimalString } from "@/lib/money";
import type { IntervalUnit, PaymentStatus, RecurringType } from "@/lib/recurring";

export type RecurringRow = {
  id: string;
  name: string;
  type: RecurringType;
  status: "active" | "paused" | "cancelled";
  account_id: string;
  account_name: string;
  category_id: string | null;
  category_name: string | null;
  currency: string;
  minor_units: number;
  amount: number;
  interval_unit: IntervalUnit;
  interval_count: number;
  start_on: string;
  total_payments: number | null;
  next_due_on: string | null;
  notes: string;
  /** Period the item is in now; null unless the item is active. */
  current_period?: { due_on: string; status: PaymentStatus } | null;
  last_payment?: { amount: number; date: string; due_on: string; transaction_id: string } | null;
};

const intervalUnits: IntervalUnit[] = ["week", "month", "year"];

export function RecurringDialog({
  accounts,
  categories,
  item,
  defaultDate,
  trigger,
  open: controlledOpen,
  onOpenChange,
}: {
  accounts: AccountOption[];
  categories: CategoryOption[];
  item?: RecurringRow;
  defaultDate: string;
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
      <DialogContent className="block max-h-[90dvh] overflow-x-hidden overflow-y-auto sm:max-w-lg">
        <DialogHeader className="mb-5">
          <DialogTitle>{t(item ? "subscriptions.editTitle" : "subscriptions.addTitle")}</DialogTitle>
        </DialogHeader>
        {open && (
          <RecurringForm
            accounts={accounts}
            categories={categories}
            item={item}
            defaultDate={defaultDate}
            onSaved={() => {
              toast.success(t("subscriptions.saved"));
              setOpen(false);
            }}
            onCancel={() => setOpen(false)}
          />
        )}
      </DialogContent>
    </Dialog>
  );
}

export function RecurringForm({
  accounts,
  categories,
  item: itemProp,
  defaultDate,
  onSaved,
  onCancel,
}: {
  accounts: AccountOption[];
  categories: CategoryOption[];
  item?: RecurringRow;
  defaultDate: string;
  onSaved: () => void;
  onCancel: () => void;
}) {
  const t = useTranslations();
  // Keep the values the form opened with: after saving, the revalidated item
  // arrives before the dialog closes, and changing defaultValue on mounted
  // inputs makes Base UI warn.
  const [item] = useState(itemProp);
  const { state, onSubmit, pending } = useFormAction(saveRecurring, onSaved);
  const errors = state.fieldErrors ?? {};

  const accountOptions = accounts.filter((a) => !a.archived);
  const [type, setType] = useState<RecurringType>(item?.type ?? "expense");
  const [accountId, setAccountId] = useState(item?.account_id ?? accountOptions[0]?.id ?? "");
  const account = item ? { currency: item.currency } : accountOptions.find((a) => a.id === accountId);
  const categoryOptions = categories.filter((c) => c.kind === type && (!c.archived || c.id === item?.category_id));

  return (
    <form onSubmit={onSubmit} className="grid min-w-0 grid-cols-[minmax(0,1fr)] gap-5" noValidate>
      {item && <input type="hidden" name="id" value={item.id} />}

      <Field id="name" label={t("subscriptions.name")} error={errors.name}>
        <Input
          id="name"
          name="name"
          maxLength={100}
          required
          autoFocus
          placeholder={t("subscriptions.namePlaceholder")}
          defaultValue={item?.name}
          aria-invalid={Boolean(errors.name)}
        />
      </Field>

      {item ? (
        <>
          <input type="hidden" name="type" value={item.type} />
          <p className="text-[13px] text-muted-foreground">
            {t(`subscriptions.types.${item.type}`)} · {item.account_name} ({item.currency}).{" "}
            {t("subscriptions.lockedHint")}
          </p>
        </>
      ) : (
        <>
          <ChipRadioGroup
            name="type"
            label={t("subscriptions.type")}
            options={(["expense", "income"] as const).map((v) => ({ value: v, label: t(`subscriptions.types.${v}`) }))}
            value={type}
            onChange={(v) => setType(v as RecurringType)}
          />
          <ChipRadioGroup
            name="account_id"
            label={t("subscriptions.account")}
            options={accountOptions.map((a) => ({ value: a.id, label: a.name, detail: a.currency }))}
            value={accountId}
            onChange={setAccountId}
            error={errors.account_id}
          />
        </>
      )}

      <Field
        id="amount"
        label={`${t("subscriptions.amount")}${account ? ` (${account.currency})` : ""}`}
        hint={t("subscriptions.amountHint")}
        error={errors.amount}
      >
        <Input
          id="amount"
          name="amount"
          inputMode="decimal"
          autoComplete="off"
          placeholder="0.00"
          defaultValue={item ? toDecimalString(item.amount, item.minor_units) : ""}
          aria-invalid={Boolean(errors.amount)}
        />
      </Field>

      <ChipRadioGroup
        key={type}
        name="category_id"
        label={`${t("subscriptions.category")} (${t("common.optional")})`}
        options={[
          { value: "", label: t("common.none") },
          ...categoryOptions.map((c) => ({
            value: c.id,
            label: c.name,
            leading: <ColorTile color={c.color} label={c.name} className="size-7 rounded-[9px] text-[13px]" />,
          })),
        ]}
        defaultValue={item && item.type === type ? (item.category_id ?? "") : ""}
        error={errors.category_id}
      />

      <div className="grid grid-cols-2 gap-4 [&>*]:min-w-0">
        <Field id="interval_count" label={t("subscriptions.every")} error={errors.interval_count}>
          <Input
            id="interval_count"
            name="interval_count"
            type="number"
            inputMode="numeric"
            min={1}
            step={1}
            defaultValue={item?.interval_count ?? 1}
            aria-invalid={Boolean(errors.interval_count)}
          />
        </Field>
        <Field id="interval_unit" label={t("subscriptions.unit")} error={errors.interval_unit}>
          <NativeSelect id="interval_unit" name="interval_unit" defaultValue={item?.interval_unit ?? "month"}>
            {intervalUnits.map((unit) => (
              <option key={unit} value={unit}>
                {t(`subscriptions.units.${unit}`)}
              </option>
            ))}
          </NativeSelect>
        </Field>
      </div>

      <div className="grid gap-4 sm:grid-cols-2 [&>*]:min-w-0">
        <Field id="start_on" label={t("subscriptions.startOn")} error={errors.start_on}>
          <Input id="start_on" name="start_on" type="date" required defaultValue={item?.start_on ?? defaultDate} />
        </Field>
        <Field
          id="total_payments"
          label={`${t("subscriptions.totalPayments")} (${t("common.optional")})`}
          hint={t("subscriptions.totalPaymentsHint")}
          error={errors.total_payments}
        >
          <Input
            id="total_payments"
            name="total_payments"
            type="number"
            inputMode="numeric"
            min={1}
            step={1}
            defaultValue={item?.total_payments ?? ""}
            aria-invalid={Boolean(errors.total_payments)}
          />
        </Field>
      </div>

      <Field id="notes" label={`${t("subscriptions.notes")} (${t("common.optional")})`} error={errors.notes}>
        <Textarea id="notes" name="notes" maxLength={500} rows={2} defaultValue={item?.notes ?? ""} />
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
          {pending ? t("common.saving") : t("common.save")}
        </Button>
      </div>
    </form>
  );
}
