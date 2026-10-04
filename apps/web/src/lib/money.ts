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
