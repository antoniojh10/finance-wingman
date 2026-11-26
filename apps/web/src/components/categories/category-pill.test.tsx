import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { CategoryPill } from "./category-pill";

describe("CategoryPill", () => {
  it("shows the name tinted with the category color", () => {
    render(<CategoryPill name="Fun" color="#ff8800" />);
    const pill = screen.getByTestId("category-pill");
    expect(pill).toHaveTextContent("Fun");
    expect(pill.style.getPropertyValue("--tile")).toBe("#ff8800");
  });

  it("falls back to the muted color when there is none", () => {
    render(<CategoryPill name="Misc" color={null} />);
    expect(screen.getByTestId("category-pill").style.getPropertyValue("--tile")).toBe("var(--muted-foreground)");
  });

  it("truncates long names", () => {
    render(<CategoryPill name="A very long category name" />);
    expect(screen.getByText("A very long category name")).toHaveClass("truncate");
  });
});
