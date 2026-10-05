import { parseAmount } from "@/lib/money";

/** Result returned by form Server Actions and consumed by useActionState. */
export type FormState = {
  ok: boolean;
  /** Changes on every successful submission so clients can react once. */
  nonce?: number;
  message?: string;
  fieldErrors?: Record<string, string>;
  /** Id of the transaction a save action created. */
  transactionId?: string;
};

export const initialFormState: FormState = { ok: false };

export const emailPattern = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

export function text(formData: FormData, key: string): string {
  const value = formData.get(key);
  return typeof value === "string" ? value.trim() : "";
}

/** Returns the trimmed value, or undefined when empty. */
export function optionalText(formData: FormData, key: string): string | undefined {
  return text(formData, key) || undefined;
}

/**
 * Parses an amount that may be zero or negative (e.g. an opening balance).
 * Returns null when invalid.
 */
export function parseSignedAmount(input: string, minorUnits: number): number | null {
  const value = input.trim();
  if (value === "" || /^[-+]?0*([.,]0*)?$/.test(value)) {
    return 0;
  }
  const negative = value.startsWith("-");
  const parsed = parseAmount(value.replace(/^[-+]/, ""), minorUnits);
  if (parsed === null) {
    return null;
  }
  return negative ? -parsed : parsed;
}

type AccountRef = { id: string; currency: string; minor_units: number };

export type TransactionBody = {
  type: "expense" | "income" | "transfer";
  account_id: string;
  amount: number;
  destination_account_id?: string;
  destination_amount?: number;
  category_id?: string;
  description?: string;
  occurred_on?: string;
};

export type TransactionFieldError = { field: string; error: "required" | "invalidAmount"; decimals?: number };

/**
 * Builds the API request body for a transaction form, converting decimal
 * amounts using each account's currency. Returns field errors instead when
 * the input is incomplete or invalid.
 */
export function buildTransactionBody(
  formData: FormData,
  accounts: AccountRef[],
): { body: TransactionBody } | { errors: TransactionFieldError[] } {
  const errors: TransactionFieldError[] = [];
  const typeValue = text(formData, "type");
  const type: TransactionBody["type"] =
    typeValue === "income" || typeValue === "transfer" ? typeValue : "expense";

  const account = accounts.find((a) => a.id === text(formData, "account_id"));
  if (!account) {
    errors.push({ field: "account_id", error: "required" });
  }
  const amount = account ? parseAmount(text(formData, "amount"), account.minor_units) : null;
  if (account && amount === null) {
    errors.push({ field: "amount", error: "invalidAmount", decimals: account.minor_units });
  }

  const body: Partial<TransactionBody> = {
    type,
    account_id: account?.id,
    amount: amount ?? undefined,
    description: optionalText(formData, "description"),
    occurred_on: optionalText(formData, "occurred_on"),
  };

  if (type === "transfer") {
    const destination = accounts.find((a) => a.id === text(formData, "destination_account_id"));
    if (!destination) {
      errors.push({ field: "destination_account_id", error: "required" });
    } else {
      body.destination_account_id = destination.id;
      if (account && destination.currency !== account.currency) {
        const received = parseAmount(text(formData, "destination_amount"), destination.minor_units);
        if (received === null) {
          errors.push({ field: "destination_amount", error: "invalidAmount", decimals: destination.minor_units });
        } else {
          body.destination_amount = received;
        }
      }
    }
  } else {
    body.category_id = optionalText(formData, "category_id");
  }

  if (errors.length > 0) {
    return { errors };
  }
  return { body: body as TransactionBody };
}
