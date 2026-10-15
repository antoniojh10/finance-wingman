import "@/test/server-mocks";

import { beforeEach, describe, expect, it } from "vitest";

import { cookieJar, mockApi } from "@/test/server-mocks";

import { assignReceiptLine, deleteReceipt, unassignReceiptLine } from "./groceries";

function form(values: Record<string, string>): FormData {
  const data = new FormData();
  for (const [key, value] of Object.entries(values)) data.set(key, value);
  return data;
}

beforeEach(() => {
  cookieJar.clear();
  cookieJar.set("fw_session", { value: "tok" });
});

describe("assignReceiptLine", () => {
  it("assigns an existing variant", async () => {
    const requests = mockApi([{ method: "POST", path: "/api/v1/groceries/receipts/r1/assign", status: 200, body: {} }]);
    const result = await assignReceiptLine({ ok: false }, form({ receipt_id: "r1", line_id: "l1", mode: "existing", variant_id: "v1" }));
    expect(result.ok).toBe(true);
    expect(requests[0]).toMatchObject({ method: "POST", body: { assignments: [{ line_id: "l1", variant_id: "v1" }] } });
  });

  it("creates a product and variant", async () => {
    const requests = mockApi([{ method: "POST", path: "/api/v1/groceries/receipts/r1/assign", status: 200, body: {} }]);
    const result = await assignReceiptLine(
      { ok: false },
      form({ receipt_id: "r1", line_id: "l1", mode: "new", product: "Milk", brand: "Hacendado", pack_size: "1", compare_unit: "l" }),
    );
    expect(result.ok).toBe(true);
    expect(requests[0].body).toEqual({
      assignments: [{ line_id: "l1", product: "Milk", brand: "Hacendado", pack_size: 1, compare_unit: "l" }],
    });
  });

  it("reports invalid fields without calling the API", async () => {
    const requests = mockApi([]);
    const result = await assignReceiptLine({ ok: false }, form({ receipt_id: "r1", line_id: "l1", mode: "new", product: "", compare_unit: "kg", pack_size: "x" }));
    expect(result.ok).toBe(false);
    expect(result.fieldErrors).toEqual({ product: "This field is required.", pack_size: "Enter a positive number." });
    expect(requests).toHaveLength(0);
  });

  it("shows API validation errors", async () => {
    mockApi([{ method: "POST", path: "/api/v1/groceries/receipts/r1/assign", status: 422, body: { status: 422, detail: "invalid" } }]);
    const result = await assignReceiptLine({ ok: false }, form({ receipt_id: "r1", line_id: "l1", mode: "existing", variant_id: "v1" }));
    expect(result).toMatchObject({ ok: false, message: "Please check the highlighted fields." });
  });

  it("reports a receipt that no longer exists", async () => {
    mockApi([{ method: "POST", path: "/api/v1/groceries/receipts/r1/assign", status: 404, body: { status: 404, detail: "not found" } }]);
    const result = await assignReceiptLine({ ok: false }, form({ receipt_id: "r1", line_id: "l1", mode: "existing", variant_id: "v1" }));
    expect(result).toMatchObject({ ok: false, message: "This item no longer exists." });
  });
});

describe("unassignReceiptLine", () => {
  it("sends a null variant", async () => {
    const requests = mockApi([{ method: "POST", path: "/api/v1/groceries/receipts/r1/assign", status: 200, body: {} }]);
    const result = await unassignReceiptLine("r1", "l1");
    expect(result.ok).toBe(true);
    expect(requests[0].body).toEqual({ assignments: [{ line_id: "l1", variant_id: null }] });
  });

  it("fails when the API does", async () => {
    mockApi([{ method: "POST", path: "/api/v1/groceries/receipts/r1/assign", status: 404, body: { status: 404 } }]);
    expect((await unassignReceiptLine("r1", "l1")).ok).toBe(false);
  });
});

describe("deleteReceipt", () => {
  it("deletes the receipt", async () => {
    const requests = mockApi([{ method: "DELETE", path: "/api/v1/groceries/receipts/r1", status: 204 }]);
    const result = await deleteReceipt("r1");
    expect(result.ok).toBe(true);
    expect(requests[0]).toMatchObject({ method: "DELETE", path: "/api/v1/groceries/receipts/r1", auth: "Bearer tok" });
  });

  it("fails when the receipt is gone", async () => {
    mockApi([{ method: "DELETE", path: "/api/v1/groceries/receipts/r1", status: 404, body: { status: 404 } }]);
    expect(await deleteReceipt("r1")).toMatchObject({ ok: false, message: "This item no longer exists." });
  });
});
