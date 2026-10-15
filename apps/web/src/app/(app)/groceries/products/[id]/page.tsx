import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { getTranslations } from "next-intl/server";

import { ProductDetail } from "@/components/groceries/products/product-detail";
import { today } from "@/lib/dates";
import { authedApi, expectData } from "@/lib/session";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("groceries.products");
  return { title: t("title") };
}

export default async function ProductPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  const api = await authedApi();

  const [pricesRes, productsRes, groups, stores, accounts] = await Promise.all([
    api.GET("/api/v1/groceries/products/{id}/prices", { params: { path: { id } } }),
    api.GET("/api/v1/groceries/products", { params: { query: { include_archived: true } } }),
    api.GET("/api/v1/groceries/groups"),
    api.GET("/api/v1/groceries/stores", { params: { query: { include_archived: true } } }),
    api.GET("/api/v1/accounts"),
  ]);
  if (pricesRes.response.status === 404 || pricesRes.response.status === 422) {
    notFound();
  }
  const prices = expectData(pricesRes);
  const product = expectData(productsRes).items.find((item) => item.id === id);
  if (!product) {
    notFound();
  }

  // Price checks are typed in a currency the user already holds, or one already priced.
  const currencies = [
    ...new Set([
      ...expectData(accounts).items.map((account) => account.currency),
      ...prices.variants.flatMap((variant) => variant.latest.map((price) => price.currency)),
    ]),
  ].sort();

  return (
    <ProductDetail
      product={product}
      prices={prices}
      stores={expectData(stores).items}
      groups={expectData(groups).items}
      currencies={currencies.length > 0 ? currencies : ["EUR"]}
      today={today(process.env.APP_TIMEZONE)}
    />
  );
}
