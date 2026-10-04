import { screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { renderWithIntl } from "@/test/render";

import { isActive, NavLinks } from "./nav-links";

vi.mock("next/navigation", () => ({ usePathname: () => "/transactions" }));

describe("isActive", () => {
  it("matches the dashboard exactly and other sections by prefix", () => {
    expect(isActive("/", "/")).toBe(true);
    expect(isActive("/accounts", "/")).toBe(false);
    expect(isActive("/accounts/123", "/accounts")).toBe(true);
    expect(isActive("/accountsx", "/accounts")).toBe(false);
  });
});

describe("NavLinks", () => {
  it("marks the current section", () => {
    renderWithIntl(<NavLinks variant="tabs" />);
    expect(screen.getByRole("link", { name: "Transactions" })).toHaveAttribute("aria-current", "page");
    expect(screen.getByRole("link", { name: "Dashboard" })).not.toHaveAttribute("aria-current");
  });
});
