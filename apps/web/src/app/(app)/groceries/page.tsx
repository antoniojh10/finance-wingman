import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";

import { GroceriesOverview } from "@/components/groceries/overview/overview";
import { monthOf, today } from "@/lib/dates";
import { availableCurrencies, parseOverviewQuery, pickCurrency } from "@/lib/groceries-insights";
import { authedApi, expectData } from "@/lib/session";

/** Full months averaged into the basket. */
const BASKET_MONTHS = 3;

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("groceries.tabs");
  return { title: t("summary") };
}

export default async function GroceriesSummaryPage({ searchParams }: { searchParams: Promise<Record<string, string | string[] | undefined>> }) {
  const query = parseOverviewQuery(await searchParams);
  const month = query.month ?? monthOf(today(process.env.APP_TIMEZONE));

  const api = await authedApi();
  const [breakdown, comparison, basket] = await Promise.all([
    api.GET("/api/v1/groceries/breakdown", { params: { query: { month } } }),
    api.GET("/api/v1/groceries/compare", { params: { query: { level: query.level } } }),
    api.GET("/api/v1/groceries/basket", { params: { query: { months: BASKET_MONTHS } } }),
  ]);
  const breakdowns = expectData(breakdown).currencies;
  const comparisons = expectData(comparison).currencies;
  const baskets = expectData(basket).currencies;

  const currencies = availableCurrencies(breakdowns, comparisons, baskets);
  const currency = pickCurrency(currencies, query.currency);

  return (
    <GroceriesOverview
      query={query}
      month={month}
      currencies={currencies}
      currency={currency}
      breakdown={breakdowns.find((entry) => entry.currency === currency)}
      comparison={comparisons.find((entry) => entry.currency === currency)}
      basket={baskets.find((entry) => entry.currency === currency)}
      basketMonths={BASKET_MONTHS}
    />
  );
}
