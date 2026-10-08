import { describe, expect, it } from "vitest";

import { databaseName, isStale, newRunId, prefix, withDatabase } from "./e2e-database.mjs";

const day = 24 * 60 * 60 * 1000;

describe("e2e database names", () => {
  it("builds unique, time-ordered names", () => {
    const a = databaseName(newRunId(1000));
    const b = databaseName(newRunId(1000));
    expect(a.startsWith(`${prefix}1000_`)).toBe(true);
    expect(a).not.toBe(b);
  });

  it("points a connection URL at another database", () => {
    expect(withDatabase("postgres://u:p@localhost:5432/postgres?sslmode=disable", "x")).toBe(
      "postgres://u:p@localhost:5432/x?sslmode=disable",
    );
  });

  it("flags only old e2e databases as stale", () => {
    const now = 10 * day;
    expect(isStale(databaseName(newRunId(now - 2 * day)), now)).toBe(true);
    expect(isStale(databaseName(newRunId(now - 1000)), now)).toBe(false);
    expect(isStale("finance_test", now)).toBe(false);
    expect(isStale(`${prefix}garbage`, now)).toBe(false);
  });
});
