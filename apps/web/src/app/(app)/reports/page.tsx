import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";

import { PageHeader } from "@/components/page-header";
import { ReportsView } from "@/components/reports/reports-view";
import { monthlyRequest, parseReportsQuery } from "@/lib/reports";
import { authedApi, expectData, getAccountOwners, getCurrentUser } from "@/lib/session";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("reports");
  return { title: t("title") };
}

export default async function ReportsPage({ searchParams }: { searchParams: Promise<Record<string, string | string[] | undefined>> }) {
  const t = await getTranslations("reports");
  const query = parseReportsQuery(await searchParams);

  const api = await authedApi();
  const [summary, owners, user] = await Promise.all([
    api.GET("/api/v1/summary/monthly", { params: { query: monthlyRequest(query) } }),
    getAccountOwners(),
    getCurrentUser(),
  ]);

  return (
    <>
      <PageHeader title={t("title")} />
      <ReportsView currencies={expectData(summary).currencies} query={query} owners={owners} userId={user.id} />
    </>
  );
}
