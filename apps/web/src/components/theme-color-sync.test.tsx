import { render } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ThemeColorSync } from "./theme-color-sync";

let resolvedTheme: string | undefined = "dark";

vi.mock("next-themes", () => ({
  useTheme: () => ({ resolvedTheme }),
}));

describe("ThemeColorSync", () => {
  afterEach(() => {
    document.head.innerHTML = "";
  });

  it("points every theme-color meta at the resolved theme", () => {
    document.head.innerHTML =
      '<meta name="theme-color" media="(prefers-color-scheme: light)" content="#f4f5fb">' +
      '<meta name="theme-color" media="(prefers-color-scheme: dark)" content="#0e0c22">';
    resolvedTheme = "dark";
    render(<ThemeColorSync />);
    const colors = [...document.querySelectorAll<HTMLMetaElement>('meta[name="theme-color"]')].map((m) => m.content);
    expect(colors).toEqual(["#0e0c22", "#0e0c22"]);
  });

  it("leaves the metas alone until the theme resolves", () => {
    document.head.innerHTML = '<meta name="theme-color" content="#f4f5fb">';
    resolvedTheme = undefined;
    render(<ThemeColorSync />);
    expect(document.querySelector<HTMLMetaElement>('meta[name="theme-color"]')?.content).toBe("#f4f5fb");
  });
});
