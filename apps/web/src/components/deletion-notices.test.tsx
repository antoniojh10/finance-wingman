import { screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { renderWithIntl } from "@/test/render";

import { DeletionNotices } from "./deletion-notices";

describe("DeletionNotices", () => {
  it("renders nothing without a scheduled deletion", () => {
    const { container } = renderWithIntl(<DeletionNotices workspace={{ name: "Home" }} />);
    expect(container).toBeEmptyDOMElement();
  });

  it("warns about a scheduled workspace deletion", () => {
    renderWithIntl(<DeletionNotices workspace={{ name: "Home", deletion_scheduled_for: "2026-10-15T12:00:00Z" }} />);
    expect(screen.getByRole("status")).toHaveTextContent("Home will be deleted on October 15, 2026.");
    expect(screen.getByRole("link", { name: "Review" })).toHaveAttribute("href", "/settings/workspace");
  });
});
