import { screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { renderWithIntl } from "@/test/render";

import { isActive, SidebarLinks, TabLinks } from "./nav-links";

vi.mock("next/navigation", () => ({
  usePathname: () => "/transactions",
  useSearchParams: () => new URLSearchParams("type=income"),
}));

describe("isActive", () => {
  it("matches the dashboard exactly and other sections by prefix", () => {
    expect(isActive("/", "/")).toBe(true);
    expect(isActive("/accounts", "/")).toBe(false);
    expect(isActive("/accounts/123", "/accounts")).toBe(true);
    expect(isActive("/accountsx", "/accounts")).toBe(false);
  });

  it("does not highlight transactions while adding one", () => {
    expect(isActive("/transactions/new", "/transactions")).toBe(false);
  });
});

describe("TabLinks", () => {
  it("marks the current section and leaves settings to the header", () => {
    renderWithIntl(<TabLinks />);
    expect(screen.getByRole("link", { name: "Transactions" })).toHaveAttribute("aria-current", "page");
    expect(screen.getByRole("link", { name: "Dashboard" })).not.toHaveAttribute("aria-current");
    expect(screen.queryByRole("link", { name: "Settings" })).not.toBeInTheDocument();
  });

  it("shows budgets and leaves subscriptions and categories to settings", () => {
    renderWithIntl(<TabLinks />);
    expect(screen.getByRole("link", { name: "Budgets" })).toHaveAttribute("href", "/budgets");
    expect(screen.getByRole("link", { name: "Accounts" })).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Subscriptions" })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Categories" })).not.toBeInTheDocument();
  });

  it("leaves reports to settings so the bar keeps four sections", () => {
    renderWithIntl(<TabLinks />);
    expect(screen.queryByRole("link", { name: "Reports" })).not.toBeInTheDocument();
  });

  it("keeps four sections around the add button", () => {
    renderWithIntl(<TabLinks />);
    expect(screen.getAllByRole("link")).toHaveLength(5);
  });

  it("adds transactions returning to the current page", () => {
    renderWithIntl(<TabLinks />);
    expect(screen.getByRole("link", { name: "Add transaction" })).toHaveAttribute(
      "href",
      "/transactions/new?return=%2Ftransactions%3Ftype%3Dincome",
    );
  });
});

describe("SidebarLinks", () => {
  it("lists budgets, categories and subscriptions", () => {
    renderWithIntl(<SidebarLinks />);
    expect(screen.getByRole("link", { name: "Categories" })).toHaveAttribute("href", "/categories");
    expect(screen.getByRole("link", { name: "Subscriptions" })).toHaveAttribute("href", "/subscriptions");
    expect(screen.getByRole("link", { name: "Budgets" })).toHaveAttribute("href", "/budgets");
  });

  it("lists reports", () => {
    renderWithIntl(<SidebarLinks />);
    expect(screen.getByRole("link", { name: "Reports" })).toHaveAttribute("href", "/reports");
  });

  it("includes settings", () => {
    renderWithIntl(<SidebarLinks />);
    expect(screen.getByRole("link", { name: "Settings" })).toHaveAttribute("href", "/settings");
  });
});
