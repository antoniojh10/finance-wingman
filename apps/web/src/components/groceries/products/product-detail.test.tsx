import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { CatalogProduct, GroceryStore, ProductGroup, ProductPrices } from "@/lib/groceries-products";
import { renderWithIntl } from "@/test/render";

import { ProductDetail } from "./product-detail";

const actions = vi.hoisted(() => ({
  updateProduct: vi.fn(),
  setProductArchived: vi.fn(),
  createVariant: vi.fn(),
  updateVariant: vi.fn(),
  deleteVariant: vi.fn(),
  createPriceCheck: vi.fn(),
}));
vi.mock("@/app/actions/groceries-catalog", () => actions);

const product: CatalogProduct = {
  id: "p1",
  name: "Whole milk",
  group_id: "g1",
  group_name: "Dairy & eggs",
  group_color: "#12a150",
  compare_unit: "l",
  display_multiplier: 1,
  archived: false,
  variant_count: 3,
  created_at: "",
  updated_at: "",
};
const groups: ProductGroup[] = [{ id: "g1", name: "Dairy & eggs", position: 1, color: null, is_food: true, created_at: "", updated_at: "" }];
const stores: GroceryStore[] = [
  { id: "s1", name: "Mercadona", archived: false, created_at: "", updated_at: "" },
  { id: "s2", name: "Lidl", archived: false, created_at: "", updated_at: "" },
  { id: "s3", name: "Closed", archived: true, created_at: "", updated_at: "" },
];

const price = (storeId: string, storeName: string, unit: number, extra: object = {}) => ({
  store_id: storeId,
  store_name: storeName,
  currency: "EUR",
  unit_price_minor: unit,
  display_unit_price: unit,
  observed_on: "2026-10-01",
  source: "receipt" as const,
  stale: false,
  ...extra,
});

