import { PlusIcon } from "lucide-react";
import type { Metadata } from "next";
import Link from "next/link";
import { getTranslations } from "next-intl/server";

import { AccountDialog } from "@/components/accounts/account-dialog";
import { AccountList } from "@/components/accounts/account-list";
import { OwnerFilter } from "@/components/owner-filter";
import { PageHeader } from "@/components/page-header";
import { Button } from "@/components/ui/button";
import { today } from "@/lib/dates";
import { parseOwner } from "@/lib/owners";
import { authedApi, expectData, getAccountOwners, getCurrentUser } from "@/lib/session";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("accounts");
  return { title: t("title") };
}

/** Accounts page URL keeping its filters. */
function accountsHref(archived: boolean, owner: string | undefined): string {
  const params = new URLSearchParams();
  if (archived) {
    params.set("archived", "1");
  }
  if (owner) {
    params.set("owner", owner);
  }
  const qs = params.toString();
  return qs ? `/accounts?${qs}` : "/accounts";
}

export default async function AccountsPage({ searchParams }: { searchParams: Promise<{ archived?: string; owner?: string }> }) {
  const t = await getTranslations();
  const params = await searchParams;
  const showArchived = params.archived === "1";
  const owner = parseOwner(params.owner);

  const api = await authedApi();
  const [accountsRes, currenciesRes, owners, user] = await Promise.all([
    api.GET("/api/v1/accounts", { params: { query: { include_archived: showArchived, owner } } }),
    api.GET("/api/v1/currencies"),
    getAccountOwners(),
    getCurrentUser(),
  ]);
  const ownership = owners.length > 1 ? { owners, userId: user.id } : undefined;
  const accounts = expectData(accountsRes).items;
  const currencies = expectData(currenciesRes).items.map((c) => ({ code: c.code, name: c.name }));
  // New accounts default to the currency used most, or MXN.
  const defaultCurrency = accounts[0]?.currency ?? process.env.DEFAULT_CURRENCY ?? "MXN";
  const defaultDate = today(process.env.APP_TIMEZONE);

  return (
    <>
      <PageHeader
        title={t("accounts.title")}
        actions={
          <AccountDialog
            currencies={currencies}
            defaultCurrency={defaultCurrency}
            defaultDate={defaultDate}
            ownership={ownership}
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
        <OwnerFilter owners={owners} userId={user.id} value={owner} hrefFor={(o) => accountsHref(showArchived, o)} />
        <AccountList
          accounts={accounts}
          currencies={currencies}
          defaultCurrency={defaultCurrency}
          defaultDate={defaultDate}
          ownership={ownership}
        />
        <Link
          href={accountsHref(!showArchived, owner)}
          className="flex min-h-11 items-center justify-self-center px-4 text-sm font-bold underline underline-offset-4"
        >
          {t(showArchived ? "accounts.hideArchived" : "accounts.showArchived")}
        </Link>
      </div>
    </>
  );
}
