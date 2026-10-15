"use client";

import { PencilIcon, Trash2Icon } from "lucide-react";
import { useLocale, useTranslations } from "next-intl";
import { useState } from "react";

import { deleteVariant } from "@/app/actions/groceries-catalog";
import { ConfirmAction } from "@/components/confirm-action";
import { Money } from "@/components/money";
import { RowActions } from "@/components/row-actions";
import { Badge } from "@/components/ui/badge";
import { intlLocale, isLocale } from "@/i18n/locales";
import { formatDate } from "@/lib/dates";
import { minorUnitsOf } from "@/lib/groceries";
import { formatMoney } from "@/lib/money";
import type { CompareUnit, GroceryStore, VariantRow } from "@/lib/groceries-products";

import { VariantDialog } from "./variant-dialog";

/**
 * Variants of a product with the latest price at each store, cheapest first.
 * Each row is a variant at a store; the row actions edit or delete the variant.
 */
export function VariantsList({
  rows,
  productId,
  unit,
  unitLabel,
  stores,
}: {
  rows: VariantRow[];
  productId: string;
  unit: CompareUnit;
  unitLabel: string;
  stores: GroceryStore[];
}) {
  const t = useTranslations("groceries.products.variants");
  const common = useTranslations("common");
  const appLocale = useLocale();
  const locale = intlLocale(isLocale(appLocale) ? appLocale : "en");
  const [editing, setEditing] = useState<VariantRow | null>(null);
  const [deleting, setDeleting] = useState<VariantRow | null>(null);

  if (rows.length === 0) {
    return <p className="text-sm text-muted-foreground">{t("empty")}</p>;
  }

  // The cheapest priced row of each currency, when there is something to compare it with.
  const priced = rows.filter((row) => row.displayUnitPrice !== null);
  const best = new Set(
    [...new Set(priced.map((row) => row.currency))]
      .map((currency) => priced.filter((row) => row.currency === currency))
      .filter((group) => group.length > 1)
      .map((group) => group[0].key),
  );

  return (
    <>
      <div className="hidden grid-cols-[minmax(0,1.4fr)_minmax(0,1.1fr)_minmax(0,0.7fr)_minmax(0,0.9fr)_2.5rem] gap-4 border-b px-1 pb-2 text-xs font-bold tracking-wide text-muted-foreground uppercase md:grid">
        <span>{t("colVariant")}</span>
        <span>{t("colStore")}</span>
        <span className="text-right">{t("colPack")}</span>
        <span className="text-right">{t("colUnit", { unit: unitLabel })}</span>
        <span />
      </div>
      <ul className="divide-y" data-testid="variant-rows">
        {rows.map((row) => {
          const minorUnits = row.currency ? minorUnitsOf(row.currency) : 2;
          return (
            <li
              key={row.key}
              data-testid="variant-row"
              className="grid grid-cols-[minmax(0,1fr)_auto_auto] items-center gap-x-4 gap-y-1 py-3 md:grid-cols-[minmax(0,1.4fr)_minmax(0,1.1fr)_minmax(0,0.7fr)_minmax(0,0.9fr)_2.5rem] md:px-1"
            >
              <div className="min-w-0">
                <p className="truncate font-bold">
                  {row.brand}
                  {row.packLabel && <span className="font-normal text-muted-foreground"> · {row.packLabel}</span>}
                </p>
                <div className="mt-1 flex flex-wrap gap-1">
                  {row.storeBrand && <Badge variant="secondary">{t("storeBrand")}</Badge>}
                  {best.has(row.key) && <Badge className="bg-income/15 text-income">{t("best")}</Badge>}
                  {row.stale && <Badge variant="outline" className="border-amber-500/60 text-amber-700 dark:text-amber-400">{t("stale")}</Badge>}
                  {row.packSize === null && <Badge variant="outline">{t("loose")}</Badge>}
                </div>
              </div>
              <div className="order-last col-span-3 min-w-0 text-sm text-muted-foreground md:order-none md:col-span-1">
                {row.storeName ? (
                  <>
                    <span className="font-medium text-foreground">{row.storeName}</span>
                    {row.observedOn && (
                      <span>
                        {" · "}
                        {formatDate(row.observedOn, locale)} ({row.source === "price_check" ? t("sourceCheck") : t("sourceReceipt")})
                      </span>
                    )}
                  </>
                ) : (
                  t("noPrice")
                )}
              </div>
              <p className="hidden text-right font-mono text-sm tabular-nums md:block" data-testid="pack-price">
                {row.packPrice !== null && row.currency ? <Money amount={row.packPrice} currency={row.currency} minorUnits={minorUnits} /> : "—"}
              </p>
              <p className="text-right font-mono font-bold tabular-nums" data-testid="unit-price">
                {row.displayUnitPrice !== null && row.currency ? (
                  <>
                    <Money amount={Math.round(row.displayUnitPrice)} currency={row.currency} minorUnits={minorUnits} />
                    <span className="block text-xs font-normal text-muted-foreground md:hidden">
                      {row.packPrice !== null ? t("packPriceInline", { price: formatMoney(row.packPrice, row.currency, minorUnits, locale) }) : unitLabel}
                    </span>
                  </>
                ) : (
                  "—"
                )}
              </p>
              <RowActions
                actions={[
                  { label: common("edit"), icon: PencilIcon, onSelect: () => setEditing(row) },
                  { label: common("delete"), icon: Trash2Icon, destructive: true, onSelect: () => setDeleting(row) },
                ]}
              />
            </li>
          );
        })}
      </ul>
      <VariantDialog
        open={editing !== null}
        onOpenChange={(open) => !open && setEditing(null)}
        productId={productId}
        unit={unit}
        stores={stores}
        variant={editing ?? undefined}
      />
      <ConfirmAction
        open={deleting !== null}
        onOpenChange={(open) => !open && setDeleting(null)}
        title={t("deleteTitle", { name: deleting?.brand ?? "" })}
        description={t("deleteDescription")}
        confirmLabel={common("delete")}
        successMessage={t("deleted")}
        action={() => deleteVariant(deleting?.variantId ?? "")}
      />
    </>
  );
}