const prices: ProductPrices = {
  product_id: "p1",
  name: "Whole milk",
  compare_unit: "l",
  display_multiplier: 1,
  variants: [
    {
      variant_id: "v1",
      brand: "Hacendado",
      pack_label: "6×1 L",
      pack_size: 6,
      store_id: "s1",
      latest: [price("s1", "Mercadona", 85)],
      history: [
        {
          store_id: "s1",
          store_name: "Mercadona",
          currency: "EUR",
          points: [
            { month: "2026-08", unit_price_minor: 80, display_unit_price: 80, observations: 1 },
            { month: "2026-09", unit_price_minor: 82, display_unit_price: 82, observations: 2 },
            { month: "2026-10", unit_price_minor: 85, display_unit_price: 85, observations: 1 },
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
      latest: [price("s1", "Mercadona", 110, { stale: true, source: "price_check", observed_on: "2026-05-01" }), price("s2", "Lidl", 95)],
      history: [
        { store_id: "s2", store_name: "Lidl", currency: "EUR", points: [{ month: "2026-10", unit_price_minor: 95, display_unit_price: 95, observations: 1 }] },
      ],
    },
    { variant_id: "v3", brand: "Generic", pack_label: null, pack_size: null, store_id: null, latest: [], history: [] },
  ],
};

function renderDetail(overrides: Partial<{ prices: ProductPrices; product: CatalogProduct }> = {}) {
  return renderWithIntl(
    <ProductDetail product={overrides.product ?? product} prices={overrides.prices ?? prices} stores={stores} groups={groups} currencies={["EUR", "USD"]} today="2026-10-15" />,
  );
}

beforeEach(() => Object.values(actions).forEach((mock) => mock.mockReset()));

describe("ProductDetail variants", () => {
  it("sorts variants by unit price with store brand, best and stale chips", () => {
    renderDetail();
    const rows = screen.getAllByTestId("variant-row");
    expect(rows.map((row) => within(row).getByText(/Hacendado|Pascual|Generic/).textContent)).toEqual([
      "Hacendado · 6×1 L",
      "Pascual · 1 L",
      "Pascual · 1 L",
      "Generic",
    ]);
    expect(within(rows[0]).getByText("Store brand")).toBeInTheDocument();
    expect(within(rows[0]).getByText("Best price")).toBeInTheDocument();
    expect(within(rows[0]).getByTestId("unit-price")).toHaveTextContent(/0[.,]85/);
    expect(within(rows[0]).getByTestId("pack-price")).toHaveTextContent(/5[.,]10/);
    expect(within(rows[1]).getByText("Lidl")).toBeInTheDocument();
    expect(within(rows[2]).getByText("Outdated")).toBeInTheDocument();
    expect(within(rows[2]).getByText(/price check/)).toBeInTheDocument();
    expect(within(rows[3]).getByText("No price yet")).toBeInTheDocument();
    expect(within(rows[3]).getByText("Loose")).toBeInTheDocument();
  });

  it("shows an empty state without variants", () => {
    renderDetail({ prices: { ...prices, variants: [] } });
    expect(screen.getByText(/No variants yet/)).toBeInTheDocument();
  });

  it("edits a variant", async () => {
    actions.updateVariant.mockResolvedValue({ ok: true, nonce: 1 });
    const user = userEvent.setup();
    renderDetail();
    await user.click(within(screen.getAllByTestId("variant-row")[0]).getByRole("button", { name: "Actions" }));
    await user.click(await screen.findByRole("menuitem", { name: "Edit" }));
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByLabelText("Brand")).toHaveValue("Hacendado");
    expect(within(dialog).getByLabelText("Store brand of")).toHaveValue("s1");
    await user.clear(within(dialog).getByLabelText("Brand"));
    await user.type(within(dialog).getByLabelText("Brand"), "Hacendado Bio");
    await user.click(within(dialog).getByRole("button", { name: "Save" }));
    await waitFor(() => expect(actions.updateVariant).toHaveBeenCalled());
    const data = actions.updateVariant.mock.calls[0][1] as FormData;
    expect(Object.fromEntries(data.entries())).toMatchObject({ id: "v1", brand: "Hacendado Bio", pack_size: "6", store_id: "s1" });
  });

  it("deletes a variant after confirming", async () => {
    actions.deleteVariant.mockResolvedValue({ ok: true, nonce: 1 });
    const user = userEvent.setup();
    renderDetail();
    await user.click(within(screen.getAllByTestId("variant-row")[3]).getByRole("button", { name: "Actions" }));
    await user.click(await screen.findByRole("menuitem", { name: "Delete" }));
    const dialog = await screen.findByRole("alertdialog");
    expect(dialog).toHaveTextContent("Delete Generic?");
    await user.click(within(dialog).getByRole("button", { name: "Delete" }));
    await waitFor(() => expect(actions.deleteVariant).toHaveBeenCalledWith("v3"));
  });

  it("adds a variant offering only active stores", async () => {
    actions.createVariant.mockResolvedValue({ ok: true, nonce: 1 });
    const user = userEvent.setup();
    renderDetail();
    await user.click(screen.getByRole("button", { name: "Add variant" }));
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).queryByRole("option", { name: "Closed" })).not.toBeInTheDocument();
    await user.type(within(dialog).getByLabelText("Brand"), "Pascual");
    await user.type(within(dialog).getByLabelText(/Pack size/), "1");
    await user.click(within(dialog).getByRole("button", { name: "Save" }));
    await waitFor(() => expect(actions.createVariant).toHaveBeenCalled());
    const data = actions.createVariant.mock.calls[0][1] as FormData;
    expect(Object.fromEntries(data.entries())).toMatchObject({ product_id: "p1", brand: "Pascual", pack_size: "1", store_id: "" });
  });
});

