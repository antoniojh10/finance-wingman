"use client";

import { useTranslations } from "next-intl";
import { useState } from "react";
import { toast } from "sonner";

import { saveAccount } from "@/app/actions/accounts";
import { Field } from "@/components/field";
import { NativeSelect } from "@/components/native-select";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { useFormAction } from "@/hooks/use-form-action";
import { toDecimalString } from "@/lib/money";
import { SHARED, type OwnerOption } from "@/lib/owners";

export const accountTypes = ["checking", "savings", "credit_card", "cash", "investment", "other"] as const;

export type EditableAccount = {
  id: string;
  name: string;
  type: (typeof accountTypes)[number];
  currency: string;
  minor_units: number;
  initial_balance: number;
  /** Date (YYYY-MM-DD) the initial balance refers to. */
  balance_as_of: string;
  /** Member the account belongs to; absent when it is shared. */
  owner?: { id: string; name: string; email: string };
};

/** Workspace members who can own accounts; only set with several members. */
export type Ownership = { owners: OwnerOption[]; userId: string };

export type CurrencyOption = { code: string; name: string };

export function AccountDialog({
  account,
  currencies,
  defaultCurrency,
  defaultDate,
  ownership,
  trigger,
  open: controlledOpen,
  onOpenChange,
}: {
  account?: EditableAccount;
  currencies: CurrencyOption[];
  defaultCurrency: string;
  ownership?: Ownership;
  /** Today, used as the default balance date of new accounts. */
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
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{t(account ? "accounts.editTitle" : "accounts.addTitle")}</DialogTitle>
        </DialogHeader>
        {open && (
          <AccountForm
            account={account}
            currencies={currencies}
            defaultCurrency={defaultCurrency}
            defaultDate={defaultDate}
            ownership={ownership}
            onSaved={() => {
              toast.success(t("accounts.saved"));
              setOpen(false);
            }}
            onCancel={() => setOpen(false)}
          />
        )}
      </DialogContent>
    </Dialog>
  );
}

export function AccountForm({
  account: accountProp,
  currencies,
  defaultCurrency,
  defaultDate,
  ownership,
  onSaved,
  onCancel,
}: {
  account?: EditableAccount;
  currencies: CurrencyOption[];
  defaultCurrency: string;
  defaultDate: string;
  ownership?: Ownership;
  onSaved: () => void;
  onCancel: () => void;
}) {
  const t = useTranslations();
  // Keep the values the form opened with: after saving, the revalidated account
  // arrives before the dialog closes, and changing defaultValue on mounted
  // inputs makes Base UI warn.
  const [account] = useState(accountProp);
  const { state, onSubmit, pending } = useFormAction(saveAccount, onSaved);
  const errors = state.fieldErrors ?? {};

  return (
    <form onSubmit={onSubmit} className="grid gap-4" noValidate>
      {account && <input type="hidden" name="id" value={account.id} />}
      <Field id="name" label={t("accounts.name")} error={errors.name}>
        <Input
          id="name"
          name="name"
          maxLength={100}
          required
          autoFocus
          placeholder={t("accounts.namePlaceholder")}
          defaultValue={account?.name}
          aria-invalid={Boolean(errors.name)}
        />
      </Field>
      <Field id="type" label={t("accounts.type")} error={errors.type}>
        <NativeSelect id="type" name="type" defaultValue={account?.type ?? "checking"}>
          {accountTypes.map((type) => (
            <option key={type} value={type}>
              {t(`accounts.types.${type}`)}
            </option>
          ))}
        </NativeSelect>
      </Field>
      {ownership && (
        <Field id="owner" label={t("owners.owner")} hint={t("owners.ownerHint")} error={errors.owner}>
          <NativeSelect id="owner" name="owner" defaultValue={account ? (account.owner?.id ?? SHARED) : ownership.userId}>
            {ownership.owners.map((o) => (
              <option key={o.id} value={o.id}>
                {o.id === ownership.userId ? t("owners.you", { name: o.name }) : o.name}
              </option>
            ))}
            <option value={SHARED}>{t("owners.sharedOption")}</option>
          </NativeSelect>
        </Field>
      )}
      <Field
        id="currency"
        label={t("accounts.currency")}
        hint={account ? t("accounts.currencyLocked") : undefined}
        error={errors.currency}
      >
        <NativeSelect id="currency" name="currency" defaultValue={account?.currency ?? defaultCurrency} disabled={Boolean(account)}>
          {currencies.map((c) => (
            <option key={c.code} value={c.code}>
              {c.code} — {c.name}
            </option>
          ))}
        </NativeSelect>
      </Field>
      <Field
        id="initial_balance"
        label={t("accounts.initialBalance")}
        hint={t("accounts.initialBalanceHint")}
        error={errors.initial_balance}
      >
        <Input
          id="initial_balance"
          name="initial_balance"
          inputMode="decimal"
          autoComplete="off"
          placeholder="0.00"
          defaultValue={account ? toDecimalString(account.initial_balance, account.minor_units) : ""}
          aria-invalid={Boolean(errors.initial_balance)}
        />
      </Field>
      <Field
        id="balance_as_of"
        label={t("accounts.balanceAsOf")}
        hint={t("accounts.balanceAsOfHint")}
        error={errors.balance_as_of}
      >
        <Input
          id="balance_as_of"
          name="balance_as_of"
          type="date"
          required
          defaultValue={account?.balance_as_of ?? defaultDate}
          aria-invalid={Boolean(errors.balance_as_of)}
        />
      </Field>
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
