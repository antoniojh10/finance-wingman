import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";

import { CategoryBoard, type CategoryMonthTotal } from "@/components/categories/category-list";
import { monthOf, monthRange, today } from "@/lib/dates";
import { authedApi, expectData } from "@/lib/session";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("categories");
  return { title: t("title") };
}

export default async function CategoriesPage({ searchParams }: { searchParams: Promise<{ archived?: string }> }) {
  const showArchived = (await searchParams).archived === "1";
  const { from, to } = monthRange(monthOf(today(process.env.APP_TIMEZONE)));
  const api = await authedApi();
  const [categoriesRes, summaryRes] = await Promise.all([
    api.GET("/api/v1/categories", { params: { query: { include_archived: showArchived } } }),
    api.GET("/api/v1/summary", { params: { query: { from, to } } }),
  ]);

  const totals: Record<string, CategoryMonthTotal[]> = {};
  for (const summary of expectData(summaryRes).currencies) {
    for (const entry of [...summary.expenses, ...summary.incomes]) {
      if (entry.category_id) {
        (totals[entry.category_id] ??= []).push({ currency: summary.currency, minor_units: summary.minor_units, total: entry.total });
      }
    }
  }

  return <CategoryBoard categories={expectData(categoriesRes).items} totals={totals} showArchived={showArchived} />;
}
