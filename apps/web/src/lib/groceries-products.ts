import type { components } from "@/lib/api/schema";
import { minorUnitsOf, parsePositiveNumber } from "@/lib/groceries";
import { parseAmount } from "@/lib/money";

// View models, URL helpers and form parsing for the groceries products tab.
// Pure, so the pages, components and actions share them without the API.

export type CatalogProduct = components["schemas"]["Product"];
export type ProductGroup = components["schemas"]["ProductGroup"];
export type GroceryStore = components["schemas"]["Store"];
export type ProductPrices = components["schemas"]["ProductPrices"];
export type CompareUnit = CatalogProduct["compare_unit"];

export const COMPARE_UNITS: CompareUnit[] = ["kg", "l", "unit"];

/** Sanitizes the products search params: only the free-text search. */
export function parseProductsQuery(params: Record<string, string | string[] | undefined>): { q?: string } {
  const raw = Array.isArray(params.q) ? params.q[0] : params.q;
  return { q: raw?.trim() || undefined };
}

export function productHref(id: string): string {
  return `/groceries/products/${id}`;
}

/** Translator of `groceries.groupNames` (a next-intl translator or any function with `has`). */
export type GroupNameTranslator = ((key: string) => string) & { has: (key: string) => boolean };

/**
 * Name to show for a product group: the translation while it is a default
 * group (it has a default key), else the name as stored. Groups the user
 * created or renamed have no key and are shown as typed.
 */
export function groupLabel(group: { name: string; default_key?: string | null }, t: GroupNameTranslator): string;
export function groupLabel(group: { name: string | null; default_key?: string | null }, t: GroupNameTranslator): string | null;
export function groupLabel(group: { name: string | null; default_key?: string | null }, t: GroupNameTranslator): string | null {
  return group.default_key && t.has(group.default_key) ? t(group.default_key) : group.name;
}

export type ProductSection = { key: string; name: string | null; defaultKey: string | null; color: string | null; products: CatalogProduct[] };

/**
 * Groups products by product group, in the order of the groups (products
 * without a group last), keeping only those that match the search.
 */
export function groupProducts(products: CatalogProduct[], groups: ProductGroup[], search?: string): ProductSection[] {
  const needle = search?.trim().toLowerCase();
  const matching = products
    .filter((product) => !needle || product.name.toLowerCase().includes(needle))
    .sort((a, b) => a.name.localeCompare(b.name));

  const sections: ProductSection[] = [...groups]
    .sort((a, b) => a.position - b.position || a.name.localeCompare(b.name))
    .map((group) => ({ key: group.id, name: group.name, defaultKey: group.default_key, color: group.color, products: matching.filter((p) => p.group_id === group.id) }));
  const known = new Set(groups.map((group) => group.id));
  sections.push({ key: "none", name: null, defaultKey: null, color: null, products: matching.filter((p) => !p.group_id || !known.has(p.group_id)) });
  return sections.filter((section) => section.products.length > 0);
}

export type VariantRow = {
  key: string;
  variantId: string;
  brand: string;
  packLabel: string | null;
  packSize: number | null;
  /** Sold only by one chain. */
  storeBrand: boolean;
  /** The chain a store brand belongs to. */
  brandStoreId: string | null;
  /** The store this price was seen at; null when the variant has no price yet. */
  storeId: string | null;
  storeName: string | null;
  currency: string | null;
  /** Price per compare unit times the product multiplier, in minor units (may be fractional). */
  displayUnitPrice: number | null;
  /** Price of one pack in minor units; null for loose variants. */
  packPrice: number | null;
  observedOn: string | null;
  source: "receipt" | "price_check" | null;
  stale: boolean;
};

/**
 * One row per (variant, store) with its latest price, cheapest first. Rows
 * are ordered by currency (never mixed), then by unit price; variants that
 * were never priced come last.
 */
