import { describe, expect, it } from "vitest";

import { accountNamesFor, actionLabel, activityHref, entityTypeLabel, fieldLabel, humanize, parseActivityQuery, type Translate } from "./activity";
import type { ActivityEntry } from "./api/client";

const messages: Record<string, string> = {
  "entityTypes.transaction": "Transactions",
  "actions.transaction.created": "created a transaction",
  "fields.occurred_on": "Date",
  unknownAction: 'did "{action}" on {entity}',
};
const t = Object.assign(
  (key: string, values: Record<string, string | number> = {}) =>
    (messages[key] ?? key).replace(/\{(\w+)\}/g, (_, name) => String(values[name])),
  { has: (key: string) => key in messages },
) as Translate;

const actorId = "3f2b8c1e-5a4d-4c3b-9e8f-1a2b3c4d5e6f";

describe("parseActivityQuery", () => {
  it("keeps valid filters", () => {
    expect(parseActivityQuery({ actor_id: actorId, channel: "mcp", entity_type: "transaction" })).toEqual({
      actor_id: actorId,
      channel: "mcp",
      entity_type: "transaction",
    });
  });

  it("drops invalid and unknown values", () => {
    expect(parseActivityQuery({ actor_id: "nope", channel: "email", entity_type: "unicorn" })).toEqual({
      actor_id: undefined,
      channel: undefined,
      entity_type: undefined,
    });
  });

  it("takes the first value of a repeated param", () => {
    expect(parseActivityQuery({ channel: ["web", "mcp"] }).channel).toBe("web");
  });
});

describe("activityHref", () => {
  it("changes one filter and keeps the others", () => {
    expect(activityHref({ channel: "mcp", actor_id: actorId }, { entity_type: "transaction" })).toBe(
      `?channel=mcp&actor_id=${actorId}&entity_type=transaction`,
    );
  });

  it("clears a filter", () => {
    expect(activityHref({ channel: "mcp" }, { channel: undefined })).toBe("?");
  });
});

describe("labels", () => {
  it("uses the copy of known actions, entity types and fields", () => {
    expect(actionLabel(t, "transaction.created", "transaction")).toBe("created a transaction");
    expect(entityTypeLabel(t, "transaction")).toBe("Transactions");
    expect(fieldLabel(t, "occurred_on")).toBe("Date");
  });

  it("falls back to a generic sentence for unknown actions and entity types", () => {
    expect(actionLabel(t, "budget.updated", "budget")).toBe('did "budget updated" on budget');
    expect(entityTypeLabel(t, "recurring_item")).toBe("recurring item");
    expect(fieldLabel(t, "estimated_amount")).toBe("estimated amount");
  });

  it("humanizes identifiers", () => {
    expect(humanize("export.requested")).toBe("export requested");
  });
});

describe("accountNamesFor", () => {
  const entry = {
    details: { account_id: "a1", destination_account_id: "a2" },
  } as unknown as ActivityEntry;

  it("resolves ids and skips the ones that no longer exist", () => {
    expect(accountNamesFor(entry, { a1: "Checking", a2: "Savings" })).toEqual({ account: "Checking", destination: "Savings" });
    expect(accountNamesFor(entry, { a1: "Checking" })).toEqual({ account: "Checking", destination: undefined });
  });
});
