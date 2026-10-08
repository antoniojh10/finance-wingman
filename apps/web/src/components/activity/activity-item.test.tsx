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
      <ActivityItem entry={e} accountNames={names} categoryNames={{ c1: "Food" }} />
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
    renderItem(entry({ action: "goal.reached", entity_type: "goal", details: {} }));
    expect(screen.getByTestId("activity-item")).toHaveTextContent('Ana did "goal reached" on goal');
  });

  it("resolves the account of account entries and shows its type", () => {
    renderItem(entry({ action: "account.archived", entity_type: "account", entity_id: "a2", details: { type: "savings", currency: "EUR" } }));
    expect(screen.getByTestId("activity-item")).toHaveTextContent("Ana archived an account");
    expect(screen.getByTestId("activity-item")).toHaveTextContent("Savings · Savings · EUR");
  });

  it("shows the category, currency and month of a budget", () => {
    renderItem(entry({ action: "budget.set", entity_type: "budget", entity_id: "c1", details: { category_id: "c1", currency: "MXN", month: "2026-10" } }));
    expect(screen.getByTestId("activity-item")).toHaveTextContent("Ana set a budget");
    expect(screen.getByTestId("activity-item")).toHaveTextContent("Food · MXN · October 2026");
  });

  it("formats the month per locale", () => {
    renderItem(entry({ action: "budget.cleared", entity_type: "budget", entity_id: "c1", details: { month: "2026-10" } }), "es");
    expect(screen.getByTestId("activity-item")).toHaveTextContent("quitó un presupuesto");
    expect(screen.getByTestId("activity-item")).toHaveTextContent("Food · octubre de 2026");
  });

  it("shows the format of an export", () => {
    renderItem(entry({ action: "export.requested", entity_type: "workspace", entity_id: "w1", details: { format: "csv" } }));
    expect(screen.getByTestId("activity-item")).toHaveTextContent("Ana requested a data export");
    expect(screen.getByTestId("activity-item")).toHaveTextContent("CSV");
  });

  it("labels the changed fields of accounts, categories and subscriptions", () => {
    renderItem(entry({ action: "recurring_item.updated", entity_type: "recurring_item", details: { type: "expense", changed: ["interval_unit", "total_payments", "category_id"] } }));
    expect(screen.getByText("Changed: Interval unit, Total payments, Category")).toBeInTheDocument();
  });

  it("has copy for every entity type and logged action", () => {
    const actions = [
      "account.created", "account.updated", "account.archived", "account.unarchived", "account.deleted",
      "category.created", "category.updated", "category.archived", "category.unarchived", "category.deleted",
      "budget.set", "budget.cleared", "recurring_item.created", "recurring_item.updated",
      "recurring_suggestion.dismissed", "export.requested",
    ];
    for (const action of actions) {
      const { unmount } = renderItem(entry({ action, entity_type: action.split(".")[0], details: {} }));
      expect(screen.getByTestId("activity-item")).not.toHaveTextContent("did \"");
      unmount();
    }
  });

  it("is localized", () => {
    renderItem(entry({ action: "transaction.deleted" }), "es");
    expect(screen.getByTestId("activity-item")).toHaveTextContent("Ana eliminó una transacción");
    expect(screen.getByTestId("activity-item")).toHaveTextContent("Gasto · Checking");
  });
});
