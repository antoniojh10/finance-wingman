import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { ActivityEntry } from "@/lib/api/client";
import { renderWithIntl } from "@/test/render";

import { ActivityFeed } from "./activity-feed";

const loadMoreActivity = vi.fn();
vi.mock("@/app/actions/activity", () => ({
  loadMoreActivity: (...args: unknown[]) => loadMoreActivity(...args),
}));

function entry(id: string): ActivityEntry {
  return {
    id,
    action: "transaction.created",
    entity_type: "transaction",
    entity_id: `t-${id}`,
    channel: "web",
    client_id: null,
    client_name: null,
    actor: { id: "u1", name: "Ana", email: "ana@example.com" },
    created_at: "2026-10-08T14:30:00Z",
    details: { type: "expense" },
  };
}

beforeEach(() => {
  loadMoreActivity.mockReset();
});

describe("ActivityFeed", () => {
  it("shows an empty state", () => {
    renderWithIntl(<ActivityFeed initialItems={[]} initialCursor={null} query={{}} accountNames={{}} />);
    expect(screen.getByTestId("activity-empty")).toHaveTextContent("No activity yet.");
    expect(screen.queryByRole("button", { name: "Load more" })).not.toBeInTheDocument();
  });

  it("has no Load more on the last page", () => {
    renderWithIntl(<ActivityFeed initialItems={[entry("1")]} initialCursor={null} query={{}} accountNames={{}} />);
    expect(screen.getAllByTestId("activity-item")).toHaveLength(1);
    expect(screen.queryByRole("button", { name: "Load more" })).not.toBeInTheDocument();
  });

  it("appends the next page with the cursor and the active filters", async () => {
    loadMoreActivity.mockResolvedValue({ items: [entry("2")], nextCursor: null });
    renderWithIntl(<ActivityFeed initialItems={[entry("1")]} initialCursor="cur1" query={{ channel: "mcp" }} accountNames={{}} />);
    await userEvent.click(screen.getByRole("button", { name: "Load more" }));
    await waitFor(() => expect(screen.getAllByTestId("activity-item")).toHaveLength(2));
    expect(loadMoreActivity).toHaveBeenCalledWith({ channel: "mcp" }, "cur1");
    expect(screen.queryByRole("button", { name: "Load more" })).not.toBeInTheDocument();
  });

  it("reports a failed load and keeps the button", async () => {
    loadMoreActivity.mockImplementation(async () => {
      throw new Error("boom");
    });
    renderWithIntl(<ActivityFeed initialItems={[entry("1")]} initialCursor="cur1" query={{}} accountNames={{}} />);
    await userEvent.click(screen.getByRole("button", { name: "Load more" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Couldn't load more activity");
    expect(screen.getByRole("button", { name: "Load more" })).toBeEnabled();
  });
});
