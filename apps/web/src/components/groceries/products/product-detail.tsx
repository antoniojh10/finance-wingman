"use client";

import { ArchiveIcon, ArchiveRestoreIcon, ArrowLeftIcon, PencilIcon, PlusIcon } from "lucide-react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { useState } from "react";

import { setProductArchived } from "@/app/actions/groceries-catalog";
import { ConfirmAction } from "@/components/confirm-action";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { defaultSeriesKey, historySeries, variantRows, type CatalogProduct, type GroceryStore, type ProductGroup, type ProductPrices } from "@/lib/groceries-products";
import { cn } from "@/lib/utils";

import { PriceCheckForm } from "./price-check-form";
import { PriceHistoryChart } from "./price-history-chart";
import { usePriceUnit } from "./price-unit";
import { ProductDialog } from "./product-dialog";
import { VariantDialog } from "./variant-dialog";
import { VariantsList } from "./variants-list";

function Section({ title, description, actions, className, children }: { title: string; description?: string; actions?: React.ReactNode; className?: string; children: React.ReactNode }) {
  return (
    <section className={cn("grid min-w-0 content-start gap-4 rounded-3xl border bg-card p-5 md:p-6", className)}>
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <h2 className="text-lg font-bold">{title}</h2>
          {description && <p className="mt-0.5 text-sm text-muted-foreground">{description}</p>}
        </div>
        {actions}
      </div>
      {children}
    </section>
  );
}

/** A product: its variants with latest prices, the price history and the price check form. */
export function ProductDetail({
  product,
  prices,
  stores,
  groups,
  currencies,
  today,
}: {
  product: CatalogProduct;
  prices: ProductPrices;
  stores: GroceryStore[];
  groups: ProductGroup[];
  currencies: string[];
  today: string;
}) {
  const t = useTranslations("groceries.products");
  const priceUnit = usePriceUnit();
  const [editing, setEditing] = useState(false);
  const [adding, setAdding] = useState(false);
  const unitLabel = priceUnit(product.compare_unit, product.display_multiplier);
  const activeStores = stores.filter((store) => !store.archived);
  const rows = variantRows(prices);
  const series = historySeries(prices);
  const checkable = prices.variants
    .filter((variant) => variant.pack_size !== null)
    .map((variant) => ({ id: variant.variant_id, label: [variant.brand, variant.pack_label ?? `${variant.pack_size} ${product.compare_unit}`].filter(Boolean).join(" · ") }));

  return (
    <div className="grid gap-6">
      <div className="grid gap-3">
        <Link href="/groceries/products" className="inline-flex min-h-8 w-fit items-center gap-1.5 text-sm font-medium text-muted-foreground hover:text-foreground">
          <ArrowLeftIcon className="size-4" />
          {t("detail.back")}
        </Link>
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="min-w-0">
            <h2 className="text-2xl font-bold tracking-tight" data-testid="product-name">
              {product.name}
            </h2>
            <div className="mt-2 flex flex-wrap gap-1.5">
              {product.group_name && (
                <Badge variant="secondary">
                  {product.group_color && <i aria-hidden className="size-2 rounded-full" style={{ background: product.group_color }} />}
                  {product.group_name}
                </Badge>
              )}
              <Badge variant="outline">{unitLabel}</Badge>
              {product.archived && <Badge variant="outline">{t("archived")}</Badge>}
            </div>
          </div>
          <div className="flex flex-wrap gap-2">
            <Button variant="outline" size="lg" onClick={() => setEditing(true)}>
              <PencilIcon />
              {t("detail.edit")}
            </Button>
            <ConfirmAction
              trigger={
                <Button variant="outline" size="lg">
                  {product.archived ? <ArchiveRestoreIcon /> : <ArchiveIcon />}
                  {product.archived ? t("detail.restore") : t("detail.archive")}
                </Button>
              }
              title={product.archived ? t("detail.restoreTitle") : t("detail.archiveTitle")}
              description={product.archived ? t("detail.restoreDescription") : t("detail.archiveDescription")}
              confirmLabel={product.archived ? t("detail.restore") : t("detail.archive")}
              successMessage={product.archived ? t("detail.restored") : t("detail.archived")}
              action={() => setProductArchived(product.id, !product.archived)}
            />
          </div>
        </div>
      </div>

      <Section
        title={t("variants.heading")}
        description={t("variants.description")}
        actions={
          <Button size="lg" onClick={() => setAdding(true)}>
            <PlusIcon />
            {t("variants.add")}
          </Button>
        }
      >
        <VariantsList rows={rows} productId={product.id} unit={product.compare_unit} unitLabel={unitLabel} stores={activeStores} />
      </Section>

      <div className="grid items-start gap-6 lg:grid-cols-2">
        <Section title={t("history.heading")} description={t("history.description")}>
          <PriceHistoryChart series={series} initialKey={defaultSeriesKey(series, rows)} unitLabel={unitLabel} />
        </Section>
        <Section title={t("priceCheck.heading")} description={t("priceCheck.description")}>
          <PriceCheckForm variants={checkable} stores={activeStores} currencies={currencies} today={today} />
        </Section>
      </div>

      <ProductDialog open={editing} onOpenChange={setEditing} product={product} groups={groups} />
      <VariantDialog open={adding} onOpenChange={setAdding} productId={product.id} unit={product.compare_unit} stores={activeStores} />
    </div>
  );
}
