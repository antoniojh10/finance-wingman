import { describe, expect, it } from "vitest";

import { parseRecurringFilters, recurringFiltersHref } from "./recurring-filters";

describe("parseRecurringFilters", () => {
  it("defaults to active when the status is missing", () => {
    expect(parseRecurringFilters({})).toEqual({ status: "active" });
  });

  it.each(["active", "paused", "cancelled", "all"])("accepts %s", (status) => {
    expect(parseRecurringFilters({ status })).toEqual({ status });
  });

  it("falls back to active for invalid values", () => {
    expect(parseRecurringFilters({ status: "bogus" })).toEqual({ status: "active" });
    expect(parseRecurringFilters({ status: "" })).toEqual({ status: "active" });
  });

  it("uses the first value of a repeated param", () => {
    expect(parseRecurringFilters({ status: ["paused", "all"] })).toEqual({ status: "paused" });
  });
});

describe("recurringFiltersHref", () => {
  it("leaves the default out", () => {
    expect(recurringFiltersHref({ status: "active" })).toBe("/subscriptions");
  });

  it("keeps a non-default status", () => {
    expect(recurringFiltersHref({ status: "all" })).toBe("/subscriptions?status=all");
  });
});
