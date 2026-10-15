import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { CatalogProduct, GroceryStore, ProductGroup } from "@/lib/groceries-products";
import { renderWithIntl } from "@/test/render";

import { ProductsList } from "./products-list";

const actions = vi.hoisted(() => ({
  createProduct: vi.fn(),
  updateProduct: vi.fn(),
  createStore: vi.fn(),
  renameStore: vi.fn(),
  setStoreArchived: vi.fn(),
  createGroup: vi.fn(),
  renameGroup: vi.fn(),
}));
vi.mock("@/app/actions/groceries-catalog", () => actions);

const group = (id: string, name: string, position: number, isFood = true): ProductGroup => ({ id, name, position, color: "#12a150", is_food: isFood, created_at: "", updated_at: "" });
const product = (id: string, name: string, groupId: string | null, extra: Partial<CatalogProduct> = {}): CatalogProduct => ({
  id,
  name,
  group_id: groupId,
  group_name: null,
  group_color: null,
  compare_unit: "l",
  display_multiplier: 1,
  archived: false,
  variant_count: 2,
  created_at: "",
  updated_at: "",
  ...extra,
});
const store = (id: string, name: string, archived = false): GroceryStore => ({ id, name, archived, created_at: "", updated_at: "" });

const groups = [group("g1", "Dairy & eggs", 1), group("g2", "Cleaning", 2, false)];
const stores = [store("s1", "Mercadona"), store("s2", "Old Market", true)];
const products = [
  product("p1", "Whole milk", "g1"),
  product("p2", "Eggs", "g1", { compare_unit: "unit", display_multiplier: 12, variant_count: 1 }),
  product("p3", "Bin bags", null, { compare_unit: "unit", variant_count: 0, archived: true }),
];

beforeEach(() => Object.values(actions).forEach((mock) => mock.mockReset()));

describe("ProductsList", () => {
  it("groups products and links each to its detail", () => {
    renderWithIntl(<ProductsList products={products} groups={groups} stores={stores} />);
    const sections = screen.getAllByTestId("product-section");
    expect(sections.map((s) => s.getAttribute("aria-label"))).toEqual(["Dairy & eggs", "No group"]);
    const milk = within(sections[0]).getByRole("link", { name: /Whole milk/ });
    expect(milk).toHaveAttribute("href", "/groceries/products/p1");
    expect(milk).toHaveTextContent("per litre · 2 variants");
    expect(within(sections[0]).getByRole("link", { name: /Eggs/ })).toHaveTextContent("per dozen · 1 variant");
    expect(within(sections[1]).getByRole("link", { name: /Bin bags/ })).toHaveTextContent("Archived");
  });

  it("shows the search and an empty state when nothing matches", () => {
    renderWithIntl(<ProductsList products={products} groups={groups} stores={stores} search="zzz" />);
    expect(screen.getByRole("textbox", { name: "Search products" })).toHaveValue("zzz");
    expect(screen.getByTestId("products-empty")).toHaveTextContent("No products match your search.");
  });

  it("invites to create the first product", () => {
    renderWithIntl(<ProductsList products={[]} groups={groups} stores={[]} />);
    expect(screen.getByTestId("products-empty")).toHaveTextContent("No products yet");
  });

  it("creates a product from the dialog", async () => {
    actions.createProduct.mockResolvedValue({ ok: true, nonce: 1 });
    const user = userEvent.setup();
    renderWithIntl(<ProductsList products={products} groups={groups} stores={stores} />);
    await user.click(screen.getByRole("button", { name: "New product" }));
    await user.type(screen.getByLabelText("Name"), "Butter");
    await user.selectOptions(screen.getByLabelText("Group"), "g1");
    await user.selectOptions(screen.getByLabelText("Compare prices by"), "kg");
    await user.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(actions.createProduct).toHaveBeenCalled());
    const data = actions.createProduct.mock.calls[0][1] as FormData;
    expect(Object.fromEntries(data.entries())).toEqual({ name: "Butter", group_id: "g1", compare_unit: "kg", display_multiplier: "1" });
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  });

  it("shows field errors in the dialog", async () => {
    actions.createProduct.mockResolvedValue({ ok: false, message: "Please check the highlighted fields.", fieldErrors: { name: "This field is required." } });
    const user = userEvent.setup();
    renderWithIntl(<ProductsList products={products} groups={groups} stores={stores} />);
    await user.click(screen.getByRole("button", { name: "New product" }));
    await user.click(screen.getByRole("button", { name: "Save" }));
    expect(await screen.findByText("This field is required.")).toBeInTheDocument();
  });
});

