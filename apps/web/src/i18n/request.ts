import { cookies, headers } from "next/headers";
import { getRequestConfig } from "next-intl/server";

import { LOCALE_COOKIE, isLocale, localeFromAcceptLanguage } from "./locales";

// The locale comes from the user's saved preference (cookie) or, before
// signing in, from the browser. URLs are not localized.
export default getRequestConfig(async () => {
  const saved = (await cookies()).get(LOCALE_COOKIE)?.value;
  const locale = isLocale(saved) ? saved : localeFromAcceptLanguage((await headers()).get("accept-language"));
  return {
    locale,
    messages: (await import(`../../messages/${locale}.json`)).default,
    timeZone: process.env.APP_TIMEZONE ?? "UTC",
  };
});
