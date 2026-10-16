import { ChevronRightIcon, SearchIcon } from "lucide-react";
import Link from "next/link";
import { useTranslations } from "next-intl";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { groupProducts, productHref, type CatalogProduct, type GroceryStore, type ProductGroup } from "@/lib/groceries-products";

import { CatalogAdmin } from "./catalog-admin";
import { NewProductButton } from "./new-product-button";
import { useGroupLabel } from "../use-group-label";
import { usePriceUnit } from "./price-unit";

/** The products tab: search, products grouped by product group, and stores/groups management. */
export function ProductsList({
  products,
  groups,
  stores,
  search,
}: {
  products: CatalogProduct[];
  groups: ProductGroup[];
  stores: GroceryStore[];
  search?: string;
}) {
  const t = useTranslations("groceries.products");
  const priceUnit = usePriceUnit();
  const label = useGroupLabel();
  const sections = groupProducts(products, groups, search);

  return (
    <div className="grid gap-8">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <form role="search" method="get" className="flex min-w-0 flex-1 gap-2 md:max-w-md">
          <div className="relative min-w-0 flex-1">
            <SearchIcon aria-hidden className="pointer-events-none absolute top-1/2 left-3.5 size-4 -translate-y-1/2 text-muted-foreground" />
            <Input name="q" defaultValue={search} aria-label={t("search")} placeholder={t("search")} autoComplete="off" className="pl-10" />
          </div>
          <Button type="submit" variant="outline" size="lg">
            {t("searchButton")}
          </Button>
        </form>
        <NewProductButton groups={groups} />
      </div>

      {sections.length === 0 ? (
        <p className="rounded-3xl border border-dashed p-8 text-center text-muted-foreground" data-testid="products-empty">
          {search ? t("emptySearch") : t("empty")}
        </p>
      ) : (
        <div className="grid gap-6">
          {sections.map((section) => {
            const name = section.name === null ? t("noGroup") : label({ name: section.name, default_key: section.defaultKey });
            return (
            <section key={section.key} aria-label={name} className="grid gap-2" data-testid="product-section">
              <h2 className="flex items-center gap-2 text-sm font-bold tracking-wide text-muted-foreground uppercase">
                <i aria-hidden className="size-2.5 rounded-full" style={{ background: section.color ?? "var(--muted-foreground)" }} />
                {name}
                <span className="font-mono font-normal">{section.products.length}</span>
              </h2>
              <ul className="grid gap-2 md:grid-cols-2">
                {section.products.map((product) => (
                  <li key={product.id}>
                    <Link
                      href={productHref(product.id)}
                      className="flex min-h-14 items-center gap-3 rounded-2xl border bg-card px-4 py-3 transition-colors hover:bg-muted focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none"
                    >
                      <div className="min-w-0 flex-1">
                        <p className="truncate font-bold">{product.name}</p>
                        <p className="text-sm text-muted-foreground">
                          {priceUnit(product.compare_unit, product.display_multiplier)} · {t("variantCount", { count: product.variant_count })}
                        </p>
                      </div>
                      {product.archived && <Badge variant="outline">{t("archived")}</Badge>}
                      <ChevronRightIcon aria-hidden className="size-4 shrink-0 text-muted-foreground" />
                    </Link>
                  </li>
                ))}
              </ul>
            </section>
            );
          })}
        </div>
      )}

      <CatalogAdmin stores={stores} groups={groups} />
    </div>
  );
}
