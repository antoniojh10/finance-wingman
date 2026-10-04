"use client";

import { ArchiveIcon, ArchiveRestoreIcon, PencilIcon, Trash2Icon } from "lucide-react";
import { useTranslations } from "next-intl";
import { useTransition } from "react";
import { toast } from "sonner";

import { deleteAccount, setAccountArchived } from "@/app/actions/accounts";
import { ConfirmAction } from "@/components/confirm-action";
import { Money } from "@/components/money";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";

import { AccountDialog, type CurrencyOption, type EditableAccount } from "./account-dialog";

export type AccountListItem = EditableAccount & { balance: number; archived: boolean };

export function AccountList({
  accounts,
  currencies,
  defaultCurrency,
}: {
  accounts: AccountListItem[];
  currencies: CurrencyOption[];
  defaultCurrency: string;
}) {
  const t = useTranslations();
  const [pending, startTransition] = useTransition();

  function toggleArchived(account: AccountListItem) {
    startTransition(async () => {
      const result = await setAccountArchived(account.id, !account.archived);
      if (result.ok) {
        toast.success(t(account.archived ? "accounts.restoredToast" : "accounts.archivedToast"));
      } else {
        toast.error(result.message);
      }
    });
  }

  if (accounts.length === 0) {
    return <p className="py-6 text-center text-sm text-muted-foreground">{t("accounts.empty")}</p>;
  }

  return (
    <ul className="divide-y">
      {accounts.map((account) => (
        <li key={account.id} className="flex items-center gap-3 py-3" data-testid="account-row">
          <div className="min-w-0 flex-1">
            <p className="flex items-center gap-2 truncate text-sm font-medium">
              {account.name}
              {account.archived && <Badge variant="secondary">{t("common.archived")}</Badge>}
            </p>
            <p className="text-xs text-muted-foreground">
              {t(`accounts.types.${account.type}`)} · {account.currency}
            </p>
          </div>
          <Money
            amount={account.balance}
            currency={account.currency}
            minorUnits={account.minor_units}
            className="text-sm font-medium"
          />
          <div className="flex shrink-0 gap-0.5">
            <AccountDialog
              account={account}
              currencies={currencies}
              defaultCurrency={defaultCurrency}
              trigger={
                <Button variant="ghost" size="icon-sm" aria-label={t("common.edit")}>
                  <PencilIcon />
                </Button>
              }
            />
            <Button
              variant="ghost"
              size="icon-sm"
              aria-label={t(account.archived ? "common.restore" : "common.archive")}
              title={t(account.archived ? "common.restore" : "common.archive")}
              onClick={() => toggleArchived(account)}
              disabled={pending}
            >
              {account.archived ? <ArchiveRestoreIcon /> : <ArchiveIcon />}
            </Button>
            <ConfirmAction
              trigger={
                <Button variant="ghost" size="icon-sm" aria-label={t("common.delete")}>
                  <Trash2Icon />
                </Button>
              }
              title={t("accounts.deleteTitle")}
              description={t("accounts.deleteDescription")}
              confirmLabel={t("common.delete")}
              successMessage={t("accounts.deleted")}
              action={() => deleteAccount(account.id)}
            />
          </div>
        </li>
      ))}
    </ul>
  );
}
