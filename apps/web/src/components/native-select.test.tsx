import { screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { renderWithIntl } from "@/test/render";

import { NativeSelect } from "./native-select";

describe("NativeSelect", () => {
  it("renders a native select with its options", () => {
    renderWithIntl(
      <NativeSelect aria-label="Account" defaultValue="2">
        <option value="1">Option 1</option>
        <option value="2">Option 2</option>
      </NativeSelect>,
    );

    const select = screen.getByRole("combobox", { name: "Account" });
    expect(select).toHaveValue("2");
    expect(screen.getAllByRole("option")).toHaveLength(2);
  });

  it("merges a custom className", () => {
    renderWithIntl(
      <NativeSelect aria-label="Account" className="custom-class">
        <option value="1">Option 1</option>
      </NativeSelect>,
    );

    expect(screen.getByRole("combobox", { name: "Account" })).toHaveClass("custom-class");
  });
});
