import { screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { renderWithIntl } from "@/test/render";

import { AssistantSteps } from "./assistant-steps";

describe("AssistantSteps", () => {
  it("explains how to refresh the connector in Claude and ChatGPT", () => {
    renderWithIntl(<AssistantSteps />);
    const items = screen.getAllByRole("listitem");
    expect(items).toHaveLength(2);
    expect(within(items[0]).getByText(/disconnect the connector and connect it again/)).toBeInTheDocument();
    expect(within(items[1]).getByText(/open the connector under Settings → Apps & Connectors and refresh it/)).toBeInTheDocument();
  });

  it("is localized", () => {
    renderWithIntl(<AssistantSteps />, { locale: "es" });
    expect(screen.getByText(/desconecta el conector y vuelve a conectarlo/)).toBeInTheDocument();
    expect(screen.getByText(/actualízalo/)).toBeInTheDocument();
  });
});
