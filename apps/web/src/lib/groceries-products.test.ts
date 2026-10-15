import { describe, expect, it } from "vitest";

import {
  buildName,
  buildPriceCheck,
  buildProduct,
  buildVariant,
  defaultSeriesKey,
  groupProducts,
  historySeries,
  parseProductsQuery,
  priceAxis,
  variantRows,
  type CatalogProduct,
  type ProductGroup,
  type ProductPrices,
} from "./groceries-products";

function form(values: Record<string, string>): FormData {
  const data = new FormData();
  for (const [key, value] of Object.entries(values)) data.set(key, value);
  return data;
}

const product = (id: string, name: string, groupId: string | null): CatalogProduct => ({
  id,
  name,
  group_id: groupId,
  group_name: null,
  group_color: null,
  compare_unit: "kg",
  display_multiplier: 1,
  archived: false,
  variant_count: 1,
  created_at: "",
  updated_at: "",
});
const group = (id: string, name: string, position: number): ProductGroup => ({ id, name, position, color: null, is_food: true, created_at: "", updated_at: "" });

describe("groupProducts", () => {
  const groups = [group("g2", "Dairy", 2), group("g1", "Produce", 1), group("g3", "Empty", 3)];
  const products = [product("a", "Milk", "g2"), product("b", "Bananas", "g1"), product("c", "Apples", "g1"), product("d", "Bin bags", null)];

  it("orders sections by group position, products by name, ungrouped last, dropping empty groups", () => {
    const sections = groupProducts(products, groups);
    expect(sections.map((s) => s.name)).toEqual(["Produce", "Dairy", null]);
    expect(sections[0].products.map((p) => p.name)).toEqual(["Apples", "Bananas"]);
  });

  it("filters by a case-insensitive search", () => {
    const sections = groupProducts(products, groups, "  APP ");
    expect(sections).toHaveLength(1);
    expect(sections[0].products.map((p) => p.name)).toEqual(["Apples"]);
  });

  it("treats a product of an unknown group as ungrouped", () => {
    expect(groupProducts([product("x", "Rice", "gone")], groups)[0].name).toBeNull();
  });
});

describe("parseProductsQuery", () => {
  it("trims the search and drops empty values", () => {
    expect(parseProductsQuery({ q: " milk " })).toEqual({ q: "milk" });
    expect(parseProductsQuery({ q: ["a", "b"] })).toEqual({ q: "a" });
    expect(parseProductsQuery({ q: "  " })).toEqual({ q: undefined });
  });
});

const prices: ProductPrices = {
  product_id: "p1",
  name: "Milk",
  compare_unit: "l",
  display_multiplier: 1,
  variants: [
    {
      variant_id: "v1",
      brand: "Hacendado",
      pack_label: "6×1 L",
      pack_size: 6,
      store_id: "s1",
      latest: [{ store_id: "s1", store_name: "Mercadona", currency: "EUR", unit_price_minor: 85, display_unit_price: 85, observed_on: "2026-10-01", source: "receipt", stale: false }],
      history: [
        {
          store_id: "s1",
          store_name: "Mercadona",
          currency: "EUR",
          points: [
            { month: "2026-10", unit_price_minor: 85, display_unit_price: 85, observations: 2 },
            { month: "2026-08", unit_price_minor: 80, display_unit_price: 80, observations: 1 },
          ],
        },
      ],
    },
    {
      variant_id: "v2",
      brand: "Pascual",
      pack_label: "1 L",
      pack_size: 1,
      store_id: null,
      latest: [
        { store_id: "s1", store_name: "Mercadona", currency: "EUR", unit_price_minor: 110, display_unit_price: 110, observed_on: "2026-05-01", source: "price_check", stale: true },
        { store_id: "s2", store_name: "Lidl", currency: "EUR", unit_price_minor: 95, display_unit_price: 95, observed_on: "2026-09-20", source: "receipt", stale: false },
      ],
      history: [],
    },
    { variant_id: "v3", brand: "Generic", pack_label: null, pack_size: null, store_id: null, latest: [], history: [] },
  ],
};

describe("variantRows", () => {
  const rows = variantRows(prices);

  it("lists one row per variant and store, cheapest unit price first, unpriced last", () => {
    expect(rows.map((r) => `${r.brand}@${r.storeName}`)).toEqual(["Hacendado@Mercadona", "Pascual@Lidl", "Pascual@Mercadona", "Generic@null"]);
  });

  it("flags store brands and stale prices", () => {
    expect(rows[0].storeBrand).toBe(true);
    expect(rows[0].brandStoreId).toBe("s1");
    expect(rows[2]).toMatchObject({ stale: true, source: "price_check", storeBrand: false });
  });

  it("derives the pack price from the unit price and pack size", () => {
    expect(rows[0].packPrice).toBe(510);
    expect(rows[1].packPrice).toBe(95);
    expect(rows[3].packPrice).toBeNull();
  });

  it("never mixes currencies in the ordering", () => {
    const mixed: ProductPrices = {
      ...prices,
      variants: [
        { ...prices.variants[1], latest: [{ ...prices.variants[1].latest[1], currency: "USD", display_unit_price: 1 }, prices.variants[1].latest[0]] },
      ],
    };
    expect(variantRows(mixed).map((r) => r.currency)).toEqual(["EUR", "USD"]);
  });
});

