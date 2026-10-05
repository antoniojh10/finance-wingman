export type AccountOption = {
  id: string;
  name: string;
  currency: string;
  minor_units: number;
  archived: boolean;
};

export type CategoryOption = {
  id: string;
  name: string;
  kind: "expense" | "income";
  color?: string | null;
  archived: boolean;
};

export type TransactionType = "expense" | "income" | "transfer";

export type TransactionRow = {
  id: string;
  type: TransactionType;
  account_id: string;
  account_name: string;
  currency: string;
  minor_units: number;
  amount: number;
  destination_account_id: string | null;
  destination_account_name: string | null;
  destination_currency: string | null;
  destination_minor_units: number | null;
  destination_amount: number | null;
  category_id: string | null;
  category_name: string | null;
  description: string;
  occurred_on: string;
  /** Recurring item (subscription) this transaction pays, if any. */
  recurring_id?: string | null;
  created_by?: { name: string; email: string };
};
