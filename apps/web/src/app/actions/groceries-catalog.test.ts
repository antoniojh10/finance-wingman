import "@/test/server-mocks";

import { beforeEach, describe, expect, it } from "vitest";

import { cookieJar, mockApi } from "@/test/server-mocks";

import {
  createGroup,
  createPriceCheck,
  createProduct,
  createStore,
  createVariant,
  deleteVariant,
  renameGroup,
  renameStore,
  setProductArchived,
  setStoreArchived,
  updateProduct,
  updateVariant,
} from "./groceries-catalog";

function form(values: Record<string, string>): FormData {
  const data = new FormData();
  for (const [key, value] of Object.entries(values)) data.set(key, value);
  return data;
}

const initial = { ok: false };

beforeEach(() => {
  cookieJar.clear();
  cookieJar.set("fw_session", { value: "tok" });
});

describe("products", () => {
  it("creates a product, leaving an empty group out", async () => {
    const requests = mockApi([{ method: "POST", path: "/api/v1/groceries/products", status: 201, body: {} }]);
    const result = await createProduct(initial, form({ name: "Milk", compare_unit: "l", display_multiplier: "1", group_id: "" }));
    expect(result.ok).toBe(true);
    expect(requests[0].body).toEqual({ name: "Milk", compare_unit: "l", display_multiplier: 1 });
  });

  it("validates before calling the API", async () => {
    const requests = mockApi([]);
    const result = await createProduct(initial, form({ name: "", compare_unit: "l" }));
    expect(result).toMatchObject({ ok: false, fieldErrors: { name: "This field is required." } });
    expect(requests).toHaveLength(0);
  });

  it("explains a duplicate name", async () => {
    mockApi([{ method: "POST", path: "/api/v1/groceries/products", status: 409, body: { status: 409, detail: "dup" } }]);
    const result = await createProduct(initial, form({ name: "Milk", compare_unit: "l" }));
    expect(result).toMatchObject({ ok: false, message: "A product with this name already exists." });
  });

  it("updates a product, sending an empty group to clear it", async () => {
    const requests = mockApi([{ method: "PATCH", path: "/api/v1/groceries/products/p1", status: 200, body: {} }]);
    const result = await updateProduct(initial, form({ id: "p1", name: "Eggs", compare_unit: "unit", display_multiplier: "12", group_id: "" }));
    expect(result.ok).toBe(true);
    expect(requests[0].body).toEqual({ name: "Eggs", compare_unit: "unit", display_multiplier: 12, group_id: "" });
  });

  it("archives and restores", async () => {
    const requests = mockApi([{ method: "PATCH", path: "/api/v1/groceries/products/p1", status: 200, body: {} }]);
    expect((await setProductArchived("p1", true)).ok).toBe(true);
    expect(requests[0].body).toEqual({ archived: true });
  });

  it("reports a product that no longer exists", async () => {
    mockApi([{ method: "PATCH", path: "/api/v1/groceries/products/p1", status: 404, body: { status: 404 } }]);
    expect(await setProductArchived("p1", true)).toMatchObject({ ok: false, message: "This item no longer exists." });
  });
});

describe("variants", () => {
  it("adds a variant to a product", async () => {
    const requests = mockApi([{ method: "POST", path: "/api/v1/groceries/products/p1/variants", status: 201, body: {} }]);
    const result = await createVariant(initial, form({ product_id: "p1", brand: "Hacendado", pack_label: "1 L", pack_size: "1", store_id: "s1" }));
    expect(result.ok).toBe(true);
    expect(requests[0].body).toEqual({ brand: "Hacendado", pack_label: "1 L", pack_size: 1, store_id: "s1" });
  });

  it("rejects an invalid pack size", async () => {
    mockApi([]);
    const result = await createVariant(initial, form({ product_id: "p1", brand: "X", pack_size: "-1" }));
    expect(result).toMatchObject({ ok: false, fieldErrors: { pack_size: "Enter a positive number." } });
  });

  it("updates a loose variant clearing the pack size", async () => {
    const requests = mockApi([{ method: "PATCH", path: "/api/v1/groceries/variants/v1", status: 200, body: {} }]);
    const result = await updateVariant(initial, form({ id: "v1", brand: "Generic", pack_label: "", pack_size: "", store_id: "" }));
    expect(result.ok).toBe(true);
    expect(requests[0].body).toEqual({ brand: "Generic", pack_label: "", clear_pack_size: true, store_id: "" });
  });

  it("explains a duplicate variant", async () => {
    mockApi([{ method: "PATCH", path: "/api/v1/groceries/variants/v1", status: 409, body: { status: 409 } }]);
    const result = await updateVariant(initial, form({ id: "v1", brand: "Generic" }));
    expect(result).toMatchObject({ ok: false, message: "This brand and pack already exist for the product." });
  });

  it("deletes a variant", async () => {
    const requests = mockApi([{ method: "DELETE", path: "/api/v1/groceries/variants/v1", status: 204 }]);
    expect((await deleteVariant("v1")).ok).toBe(true);
    expect(requests[0].method).toBe("DELETE");
  });

  it("fails to delete an unknown variant", async () => {
    mockApi([{ method: "DELETE", path: "/api/v1/groceries/variants/v1", status: 404, body: { status: 404 } }]);
    expect(await deleteVariant("v1")).toMatchObject({ ok: false, message: "This item no longer exists." });
  });
});

