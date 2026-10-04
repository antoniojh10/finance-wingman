export const locales = ["en", "es"] as const;
export type Locale = (typeof locales)[number];
export const defaultLocale: Locale = "en";
export const LOCALE_COOKIE = "NEXT_LOCALE";

export function isLocale(value: unknown): value is Locale {
  return typeof value === "string" && (locales as readonly string[]).includes(value);
}

/** Picks the first supported language from an Accept-Language header. */
export function localeFromAcceptLanguage(header: string | null | undefined): Locale {
  for (const part of (header ?? "").split(",")) {
    const lang = part.split(";")[0].trim().toLowerCase().split("-")[0];
    if (isLocale(lang)) {
      return lang;
    }
  }
  return defaultLocale;
}

/** Regional tag used for number and date formatting. */
export function intlLocale(locale: Locale): string {
  return locale === "es" ? "es-MX" : "en-US";
}
