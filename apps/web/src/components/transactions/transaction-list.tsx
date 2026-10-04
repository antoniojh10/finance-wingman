"use client";

import { ArrowRightIcon, PencilIcon, Trash2Icon } from "lucide-react";
import { useLocale, useTranslations } from "next-intl";

import { deleteTransaction } from "@/app/actions/transactions";
import { ConfirmAction } from "@/components/confirm-action";
import { Money } from "@/components/money";
import { Button } from "@/components/ui/button";
import { formatDate } from "@/lib/dates";
import { intlLocale, isLocale } from "@/i18n/locales";

import { TransactionDialog } from "./transaction-dialog";
import type { AccountOption, CategoryOption, TransactionRow } from "./types";

export function TransactionList({
  transactions,
  accounts,
  categories,
  defaultDate,
  editable = true,
}: {
  transactions: TransactionRow[];
  accounts: AccountOption[];
  categories: CategoryOption[];
  defaultDate: string;
  editable?: boolean;
}) {
  const t = useTranslations();
  const locale = useLocale();
  const fmtLocale = intlLocale(isLocale(locale) ? locale : "en");

  if (transactions.length === 0) {
    return <p className="py-6 text-center text-sm text-muted-foreground">{t("transactions.empty")}</p>;
  }

  return (
    <ul className="divide-y">
      {transactions.map((tx) => {
        const tone = tx.type === "income" ? "income" : tx.type === "expense" ? "expense" : "neutral";
        const title = tx.description || (tx.type === "transfer" ? t("transactions.types.transfer") : (tx.category_name ?? t("common.uncategorized")));
        return (
          <li key={tx.id} className="flex items-center gap-3 py-3" data-testid="transaction-row">
            <div className="min-w-0 flex-1">
              <p className="truncate text-sm font-medium">{title}</p>
              <p className="flex flex-wrap items-center gap-x-1.5 text-xs text-muted-foreground">
                <span>{formatDate(tx.occurred_on, fmtLocale)}</span>
                <span aria-hidden>·</span>
                <span className="inline-flex items-center gap-1">
                  {tx.account_name}
                  {tx.destination_account_name && (
                    <>
                      <ArrowRightIcon className="size-3" aria-label="→" />
                      {tx.destination_account_name}
                    </>
                  )}
                </span>
                {tx.type !== "transfer" && tx.description && (
                  <>
                    <span aria-hidden>·</span>
                    <span>{tx.category_name ?? t("common.uncategorized")}</span>
                  </>
                )}
                {tx.created_by && (
                  <span className="hidden sm:inline">
                    <span aria-hidden>· </span>
                    {t("transactions.recordedBy", { name: tx.created_by.name || tx.created_by.email })}
                  </span>
                )}
              </p>
            </div>
            <div className="text-right">
              <Money
                amount={tx.amount}
                currency={tx.currency}
                minorUnits={tx.minor_units}
                tone={tone}
                signed
                className="text-sm font-medium"
              />
              {tx.type === "transfer" &&
                tx.destination_currency &&
                tx.destination_currency !== tx.currency &&
                tx.destination_amount != null &&
                tx.destination_minor_units != null && (
                  <p className="text-xs text-muted-foreground">
                    <Money amount={tx.destination_amount} currency={tx.destination_currency} minorUnits={tx.destination_minor_units} />
                  </p>
                )}
            </div>
            {editable && (
              <div className="flex shrink-0 gap-0.5">
                <TransactionDialog
                  accounts={accounts}
                  categories={categories}
                  transaction={tx}
                  defaultDate={defaultDate}
                  trigger={
                    <Button variant="ghost" size="icon-sm" aria-label={t("common.edit")}>
                      <PencilIcon />
                    </Button>
                  }
                />
                <ConfirmAction
                  trigger={
                    <Button variant="ghost" size="icon-sm" aria-label={t("common.delete")}>
                      <Trash2Icon />
                    </Button>
                  }
                  title={t("transactions.deleteTitle")}
                  description={t("transactions.deleteDescription")}
                  confirmLabel={t("common.delete")}
                  successMessage={t("transactions.deleted")}
                  action={() => deleteTransaction(tx.id)}
                />
              </div>
            )}
          </li>
        );
      })}
    </ul>
  );
}
