import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { AccountSparkline, sparklinePoints } from "./account-sparkline";

const series = (...balances: number[]) => balances.map((balance, i) => ({ date: `2026-11-${String(i + 1).padStart(2, "0")}`, balance }));

describe("sparklinePoints", () => {
  it("puts the lowest balance at the bottom and the highest at the top", () => {
    const [first, , last] = sparklinePoints(series(100, 150, 200));
    expect(first.y).toBeGreaterThan(last.y);
    expect(first.x).toBeLessThan(last.x);
  });

  it("draws a flat series in the middle", () => {
    const ys = new Set(sparklinePoints(series(500, 500, 500)).map((p) => p.y));
    expect(ys.size).toBe(1);
    expect([...ys][0]).toBe(16);
  });
});

describe("AccountSparkline", () => {
  it("renders a labelled line with a dot on the last point", () => {
    render(<AccountSparkline points={series(1, 3, 2)} label="Trend" />);
    const svg = screen.getByRole("img", { name: "Trend" });
    expect(svg.querySelectorAll("path")).toHaveLength(3);
  });

  it("renders nothing without at least two points", () => {
    const { container } = render(<AccountSparkline points={series(1)} label="Trend" />);
    expect(container).toBeEmptyDOMElement();
    const empty = render(<AccountSparkline points={[]} label="Trend" />);
    expect(empty.container).toBeEmptyDOMElement();
  });
});
