"use client";

import { useTranslations } from "next-intl";
import { useState } from "react";
import { toast } from "sonner";

import { saveTransaction } from "@/app/actions/transactions";
import { ChipRadioGroup } from "@/components/chip-radio";
import { ColorTile } from "@/components/color-tile";
import { Field } from "@/components/field";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useFormAction } from "@/hooks/use-form-action";
import { toDecimalString } from "@/lib/money";
import { cn } from "@/lib/utils";

import type { AccountOption, CategoryOption, TransactionRow, TransactionType } from "./types";

const types: TransactionType[] = ["expense", "income", "transfer"];

const typeStyles: Record<TransactionType, string> = {
  expense: "bg-expense text-card",
  income: "bg-income text-card",
  transfer: "bg-primary text-primary-foreground",
};

const amountColors: Record<TransactionType, string> = {
  expense: "text-expense",
  income: "text-income",
  transfer: "text-primary",
};

export function TransactionDialog({
  accounts,
  categories,
  transaction,
  defaultDate,
  trigger,
  open: controlledOpen,
  onOpenChange,
}: {
  accounts: AccountOption[];
  categories: CategoryOption[];
  transaction?: TransactionRow;
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
      <DialogContent className="max-h-[90dvh] overflow-y-auto sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{t(transaction ? "transactions.editTitle" : "transactions.addTitle")}</DialogTitle>
        </DialogHeader>
        {open && (
          <TransactionForm
            accounts={accounts}
            categories={categories}
            transaction={transaction}
            defaultDate={defaultDate}
            onSaved={() => {
              toast.success(t("transactions.saved"));
              setOpen(false);
            }}
            onCancel={() => setOpen(false)}
          />
        )}
      </DialogContent>
    </Dialog>
  );
}

