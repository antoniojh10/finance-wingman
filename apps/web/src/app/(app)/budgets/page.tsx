import { PencilIcon } from "lucide-react";
import type { Metadata } from "next";
import Link from "next/link";
import { getTranslations } from "next-intl/server";

import { BudgetEditor } from "@/components/budgets/budget-editor";
import { BudgetList } from "@/components/budgets/budget-list";
import { MonthPicker } from "@/components/dashboard/month-picker";
import { OwnerFilter } from "@/components/owner-filter";
import { Button } from "@/components/ui/button";
import { buildEditCurrencies } from "@/lib/budgets";
import { monthOf, parseMonth, today } from "@/lib/dates";
import { parseOwner } from "@/lib/owners";
import { authedApi, expectData, getAccountOwners, getCurrentUser } from "@/lib/session";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("budgets");
  return { title: t("title") };
}

/** Budgets URL for a month and owner view, optionally in edit mode. */
function budgetsHref(month: string, owner: string | undefined, edit = false): string {
  const params = new URLSearchParams({ month });
  if (owner) {
    params.set("owner", owner);
  }
  if (edit) {
    params.set("edit", "1");
  }
  return `/budgets?${params}`;
}

export default async function BudgetsPage({ searchParams }: { searchParams: Promise<{ month?: string; owner?: string; edit?: string }> }) {
  const t = await getTranslations("budgets");
  const params = await searchParams;
  const month = parseMonth(params.month, monthOf(today(process.env.APP_TIMEZONE)));
  const owner = parseOwner(params.owner);
  const editing = params.edit === "1";
  const viewHref = budgetsHref(month, owner);

  const api = await authedApi();
  const statusRes = api.GET("/api/v1/budgets", { params: { query: { month, owner } } });

  let body: React.ReactNode;
  if (editing) {
    // Edit mode lists every expense category, not only those with activity.
    const [status, suggestions, categories, accounts] = await Promise.all([
      statusRes,
      api.GET("/api/v1/budgets/suggestions", { params: { query: { month } } }),
      api.GET("/api/v1/categories", { params: { query: { kind: "expense", include_archived: true } } }),
      api.GET("/api/v1/accounts"),
    ]);
    const currencies = buildEditCurrencies({
      status: expectData(status).currencies,
      accounts: expectData(accounts).items,
      categories: expectData(categories).items,
      suggestions: expectData(suggestions).currencies,
    });
    body = <BudgetEditor month={month} currencies={currencies} doneHref={viewHref} />;
  } else {
    const [status, owners, user] = await Promise.all([statusRes, getAccountOwners(), getCurrentUser()]);
    body = (
      <>
        <OwnerFilter owners={owners} userId={user.id} value={owner} hrefFor={(o) => budgetsHref(month, o)} />
        {owner && <p className="text-[13px] text-muted-foreground">{t("ownerHint")}</p>}
        <BudgetList currencies={expectData(status).currencies} month={month} editHref={budgetsHref(month, owner, true)} />
      </>
    );
  }

  return (
    <div className="grid grid-cols-1 gap-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-3xl font-bold tracking-tight">{t("title")}</h1>
        {!editing && (
          <Button variant="secondary" nativeButton={false} render={<Link href={budgetsHref(month, owner, true)} />}>
            <PencilIcon />
            {t("edit")}
          </Button>
        )}
      </div>
      <MonthPicker month={month} basePath="/budgets" keep={{ owner }} />
      {body}
    </div>
  );
}
