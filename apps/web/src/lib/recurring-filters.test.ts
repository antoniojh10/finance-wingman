import { describe, expect, it } from "vitest";

import { parseRecurringFilters, recurringFiltersHref } from "./recurring-filters";

describe("parseRecurringFilters", () => {
  it("uses the defaults when there are no params", () => {
    expect(parseRecurringFilters({})).toEqual({ status: "active", group: true });
  });

  it.each(["active", "paused", "cancelled", "all"])("accepts the status %s", (status) => {
    expect(parseRecurringFilters({ status })).toMatchObject({ status });
  });

  it("falls back to active for invalid statuses", () => {
    expect(parseRecurringFilters({ status: "bogus" }).status).toBe("active");
    expect(parseRecurringFilters({ status: "" }).status).toBe("active");
  });

  it("uses the first value of a repeated param", () => {
    expect(parseRecurringFilters({ status: ["paused", "all"] }).status).toBe("paused");
    expect(parseRecurringFilters({ category: ["a", "b"] }).category).toBe("a");
  });

  it("reads the category id and none", () => {
    expect(parseRecurringFilters({ category: "cat-1" }).category).toBe("cat-1");
    expect(parseRecurringFilters({ category: "none" }).category).toBe("none");
  });

  it("ignores an empty category", () => {
    expect(parseRecurringFilters({ category: " " })).toEqual({ status: "active", group: true });
  });

  it("turns grouping off only with group=0", () => {
    expect(parseRecurringFilters({ group: "0" }).group).toBe(false);
    expect(parseRecurringFilters({ group: "1" }).group).toBe(true);
    expect(parseRecurringFilters({ group: "off" }).group).toBe(true);
  });
});

describe("recurringFiltersHref", () => {
  it("leaves the defaults out", () => {
    expect(recurringFiltersHref({ status: "active", group: true })).toBe("/subscriptions");
    expect(recurringFiltersHref({})).toBe("/subscriptions");
  });

  it("keeps non-default values", () => {
    expect(recurringFiltersHref({ status: "all" })).toBe("/subscriptions?status=all");
    expect(recurringFiltersHref({ group: false })).toBe("/subscriptions?group=0");
    expect(recurringFiltersHref({ category: "none" })).toBe("/subscriptions?category=none");
  });

  it("combines every filter", () => {
    expect(recurringFiltersHref({ status: "paused", category: "c1", group: false })).toBe(
      "/subscriptions?status=paused&category=c1&group=0",
    );
  });
});
