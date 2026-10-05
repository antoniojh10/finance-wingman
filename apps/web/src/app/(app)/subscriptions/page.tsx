import { PlusIcon } from "lucide-react";
import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";

import { PageHeader } from "@/components/page-header";
import { RecurringDialog } from "@/components/recurring/recurring-dialog";
import { RecurringList } from "@/components/recurring/recurring-list";
import { RecurringSummary } from "@/components/recurring/recurring-summary";
import { Button } from "@/components/ui/button";
import { today } from "@/lib/dates";
import { authedApi, expectData } from "@/lib/session";
import { toAccountOption, toCategoryOption } from "@/lib/view-models";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("subscriptions");
  return { title: t("title") };
}

export default async function SubscriptionsPage() {
  const t = await getTranslations("subscriptions");
  const api = await authedApi();
  const [itemsRes, summaryRes, accountsRes, categoriesRes] = await Promise.all([
    api.GET("/api/v1/recurring"),
    api.GET("/api/v1/recurring/summary"),
    api.GET("/api/v1/accounts", { params: { query: { include_archived: true } } }),
    api.GET("/api/v1/categories", { params: { query: { include_archived: true } } }),
  ]);
  const accounts = expectData(accountsRes).items.map(toAccountOption);
  const categories = expectData(categoriesRes).items.map(toCategoryOption);
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
        <RecurringSummary currencies={expectData(summaryRes).currencies} />
        <RecurringList
          items={expectData(itemsRes).items}
          accounts={accounts}
          categories={categories}
          defaultDate={defaultDate}
        />
      </div>
    </>
  );
}
