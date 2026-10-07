// Amounts are stored as integer minor units (e.g. cents). These helpers
// convert between user input, minor units, and localized display strings.

/**
 * Parses a user-entered decimal amount into minor units. Accepts "." or ","
 * as decimal separator (but not thousands separators). Returns null when the
 * input is not a valid, positive amount for the currency.
 */
export function parseAmount(input: string, minorUnits: number): number | null {
  let value = input.trim().replace(/\s/g, "");
  if (!value.includes(".") && (value.match(/,/g) ?? []).length === 1) {
    value = value.replace(",", ".");
  }
  if (!/^\d*(\.\d*)?$/.test(value) || value === "" || value === ".") {
    return null;
  }
  const [whole, fraction = ""] = value.split(".");
  const trimmedFraction = fraction.replace(/0+$/, "");
  if (trimmedFraction.length > minorUnits) {
    return null;
  }
  const digits = (whole || "0") + trimmedFraction.padEnd(minorUnits, "0");
  const result = Number(digits);
  if (!Number.isSafeInteger(result) || result <= 0) {
    return null;
  }
  return result;
}

/** Renders minor units as a plain decimal string, e.g. 1250 -> "12.50". */
export function toDecimalString(amount: number, minorUnits: number): string {
  const negative = amount < 0;
  const digits = Math.abs(amount).toString().padStart(minorUnits + 1, "0");
  const whole = minorUnits === 0 ? digits : digits.slice(0, -minorUnits);
  const fraction = minorUnits === 0 ? "" : "." + digits.slice(-minorUnits);
  return (negative ? "-" : "") + whole + fraction;
}

/** Formats minor units as a localized currency string, e.g. "$1,250.00". */
export function formatMoney(amount: number, currency: string, minorUnits: number, locale: string): string {
  return new Intl.NumberFormat(locale, {
    style: "currency",
    currency,
    minimumFractionDigits: minorUnits,
    maximumFractionDigits: minorUnits,
  }).format(amount / 10 ** minorUnits);
}

/**
 * Formats a decimal amount as the user types it, grouping the whole part in
 * thousands with spaces, e.g. "1000.5" -> "1 000.5". Drops characters that
 * cannot be part of an amount and keeps the decimal separator as typed. When
 * both "." and "," appear (e.g. a pasted "1,000.50"), the last one is the
 * decimal separator; a separator repeated on its own is a thousands one.
 */
export function formatAmountInput(input: string, signed = false): string {
  const negative = signed && input.trimStart().startsWith("-");
  const value = input.replace(/[^\d.,]/g, "");
  const separators = value.match(/[.,]/g) ?? [];
  const lastDot = value.lastIndexOf(".");
  const lastComma = value.lastIndexOf(",");
  let decimalIndex = -1;
  if (lastDot >= 0 && lastComma >= 0) {
    decimalIndex = Math.max(lastDot, lastComma);
  } else if (separators.length === 1) {
    decimalIndex = Math.max(lastDot, lastComma);
  }
  const whole = (decimalIndex >= 0 ? value.slice(0, decimalIndex) : value).replace(/\D/g, "");
  const grouped = whole.replace(/\B(?=(\d{3})+(?!\d))/g, " ");
  const fraction = decimalIndex >= 0 ? value[decimalIndex] + value.slice(decimalIndex + 1).replace(/\D/g, "") : "";
  return (negative ? "-" : "") + grouped + fraction;
}
