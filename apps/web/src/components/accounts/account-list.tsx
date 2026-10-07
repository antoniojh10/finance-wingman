"use client";

import { ArchiveIcon, ArchiveRestoreIcon, PencilIcon, Trash2Icon } from "lucide-react";
import { useTranslations } from "next-intl";
import { useState, useTransition } from "react";
import { toast } from "sonner";

import { deleteAccount, setAccountArchived } from "@/app/actions/accounts";
import { ColorTile } from "@/components/color-tile";
import { ConfirmAction } from "@/components/confirm-action";
import { Money } from "@/components/money";
import { RowActions } from "@/components/row-actions";
import { Badge } from "@/components/ui/badge";
import { ownerName } from "@/lib/owners";
import { cn } from "@/lib/utils";

import { AccountDialog, type CurrencyOption, type EditableAccount, type Ownership } from "./account-dialog";
import { accountStyles } from "./account-style";

export type AccountListItem = EditableAccount & { balance: number; archived: boolean };

type Group = { currency: string; minorUnits: number; total: number; active: number; accounts: AccountListItem[] };

/** Groups accounts by currency; totals only count active accounts. */
export function groupByCurrency(accounts: AccountListItem[]): Group[] {
  const groups = new Map<string, Group>();
  for (const account of accounts) {
    const group = groups.get(account.currency) ?? {
      currency: account.currency,
      minorUnits: account.minor_units,
      total: 0,
      active: 0,
      accounts: [],
    };
    group.accounts.push(account);
    if (!account.archived) {
      group.total += account.balance;
      group.active += 1;
    }
    groups.set(account.currency, group);
  }
  return [...groups.values()];
}

// Each currency's header card gets its own color, in order.
const headerStyles = ["bg-hero text-hero-foreground", "bg-lime text-lime-foreground", "bg-primary text-primary-foreground"];

export function AccountList({
  accounts,
  currencies,
  defaultCurrency,
  defaultDate,
  ownership,
}: {
  accounts: AccountListItem[];
  currencies: CurrencyOption[];
  defaultCurrency: string;
  defaultDate: string;
  ownership?: Ownership;
}) {
  const t = useTranslations();

  if (accounts.length === 0) {
    return <p className="rounded-3xl bg-card py-10 text-center text-sm text-muted-foreground">{t("accounts.empty")}</p>;
  }

  return (
    <div className="grid grid-cols-1 gap-6 lg:grid-cols-2 lg:items-start">
      {groupByCurrency(accounts).map((group, index) => (
        <section key={group.currency} className="grid min-w-0 grid-cols-1 gap-2.5" aria-label={group.currency}>
          <div className={cn("flex items-center justify-between gap-3 rounded-[22px] px-4.5 py-4", headerStyles[index % headerStyles.length])}>
            <div>
              <h2 className="font-sans text-[13px] font-bold tracking-wider">{group.currency}</h2>
              <p className="text-[13px] opacity-80">{t("accounts.activeCount", { count: group.active })}</p>
            </div>
            <Money
              amount={group.total}
              currency={group.currency}
              minorUnits={group.minorUnits}
              className="font-heading text-[26px] font-bold tracking-tight"
            />
          </div>
          <ul className="divide-y divide-border/70 rounded-3xl bg-card px-4 ring-1 ring-foreground/5">
            {group.accounts.map((account) => (
              <AccountRow
                key={account.id}
                account={account}
                currencies={currencies}
                defaultCurrency={defaultCurrency}
                defaultDate={defaultDate}
                ownership={ownership}
              />
            ))}
          </ul>
        </section>
      ))}
    </div>
  );
}

function AccountRow({
  account,
  currencies,
  defaultCurrency,
  defaultDate,
  ownership,
}: {
  account: AccountListItem;
  currencies: CurrencyOption[];
  defaultCurrency: string;
  defaultDate: string;
  ownership?: Ownership;
}) {
  const t = useTranslations();
  const [pending, startTransition] = useTransition();
  const [editing, setEditing] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const { color, icon: Icon } = accountStyles[account.type];

  function toggleArchived() {
    startTransition(async () => {
      const result = await setAccountArchived(account.id, !account.archived);
      if (result.ok) {
        toast.success(t(account.archived ? "accounts.restoredToast" : "accounts.archivedToast"));
      } else {
        toast.error(result.message);
      }
    });
  }

  return (
    <li className={cn("flex min-h-17.5 items-center gap-3 py-2.5", account.archived && "opacity-70")} data-testid="account-row">
      <ColorTile color={color} className="size-11 rounded-[15px]">
        <Icon />
      </ColorTile>
      <div className="min-w-0 flex-1">
        <p className="flex items-center gap-2 text-[15.5px] font-semibold">
          <span className="min-w-0 truncate">{account.name}</span>
          {account.archived && <Badge variant="secondary">{t("common.archived")}</Badge>}
        </p>
        <p className="text-[12.5px] text-muted-foreground">
          {t(`accounts.types.${account.type}`)}
          {ownership && ` · ${account.owner ? ownerName(account.owner) : t("owners.shared")}`}
        </p>
      </div>
      <Money
        amount={account.balance}
        currency={account.currency}
        minorUnits={account.minor_units}
        className={cn("text-[15.5px] font-bold whitespace-nowrap", account.balance < 0 && "text-expense")}
      />
      <RowActions
        className="-mr-2 text-muted-foreground"
        actions={[
          { label: t("common.edit"), icon: PencilIcon, onSelect: () => setEditing(true) },
          {
            label: t(account.archived ? "common.restore" : "common.archive"),
            icon: account.archived ? ArchiveRestoreIcon : ArchiveIcon,
            onSelect: toggleArchived,
            disabled: pending,
          },
          { label: t("common.delete"), icon: Trash2Icon, onSelect: () => setDeleting(true), destructive: true },
        ]}
      />
      <AccountDialog
        account={account}
        currencies={currencies}
        defaultCurrency={defaultCurrency}
        defaultDate={defaultDate}
        ownership={ownership}
        open={editing}
        onOpenChange={setEditing}
      />
      <ConfirmAction
        open={deleting}
        onOpenChange={setDeleting}
        title={t("accounts.deleteTitle")}
        description={t("accounts.deleteDescription")}
        confirmLabel={t("common.delete")}
        successMessage={t("accounts.deleted")}
        action={() => deleteAccount(account.id)}
      />
    </li>
  );
}
