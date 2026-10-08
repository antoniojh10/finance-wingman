import { screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { renderWithIntl } from "@/test/render";

import { ActivityFilters } from "./activity-filters";

const members = [
  { id: "3f2b8c1e-5a4d-4c3b-9e8f-1a2b3c4d5e6f", name: "Ana" },
  { id: "4f2b8c1e-5a4d-4c3b-9e8f-1a2b3c4d5e6f", name: "Luis" },
];

describe("ActivityFilters", () => {
  it("links each option to the filtered feed, keeping the other filters", () => {
    renderWithIntl(<ActivityFilters query={{ channel: "mcp" }} members={members} />);
    const member = within(screen.getByRole("navigation", { name: "Member" }));
    expect(member.getByRole("link", { name: "Ana" })).toHaveAttribute("href", `?channel=mcp&actor_id=${members[0].id}`);
    const channel = within(screen.getByRole("navigation", { name: "Channel" }));
    expect(channel.getByRole("link", { name: "AI assistants" })).toHaveAttribute("aria-current", "page");
    expect(channel.getByRole("link", { name: "All" })).toHaveAttribute("href", "?");
    const entity = within(screen.getByRole("navigation", { name: "Type of record" }));
    expect(entity.getByRole("link", { name: "Transactions" })).toHaveAttribute("href", "?channel=mcp&entity_type=transaction");
  });

  it("hides the member filter in single-member workspaces", () => {
    renderWithIntl(<ActivityFilters query={{}} members={[members[0]]} />);
    expect(screen.queryByRole("navigation", { name: "Member" })).not.toBeInTheDocument();
    expect(screen.getByRole("navigation", { name: "Channel" })).toBeInTheDocument();
  });

  it("marks the active member", () => {
    renderWithIntl(<ActivityFilters query={{ actor_id: members[1].id }} members={members} />);
    expect(screen.getByRole("link", { name: "Luis" })).toHaveAttribute("aria-current", "page");
  });
});
