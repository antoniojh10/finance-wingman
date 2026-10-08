import { describe, expect, it } from "vitest";

import { DELETION_GRACE_DAYS, deletionDate } from "./deletion";

describe("deletionDate", () => {
  it("adds the grace period", () => {
    expect(DELETION_GRACE_DAYS).toBe(7);
    expect(deletionDate(Date.UTC(2026, 9, 8, 12)).toISOString()).toBe("2026-10-15T12:00:00.000Z");
  });
});
