import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { getTranslations } from "next-intl/server";

import { ReceiptDetail } from "@/components/groceries/receipt-detail";
import { buildCatalog } from "@/lib/groceries";
import { authedApi, expectData } from "@/lib/session";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("groceries.receipts");
  return { title: t("title") };
}

export default async function ReceiptPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  const api = await authedApi();

  const [receiptRes, productsRes, groupsRes] = await Promise.all([
    api.GET("/api/v1/groceries/receipts/{id}", { params: { path: { id } } }),
    api.GET("/api/v1/groceries/products"),
    api.GET("/api/v1/groceries/groups"),
  ]);
  if (receiptRes.response.status === 404 || receiptRes.response.status === 422) {
    notFound();
  }
  const receipt = expectData(receiptRes);
  const products = expectData(productsRes).items;

  // Variants are listed per product; only products that have some are asked.
  const variants = (
    await Promise.all(
      products
        .filter((product) => product.variant_count > 0)
        .map(async (product) =>
          expectData(await api.GET("/api/v1/groceries/products/{id}/variants", { params: { path: { id: product.id } } })).items,
        ),
    )
  ).flat();

  return (
    <ReceiptDetail
      receipt={receipt}
      catalog={buildCatalog(products, variants).filter((entry) => entry.variants.length > 0)}
      groups={expectData(groupsRes).items.map((group) => ({ id: group.id, name: group.name }))}
    />
  );
}
