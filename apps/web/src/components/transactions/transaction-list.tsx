"use client";

import { ArrowDownLeftIcon, ArrowLeftRightIcon, ArrowRightIcon, PencilIcon, Trash2Icon } from "lucide-react";
import { useLocale, useTranslations } from "next-intl";
import { useState } from "react";

import { deleteTransaction } from "@/app/actions/transactions";
import { ColorTile } from "@/components/color-tile";
import { ConfirmAction } from "@/components/confirm-action";
import { Money } from "@/components/money";
import { RowActions } from "@/components/row-actions";
import { formatDate } from "@/lib/dates";
import { intlLocale, isLocale } from "@/i18n/locales";
import { cn } from "@/lib/utils";

import { TransactionDialog } from "./transaction-dialog";
import type { AccountOption, CategoryOption, TransactionRow } from "./types";

type ListProps = {
  transactions: TransactionRow[];
  accounts: AccountOption[];
  categories: CategoryOption[];
  /** Today's date; also the default date when editing. */
  defaultDate: string;
  editable?: boolean;
};

/** Splits transactions (already sorted newest first) into runs of the same day. */
export function groupByDay(transactions: TransactionRow[]): { date: string; items: TransactionRow[] }[] {
  const groups: { date: string; items: TransactionRow[] }[] = [];
  for (const tx of transactions) {
    const last = groups.at(-1);
    if (last?.date === tx.occurred_on) {
      last.items.push(tx);
    } else {
      groups.push({ date: tx.occurred_on, items: [tx] });
    }
  }
  return groups;
}

function previousDay(date: string): string {
  const d = new Date(`${date}T00:00:00Z`);
  d.setUTCDate(d.getUTCDate() - 1);
  return d.toISOString().slice(0, 10);
}

/** Transactions as one list, or one card per day when `groupByDate` is set. */
export function TransactionList({ groupByDate = false, ...props }: ListProps & { groupByDate?: boolean }) {
  const t = useTranslations("transactions");
  const locale = useLocale();
  const fmtLocale = intlLocale(isLocale(locale) ? locale : "en");

  if (props.transactions.length === 0) {
    return <p className="py-8 text-center text-sm text-muted-foreground">{t("empty")}</p>;
  }
  if (!groupByDate) {
    return <Rows {...props} />;
  }

  const yesterday = previousDay(props.defaultDate);
  return (
    <div className="grid gap-5">
      {groupByDay(props.transactions).map((group) => {
        const date = formatDate(group.date, fmtLocale);
        const relative = group.date === props.defaultDate ? t("today") : group.date === yesterday ? t("yesterday") : null;
        return (
          <section key={group.date} className="grid gap-2" aria-label={date}>
            <h2 className="px-1 font-sans text-xs font-bold tracking-wider text-muted-foreground uppercase">
              {relative ? `${relative} · ${date}` : date}
            </h2>
            <div className="rounded-3xl bg-card px-4 ring-1 ring-foreground/5">
              <Rows {...props} transactions={group.items} />
            </div>
          </section>
        );
      })}
    </div>
  );
}

function Rows({ transactions, ...props }: ListProps) {
  return (
    <ul className="divide-y divide-border/70">
      {transactions.map((tx) => (
        <TransactionItem key={tx.id} tx={tx} {...props} />
      ))}
    </ul>
  );
}

function TransactionItem({ tx, accounts, categories, defaultDate, editable = true }: Omit<ListProps, "transactions"> & { tx: TransactionRow }) {
  const t = useTranslations();
  const locale = useLocale();
  const fmtLocale = intlLocale(isLocale(locale) ? locale : "en");
  const [editing, setEditing] = useState(false);
  const [deleting, setDeleting] = useState(false);

  const tone = tx.type === "income" ? "income" : tx.type === "expense" ? "expense" : "neutral";
  const categoryName = tx.category_name ?? t("common.uncategorized");
  const title = tx.description || (tx.type === "transfer" ? t("transactions.types.transfer") : categoryName);
  const color = categories.find((c) => c.id === tx.category_id)?.color;
  const recordedBy = tx.created_by ? tx.created_by.name || tx.created_by.email : null;

  return (
    <li className="flex items-center gap-3 py-3" data-testid="transaction-row">
      {tx.type === "income" ? (
        <ColorTile color="var(--income)">
          <ArrowDownLeftIcon />
        </ColorTile>
      ) : tx.type === "transfer" ? (
        <ColorTile>
          <ArrowLeftRightIcon />
        </ColorTile>
      ) : (
        <ColorTile color={color} label={tx.category_name ?? "?"} />
      )}
      <div className="min-w-0 flex-1">
        <p className="truncate text-[15px] font-semibold">{title}</p>
        <p className="flex min-w-0 flex-wrap items-center gap-x-1.5 text-[12.5px] text-muted-foreground">
          {tx.type !== "transfer" && tx.description && (
            <>
              <span className="truncate">{categoryName}</span>
              <span aria-hidden>·</span>
            </>
          )}
          <span className="inline-flex items-center gap-1">
            {tx.account_name}
            {tx.destination_account_name && (
              <>
                <ArrowRightIcon className="size-3" aria-label="→" />
                {tx.destination_account_name}
              </>
            )}
          </span>
          <span aria-hidden>·</span>
          <span>{formatDate(tx.occurred_on, fmtLocale)}</span>
          {recordedBy && (
            <span title={t("transactions.recordedBy", { name: recordedBy })} className="inline-flex">
              <span className="sr-only">{t("transactions.recordedBy", { name: recordedBy })}</span>
              <ColorTile label={recordedBy} color="var(--primary)" className="size-4.5 rounded-full text-[10px]" />
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
          className={cn("text-[15px] font-bold whitespace-nowrap", tone === "neutral" && "text-muted-foreground")}
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
        <>
          <RowActions
            className="-mr-2 text-muted-foreground"
            actions={[
              { label: t("common.edit"), icon: PencilIcon, onSelect: () => setEditing(true) },
              { label: t("common.delete"), icon: Trash2Icon, onSelect: () => setDeleting(true), destructive: true },
            ]}
          />
          <TransactionDialog
            accounts={accounts}
            categories={categories}
            transaction={tx}
            defaultDate={defaultDate}
            open={editing}
            onOpenChange={setEditing}
          />
          <ConfirmAction
            open={deleting}
            onOpenChange={setDeleting}
            title={t("transactions.deleteTitle")}
            description={t("transactions.deleteDescription")}
            confirmLabel={t("common.delete")}
            successMessage={t("transactions.deleted")}
            action={() => deleteTransaction(tx.id)}
          />
        </>
      )}
    </li>
  );
}
