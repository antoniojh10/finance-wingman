import { screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { renderWithIntl } from "@/test/render";

import { ExportDataCard } from "./export-data-card";

describe("ExportDataCard", () => {
  it("links to the JSON and CSV exports", () => {
    renderWithIntl(<ExportDataCard />);
    expect(screen.getByRole("heading", { name: "Export your data" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Download JSON" })).toHaveAttribute("href", "/export?format=json");
    expect(screen.getByRole("link", { name: "Download CSV (zip)" })).toHaveAttribute("href", "/export?format=csv");
  });

  it("is localized", () => {
    renderWithIntl(<ExportDataCard />, { locale: "es" });
    expect(screen.getByRole("link", { name: "Descargar JSON" })).toBeInTheDocument();
  });
});
