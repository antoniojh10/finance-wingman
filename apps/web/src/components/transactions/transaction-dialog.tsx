"use client";

import { useTranslations } from "next-intl";
import { useState } from "react";
import { toast } from "sonner";

import { saveTransaction } from "@/app/actions/transactions";
import { Field } from "@/components/field";
import { NativeSelect } from "@/components/native-select";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { useFormAction } from "@/hooks/use-form-action";
import { toDecimalString } from "@/lib/money";
import { cn } from "@/lib/utils";

import type { AccountOption, CategoryOption, TransactionRow, TransactionType } from "./types";

const types: TransactionType[] = ["expense", "income", "transfer"];

export function TransactionDialog({
  accounts,
  categories,
  transaction,
  defaultDate,
  trigger,
}: {
  accounts: AccountOption[];
  categories: CategoryOption[];
  transaction?: TransactionRow;
  defaultDate: string;
  trigger: React.ReactElement;
}) {
  const t = useTranslations();
  const [open, setOpen] = useState(false);

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={trigger} />
      <DialogContent className="max-h-[90dvh] overflow-y-auto sm:max-w-md">
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
  transaction,
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
  const destination = accountOptions.find((a) => a.id === destinationId);
  const crossCurrency = type === "transfer" && account && destination && account.currency !== destination.currency;
  const categoryOptions = categories.filter(
    (c) => c.kind === type && (!c.archived || c.id === transaction?.category_id),
  );
  const errors = state.fieldErrors ?? {};

  return (
    <form onSubmit={onSubmit} className="grid gap-4" noValidate>
      {transaction && <input type="hidden" name="id" value={transaction.id} />}
      <input type="hidden" name="type" value={type} />

      <div role="radiogroup" aria-label={t("transactions.type")} className="grid grid-cols-3 gap-1 rounded-lg bg-muted p-1">
        {types.map((option) => (
          <button
            key={option}
            type="button"
            role="radio"
            aria-checked={type === option}
            onClick={() => setType(option)}
            className={cn(
              "rounded-md px-2 py-1.5 text-sm font-medium transition-colors",
              type === option ? "bg-background shadow-sm" : "text-muted-foreground hover:text-foreground",
            )}
          >
            {t(`transactions.types.${option}`)}
          </button>
        ))}
      </div>

      <Field
        id="account_id"
        label={t(type === "transfer" ? "transactions.fromAccount" : "transactions.account")}
        error={errors.account_id}
      >
        <NativeSelect id="account_id" name="account_id" value={accountId} onChange={(e) => setAccountId(e.target.value)}>
          {accountOptions.map((a) => (
            <option key={a.id} value={a.id}>
              {a.name} ({a.currency})
            </option>
          ))}
        </NativeSelect>
      </Field>

      <Field id="amount" label={`${t("transactions.amount")}${account ? ` (${account.currency})` : ""}`} error={errors.amount}>
        <Input
          id="amount"
          name="amount"
          inputMode="decimal"
          autoComplete="off"
          placeholder="0.00"
          defaultValue={transaction ? toDecimalString(transaction.amount, transaction.minor_units) : ""}
          aria-invalid={Boolean(errors.amount)}
          autoFocus={!transaction}
          required
        />
      </Field>

      {type === "transfer" ? (
        <>
          <Field id="destination_account_id" label={t("transactions.toAccount")} error={errors.destination_account_id}>
            <NativeSelect
              id="destination_account_id"
              name="destination_account_id"
              value={destinationId}
              onChange={(e) => setDestinationId(e.target.value)}
            >
              {accountOptions
                .filter((a) => a.id !== accountId)
                .map((a) => (
                  <option key={a.id} value={a.id}>
                    {a.name} ({a.currency})
                  </option>
                ))}
            </NativeSelect>
          </Field>
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
        <Field id="category_id" label={t("transactions.category")} error={errors.category_id}>
          <NativeSelect id="category_id" name="category_id" defaultValue={transaction?.category_id ?? ""} key={type}>
            <option value="">{t("common.none")}</option>
            {categoryOptions.map((c) => (
              <option key={c.id} value={c.id}>
                {c.name}
              </option>
            ))}
          </NativeSelect>
        </Field>
      )}

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
