import type { Metadata } from "next";
import Link from "next/link";
import { getTranslations } from "next-intl/server";

import { PageHeader } from "@/components/page-header";
import { TransactionFilters } from "@/components/transactions/transaction-filters";
import { TransactionList } from "@/components/transactions/transaction-list";
import { Button } from "@/components/ui/button";
import { today } from "@/lib/dates";
import { pageHref, parseTransactionQuery } from "@/lib/query";
import { authedApi, expectData } from "@/lib/session";
import { toAccountOption, toCategoryOption, toRecurringNames, toTransactionRow } from "@/lib/view-models";

const PAGE_SIZE = 25;

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("transactions");
  return { title: t("title") };
}

export default async function TransactionsPage({
  searchParams,
}: {
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const t = await getTranslations();
  const query = parseTransactionQuery(await searchParams);
  const todayDate = today(process.env.APP_TIMEZONE);

  const api = await authedApi();
  const [accountsRes, categoriesRes, recurringRes, transactionsRes] = await Promise.all([
    api.GET("/api/v1/accounts", { params: { query: { include_archived: true } } }),
    api.GET("/api/v1/categories", { params: { query: { include_archived: true } } }),
    api.GET("/api/v1/recurring"),
    api.GET("/api/v1/transactions", {
      params: {
        query: {
          account_id: query.account_id,
          category_id: query.category_id,
          type: query.type,
          from: query.from,
          to: query.to,
          q: query.q,
          limit: PAGE_SIZE,
          offset: (query.page - 1) * PAGE_SIZE,
        },
      },
    }),
  ]);
  const accounts = expectData(accountsRes).items.map(toAccountOption);
  const categories = expectData(categoriesRes).items.map(toCategoryOption);
  const page = expectData(transactionsRes);
  const hasActiveAccounts = accounts.some((a) => !a.archived);

  const first = page.items.length > 0 ? page.offset + 1 : 0;
  const last = page.offset + page.items.length;

  return (
    <>
      <PageHeader title={t("transactions.title")} />
      <div className="grid grid-cols-1 gap-5">
        {!hasActiveAccounts && (
          <p className="rounded-2xl bg-card px-5 py-4 text-sm ring-1 ring-foreground/5">
            {t("transactions.needAccount")}{" "}
            <Link href="/accounts" className="font-semibold text-primary underline underline-offset-4">
              {t("accounts.add")}
            </Link>
          </p>
        )}
        <TransactionFilters values={query} accounts={accounts} categories={categories} />
        <TransactionList
          transactions={page.items.map(toTransactionRow)}
          accounts={accounts}
          categories={categories}
          defaultDate={todayDate}
          recurringNames={toRecurringNames(expectData(recurringRes).items)}
          groupByDate
        />
        {page.total > 0 && (
          <div className="flex items-center justify-between gap-3 text-sm text-muted-foreground">
            <span>{t("transactions.showing", { from: first, to: last, total: page.total })}</span>
            <div className="flex gap-2">
              {query.page > 1 && (
                <Button nativeButton={false} render={<Link href={pageHref(query, query.page - 1)} />} variant="outline" size="sm">
                  {t("common.previous")}
                </Button>
              )}
              {last < page.total && (
                <Button nativeButton={false} render={<Link href={pageHref(query, query.page + 1)} />} variant="outline" size="sm">
                  {t("common.next")}
                </Button>
              )}
            </div>
          </div>
        )}
      </div>
    </>
  );
}
