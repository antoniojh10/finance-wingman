import { describe, expect, it } from "vitest";

import { toAccountSeries } from "./view-models";

describe("toAccountSeries", () => {
  it("indexes each account's daily balances by account id", () => {
    const series = toAccountSeries({
      accounts: [
        { account_id: "a", days: [{ date: "2026-11-19", balance: 100 }, { date: "2026-11-20", balance: 150 }] },
        { account_id: "b", days: [] },
      ],
    });
    expect(series.get("a")).toHaveLength(2);
    expect(series.get("b")).toEqual([]);
    expect(series.get("missing")).toBeUndefined();
  });
});
