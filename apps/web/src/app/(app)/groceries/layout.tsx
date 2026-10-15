import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";

import { GroceriesTabs } from "@/components/groceries/groceries-tabs";
import { PageHeader } from "@/components/page-header";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("groceries");
  return { title: t("title") };
}

/** Shared header and sub-navigation of every groceries page. */
export default async function GroceriesLayout({ children }: { children: React.ReactNode }) {
  const t = await getTranslations("groceries");
  return (
    <>
      <PageHeader title={t("title")} />
      <GroceriesTabs />
      {children}
    </>
  );
}