describe("CatalogAdmin", () => {
  it("lists stores with archived last and groups with the non-food chip", () => {
    renderWithIntl(<ProductsList products={products} groups={groups} stores={stores} />);
    const rows = within(screen.getByTestId("stores-admin")).getAllByTestId("store-row");
    expect(rows.map((r) => r.textContent)).toEqual([expect.stringContaining("Mercadona"), expect.stringContaining("Old Market")]);
    expect(rows[1]).toHaveTextContent("Archived");
    const groupRows = within(screen.getByTestId("groups-admin")).getAllByTestId("group-row");
    expect(groupRows[1]).toHaveTextContent("Non-food");
  });

  it("renames a store inline", async () => {
    actions.renameStore.mockResolvedValue({ ok: true, nonce: 1 });
    const user = userEvent.setup();
    renderWithIntl(<ProductsList products={products} groups={groups} stores={stores} />);
    await user.click(screen.getByRole("button", { name: "Rename Mercadona" }));
    const input = screen.getByLabelText("Store name");
    await user.clear(input);
    await user.type(input, "Mercadona SA");
    await user.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(actions.renameStore).toHaveBeenCalled());
    const data = actions.renameStore.mock.calls[0][1] as FormData;
    expect(Object.fromEntries(data.entries())).toEqual({ id: "s1", name: "Mercadona SA" });
  });

  it("archives and restores stores", async () => {
    actions.setStoreArchived.mockResolvedValue({ ok: true, nonce: 1 });
    const user = userEvent.setup();
    renderWithIntl(<ProductsList products={products} groups={groups} stores={stores} />);
    await user.click(screen.getByRole("button", { name: "Archive Mercadona" }));
    await waitFor(() => expect(actions.setStoreArchived).toHaveBeenCalledWith("s1", true));
    await user.click(screen.getByRole("button", { name: "Restore Old Market" }));
    await waitFor(() => expect(actions.setStoreArchived).toHaveBeenCalledWith("s2", false));
  });

  it("adds a store and shows a duplicate error", async () => {
    actions.createStore.mockResolvedValue({ ok: false, message: "A store with this name already exists." });
    const user = userEvent.setup();
    renderWithIntl(<ProductsList products={products} groups={groups} stores={stores} />);
    const admin = within(screen.getByTestId("stores-admin"));
    await user.type(admin.getByLabelText("New store"), "Mercadona");
    await user.click(admin.getByRole("button", { name: "Add" }));
    expect(await admin.findByRole("alert")).toHaveTextContent("A store with this name already exists.");
  });

  it("adds a group with the food flag", async () => {
    actions.createGroup.mockResolvedValue({ ok: true, nonce: 1 });
    const user = userEvent.setup();
    renderWithIntl(<ProductsList products={products} groups={groups} stores={stores} />);
    const admin = within(screen.getByTestId("groups-admin"));
    await user.type(admin.getByLabelText("New group"), "Pets");
    await user.click(admin.getByRole("checkbox", { name: "Food group" }));
    await user.click(admin.getByRole("button", { name: "Add" }));
    await waitFor(() => expect(actions.createGroup).toHaveBeenCalled());
    const data = actions.createGroup.mock.calls[0][1] as FormData;
    expect(Object.fromEntries(data.entries())).toEqual({ name: "Pets" });
  });
});
