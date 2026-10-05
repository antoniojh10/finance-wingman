import type { Metadata } from "next";
import Link from "next/link";
import { getTranslations } from "next-intl/server";

import { AccountStrip } from "@/components/dashboard/account-strip";
import { BalanceOverview } from "@/components/dashboard/balance-overview";
import { MonthPicker } from "@/components/dashboard/month-picker";
import { SubscriptionsCard } from "@/components/dashboard/subscriptions-card";
import { PageHeader } from "@/components/page-header";
import { TransactionList } from "@/components/transactions/transaction-list";
import { Button } from "@/components/ui/button";
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { monthOf, monthRange, parseMonth, today } from "@/lib/dates";
import { authedApi, expectData } from "@/lib/session";
import { toAccountOption, toCategoryOption, toRecurringNames, toTransactionRow } from "@/lib/view-models";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("dashboard");
  return { title: t("title") };
}

export default async function DashboardPage({ searchParams }: { searchParams: Promise<{ month?: string }> }) {
  const t = await getTranslations();
  const todayDate = today(process.env.APP_TIMEZONE);
  const month = parseMonth((await searchParams).month, monthOf(todayDate));
  const { from, to } = monthRange(month);

  const api = await authedApi();
  const [summaryRes, accountsRes, categoriesRes, recentRes, upcomingRes, committedRes, recurringRes] = await Promise.all([
    api.GET("/api/v1/summary", { params: { query: { from, to } } }),
    api.GET("/api/v1/accounts"),
    api.GET("/api/v1/categories"),
    api.GET("/api/v1/transactions", { params: { query: { from, to, limit: 6 } } }),
    api.GET("/api/v1/recurring/upcoming", { params: { query: { days: 7 } } }),
    api.GET("/api/v1/recurring/summary"),
    api.GET("/api/v1/recurring"),
  ]);
  const summary = expectData(summaryRes);
  const accounts = expectData(accountsRes).items;
  const categories = expectData(categoriesRes).items.map(toCategoryOption);
  const recent = expectData(recentRes);
  const accountOptions = accounts.map(toAccountOption);

  if (accounts.length === 0) {
    return (
      <>
        <PageHeader title={t("dashboard.title")} />
        <Card className="mx-auto max-w-md text-center">
          <CardHeader>
            <CardTitle>{t("dashboard.emptyTitle")}</CardTitle>
            <CardDescription>{t("dashboard.emptyDescription")}</CardDescription>
          </CardHeader>
          <CardContent>
            <Button nativeButton={false} render={<Link href="/accounts" />}>
              {t("dashboard.createAccount")}
            </Button>
          </CardContent>
        </Card>
      </>
    );
  }

  return (
    <div className="grid grid-cols-1 gap-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="sr-only text-3xl font-bold tracking-tight md:not-sr-only">{t("dashboard.title")}</h1>
        <MonthPicker month={month} caption={t("dashboard.transactionCount", { count: recent.total })} />
      </div>

      <BalanceOverview currencies={summary.currencies} />

      <SubscriptionsCard
        upcoming={expectData(upcomingRes).items}
        committed={expectData(committedRes).currencies}
        defaultDate={todayDate}
      />

      <section className="grid gap-2.5" aria-labelledby="accounts-heading">
        <div className="flex items-center justify-between px-1">
          <h2 id="accounts-heading" className="text-[19px] font-bold tracking-tight">
            {t("dashboard.accounts")}
          </h2>
          <Link href="/accounts" className="flex min-h-11 items-center text-sm font-bold text-primary">
            {t("dashboard.viewAll")}
          </Link>
        </div>
        <AccountStrip accounts={accounts} />
      </section>

      <Card>
        <CardHeader>
          <CardTitle>
            <h2>{t("dashboard.recentTransactions")}</h2>
          </CardTitle>
          <CardAction>
            <Link href={`/transactions?from=${from}&to=${to}`} className="flex min-h-9 items-center text-sm font-bold text-primary">
              {t("dashboard.viewAll")}
            </Link>
          </CardAction>
        </CardHeader>
        <CardContent>
          <TransactionList
            transactions={recent.items.map(toTransactionRow)}
            accounts={accountOptions}
            categories={categories}
            defaultDate={todayDate}
            recurringNames={toRecurringNames(expectData(recurringRes).items)}
          />
        </CardContent>
      </Card>
    </div>
  );
}
