import { SearchIcon, SlidersHorizontalIcon } from "lucide-react";
import Link from "next/link";
import { useTranslations } from "next-intl";

import { Field } from "@/components/field";
import { NativeSelect } from "@/components/native-select";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { SHARED, type OwnerOption } from "@/lib/owners";
import { filterHref, type TransactionQuery } from "@/lib/query";
import { cn } from "@/lib/utils";

import type { AccountOption, CategoryOption } from "./types";

const types = [undefined, "expense", "income", "transfer"] as const;

/**
 * Search plus type chips, with the less common filters folded away. The
 * form is a plain GET form and the chips are links, so filtering works
 * without JavaScript.
 */
export function TransactionFilters({
  values,
  accounts,
  categories,
  owners,
}: {
  values: TransactionQuery;
  accounts: AccountOption[];
  categories: CategoryOption[];
  /** Workspace members, to filter by the accounts' owner; only with several members. */
  owners?: { owners: OwnerOption[]; userId: string };
}) {
  const t = useTranslations();
  const advanced = [values.account_id, values.category_id, values.from, values.to, values.owner].filter(Boolean).length;

  return (
    <div className="grid gap-3">
      <form method="get" className="grid gap-3" role="search">
        {values.type && <input type="hidden" name="type" value={values.type} />}
        <div className="flex gap-2">
          <label htmlFor="q" className="sr-only">
            {t("transactions.search")}
          </label>
          <div className="relative flex-1">
            <SearchIcon className="pointer-events-none absolute top-1/2 left-4 size-5 -translate-y-1/2 text-muted-foreground" aria-hidden />
            <Input
              id="q"
              name="q"
              type="search"
              defaultValue={values.q}
              placeholder={t("transactions.searchPlaceholder")}
              className="h-12.5 rounded-2xl pl-11"
            />
          </div>
          <Button type="submit" size="lg" className="h-12.5">
            {t("transactions.filter")}
          </Button>
        </div>

        <details className="group rounded-2xl bg-card ring-1 ring-foreground/5" open={advanced > 0}>
          <summary className="flex min-h-11 cursor-pointer list-none items-center gap-2 px-4 text-sm font-semibold [&::-webkit-details-marker]:hidden">
            <SlidersHorizontalIcon className="size-4" aria-hidden />
            {t("transactions.moreFilters")}
            {advanced > 0 && (
              <span className="flex size-5 items-center justify-center rounded-full bg-primary text-[11px] font-bold text-primary-foreground">
                {advanced}
              </span>
            )}
          </summary>
          <div className="grid gap-3 px-4 pb-4 sm:grid-cols-2 lg:grid-cols-4">
            <Field id="filter-account" label={t("transactions.account")}>
              <NativeSelect id="filter-account" name="account_id" defaultValue={values.account_id ?? ""}>
                <option value="">{t("common.all")}</option>
                {accounts.map((a) => (
                  <option key={a.id} value={a.id}>
                    {a.name}
                  </option>
                ))}
              </NativeSelect>
            </Field>
            {owners && (
              <Field id="filter-owner" label={t("owners.owner")}>
                <NativeSelect id="filter-owner" name="owner" defaultValue={values.owner ?? ""}>
                  <option value="">{t("owners.everyone")}</option>
                  {owners.owners.map((o) => (
                    <option key={o.id} value={o.id}>
                      {o.id === owners.userId ? t("owners.you", { name: o.name }) : o.name}
                    </option>
                  ))}
                  <option value={SHARED}>{t("owners.shared")}</option>
                </NativeSelect>
              </Field>
            )}
            <Field id="filter-category" label={t("transactions.category")}>
              <NativeSelect id="filter-category" name="category_id" defaultValue={values.category_id ?? ""}>
                <option value="">{t("common.all")}</option>
                {categories.map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.name}
                  </option>
                ))}
              </NativeSelect>
            </Field>
            <Field id="filter-from" label={t("transactions.from")}>
              <Input id="filter-from" name="from" type="date" defaultValue={values.from} />
            </Field>
            <Field id="filter-to" label={t("transactions.to")}>
              <Input id="filter-to" name="to" type="date" defaultValue={values.to} />
            </Field>
            <div className="sm:col-span-2 lg:col-span-4">
              <Link href="/transactions" className="text-sm font-semibold text-primary underline-offset-4 hover:underline">
                {t("transactions.clearFilters")}
              </Link>
            </div>
          </div>
        </details>
      </form>

      <nav aria-label={t("transactions.type")} className="-mx-4 flex gap-2 overflow-x-auto px-4 pb-0.5 md:mx-0 md:px-0">
        {types.map((type) => {
          const active = values.type === type;
          return (
            <Link
              key={type ?? "all"}
              href={filterHref(values, { type })}
              aria-current={active ? "page" : undefined}
              className={cn(
                "flex h-10 shrink-0 items-center rounded-full border-[1.5px] px-4 text-sm font-bold transition-colors",
                active ? "border-foreground bg-foreground text-background" : "border-border bg-card hover:border-foreground/30",
              )}
            >
              {type ? t(`transactions.types.${type}`) : t("common.all")}
            </Link>
          );
        })}
      </nav>
    </div>
  );
}