describe("createPriceCheck", () => {
  const values = { variant_id: "v1", store_id: "s1", price: "1.29", currency: "EUR", observed_on: "2026-10-10", note: "" };

  it("sends the price in minor units", async () => {
    const requests = mockApi([{ method: "POST", path: "/api/v1/groceries/price-checks", status: 201, body: {} }]);
    expect((await createPriceCheck(initial, form(values))).ok).toBe(true);
    expect(requests[0].body).toEqual({ variant_id: "v1", store_id: "s1", currency: "EUR", price_minor: 129, observed_on: "2026-10-10" });
  });

  it("reports invalid fields without calling the API", async () => {
    const requests = mockApi([]);
    const result = await createPriceCheck(initial, form({ ...values, price: "x", currency: "E" }));
    expect(result).toMatchObject({ ok: false, fieldErrors: { currency: "Enter a 3-letter currency code." } });
    expect(requests).toHaveLength(0);
  });

  it("shows API validation errors", async () => {
    mockApi([{ method: "POST", path: "/api/v1/groceries/price-checks", status: 422, body: { status: 422, detail: "pack size" } }]);
    expect(await createPriceCheck(initial, form(values))).toMatchObject({ ok: false, message: "Please check the highlighted fields." });
  });
});

describe("stores and groups", () => {
  it("creates and renames a store", async () => {
    const requests = mockApi([
      { method: "POST", path: "/api/v1/groceries/stores", status: 201, body: {} },
      { method: "PATCH", path: "/api/v1/groceries/stores/s1", status: 200, body: {} },
    ]);
    expect((await createStore(initial, form({ name: "Lidl" }))).ok).toBe(true);
    expect((await renameStore(initial, form({ id: "s1", name: "Lidl Spain" }))).ok).toBe(true);
    expect(requests.map((r) => r.body)).toEqual([{ name: "Lidl" }, { name: "Lidl Spain" }]);
  });

  it("requires a store name", async () => {
    mockApi([]);
    expect(await createStore(initial, form({ name: " " }))).toMatchObject({ ok: false, fieldErrors: { name: "This field is required." } });
  });

  it("explains a duplicate store", async () => {
    mockApi([{ method: "POST", path: "/api/v1/groceries/stores", status: 409, body: { status: 409 } }]);
    expect(await createStore(initial, form({ name: "Lidl" }))).toMatchObject({ ok: false, message: "A store with this name already exists." });
  });

  it("archives a store", async () => {
    const requests = mockApi([{ method: "PATCH", path: "/api/v1/groceries/stores/s1", status: 200, body: {} }]);
    expect((await setStoreArchived("s1", true)).ok).toBe(true);
    expect(requests[0].body).toEqual({ archived: true });
  });

  it("creates and renames a group", async () => {
    const requests = mockApi([
      { method: "POST", path: "/api/v1/groceries/groups", status: 201, body: {} },
      { method: "PATCH", path: "/api/v1/groceries/groups/g1", status: 200, body: {} },
    ]);
    expect((await createGroup(initial, form({ name: "Pets", is_food: "on" }))).ok).toBe(true);
    expect((await renameGroup(initial, form({ id: "g1", name: "Pet food" }))).ok).toBe(true);
    expect(requests.map((r) => r.body)).toEqual([{ name: "Pets", is_food: true }, { name: "Pet food" }]);
  });

  it("explains a duplicate group", async () => {
    mockApi([{ method: "PATCH", path: "/api/v1/groceries/groups/g1", status: 409, body: { status: 409 } }]);
    expect(await renameGroup(initial, form({ id: "g1", name: "Dairy" }))).toMatchObject({ ok: false, message: "A group with this name already exists." });
  });
});
