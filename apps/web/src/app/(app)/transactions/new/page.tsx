import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";

import { NewTransactionView } from "@/components/transactions/new-transaction-view";
import { today } from "@/lib/dates";
import { safeReturnPath } from "@/lib/query";
import { authedApi, expectData } from "@/lib/session";
import { toAccountOption, toCategoryOption } from "@/lib/view-models";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("transactions");
  return { title: t("addTitle") };
}

export default async function NewTransactionPage({ searchParams }: { searchParams: Promise<{ return?: string }> }) {
  const returnTo = safeReturnPath((await searchParams).return);
  const api = await authedApi();
  const [accountsRes, categoriesRes] = await Promise.all([api.GET("/api/v1/accounts"), api.GET("/api/v1/categories")]);

  return (
    <NewTransactionView
      accounts={expectData(accountsRes).items.map(toAccountOption)}
      categories={expectData(categoriesRes).items.map(toCategoryOption)}
      defaultDate={today(process.env.APP_TIMEZONE)}
      returnTo={returnTo}
    />
  );
}
