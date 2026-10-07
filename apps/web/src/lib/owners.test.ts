import { describe, expect, it } from "vitest";

import { accountLabel, ownerName, parseOwner, toOwnerOption } from "./owners";
import { toAccountOption, toTransactionRow } from "./view-models";
import type { Account, Transaction } from "./api/client";

const ana = { id: "u2", name: "Ana", email: "ana@example.com" };

describe("owners", () => {
  it("names members by name, or email when they have none", () => {
    expect(ownerName(ana)).toBe("Ana");
    expect(ownerName({ name: "  ", email: "luis@example.com" })).toBe("luis@example.com");
    expect(toOwnerOption({ user_id: "u2", name: "", email: "ana@example.com" })).toEqual({ id: "u2", name: "ana@example.com" });
  });

  it("labels members' accounts with their owner only when asked", () => {
    expect(accountLabel("BNP", ana, true)).toBe("BNP (Ana)");
    expect(accountLabel("BNP", ana, false)).toBe("BNP");
    expect(accountLabel("Joint", undefined, true)).toBe("Joint");
  });

  it("accepts a user id or shared as owner filter", () => {
    expect(parseOwner("shared")).toBe("shared");
    expect(parseOwner(["3f2b8c1e-5a4d-4c3b-9e8f-1a2b3c4d5e6f"])).toBe("3f2b8c1e-5a4d-4c3b-9e8f-1a2b3c4d5e6f");
    expect(parseOwner("Ana")).toBeUndefined();
    expect(parseOwner(undefined)).toBeUndefined();
  });

  it("labels accounts and transactions in view models", () => {
    const account = { id: "a", name: "BNP", currency: "EUR", minor_units: 2, archived: false, owner: ana } as Account;
    expect(toAccountOption(account, true).name).toBe("BNP (Ana)");
    expect(toAccountOption(account).name).toBe("BNP");

    const tx = {
      id: "t",
      type: "transfer",
      account_name: "BNP",
      account_owner: ana,
      destination_account_name: "Joint",
      destination_account_owner: undefined,
    } as unknown as Transaction;
    const row = toTransactionRow(tx, true);
    expect(row.account_name).toBe("BNP (Ana)");
    expect(row.destination_account_name).toBe("Joint");
  });
});
