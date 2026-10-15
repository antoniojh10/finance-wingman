import type { components } from "@/lib/api/schema";
import { parseMonth } from "@/lib/dates";

// View models and URL helpers for the groceries section. Pure, so the pages
// and the components can share them and tests do not need the API.

export type Receipt = components["schemas"]["Receipt"];
export type ReceiptSummary = components["schemas"]["ReceiptSummary"];
export type ReceiptLine = components["schemas"]["ReceiptLine"];
export type ReceiptWarning = components["schemas"]["Warning"];

/**
 * The sub-navigation of the groceries section. Add a tab here (and its
 * "groceries.tabs.<key>" message) to extend it.
 */
export const GROCERIES_TABS = [
  { key: "summary", href: "/groceries" },
  { key: "receipts", href: "/groceries/receipts" },
  { key: "products", href: "/groceries/products" },
] as const;

export type GroceriesTabKey = (typeof GROCERIES_TABS)[number]["key"];

/** The tab a pathname belongs to: the longest tab link that prefixes it. */
export function activeGroceriesTab(pathname: string): GroceriesTabKey | undefined {
  const match = [...GROCERIES_TABS]
    .filter((tab) => pathname === tab.href || pathname.startsWith(`${tab.href}/`))
    .sort((a, b) => b.href.length - a.href.length)[0];
  return match?.key;
}

export const ALL_MONTHS = "all";

export type ReceiptsQuery = {
  /** YYYY-MM, or "all" for every month; undefined means the current month. */
  month?: string;
  storeId?: string;
};

const uuidPattern = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

function first(value: string | string[] | undefined): string | undefined {
  return (Array.isArray(value) ? value[0] : value)?.trim() || undefined;
}

/** Sanitizes the receipts search params, dropping invalid values. */
export function parseReceiptsQuery(params: Record<string, string | string[] | undefined>): ReceiptsQuery {
  const raw = first(params.month);
  const month = raw === ALL_MONTHS ? ALL_MONTHS : parseMonth(raw, "");
  const store = first(params.store);
  return { month: month || undefined, storeId: store && uuidPattern.test(store) ? store : undefined };
}

/** Receipts URL for a query with some values changed; empty values are left out. */
export function receiptsHref(query: ReceiptsQuery, changes: Partial<ReceiptsQuery> = {}): string {
  const next = { ...query, ...changes };
  const params = new URLSearchParams();
  if (next.month) {
    params.set("month", next.month);
  }
  if (next.storeId) {
    params.set("store", next.storeId);
  }
  const qs = params.toString();
  return qs ? `/groceries/receipts?${qs}` : "/groceries/receipts";
}

/** Decimal digits of a currency (2 for EUR, 0 for JPY), used to format minor units. */
export function minorUnitsOf(currency: string): number {
  try {
    return new Intl.NumberFormat("en", { style: "currency", currency }).resolvedOptions().maximumFractionDigits ?? 2;
  } catch {
    return 2;
  }
}

/** How a receipt line is mapped: to a product variant, or still waiting. */
export function lineStatus(line: Pick<ReceiptLine, "variant_id">): "assigned" | "unassigned" {
  return line.variant_id ? "assigned" : "unassigned";
}

/** "Hacendado · 1 L" style label of the variant a line is assigned to. */
export function variantLabel(line: Pick<ReceiptLine, "brand" | "pack_label">): string {
  return [line.brand, line.pack_label].filter(Boolean).join(" · ");
}

/** Quantity as shown on the receipt: whole numbers without decimals, up to 3 otherwise. */
export function formatQuantity(quantity: number): string {
  return Number.isInteger(quantity) ? String(quantity) : String(Number(quantity.toFixed(3)));
}

/** Parses a decimal typed by the user ("1,5" or "1.5"); null when not a positive number. */
export function parsePositiveNumber(input: string): number | null {
  const value = input.trim().replace(",", ".");
  if (!/^\d+(\.\d+)?$/.test(value)) {
    return null;
  }
  const parsed = Number(value);
  return parsed > 0 ? parsed : null;
}

type CompareUnit = "kg" | "l" | "unit";
type CatalogProduct = { id: string; name: string; compare_unit: CompareUnit };
type CatalogVariant = { id: string; product_id: string; brand: string; pack_size: number | null; pack_label: string | null };

/** A product with its variants, ready for the line assignment form. */
export type CatalogEntry = {
  id: string;
  name: string;
  compareUnit: CompareUnit;
  variants: { id: string; label: string }[];
};

/** Joins products and variants into the choices of the assignment form. */
export function buildCatalog(products: CatalogProduct[], variants: CatalogVariant[]): CatalogEntry[] {
  return products
    .map((product) => ({
      id: product.id,
      name: product.name,
      compareUnit: product.compare_unit,
      variants: variants
        .filter((variant) => variant.product_id === product.id)
        .map((variant) => ({
          id: variant.id,
          label: [variant.brand, variant.pack_label ?? (variant.pack_size ? `${variant.pack_size} ${product.compare_unit}` : null)]
            .filter(Boolean)
            .join(" · "),
        })),
    }))
    .sort((a, b) => a.name.localeCompare(b.name));
}

export type AssignmentBody = components["schemas"]["AssignLineInput"];

export type AssignmentResult = { body: AssignmentBody } | { errors: Record<string, "required" | "invalidNumber"> };

/**
 * Builds the assignment of one line from the form: either an existing
 * variant, or a new product/variant with its brand and pack.
 */
export function buildAssignment(formData: FormData): AssignmentResult {
  const get = (key: string) => {
    const value = formData.get(key);
    return typeof value === "string" ? value.trim() : "";
  };
  const lineId = get("line_id");

  if (get("mode") !== "new") {
    const variantId = get("variant_id");
    return variantId ? { body: { line_id: lineId, variant_id: variantId } } : { errors: { variant_id: "required" } };
  }

  const errors: Record<string, "required" | "invalidNumber"> = {};
  const product = get("product");
  if (!product) {
    errors.product = "required";
  }
  const compareUnit = get("compare_unit");
  if (!["kg", "l", "unit"].includes(compareUnit)) {
    errors.compare_unit = "required";
  }
  let packSize: number | undefined;
  if (get("pack_size")) {
    const parsed = parsePositiveNumber(get("pack_size"));
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
    body: {
      line_id: lineId,
      product,
      brand: get("brand") || undefined,
      pack_size: packSize,
      pack_label: get("pack_label") || undefined,
      compare_unit: compareUnit as CompareUnit,
      group: get("group") || undefined,
    },
  };
}

/** The transactions list narrowed to the day (and expenses) of a receipt. */
export function transactionHref(purchasedOn: string): string {
  return `/transactions?${new URLSearchParams({ type: "expense", from: purchasedOn, to: purchasedOn })}`;
}
