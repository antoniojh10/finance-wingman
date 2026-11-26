import { PlusIcon } from "lucide-react";
import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";

import { PageHeader } from "@/components/page-header";
import { RecurringDialog } from "@/components/recurring/recurring-dialog";
import { CategoryBreakdown } from "@/components/recurring/category-breakdown";
import { RecurringList } from "@/components/recurring/recurring-list";
import { RecurringSummary } from "@/components/recurring/recurring-summary";
import { SuggestionsList } from "@/components/recurring/suggestions-list";
import { Button } from "@/components/ui/button";
import { today } from "@/lib/dates";
import { parseRecurringFilters } from "@/lib/recurring-filters";
import { authedApi, expectData, showAccountOwners } from "@/lib/session";
import { toAccountOption, toCategoryOption } from "@/lib/view-models";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("subscriptions");
  return { title: t("title") };
}

export default async function SubscriptionsPage({
  searchParams,
}: {
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const filters = parseRecurringFilters(await searchParams);
  const t = await getTranslations("subscriptions");
  const api = await authedApi();
  const [itemsRes, summaryRes, suggestionsRes, accountsRes, categoriesRes, showOwners] = await Promise.all([
    api.GET("/api/v1/recurring"),
    api.GET("/api/v1/recurring/summary"),
    api.GET("/api/v1/recurring/suggestions"),
    api.GET("/api/v1/accounts", { params: { query: { include_archived: true } } }),
    api.GET("/api/v1/categories", { params: { query: { include_archived: true } } }),
    showAccountOwners(),
  ]);
  const accounts = expectData(accountsRes).items.map((a) => toAccountOption(a, showOwners));
  const categories = expectData(categoriesRes).items.map(toCategoryOption);
  const summary = expectData(summaryRes);
  const defaultDate = today(process.env.APP_TIMEZONE);

  return (
    <>
      <PageHeader
        title={t("title")}
        actions={
          <RecurringDialog
            accounts={accounts}
            categories={categories}
            defaultDate={defaultDate}
            trigger={
              <Button>
                <PlusIcon />
                {t("add")}
              </Button>
            }
          />
        }
      />
      <div className="grid grid-cols-1 gap-6">
        <div className="grid grid-cols-1 items-start gap-3 lg:grid-cols-[minmax(0,0.85fr)_minmax(0,1.4fr)]">
          <RecurringSummary currencies={summary.currencies} />
          <CategoryBreakdown currencies={summary.currencies} filters={filters} className="lg:mt-8" />
        </div>
        <SuggestionsList suggestions={expectData(suggestionsRes).items} />
        <RecurringList
          items={expectData(itemsRes).items}
          accounts={accounts}
          categories={categories}
          defaultDate={defaultDate}
          status={filters.status}
          category={filters.category}
          group={filters.group}
        />
      </div>
    </>
  );
}
