import { useTranslations } from "next-intl";

import { Money } from "@/components/money";
import { Badge } from "@/components/ui/badge";
import { minorUnitsOf } from "@/lib/groceries";
import { basketBars, basketSavings, overviewHref, type Basket, type OverviewQuery } from "@/lib/groceries-insights";
import { cn } from "@/lib/utils";

import { Segmented } from "./segmented";

/**
 * The monthly basket: its cost at each store (only the stores pricing every
 * product, or all of them) and what was paid against the best single store
 * and the basket split across stores.
 */
export function BasketCard({ data, months, query }: { data: Basket | undefined; months: number; query: OverviewQuery }) {
  const t = useTranslations("groceries.overview");

  if (!data || data.products.length === 0 || data.stores.length === 0) {
    return (
      <section className="grid gap-3 rounded-3xl bg-card p-5 ring-1 ring-foreground/5" aria-labelledby="basket-title" data-testid="groceries-basket">
        <h2 id="basket-title" className="font-heading text-lg font-bold">
          {t("basketTitle")}
        </h2>
        <p className="py-6 text-center text-sm text-muted-foreground" data-testid="basket-empty">
          {t("basketEmpty")}
        </p>
      </section>
    );
  }

  const minorUnits = minorUnitsOf(data.currency);
  const bars = basketBars(data, query.basket);
  const savings = basketSavings(data);
  const incomplete = data.stores.filter((store) => !store.complete).length;

  return (
    <section className="grid min-w-0 gap-4" aria-labelledby="basket-title" data-testid="groceries-basket">
      <div className="grid gap-1">
        <h2 id="basket-title" className="font-heading text-lg font-bold">
          {t("basketTitle")}
        </h2>
        <p className="text-sm text-muted-foreground">{t("basketHelp", { months, count: data.products.length })}</p>
      </div>
      <div className="grid gap-4 md:grid-cols-2 [&>*]:min-w-0">
        <div className="grid content-start gap-4 rounded-3xl bg-card p-5 ring-1 ring-foreground/5">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <h3 className="font-heading text-base font-bold">{t("basketCost")}</h3>
            <Segmented
              label={t("basketScope")}
              options={[
                { key: "comparable", label: t("scopeComparable"), href: overviewHref(query, { basket: "comparable" }), active: query.basket === "comparable" },
                { key: "all", label: t("scopeAll"), href: overviewHref(query, { basket: "all" }), active: query.basket === "all" },
              ]}
            />
          </div>
          {bars.length === 0 ? (
            <p className="py-4 text-sm text-muted-foreground">{t("basketNoComparable")}</p>
          ) : (
            <ul className="grid gap-3.5">
              {bars.map((bar) => (
                <li key={bar.storeId} className="grid gap-1.5" data-testid="basket-row">
                  <div className="flex items-baseline justify-between gap-3 text-sm">
                    <span className="flex min-w-0 items-center gap-2">
                      <span className="truncate font-bold">{bar.name}</span>
                      {bar.best && (
                        <Badge variant="secondary" className="h-5 bg-income-soft px-2 text-[11px] text-income">
                          {t("cheapest")}
                        </Badge>
                      )}
                      {!bar.complete && (
                        <Badge variant="outline" className="h-5 px-2 text-[11px] text-amber-800 dark:text-amber-300">
                          {t("missing", { count: bar.missing })}
                        </Badge>
                      )}
                    </span>
                    <Money amount={bar.cost} currency={data.currency} minorUnits={minorUnits} className="shrink-0 font-bold" />
                  </div>
                  <div className="h-2.5 rounded-full bg-muted" aria-hidden>
                    <div
                      className={cn("h-full rounded-full", bar.best ? "bg-income" : "bg-primary", !bar.complete && "opacity-45")}
                      style={{ width: `${Math.max(bar.width * 100, 1.5)}%` }}
                    />
                  </div>
                </li>
              ))}
            </ul>
          )}
          {query.basket === "comparable" && incomplete > 0 && <p className="text-[13px] text-muted-foreground">{t("hiddenStores", { count: incomplete })}</p>}
          {query.basket === "all" && incomplete > 0 && <p className="text-[13px] text-muted-foreground">{t("incompleteNote")}</p>}
        </div>
        <div className="grid content-start gap-3 rounded-3xl bg-card p-5 ring-1 ring-foreground/5">
          <h3 className="font-heading text-base font-bold">{t("savingsTitle")}</h3>
          <dl className="grid gap-1" data-testid="basket-savings">
            {savings.map((row) => (
              <div key={row.key} className="flex items-baseline justify-between gap-3 border-b border-border py-2.5 last:border-b-0" data-testid={`savings-${row.key}`}>
                <dt className="grid text-sm">
                  <span className="font-bold">{row.key === "bestStore" ? t("bestStore", { store: row.storeName ?? "" }) : t(row.key)}</span>
                  {row.saving !== null && row.saving > 0 && (
                    <span className="text-[13px] text-income">{t("saves")}</span>
                  )}
                </dt>
                <dd className="grid justify-items-end">
                  <Money amount={row.cost} currency={data.currency} minorUnits={minorUnits} className="font-bold" />
                  {row.saving !== null && row.saving > 0 && <Money amount={row.saving} currency={data.currency} minorUnits={minorUnits} className="text-[13px] text-income" />}
                </dd>
              </div>
            ))}
          </dl>
          {!data.best_single_store && <p className="text-[13px] text-muted-foreground">{t("noBestStore")}</p>}
          {data.split.unpriced_products.length > 0 && (
            <p className="text-[13px] text-muted-foreground">{t("unpriced", { names: data.split.unpriced_products.map((product) => product.name).join(", ") })}</p>
          )}
          <p className="text-[13px] text-muted-foreground">{t("splitNote")}</p>
        </div>
      </div>
    </section>
  );
}
