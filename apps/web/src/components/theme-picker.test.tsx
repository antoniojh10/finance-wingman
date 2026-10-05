import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { renderWithIntl } from "@/test/render";

import { ThemePicker } from "./theme-picker";

const setTheme = vi.fn();
let currentTheme: string | undefined = "dark";

vi.mock("next-themes", () => ({
  useTheme: () => ({ theme: currentTheme, setTheme }),
}));

describe("ThemePicker", () => {
  beforeEach(() => {
    setTheme.mockClear();
    currentTheme = "dark";
  });

  it("offers system, light and dark with the stored one selected", () => {
    renderWithIntl(<ThemePicker />);
    expect(screen.getByRole("radiogroup", { name: "Theme" })).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: "Dark" })).toBeChecked();
    expect(screen.getByRole("radio", { name: "Light" })).not.toBeChecked();
    expect(screen.getByRole("radio", { name: "System" })).not.toBeChecked();
  });

  it("selects system when no theme is stored", () => {
    currentTheme = undefined;
    renderWithIntl(<ThemePicker />);
    expect(screen.getByRole("radio", { name: "System" })).toBeChecked();
  });

  it("applies the chosen theme", async () => {
    renderWithIntl(<ThemePicker />);
    await userEvent.click(screen.getByRole("radio", { name: "Light" }));
    expect(setTheme).toHaveBeenCalledWith("light");
  });
});
