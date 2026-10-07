import { screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { renderWithIntl } from "@/test/render";

import { OwnerFilter } from "./owner-filter";

const owners = [
  { id: "u1", name: "Luis" },
  { id: "u2", name: "Ana" },
];
const hrefFor = (owner: string | undefined) => (owner ? `/?owner=${owner}` : "/");

describe("OwnerFilter", () => {
  it("links to everyone, the user's own, each other member's and the shared accounts", () => {
    renderWithIntl(<OwnerFilter owners={owners} userId="u1" value="u2" hrefFor={hrefFor} />);
    const links = screen.getAllByRole("link");
    expect(links.map((l) => l.textContent)).toEqual(["Everyone", "Mine", "Ana", "Shared"]);
    expect(screen.getByRole("link", { name: "Everyone" })).toHaveAttribute("href", "/");
    expect(screen.getByRole("link", { name: "Mine" })).toHaveAttribute("href", "/?owner=u1");
    expect(screen.getByRole("link", { name: "Shared" })).toHaveAttribute("href", "/?owner=shared");
    expect(screen.getByRole("link", { name: "Ana" })).toHaveAttribute("aria-current", "page");
  });

  it("renders nothing for a single-member workspace", () => {
    const { container } = renderWithIntl(<OwnerFilter owners={[owners[0]]} userId="u1" hrefFor={hrefFor} />);
    expect(container).toBeEmptyDOMElement();
  });
});
