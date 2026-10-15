import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";

import { ReceiptsList } from "@/components/groceries/receipts-list";
import { monthOf, today } from "@/lib/dates";
import { ALL_MONTHS, parseReceiptsQuery } from "@/lib/groceries";
import { authedApi, expectData } from "@/lib/session";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("groceries.receipts");
  return { title: t("title") };
}

export default async function ReceiptsPage({ searchParams }: { searchParams: Promise<Record<string, string | string[] | undefined>> }) {
  const query = parseReceiptsQuery(await searchParams);
  const currentMonth = monthOf(today(process.env.APP_TIMEZONE));
  const month = query.month ?? currentMonth;

  const api = await authedApi();
  const [receipts, stores] = await Promise.all([
    api.GET("/api/v1/groceries/receipts", {
      params: { query: { month: month === ALL_MONTHS ? undefined : month, store_id: query.storeId } },
    }),
    api.GET("/api/v1/groceries/stores"),
  ]);

  return (
    <ReceiptsList
      receipts={expectData(receipts).items}
      stores={expectData(stores).items.map((store) => ({ id: store.id, name: store.name }))}
      query={query}
      month={month}
      currentMonth={currentMonth}
    />
  );
}
