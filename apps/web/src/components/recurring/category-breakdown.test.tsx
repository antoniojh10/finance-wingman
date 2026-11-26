import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { renderWithIntl } from "@/test/render";

import { type BreakdownCurrency, CategoryBreakdown } from "./category-breakdown";

const replace = vi.fn();
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace }) }));

beforeEach(() => replace.mockReset());

const mxn: BreakdownCurrency = {
  currency: "MXN",
  minor_units: 2,
  expense: 100000,
  categories: [
    { category_id: "home", category_name: "Home", category_color: "#ffb020", count: 2, expense: 60000 },
    { category_id: "fun", category_name: "Fun", category_color: "#f05aa6", count: 1, expense: 30000 },
    { category_id: null, category_name: null, category_color: null, count: 1, expense: 10000 },
  ],
};
const usd: BreakdownCurrency = {
  currency: "USD",
  minor_units: 2,
  expense: 1000,
  categories: [{ category_id: "dev", category_name: "Dev", category_color: null, count: 1, expense: 1000 }],
};
const defaults = { status: "active", group: true } as const;

describe("CategoryBreakdown", () => {
  it("shows amount, item count and share per category, uncategorized last", () => {
    renderWithIntl(<CategoryBreakdown currencies={[mxn]} filters={defaults} />);
    const rows = screen.getAllByTestId("breakdown-row");
    expect(rows).toHaveLength(3);
    expect(within(rows[0]).getByText("Home")).toBeInTheDocument();
    expect(rows[0]).toHaveTextContent("2 items");
    expect(rows[0]).toHaveTextContent("MX$600.00");
    expect(within(rows[0]).getByTestId("breakdown-percent")).toHaveTextContent("60%");
    expect(rows[1]).toHaveTextContent("1 item");
    expect(within(rows[1]).getByTestId("breakdown-percent")).toHaveTextContent("30%");
    expect(rows[2]).toHaveTextContent("Uncategorized");
    expect(within(rows[2]).getByTestId("breakdown-percent")).toHaveTextContent("10%");
  });

  it("shows <1% for a tiny share", () => {
    const tiny = { ...mxn, categories: [{ ...mxn.categories[0], expense: 100000 }, { ...mxn.categories[1], expense: 100 }] };
    renderWithIntl(<CategoryBreakdown currencies={[tiny]} filters={defaults} />);
    expect(screen.getAllByTestId("breakdown-percent")[1]).toHaveTextContent("<1%");
  });

  it("draws one stacked bar segment per category sized by amount", () => {
    renderWithIntl(<CategoryBreakdown currencies={[mxn]} filters={defaults} />);
    const segments = screen.getAllByTestId("breakdown-segment");
    expect(segments.map((s) => s.style.flexGrow)).toEqual(["60000", "30000", "10000"]);
    expect(segments[0].style.getPropertyValue("--c")).toBe("#ffb020");
    expect(segments[2].style.getPropertyValue("--c")).toBe("var(--muted-foreground)");
  });

  it("filters by the pressed category and keeps the other filters", async () => {
    const user = userEvent.setup();
    renderWithIntl(<CategoryBreakdown currencies={[mxn]} filters={{ status: "all", group: false }} />);
    await user.click(screen.getAllByTestId("breakdown-row")[0]);
    expect(replace).toHaveBeenCalledWith("/subscriptions?status=all&category=home&group=0", { scroll: false });
  });

  it("filters uncategorized with none", async () => {
    const user = userEvent.setup();
    renderWithIntl(<CategoryBreakdown currencies={[mxn]} filters={defaults} />);
    await user.click(screen.getAllByTestId("breakdown-row")[2]);
    expect(replace).toHaveBeenCalledWith("/subscriptions?category=none", { scroll: false });
  });

  it("clears the filter when the selected row is pressed again", async () => {
    const user = userEvent.setup();
    renderWithIntl(<CategoryBreakdown currencies={[mxn]} filters={{ ...defaults, category: "home" }} />);
    const rows = screen.getAllByTestId("breakdown-row");
    expect(rows[0]).toHaveAttribute("aria-pressed", "true");
    expect(rows[1]).toHaveAttribute("aria-pressed", "false");
    await user.click(rows[0]);
    expect(replace).toHaveBeenCalledWith("/subscriptions", { scroll: false });
  });

  it("shows currency tabs only with several currencies", async () => {
    const user = userEvent.setup();
    const view = renderWithIntl(<CategoryBreakdown currencies={[mxn]} filters={defaults} />);
    expect(screen.queryByRole("button", { name: "USD" })).not.toBeInTheDocument();
    view.unmount();
    renderWithIntl(<CategoryBreakdown currencies={[mxn, usd]} filters={defaults} />);
    expect(screen.getByRole("button", { name: "MXN" })).toHaveAttribute("aria-pressed", "true");
    await user.click(screen.getByRole("button", { name: "USD" }));
    const rows = screen.getAllByTestId("breakdown-row");
    expect(rows).toHaveLength(1);
    expect(rows[0]).toHaveTextContent("$10.00");
    expect(within(rows[0]).getByTestId("breakdown-percent")).toHaveTextContent("100%");
  });

  it("says so when a currency has no active expenses", () => {
    renderWithIntl(<CategoryBreakdown currencies={[{ ...mxn, expense: 0, categories: [] }]} filters={defaults} />);
    expect(screen.getByText("No active expenses in MXN.")).toBeInTheDocument();
  });

  it("renders nothing without currencies", () => {
    const { container } = renderWithIntl(<CategoryBreakdown currencies={[]} filters={defaults} />);
    expect(container).toBeEmptyDOMElement();
  });
});
