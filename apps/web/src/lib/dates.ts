// Calendar helpers working with ISO dates (YYYY-MM-DD) and months (YYYY-MM).

const monthPattern = /^(\d{4})-(0[1-9]|1[0-2])$/;

function pad(n: number): string {
  return n.toString().padStart(2, "0");
}

/** Returns today's date as YYYY-MM-DD in the given time zone. */
export function today(timeZone?: string, now: Date = new Date()): string {
  const parts = new Intl.DateTimeFormat("en-CA", {
    timeZone,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).formatToParts(now);
  const get = (type: string) => parts.find((p) => p.type === type)?.value ?? "";
  return `${get("year")}-${get("month")}-${get("day")}`;
}

/** Returns the month (YYYY-MM) for a date (YYYY-MM-DD). */
export function monthOf(date: string): string {
  return date.slice(0, 7);
}

/** Validates a YYYY-MM string, falling back to the given default. */
export function parseMonth(value: string | undefined | null, fallback: string): string {
  return value && monthPattern.test(value) ? value : fallback;
}

/** Returns the first and last day of a month. */
export function monthRange(month: string): { from: string; to: string } {
  const match = monthPattern.exec(month);
  if (!match) {
    throw new Error(`invalid month ${month}`);
  }
  const year = Number(match[1]);
  const m = Number(match[2]);
  const lastDay = new Date(Date.UTC(year, m, 0)).getUTCDate();
  return { from: `${year}-${pad(m)}-01`, to: `${year}-${pad(m)}-${pad(lastDay)}` };
}

/** Adds (or subtracts) whole months to a YYYY-MM string. */
export function shiftMonth(month: string, delta: number): string {
  const match = monthPattern.exec(month);
  if (!match) {
    throw new Error(`invalid month ${month}`);
  }
  const date = new Date(Date.UTC(Number(match[1]), Number(match[2]) - 1 + delta, 1));
  return `${date.getUTCFullYear()}-${pad(date.getUTCMonth() + 1)}`;
}

/** Formats a month for display, e.g. "October 2026" / "octubre de 2026". */
export function formatMonth(month: string, locale: string): string {
  const { from } = monthRange(month);
  return new Intl.DateTimeFormat(locale, { month: "long", year: "numeric", timeZone: "UTC" }).format(
    new Date(`${from}T00:00:00Z`),
  );
}

/** Formats an ISO date for display without shifting it across time zones. */
export function formatDate(date: string, locale: string): string {
  return new Intl.DateTimeFormat(locale, { day: "numeric", month: "short", year: "numeric", timeZone: "UTC" }).format(
    new Date(`${date}T00:00:00Z`),
  );
}
