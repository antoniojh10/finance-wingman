import { screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import type { ActivityEntry } from "@/lib/api/client";
import { renderWithIntl } from "@/test/render";

import { ActivityItem } from "./activity-item";

const names = { a1: "Checking", a2: "Savings" };

function entry(overrides: Partial<ActivityEntry> = {}): ActivityEntry {
  return {
    id: "e1",
    action: "transaction.created",
    entity_type: "transaction",
    entity_id: "t1",
    channel: "web",
    client_id: null,
    client_name: null,
    actor: { id: "u1", name: "Ana", email: "ana@example.com" },
    created_at: "2026-10-08T14:30:00Z",
    details: { type: "expense", account_id: "a1" },
    ...overrides,
  };
}

function renderItem(e: ActivityEntry, locale: "en" | "es" = "en") {
  return renderWithIntl(
    <ul>
      <ActivityItem entry={e} accountNames={names} />
    </ul>,
    { locale },
  );
}

describe("ActivityItem", () => {
  it("shows who did what, the type, the account name and the date", () => {
    renderItem(entry());
    const item = screen.getByTestId("activity-item");
    expect(item).toHaveTextContent("Ana created a transaction");
    expect(item).toHaveTextContent("Expense · Checking");
    expect(item).toHaveTextContent("Oct 8, 2026, 2:30 PM");
    expect(item).toHaveAttribute("data-channel", "web");
    expect(screen.queryByTestId("activity-mcp-badge")).not.toBeInTheDocument();
  });

  it("highlights MCP changes with the client name", () => {
    renderItem(entry({ channel: "mcp", client_id: "c1", client_name: "Claude" }));
    expect(screen.getByTestId("activity-mcp-badge")).toHaveTextContent("via Claude");
    expect(screen.getByTestId("activity-item")).toHaveAttribute("data-channel", "mcp");
  });

  it("names the MCP channel when the client is unknown", () => {
    renderItem(entry({ channel: "mcp" }));
    expect(screen.getByTestId("activity-mcp-badge")).toHaveTextContent("via AI assistant");
  });

  it("shows Former member when the actor was erased", () => {
    renderItem(entry({ actor: undefined }));
    expect(screen.getByTestId("activity-item")).toHaveTextContent("Former member created a transaction");
  });

  it("falls back to the email when the actor has no name", () => {
    renderItem(entry({ actor: { id: "u1", name: " ", email: "ana@example.com" } }));
    expect(screen.getByTestId("activity-item")).toHaveTextContent("ana@example.com created a transaction");
  });

  it("shows transfers as source to destination", () => {
    renderItem(entry({ details: { type: "transfer", account_id: "a1", destination_account_id: "a2" } }));
    expect(screen.getByTestId("activity-item")).toHaveTextContent("Transfer · Checking → Savings");
  });

  it("omits accounts that no longer resolve", () => {
    renderItem(entry({ details: { type: "income", account_id: "gone" } }));
    expect(screen.getByTestId("activity-item")).toHaveTextContent(/^.*Income(?! ·).*$/);
    expect(screen.getByTestId("activity-item")).not.toHaveTextContent("gone");
  });

  it("lists changed fields with human labels, falling back to the field name", () => {
    renderItem(entry({ action: "transaction.updated", details: { type: "expense", account_id: "a1", changed: ["amount", "occurred_on", "tax_code"] } }));
    expect(screen.getByTestId("activity-item")).toHaveTextContent("updated a transaction");
    expect(screen.getByText("Changed: Amount, Date, tax code")).toBeInTheDocument();
  });

  it("renders unknown actions and entity types with a generic sentence", () => {
    renderItem(entry({ action: "budget.updated", entity_type: "budget", details: {} }));
    expect(screen.getByTestId("activity-item")).toHaveTextContent('Ana did "budget updated" on budget');
  });

  it("is localized", () => {
    renderItem(entry({ action: "transaction.deleted" }), "es");
    expect(screen.getByTestId("activity-item")).toHaveTextContent("Ana eliminó una transacción");
    expect(screen.getByTestId("activity-item")).toHaveTextContent("Gasto · Checking");
  });
});
