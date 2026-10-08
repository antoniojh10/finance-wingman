import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";

import { fetchActivityPage } from "@/app/actions/activity";
import { ActivityFeed } from "@/components/activity/activity-feed";
import { ActivityFilters } from "@/components/activity/activity-filters";
import { PageHeader } from "@/components/page-header";
import { activityHref, parseActivityQuery } from "@/lib/activity";
import { authedApi, expectData, getAccountOwners } from "@/lib/session";
import { toAccountOption } from "@/lib/view-models";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("activity");
  return { title: t("title") };
}

export default async function ActivityPage({ searchParams }: { searchParams: Promise<Record<string, string | string[] | undefined>> }) {
  const t = await getTranslations("activity");
  const query = parseActivityQuery(await searchParams);
  const api = await authedApi();
  const [page, accountsRes, members] = await Promise.all([
    fetchActivityPage(query),
    // Archived accounts are included: old entries still point at them.
    api.GET("/api/v1/accounts", { params: { query: { include_archived: true } } }),
    getAccountOwners(),
  ]);
  const accountNames = Object.fromEntries(
    expectData(accountsRes).items.map((a) => {
      const option = toAccountOption(a, members.length > 1);
      return [option.id, option.name];
    }),
  );

  return (
    <>
      <PageHeader title={t("title")} />
      <div className="grid gap-5">
        <p className="text-sm text-muted-foreground">{t("description")}</p>
        <ActivityFilters query={query} members={members} />
        <ActivityFeed key={activityHref(query, {})} initialItems={page.items} initialCursor={page.nextCursor} query={query} accountNames={accountNames} />
      </div>
    </>
  );
}