export function TransactionForm({
  accounts,
  categories,
  transaction: transactionProp,
  defaultDate,
  onSaved,
  onCancel,
}: {
  accounts: AccountOption[];
  categories: CategoryOption[];
  transaction?: TransactionRow;
  defaultDate: string;
  onSaved: () => void;
  onCancel: () => void;
}) {
  const t = useTranslations();
  // Keep the values the form opened with: after saving, the revalidated transaction
  // arrives before the dialog closes, and changing defaultValue on mounted
  // inputs makes Base UI warn.
  const [transaction] = useState(transactionProp);
  const { state, onSubmit, pending } = useFormAction(saveTransaction, onSaved);

  // Archived accounts and categories are only offered when already selected.
  const accountOptions = accounts.filter(
    (a) => !a.archived || a.id === transaction?.account_id || a.id === transaction?.destination_account_id,
  );
  const [type, setType] = useState<TransactionType>(transaction?.type ?? "expense");
  const [accountId, setAccountId] = useState(transaction?.account_id ?? accountOptions[0]?.id ?? "");
  const [destinationId, setDestinationId] = useState(
    transaction?.destination_account_id ?? accountOptions.find((a) => a.id !== accountId)?.id ?? "",
  );

  const account = accountOptions.find((a) => a.id === accountId);
  // The destination can never be the source account; fall back to the next one.
  const destinationValue = destinationId !== accountId ? destinationId : (accountOptions.find((a) => a.id !== accountId)?.id ?? "");
  const destination = accountOptions.find((a) => a.id === destinationValue);
  const crossCurrency = type === "transfer" && account && destination && account.currency !== destination.currency;
  const categoryOptions = categories.filter(
    (c) => c.kind === type && (!c.archived || c.id === transaction?.category_id),
  );
  const errors = state.fieldErrors ?? {};

  const accountChip = (a: AccountOption) => ({ value: a.id, label: a.name, detail: a.currency });
  const sign = type === "expense" ? "−" : type === "income" ? "+" : "";

  return (
    <form onSubmit={onSubmit} className="grid gap-5" noValidate>
      {transaction && <input type="hidden" name="id" value={transaction.id} />}
      <input type="hidden" name="type" value={type} />

      <div role="radiogroup" aria-label={t("transactions.type")} className="grid grid-cols-3 gap-1 rounded-2xl bg-muted p-1">
        {types.map((option) => (
          <button
            key={option}
            type="button"
            role="radio"
            aria-checked={type === option}
            onClick={() => setType(option)}
            className={cn(
              "min-h-11 rounded-xl px-2 text-sm font-bold transition-colors",
              type === option ? typeStyles[option] : "text-muted-foreground hover:text-foreground",
            )}
          >
            {t(`transactions.types.${option}`)}
          </button>
        ))}
      </div>

      <div className="grid gap-1.5 rounded-3xl bg-card px-5 pt-4 pb-3 ring-1 ring-foreground/5">
        <Label htmlFor="amount">{`${t("transactions.amount")}${account ? ` (${account.currency})` : ""}`}</Label>
        <div className={cn("flex items-baseline gap-1 font-heading font-bold", amountColors[type])}>
          <span aria-hidden className="text-4xl">
            {sign}
          </span>
          <input
            id="amount"
            name="amount"
            inputMode="decimal"
            autoComplete="off"
            placeholder="0.00"
            defaultValue={transaction ? toDecimalString(transaction.amount, transaction.minor_units) : ""}
            aria-invalid={Boolean(errors.amount)}
            aria-describedby={errors.amount ? "amount-error" : undefined}
            autoFocus={!transaction}
            required
            className="w-full min-w-0 bg-transparent text-5xl tracking-tight tabular-nums outline-none placeholder:text-muted-foreground/45"
          />
          {account && <span className="font-sans text-sm font-bold tracking-wide text-muted-foreground">{account.currency}</span>}
        </div>
        {errors.amount && (
          <p id="amount-error" className="text-xs text-destructive" role="alert">
            {errors.amount}
          </p>
        )}
      </div>

      <ChipRadioGroup
        name="account_id"
        label={t(type === "transfer" ? "transactions.fromAccount" : "transactions.account")}
        options={accountOptions.map(accountChip)}
        value={accountId}
        onChange={setAccountId}
        error={errors.account_id}
      />

      {type === "transfer" ? (
        <>
          <ChipRadioGroup
            name="destination_account_id"
            label={t("transactions.toAccount")}
            options={accountOptions.filter((a) => a.id !== accountId).map(accountChip)}
            value={destinationValue}
            onChange={setDestinationId}
            error={errors.destination_account_id}
          />
          {crossCurrency && destination && (
            <Field
              id="destination_amount"
              label={t("transactions.destinationAmount", { currency: destination.currency })}
              error={errors.destination_amount}
            >
              <Input
                id="destination_amount"
                name="destination_amount"
                inputMode="decimal"
                autoComplete="off"
                placeholder="0.00"
                defaultValue={
                  transaction?.destination_amount != null && transaction.destination_minor_units != null
                    ? toDecimalString(transaction.destination_amount, transaction.destination_minor_units)
                    : ""
                }
                aria-invalid={Boolean(errors.destination_amount)}
              />
            </Field>
          )}
        </>
      ) : (
        <ChipRadioGroup
          key={type}
          name="category_id"
          label={`${t("transactions.category")} (${t("common.optional")})`}
          options={[
            { value: "", label: t("common.none") },
            ...categoryOptions.map((c) => ({
              value: c.id,
              label: c.name,
              leading: <ColorTile color={c.color} label={c.name} className="size-7 rounded-[9px] text-[13px]" />,
            })),
          ]}
          defaultValue={transaction?.type === type ? (transaction.category_id ?? "") : ""}
          error={errors.category_id}
        />
      )}

      <div className="grid gap-4 sm:grid-cols-[11rem_1fr]">
        <Field id="occurred_on" label={t("transactions.date")} error={errors.occurred_on}>
          <Input id="occurred_on" name="occurred_on" type="date" defaultValue={transaction?.occurred_on ?? defaultDate} required />
        </Field>
        <Field id="description" label={t("transactions.description")} error={errors.description}>
          <Input
            id="description"
            name="description"
            maxLength={500}
            placeholder={t("transactions.descriptionPlaceholder")}
            defaultValue={transaction?.description ?? ""}
          />
        </Field>
      </div>

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
