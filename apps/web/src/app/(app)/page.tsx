import type { Metadata } from "next";
import Link from "next/link";
import { getTranslations } from "next-intl/server";

import { AccountStrip } from "@/components/dashboard/account-strip";
import { BalanceOverview } from "@/components/dashboard/balance-overview";
import { BudgetsCard } from "@/components/dashboard/budgets-card";
import { MonthPicker } from "@/components/dashboard/month-picker";
import { OwnerFilter } from "@/components/owner-filter";
import { SubscriptionsCard } from "@/components/dashboard/subscriptions-card";
import { PageHeader } from "@/components/page-header";
import { TransactionList } from "@/components/transactions/transaction-list";
import { Button } from "@/components/ui/button";
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { monthOf, monthRange, parseMonth, today } from "@/lib/dates";
import { accountLabel, parseOwner } from "@/lib/owners";
import { authedApi, expectData, getAccountOwners, getCurrentUser } from "@/lib/session";
import { toAccountOption, toCategoryOption, toRecurringNames, toRecurringOption, toTransactionRow } from "@/lib/view-models";
import { NEW_ACCOUNT_HREF } from "@/components/accounts/new-account-dialog";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("dashboard");
  return { title: t("title") };
}

/** Dashboard URL for a month and owner view. */
function dashboardHref(month: string, owner: string | undefined): string {
  const params = new URLSearchParams({ month });
  if (owner) {
    params.set("owner", owner);
  }
  return `/?${params}`;
}

export default async function DashboardPage({ searchParams }: { searchParams: Promise<{ month?: string; owner?: string }> }) {
  const t = await getTranslations();
  const todayDate = today(process.env.APP_TIMEZONE);
  const params = await searchParams;
  const month = parseMonth(params.month, monthOf(todayDate));
  const { from, to } = monthRange(month);
  // An optional view of one member's (or the shared) accounts; the
  // default is the whole workspace.
  const owner = parseOwner(params.owner);

  const api = await authedApi();
  const [summaryRes, accountsRes, categoriesRes, recentRes, upcomingRes, committedRes, recurringRes, suggestionsRes, budgetsRes, owners, user] = await Promise.all([
    api.GET("/api/v1/summary", { params: { query: { from, to, owner } } }),
    api.GET("/api/v1/accounts", { params: { query: { owner } } }),
    api.GET("/api/v1/categories"),
    api.GET("/api/v1/transactions", { params: { query: { from, to, owner, limit: 6 } } }),
    api.GET("/api/v1/recurring/upcoming", { params: { query: { days: 7 } } }),
    api.GET("/api/v1/recurring/summary"),
    api.GET("/api/v1/recurring"),
    api.GET("/api/v1/recurring/suggestions"),
    api.GET("/api/v1/budgets", { params: { query: { month, owner } } }),
    getAccountOwners(),
    getCurrentUser(),
  ]);
  const showOwners = owners.length > 1;
  const summary = expectData(summaryRes);
  const accounts = expectData(accountsRes).items;
  const categories = expectData(categoriesRes).items.map(toCategoryOption);
  const recent = expectData(recentRes);
  const accountOptions = accounts.map((a) => toAccountOption(a, showOwners));

  if (accounts.length === 0 && !owner) {
    return (
      <>
        <PageHeader title={t("dashboard.title")} />
        <Card className="mx-auto max-w-md text-center">
          <CardHeader>
            <CardTitle>{t("dashboard.emptyTitle")}</CardTitle>
            <CardDescription>{t("dashboard.emptyDescription")}</CardDescription>
          </CardHeader>
          <CardContent>
            <Button nativeButton={false} render={<Link href={NEW_ACCOUNT_HREF} />}>
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
        <MonthPicker month={month} caption={t("dashboard.transactionCount", { count: recent.total })} keep={{ owner }} />
      </div>

      <OwnerFilter owners={owners} userId={user.id} value={owner} hrefFor={(o) => dashboardHref(month, o)} />

      <BalanceOverview currencies={summary.currencies} />

      <SubscriptionsCard
        upcoming={expectData(upcomingRes).items}
        committed={expectData(committedRes).currencies}
        suggestionCount={expectData(suggestionsRes).items.length}
        defaultDate={todayDate}
      />

      <BudgetsCard currencies={expectData(budgetsRes).currencies} href={`/budgets?${new URLSearchParams({ month, ...(owner && { owner }) })}`} />

      <section className="grid gap-2.5" aria-labelledby="accounts-heading">
        <div className="flex items-center justify-between px-1">
          <h2 id="accounts-heading" className="text-[19px] font-bold tracking-tight">
            {t("dashboard.accounts")}
          </h2>
          <Link href={owner ? `/accounts?owner=${owner}` : "/accounts"} className="flex min-h-11 items-center text-sm font-bold text-primary">
            {t("dashboard.viewAll")}
          </Link>
        </div>
        <AccountStrip accounts={accounts.map((a) => ({ ...a, name: accountLabel(a.name, a.owner, showOwners) }))} />
      </section>

      <Card>
        <CardHeader>
          <CardTitle>
            <h2>{t("dashboard.recentTransactions")}</h2>
          </CardTitle>
          <CardAction>
            <Link href={`/transactions?${new URLSearchParams({ from, to, ...(owner && { owner }) })}`} className="flex min-h-9 items-center text-sm font-bold text-primary">
              {t("dashboard.viewAll")}
            </Link>
          </CardAction>
        </CardHeader>
        <CardContent>
          <TransactionList
            transactions={recent.items.map((tx) => toTransactionRow(tx, showOwners))}
            accounts={accountOptions}
            categories={categories}
            defaultDate={todayDate}
            recurringNames={toRecurringNames(expectData(recurringRes).items)}
            recurringItems={expectData(recurringRes).items.map(toRecurringOption)}
          />
        </CardContent>
      </Card>
    </div>
  );
}
