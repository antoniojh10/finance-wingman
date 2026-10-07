import type { Account, Category, RecurringItem, Transaction } from "@/lib/api/client";
import type { AccountOption, CategoryOption, RecurringOption, TransactionRow } from "@/components/transactions/types";
import { accountLabel } from "@/lib/owners";

/** showOwner labels members' accounts with their owner, e.g. "BNP (Ana)". */
export function toAccountOption(a: Account, showOwner = false): AccountOption {
  return { id: a.id, name: accountLabel(a.name, a.owner, showOwner), currency: a.currency, minor_units: a.minor_units, archived: a.archived };
}

export function toCategoryOption(c: Category): CategoryOption {
  return { id: c.id, name: c.name, kind: c.kind, color: c.color, archived: c.archived };
}

/** showOwner labels members' accounts with their owner, e.g. "BNP (Ana)". */
export function toTransactionRow(tx: Transaction, showOwner = false): TransactionRow {
  return {
    id: tx.id,
    type: tx.type,
    account_id: tx.account_id,
    account_name: accountLabel(tx.account_name, tx.account_owner, showOwner),
    currency: tx.currency,
    minor_units: tx.minor_units,
    amount: tx.amount,
    destination_account_id: tx.destination_account_id,
    destination_account_name: tx.destination_account_name
      ? accountLabel(tx.destination_account_name, tx.destination_account_owner, showOwner)
      : null,
    destination_currency: tx.destination_currency ?? null,
    destination_minor_units: tx.destination_minor_units ?? null,
    destination_amount: tx.destination_amount,
    category_id: tx.category_id,
    category_name: tx.category_name ?? null,
    description: tx.description,
    occurred_on: tx.occurred_on,
    recurring_id: tx.recurring_id,
    created_by: tx.created_by ? { name: tx.created_by.name, email: tx.created_by.email } : undefined,
  };
}

/** Recurring item names by id, to label the transactions that pay them. */
export function toRecurringNames(items: { id: string; name: string }[]): Record<string, string> {
  return Object.fromEntries(items.map((item) => [item.id, item.name]));
}

export function toRecurringOption(item: RecurringItem): RecurringOption {
  return {
    id: item.id,
    name: item.name,
    type: item.type,
    account_id: item.account_id,
    status: item.status,
    current_due_on: item.current_period?.due_on ?? null,
    next_due_on: item.next_due_on,
  };
}
