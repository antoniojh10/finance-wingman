import { screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { renderWithIntl } from "@/test/render";

import { Money } from "./money";

describe("Money", () => {
  it("formats amounts for the locale", () => {
    renderWithIntl(<Money amount={123456} currency="USD" minorUnits={2} />);
    expect(screen.getByText("$1,234.56")).toBeInTheDocument();
  });

  it("uses Mexican formatting in Spanish", () => {
    renderWithIntl(<Money amount={2500} currency="MXN" minorUnits={2} />, { locale: "es" });
    expect(screen.getByText("$25.00")).toBeInTheDocument();
  });

  it("prefixes signs by tone when signed", () => {
    renderWithIntl(
      <>
        <Money amount={1000} currency="USD" minorUnits={2} tone="income" signed />
        <Money amount={1000} currency="USD" minorUnits={2} tone="expense" signed />
        <Money amount={-500} currency="USD" minorUnits={2} />
      </>,
    );
    expect(screen.getByText("+$10.00")).toHaveClass("text-income");
    expect(screen.getByText("−$10.00")).not.toHaveClass("text-income");
    expect(screen.getByText("−$5.00")).toBeInTheDocument();
  });
});
