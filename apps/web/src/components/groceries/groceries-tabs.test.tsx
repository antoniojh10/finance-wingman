import { screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { renderWithIntl } from "@/test/render";

import { GroceriesTabs } from "./groceries-tabs";

let pathname = "/groceries/receipts/abc";
vi.mock("next/navigation", () => ({ usePathname: () => pathname }));

describe("GroceriesTabs", () => {
  it("links every tab and marks the current one", () => {
    renderWithIntl(<GroceriesTabs />);
    expect(screen.getByRole("link", { name: "Summary" })).toHaveAttribute("href", "/groceries");
    expect(screen.getByRole("link", { name: "Receipts" })).toHaveAttribute("href", "/groceries/receipts");
    expect(screen.getByRole("link", { name: "Receipts" })).toHaveAttribute("aria-current", "page");
    expect(screen.getByRole("link", { name: "Summary" })).not.toHaveAttribute("aria-current");
  });

  it("marks the summary on its own page", () => {
    pathname = "/groceries";
    renderWithIntl(<GroceriesTabs />);
    expect(screen.getByRole("link", { name: "Summary" })).toHaveAttribute("aria-current", "page");
  });
});
