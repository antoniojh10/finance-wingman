import { PlusIcon } from "lucide-react";
import type { Metadata } from "next";
import Link from "next/link";
import { getTranslations } from "next-intl/server";

import { CategoryBreakdown } from "@/components/dashboard/category-breakdown";
import { MonthPicker } from "@/components/dashboard/month-picker";
import { SummaryCards } from "@/components/dashboard/summary-cards";
import { Money } from "@/components/money";
import { PageHeader } from "@/components/page-header";
import { TransactionDialog } from "@/components/transactions/transaction-dialog";
import { TransactionList } from "@/components/transactions/transaction-list";
import { Button } from "@/components/ui/button";
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { monthOf, monthRange, parseMonth, today } from "@/lib/dates";
import { authedApi, expectData } from "@/lib/session";
import { toAccountOption, toCategoryOption, toTransactionRow } from "@/lib/view-models";

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
  const [summaryRes, accountsRes, categoriesRes, recentRes] = await Promise.all([
    api.GET("/api/v1/summary", { params: { query: { from, to } } }),
    api.GET("/api/v1/accounts"),
    api.GET("/api/v1/categories"),
    api.GET("/api/v1/transactions", { params: { query: { from, to, limit: 8 } } }),
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
    <>
      <PageHeader
        title={t("dashboard.title")}
        actions={
          <>
            <MonthPicker month={month} />
            <TransactionDialog
              accounts={accountOptions}
              categories={categories}
              defaultDate={todayDate}
              trigger={
                <Button>
                  <PlusIcon />
                  {t("transactions.add")}
                </Button>
              }
            />
          </>
        }
      />

      <div className="grid gap-8">
        {summary.currencies.map((currency) => (
          <section key={currency.currency} className="grid gap-4" aria-label={currency.currency}>
            <SummaryCards summary={currency} />
            <Card>
              <CardHeader>
                <CardTitle>{t("dashboard.spendingByCategory")}</CardTitle>
                <CardDescription>{currency.currency}</CardDescription>
              </CardHeader>
              <CardContent>
                <CategoryBreakdown totals={currency.expenses} currency={currency.currency} minorUnits={currency.minor_units} />
              </CardContent>
            </Card>
          </section>
        ))}

        <div className="grid gap-4 lg:grid-cols-[2fr_1fr]">
          <Card>
            <CardHeader>
              <CardTitle>{t("dashboard.recentTransactions")}</CardTitle>
              <CardAction>
                <Button nativeButton={false} render={<Link href={`/transactions?from=${from}&to=${to}`} />} variant="link" size="sm">
                  {t("dashboard.viewAll")}
                </Button>
              </CardAction>
            </CardHeader>
            <CardContent>
              <TransactionList
                transactions={recent.items.map(toTransactionRow)}
                accounts={accountOptions}
                categories={categories}
                defaultDate={todayDate}
              />
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>{t("dashboard.accounts")}</CardTitle>
            </CardHeader>
            <CardContent>
              <ul className="grid gap-3">
                {accounts.map((a) => (
                  <li key={a.id} className="flex items-center justify-between gap-3 text-sm">
                    <span className="min-w-0">
                      <span className="block truncate font-medium">{a.name}</span>
                      <span className="text-xs text-muted-foreground">{t(`accounts.types.${a.type}`)}</span>
                    </span>
                    <Money amount={a.balance} currency={a.currency} minorUnits={a.minor_units} className="font-medium" />
                  </li>
                ))}
              </ul>
            </CardContent>
          </Card>
        </div>
      </div>
    </>
  );
}
