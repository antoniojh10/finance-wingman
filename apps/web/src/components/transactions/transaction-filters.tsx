import Link from "next/link";
import { useTranslations } from "next-intl";

import { Field } from "@/components/field";
import { NativeSelect } from "@/components/native-select";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

import type { AccountOption, CategoryOption } from "./types";

export type TransactionFilterValues = {
  account_id?: string;
  category_id?: string;
  type?: string;
  from?: string;
  to?: string;
  q?: string;
};

/** Filters submitted as a plain GET form, so they work without JavaScript. */
export function TransactionFilters({
  values,
  accounts,
  categories,
}: {
  values: TransactionFilterValues;
  accounts: AccountOption[];
  categories: CategoryOption[];
}) {
  const t = useTranslations();
  return (
    <form method="get" className="grid gap-3 rounded-xl border p-4 sm:grid-cols-2 lg:grid-cols-6" role="search">
      <div className="sm:col-span-2 lg:col-span-2">
        <Field id="q" label={t("transactions.search")}>
          <Input id="q" name="q" type="search" defaultValue={values.q} placeholder={t("transactions.searchPlaceholder")} />
        </Field>
      </div>
      <Field id="filter-type" label={t("transactions.type")}>
        <NativeSelect id="filter-type" name="type" defaultValue={values.type ?? ""}>
          <option value="">{t("common.all")}</option>
          {(["expense", "income", "transfer"] as const).map((type) => (
            <option key={type} value={type}>
              {t(`transactions.types.${type}`)}
            </option>
          ))}
        </NativeSelect>
      </Field>
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
      <div className="grid grid-cols-2 gap-3 sm:col-span-2 lg:col-span-1 lg:grid-cols-1">
        <Field id="filter-from" label={t("transactions.from")}>
          <Input id="filter-from" name="from" type="date" defaultValue={values.from} />
        </Field>
        <Field id="filter-to" label={t("transactions.to")}>
          <Input id="filter-to" name="to" type="date" defaultValue={values.to} />
        </Field>
      </div>
      <div className="flex gap-2 sm:col-span-2 lg:col-span-6 lg:justify-end">
        <Button nativeButton={false} render={<Link href="/transactions" />} variant="ghost">
          {t("transactions.clearFilters")}
        </Button>
        <Button type="submit">{t("transactions.filter")}</Button>
      </div>
    </form>
  );
}