export function variantRows(prices: ProductPrices): VariantRow[] {
  const rows: VariantRow[] = [];
  for (const variant of prices.variants) {
    const base = {
      variantId: variant.variant_id,
      brand: variant.brand,
      packLabel: variant.pack_label,
      packSize: variant.pack_size,
      storeBrand: variant.store_id !== null,
      brandStoreId: variant.store_id,
    };
    if (variant.latest.length === 0) {
      rows.push({
        ...base,
        key: variant.variant_id,
        storeId: null,
        storeName: null,
        currency: null,
        displayUnitPrice: null,
        packPrice: null,
        observedOn: null,
        source: null,
        stale: false,
      });
      continue;
    }
    for (const price of variant.latest) {
      rows.push({
        ...base,
        key: `${variant.variant_id}:${price.store_id}`,
        storeId: price.store_id,
        storeName: price.store_name,
        currency: price.currency,
        displayUnitPrice: price.display_unit_price,
        packPrice: variant.pack_size ? Math.round(price.unit_price_minor * variant.pack_size) : null,
        observedOn: price.observed_on,
        source: price.source,
        stale: price.stale,
      });
    }
  }
  return rows.sort((a, b) => {
    if ((a.currency === null) !== (b.currency === null)) {
      return a.currency === null ? 1 : -1;
    }
    return (
      (a.currency ?? "").localeCompare(b.currency ?? "") ||
      (a.displayUnitPrice ?? 0) - (b.displayUnitPrice ?? 0) ||
      a.brand.localeCompare(b.brand)
    );
  });
}

export type HistorySeries = {
  key: string;
  brand: string;
  packLabel: string | null;
  storeName: string;
  currency: string;
  points: { month: string; value: number; observations: number }[];
};

/** Monthly average unit price series, one per (variant, store) that has history. */
export function historySeries(prices: ProductPrices): HistorySeries[] {
  const series: HistorySeries[] = [];
  for (const variant of prices.variants) {
    for (const entry of variant.history) {
      if (entry.points.length === 0) {
        continue;
      }
      series.push({
        key: `${variant.variant_id}:${entry.store_id}`,
        brand: variant.brand,
        packLabel: variant.pack_label,
        storeName: entry.store_name,
        currency: entry.currency,
        points: [...entry.points]
          .sort((a, b) => a.month.localeCompare(b.month))
          .map((point) => ({ month: point.month, value: point.display_unit_price, observations: point.observations })),
      });
    }
  }
  return series.sort((a, b) => a.brand.localeCompare(b.brand) || a.storeName.localeCompare(b.storeName));
}

/** The series to show first: the one of the cheapest priced row, else the first. */
export function defaultSeriesKey(series: HistorySeries[], rows: VariantRow[]): string | undefined {
  const cheapest = rows.find((row) => row.storeId !== null && series.some((s) => s.key === row.key));
  return cheapest?.key ?? series[0]?.key;
}

/** A "nice" y axis around the values: ticks at 1, 2, 2.5, 5 steps. */
export function priceAxis(values: number[], count = 4): { min: number; max: number; ticks: number[] } {
  const low = Math.min(...values);
  const high = Math.max(...values);
  const span = high - low || Math.max(high * 0.2, 1);
  const rawStep = (span * 1.3) / count;
  const magnitude = 10 ** Math.floor(Math.log10(rawStep));
  const step = [1, 2, 2.5, 5, 10].map((m) => m * magnitude).find((s) => s >= rawStep) ?? magnitude * 10;
  const min = Math.max(0, Math.floor((low - span * 0.1) / step) * step);
  const ticks: number[] = [];
  for (let value = min; ; value += step) {
    ticks.push(Number(value.toFixed(6)));
    if (value >= high + span * 0.1) {
      break;
    }
  }
  return { min, max: ticks[ticks.length - 1], ticks };
}

// ---- Form parsing ----------------------------------------------------------

