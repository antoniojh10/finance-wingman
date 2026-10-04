import { render, type RenderOptions } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";

import en from "../../messages/en.json";
import es from "../../messages/es.json";

const messages = { en, es };

/** Renders UI inside the i18n provider (English by default). */
export function renderWithIntl(ui: React.ReactElement, { locale = "en", ...options }: RenderOptions & { locale?: "en" | "es" } = {}) {
  return render(
    <NextIntlClientProvider locale={locale} messages={messages[locale]} timeZone="UTC">
      {ui}
    </NextIntlClientProvider>,
    options,
  );
}