describe("historySeries", () => {
  it("builds one chronological series per variant and store with history", () => {
    const series = historySeries(prices);
    expect(series).toHaveLength(1);
    expect(series[0]).toMatchObject({ key: "v1:s1", brand: "Hacendado", storeName: "Mercadona", currency: "EUR" });
    expect(series[0].points.map((p) => p.month)).toEqual(["2026-08", "2026-10"]);
  });

  it("defaults to the series of the cheapest row, else the first", () => {
    const series = historySeries(prices);
    expect(defaultSeriesKey(series, variantRows(prices))).toBe("v1:s1");
    expect(defaultSeriesKey(series, [])).toBe("v1:s1");
    expect(defaultSeriesKey([], [])).toBeUndefined();
  });
});

describe("priceAxis", () => {
  it("covers the values with ascending ticks", () => {
    const axis = priceAxis([80, 85, 92]);
    expect(axis.min).toBeLessThanOrEqual(80);
    expect(axis.max).toBeGreaterThanOrEqual(92);
    expect(axis.ticks).toEqual([...axis.ticks].sort((a, b) => a - b));
    expect(axis.ticks[axis.ticks.length - 1]).toBe(axis.max);
  });

  it("handles a flat series", () => {
    const axis = priceAxis([100, 100]);
    expect(axis.min).toBeLessThan(100);
    expect(axis.max).toBeGreaterThan(100);
  });
});

describe("buildProduct", () => {
  it("builds a product with defaults", () => {
    expect(buildProduct(form({ name: " Milk ", compare_unit: "l", group_id: "" }))).toEqual({
      body: { name: "Milk", compare_unit: "l", display_multiplier: 1, group_id: "" },
    });
  });

  it("accepts a per-dozen multiplier", () => {
    expect(buildProduct(form({ name: "Eggs", compare_unit: "unit", display_multiplier: "12", group_id: "g1" }))).toMatchObject({ body: { display_multiplier: 12, group_id: "g1" } });
  });

  it("rejects missing name, bad unit and fractional multipliers", () => {
    expect(buildProduct(form({ name: " ", compare_unit: "m", display_multiplier: "1.5" }))).toEqual({
      errors: { name: "required", compare_unit: "required", display_multiplier: "invalidNumber" },
    });
  });
});

describe("buildVariant", () => {
  it("builds a packed variant", () => {
    expect(buildVariant(form({ brand: "Hacendado", pack_label: "6×1 L", pack_size: "6,5", store_id: "s1" }))).toEqual({
      body: { brand: "Hacendado", pack_label: "6×1 L", pack_size: 6.5, clear_pack_size: false, store_id: "s1" },
    });
  });

  it("clears the pack size of a loose variant", () => {
    expect(buildVariant(form({ brand: "Generic", pack_size: "" }))).toMatchObject({ body: { pack_size: undefined, clear_pack_size: true, store_id: "" } });
  });

  it("requires a brand and a valid pack size", () => {
    expect(buildVariant(form({ brand: "", pack_size: "abc" }))).toEqual({ errors: { brand: "required", pack_size: "invalidNumber" } });
  });
});

describe("buildPriceCheck", () => {
  const valid = { variant_id: "v1", store_id: "s1", price: "1,29", currency: "eur", observed_on: "2026-10-10", note: " offer " };

  it("converts the price to minor units of the currency", () => {
    expect(buildPriceCheck(form(valid))).toEqual({
      body: { variant_id: "v1", store_id: "s1", currency: "EUR", price_minor: 129, observed_on: "2026-10-10", note: "offer" },
    });
    expect(buildPriceCheck(form({ ...valid, currency: "JPY", price: "129" }))).toMatchObject({ body: { price_minor: 129 } });
  });

  it("leaves optional fields out", () => {
    expect(buildPriceCheck(form({ ...valid, observed_on: "", note: "" }))).toMatchObject({ body: { observed_on: undefined, note: undefined } });
  });

  it("reports missing and invalid values", () => {
    expect(buildPriceCheck(form({ currency: "EU", price: "1" }))).toEqual({
      errors: { variant_id: "required", store_id: "required", currency: "invalidCurrency" },
    });
    expect(buildPriceCheck(form({ ...valid, price: "0" }))).toEqual({ errors: { price: "invalidAmount" } });
    expect(buildPriceCheck(form({ ...valid, price: "" }))).toEqual({ errors: { price: "required" } });
    expect(buildPriceCheck(form({ ...valid, price: "1.234" }))).toEqual({ errors: { price: "invalidAmount" } });
  });
});

describe("buildName", () => {
  it("trims the name and reads the food checkbox", () => {
    expect(buildName(form({ name: " Pets ", is_food: "on" }))).toEqual({ body: { name: "Pets", is_food: true } });
    expect(buildName(form({ name: "Pets" }))).toEqual({ body: { name: "Pets", is_food: false } });
  });

  it("requires a name", () => {
    expect(buildName(form({ name: "" }))).toEqual({ errors: { name: "required" } });
  });
});