export type FieldError = "required" | "invalidNumber" | "invalidAmount" | "invalidCurrency";
export type Built<T> = { body: T } | { errors: Record<string, FieldError> };

function field(formData: FormData, key: string): string {
  const value = formData.get(key);
  return typeof value === "string" ? value.trim() : "";
}

export type ProductBody = { name: string; compare_unit: CompareUnit; display_multiplier: number; group_id: string };

/** Product form: name, group (may be empty), unit and per-dozen style multiplier. */
export function buildProduct(formData: FormData): Built<ProductBody> {
  const errors: Record<string, FieldError> = {};
  const name = field(formData, "name");
  if (!name) {
    errors.name = "required";
  }
  const unit = field(formData, "compare_unit");
  if (!COMPARE_UNITS.includes(unit as CompareUnit)) {
    errors.compare_unit = "required";
  }
  let multiplier = 1;
  if (field(formData, "display_multiplier")) {
    const parsed = parsePositiveNumber(field(formData, "display_multiplier"));
    if (parsed === null || !Number.isInteger(parsed)) {
      errors.display_multiplier = "invalidNumber";
    } else {
      multiplier = parsed;
    }
  }
  if (Object.keys(errors).length > 0) {
    return { errors };
  }
  return { body: { name, compare_unit: unit as CompareUnit, display_multiplier: multiplier, group_id: field(formData, "group_id") } };
}

export type VariantBody = { brand: string; pack_label: string; pack_size?: number; clear_pack_size: boolean; store_id: string };

/** Variant form: brand, pack label, pack size (empty = loose) and the chain of a store brand. */
export function buildVariant(formData: FormData): Built<VariantBody> {
  const errors: Record<string, FieldError> = {};
  const brand = field(formData, "brand");
  if (!brand) {
    errors.brand = "required";
  }
  let packSize: number | undefined;
  if (field(formData, "pack_size")) {
    const parsed = parsePositiveNumber(field(formData, "pack_size"));
    if (parsed === null) {
      errors.pack_size = "invalidNumber";
    } else {
      packSize = parsed;
    }
  }
  if (Object.keys(errors).length > 0) {
    return { errors };
  }
  return {
    body: { brand, pack_label: field(formData, "pack_label"), pack_size: packSize, clear_pack_size: packSize === undefined, store_id: field(formData, "store_id") },
  };
}

export type PriceCheckBody = components["schemas"]["CreatePriceCheckInput"];

/** Price check form: the pack price seen at a store, in the typed currency. */
export function buildPriceCheck(formData: FormData): Built<PriceCheckBody> {
  const errors: Record<string, FieldError> = {};
  const variantId = field(formData, "variant_id");
  if (!variantId) {
    errors.variant_id = "required";
  }
  const storeId = field(formData, "store_id");
  if (!storeId) {
    errors.store_id = "required";
  }
  const currency = field(formData, "currency").toUpperCase();
  if (!/^[A-Z]{3}$/.test(currency)) {
    errors.currency = "invalidCurrency";
  }
  let price = 0;
  if (!field(formData, "price")) {
    errors.price = "required";
  } else if (!errors.currency) {
    const parsed = parseAmount(field(formData, "price"), minorUnitsOf(currency));
    if (parsed === null) {
      errors.price = "invalidAmount";
    } else {
      price = parsed;
    }
  }
  if (Object.keys(errors).length > 0) {
    return { errors };
  }
  return {
    body: {
      variant_id: variantId,
      store_id: storeId,
      currency,
      price_minor: price,
      observed_on: field(formData, "observed_on") || undefined,
      note: field(formData, "note") || undefined,
    },
  };
}

/** Name-only forms (stores, groups); groups also carry the food flag. */
export function buildName(formData: FormData): Built<{ name: string; is_food: boolean }> {
  const name = field(formData, "name");
  if (!name) {
    return { errors: { name: "required" } };
  }
  return { body: { name, is_food: field(formData, "is_food") === "on" } };
}