describe("ProductDetail header", () => {
  it("shows the group and the price unit", () => {
    renderDetail({ product: { ...product, compare_unit: "unit", display_multiplier: 12 } });
    expect(screen.getByTestId("product-name")).toHaveTextContent("Whole milk");
    expect(screen.getByText("Dairy & eggs")).toBeInTheDocument();
    expect(screen.getAllByText("per dozen").length).toBeGreaterThan(0);
  });

  it("archives after confirming", async () => {
    actions.setProductArchived.mockResolvedValue({ ok: true, nonce: 1 });
    const user = userEvent.setup();
    renderDetail();
    await user.click(screen.getByRole("button", { name: "Archive" }));
    const dialog = await screen.findByRole("alertdialog");
    await user.click(within(dialog).getByRole("button", { name: "Archive" }));
    await waitFor(() => expect(actions.setProductArchived).toHaveBeenCalledWith("p1", true));
  });

  it("restores an archived product", async () => {
    actions.setProductArchived.mockResolvedValue({ ok: true, nonce: 1 });
    const user = userEvent.setup();
    renderDetail({ product: { ...product, archived: true } });
    await user.click(screen.getByRole("button", { name: "Restore" }));
    await user.click(within(await screen.findByRole("alertdialog")).getByRole("button", { name: "Restore" }));
    await waitFor(() => expect(actions.setProductArchived).toHaveBeenCalledWith("p1", false));
  });

  it("edits the product", async () => {
    actions.updateProduct.mockResolvedValue({ ok: true, nonce: 1 });
    const user = userEvent.setup();
    renderDetail();
    await user.click(screen.getByRole("button", { name: "Edit" }));
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByLabelText("Name")).toHaveValue("Whole milk");
    await user.click(within(dialog).getByRole("button", { name: "Save" }));
    await waitFor(() => expect(actions.updateProduct).toHaveBeenCalled());
    const data = actions.updateProduct.mock.calls[0][1] as FormData;
    expect(Object.fromEntries(data.entries())).toEqual({ id: "p1", name: "Whole milk", group_id: "g1", compare_unit: "l", display_multiplier: "1" });
  });
});

describe("price history", () => {
  it("draws the series of the cheapest variant and switches with the selector", async () => {
    const user = userEvent.setup();
    renderDetail();
    const chart = screen.getByTestId("price-history-chart");
    expect(chart).toHaveAccessibleName("Monthly price of Hacendado · 6×1 L @ Mercadona, per litre");
    expect(chart.querySelectorAll("circle")).toHaveLength(3);
    await user.selectOptions(screen.getByLabelText("Variant and store"), "v2:s2");
    expect(screen.getByTestId("price-history-chart").querySelectorAll("circle")).toHaveLength(1);
  });

  it("explains when there is no history", () => {
    renderDetail({ prices: { ...prices, variants: [prices.variants[2]] } });
    expect(screen.getByText(/no price history yet/)).toBeInTheDocument();
  });
});

describe("price check form", () => {
  it("records a price for a packed variant at an active store", async () => {
    actions.createPriceCheck.mockResolvedValue({ ok: true, nonce: 1 });
    const user = userEvent.setup();
    renderDetail();
    const form = within(screen.getByRole("form", { name: "Add a price check" }));
    expect(form.queryByRole("option", { name: "Generic" })).not.toBeInTheDocument();
    expect(form.queryByRole("option", { name: "Closed" })).not.toBeInTheDocument();
    await user.selectOptions(form.getByLabelText("Brand and pack"), "v2");
    await user.selectOptions(form.getByLabelText("Store"), "s2");
    await user.type(form.getByLabelText("Pack price"), "1,19");
    await user.click(form.getByRole("button", { name: "Save price" }));
    await waitFor(() => expect(actions.createPriceCheck).toHaveBeenCalled());
    const data = actions.createPriceCheck.mock.calls[0][1] as FormData;
    expect(Object.fromEntries(data.entries())).toEqual({ variant_id: "v2", store_id: "s2", price: "1,19", currency: "EUR", observed_on: "2026-10-15", note: "" });
  });

  it("shows validation errors from the action", async () => {
    actions.createPriceCheck.mockResolvedValue({ ok: false, message: "Please check the highlighted fields.", fieldErrors: { price: "Enter a positive number." } });
    const user = userEvent.setup();
    renderDetail();
    await user.click(within(screen.getByRole("form", { name: "Add a price check" })).getByRole("button", { name: "Save price" }));
    expect(await screen.findByText("Enter a positive number.")).toBeInTheDocument();
  });

  it("asks for a variant with a pack size first", () => {
    renderDetail({ prices: { ...prices, variants: [prices.variants[2]] } });
    expect(screen.getByText(/Add a variant with a pack size/)).toBeInTheDocument();
  });
});
