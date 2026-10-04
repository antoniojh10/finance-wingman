import { describe, expect, it } from "vitest";
import { formatDate, formatMonth, monthOf, monthRange, parseMonth, shiftMonth, today } from "./dates";

describe("dates", () => {
  it("computes month ranges, including leap years", () => {
    expect(monthRange("2026-10")).toEqual({ from: "2026-10-01", to: "2026-10-31" });
    expect(monthRange("2026-02")).toEqual({ from: "2026-02-01", to: "2026-02-28" });
    expect(monthRange("2028-02")).toEqual({ from: "2028-02-01", to: "2028-02-29" });
    expect(() => monthRange("2026-13")).toThrow();
  });

  it("shifts months across years", () => {
    expect(shiftMonth("2026-01", -1)).toBe("2025-12");
    expect(shiftMonth("2026-12", 1)).toBe("2027-01");
    expect(shiftMonth("2026-05", 0)).toBe("2026-05");
  });

  it("validates months with a fallback", () => {
    expect(parseMonth("2026-09", "2026-10")).toBe("2026-09");
    expect(parseMonth("2026-9", "2026-10")).toBe("2026-10");
    expect(parseMonth(undefined, "2026-10")).toBe("2026-10");
  });

  it("resolves today in a time zone", () => {
    const now = new Date("2026-10-05T03:00:00Z");
    expect(today("UTC", now)).toBe("2026-10-05");
    expect(today("America/Mexico_City", now)).toBe("2026-10-04");
    expect(monthOf("2026-10-04")).toBe("2026-10");
  });

  it("formats dates and months per locale without time zone drift", () => {
    expect(formatMonth("2026-10", "en-US")).toBe("October 2026");
    expect(formatMonth("2026-10", "es-MX")).toBe("octubre de 2026");
    expect(formatDate("2026-01-01", "en-US")).toBe("Jan 1, 2026");
  });
});
