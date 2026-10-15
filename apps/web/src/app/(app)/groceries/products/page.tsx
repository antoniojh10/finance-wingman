import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";

import { ProductsList } from "@/components/groceries/products/products-list";
import { parseProductsQuery } from "@/lib/groceries-products";
import { authedApi, expectData } from "@/lib/session";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("groceries.products");
  return { title: t("title") };
}

export default async function ProductsPage({ searchParams }: { searchParams: Promise<Record<string, string | string[] | undefined>> }) {
  const { q } = parseProductsQuery(await searchParams);
  const api = await authedApi();
  const [products, groups, stores] = await Promise.all([
    api.GET("/api/v1/groceries/products"),
    api.GET("/api/v1/groceries/groups"),
    api.GET("/api/v1/groceries/stores", { params: { query: { include_archived: true } } }),
  ]);

  return <ProductsList products={expectData(products).items} groups={expectData(groups).items} stores={expectData(stores).items} search={q} />;
}
