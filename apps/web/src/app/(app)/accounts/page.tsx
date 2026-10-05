import { PlusIcon } from "lucide-react";
import type { Metadata } from "next";
import Link from "next/link";
import { getTranslations } from "next-intl/server";

import { AccountDialog } from "@/components/accounts/account-dialog";
import { AccountList } from "@/components/accounts/account-list";
import { PageHeader } from "@/components/page-header";
import { Button } from "@/components/ui/button";
import { authedApi, expectData } from "@/lib/session";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("accounts");
  return { title: t("title") };
}

export default async function AccountsPage({ searchParams }: { searchParams: Promise<{ archived?: string }> }) {
  const t = await getTranslations();
  const showArchived = (await searchParams).archived === "1";

  const api = await authedApi();
  const [accountsRes, currenciesRes] = await Promise.all([
    api.GET("/api/v1/accounts", { params: { query: { include_archived: showArchived } } }),
    api.GET("/api/v1/currencies"),
  ]);
  const accounts = expectData(accountsRes).items;
  const currencies = expectData(currenciesRes).items.map((c) => ({ code: c.code, name: c.name }));
  // New accounts default to the currency used most, or MXN.
  const defaultCurrency = accounts[0]?.currency ?? process.env.DEFAULT_CURRENCY ?? "MXN";

  return (
    <>
      <PageHeader
        title={t("accounts.title")}
        actions={
          <AccountDialog
            currencies={currencies}
            defaultCurrency={defaultCurrency}
            trigger={
              <Button>
                <PlusIcon />
                {t("accounts.add")}
              </Button>
            }
          />
        }
      />
      <div className="grid grid-cols-1 gap-4">
        <AccountList accounts={accounts} currencies={currencies} defaultCurrency={defaultCurrency} />
        <Link
          href={showArchived ? "/accounts" : "/accounts?archived=1"}
          className="flex min-h-11 items-center justify-self-center px-4 text-sm font-bold underline underline-offset-4"
        >
          {t(showArchived ? "accounts.hideArchived" : "accounts.showArchived")}
        </Link>
      </div>
    </>
  );
}
