import { describe, expect, it } from "vitest";

import {
  activeGroceriesTab,
  buildAssignment,
  buildCatalog,
  formatQuantity,
  lineStatus,
  minorUnitsOf,
  parsePositiveNumber,
  parseReceiptsQuery,
  receiptsHref,
  transactionHref,
  variantLabel,
} from "./groceries";

const store = "6f1c2a3e-1b2c-4d5e-8f90-123456789abc";

function form(values: Record<string, string>): FormData {
  const data = new FormData();
  for (const [key, value] of Object.entries(values)) data.set(key, value);
  return data;
}

describe("activeGroceriesTab", () => {
  it("matches the summary only on its own page", () => {
    expect(activeGroceriesTab("/groceries")).toBe("summary");
    expect(activeGroceriesTab("/groceries/receipts")).toBe("receipts");
    expect(activeGroceriesTab("/groceries/receipts/abc")).toBe("receipts");
    expect(activeGroceriesTab("/transactions")).toBeUndefined();
  });
});

describe("parseReceiptsQuery", () => {
  it("keeps valid months, the all marker and store ids", () => {
    expect(parseReceiptsQuery({ month: "2026-10", store })).toEqual({ month: "2026-10", storeId: store });
    expect(parseReceiptsQuery({ month: "all" })).toEqual({ month: "all", storeId: undefined });
  });

  it("drops invalid values", () => {
    expect(parseReceiptsQuery({ month: "october", store: "nope" })).toEqual({ month: undefined, storeId: undefined });
    expect(parseReceiptsQuery({})).toEqual({ month: undefined, storeId: undefined });
  });
});

describe("receiptsHref", () => {
  it("leaves empty values out and applies changes", () => {
    expect(receiptsHref({})).toBe("/groceries/receipts");
    expect(receiptsHref({ month: "2026-10", storeId: store })).toBe(`/groceries/receipts?month=2026-10&store=${store}`);
    expect(receiptsHref({ month: "2026-10", storeId: store }, { storeId: undefined })).toBe("/groceries/receipts?month=2026-10");
  });
});

describe("transactionHref", () => {
  it("narrows the transactions to the purchase day", () => {
    expect(transactionHref("2026-09-27")).toBe("/transactions?type=expense&from=2026-09-27&to=2026-09-27");
  });
});

describe("helpers", () => {
  it("knows the decimals of a currency", () => {
    expect(minorUnitsOf("EUR")).toBe(2);
    expect(minorUnitsOf("JPY")).toBe(0);
    expect(minorUnitsOf("???")).toBe(2);
  });

  it("tells assigned lines from unassigned ones", () => {
    expect(lineStatus({ variant_id: "v1" })).toBe("assigned");
    expect(lineStatus({ variant_id: null })).toBe("unassigned");
  });

  it("labels a variant with brand and pack", () => {
    expect(variantLabel({ brand: "Hacendado", pack_label: "1 L" })).toBe("Hacendado · 1 L");
    expect(variantLabel({ brand: "Generic", pack_label: null })).toBe("Generic");
    expect(variantLabel({ brand: null, pack_label: null })).toBe("");
  });

  it("formats quantities", () => {
    expect(formatQuantity(2)).toBe("2");
    expect(formatQuantity(0.75)).toBe("0.75");
    expect(formatQuantity(1.2340001)).toBe("1.234");
  });

  it("parses positive decimals with comma or dot", () => {
    expect(parsePositiveNumber("1,5")).toBe(1.5);
    expect(parsePositiveNumber(" 0.125 ")).toBe(0.125);
    expect(parsePositiveNumber("0")).toBeNull();
    expect(parsePositiveNumber("-1")).toBeNull();
    expect(parsePositiveNumber("abc")).toBeNull();
  });
});

describe("buildCatalog", () => {
  it("groups variants under their product, sorted by name", () => {
    const catalog = buildCatalog(
      [
        { id: "p2", name: "Yogurt", compare_unit: "unit" },
        { id: "p1", name: "Milk", compare_unit: "l" },
      ],
      [
        { id: "v1", product_id: "p1", brand: "Hacendado", pack_size: 1, pack_label: "1 L" },
        { id: "v2", product_id: "p1", brand: "Generic", pack_size: 0.5, pack_label: null },
        { id: "v3", product_id: "p2", brand: "Danone", pack_size: null, pack_label: null },
      ],
    );
    expect(catalog.map((entry) => entry.name)).toEqual(["Milk", "Yogurt"]);
    expect(catalog[0].variants).toEqual([
      { id: "v1", label: "Hacendado · 1 L" },
      { id: "v2", label: "Generic · 0.5 l" },
    ]);
    expect(catalog[1].variants).toEqual([{ id: "v3", label: "Danone" }]);
  });
});

describe("buildAssignment", () => {
  it("assigns an existing variant", () => {
    expect(buildAssignment(form({ line_id: "l1", mode: "existing", variant_id: "v1" }))).toEqual({ body: { line_id: "l1", variant_id: "v1" } });
  });

  it("requires a variant in existing mode", () => {
    expect(buildAssignment(form({ line_id: "l1", mode: "existing", variant_id: "" }))).toEqual({ errors: { variant_id: "required" } });
  });

  it("builds a new product with its pack", () => {
    const result = buildAssignment(
      form({ line_id: "l1", mode: "new", product: " Whole milk ", brand: "Hacendado", pack_size: "1,5", pack_label: "1.5 L", compare_unit: "l", group: "Dairy & eggs" }),
    );
    expect(result).toEqual({
      body: { line_id: "l1", product: "Whole milk", brand: "Hacendado", pack_size: 1.5, pack_label: "1.5 L", compare_unit: "l", group: "Dairy & eggs" },
    });
  });

  it("leaves optional fields out", () => {
    const result = buildAssignment(form({ line_id: "l1", mode: "new", product: "Apples", compare_unit: "kg" }));
    expect(result).toEqual({
      body: { line_id: "l1", product: "Apples", brand: undefined, pack_size: undefined, pack_label: undefined, compare_unit: "kg", group: undefined },
    });
  });

  it("reports missing and invalid fields", () => {
    expect(buildAssignment(form({ line_id: "l1", mode: "new", product: "", compare_unit: "", pack_size: "x" }))).toEqual({
      errors: { product: "required", compare_unit: "required", pack_size: "invalidNumber" },
    });
  });
});
