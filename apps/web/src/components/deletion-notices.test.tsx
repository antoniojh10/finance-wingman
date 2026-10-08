import { screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { renderWithIntl } from "@/test/render";

import { DeletionNotices } from "./deletion-notices";

describe("DeletionNotices", () => {
  it("renders nothing without a scheduled deletion", () => {
    const { container } = renderWithIntl(<DeletionNotices workspace={{ name: "Home" }} />);
    expect(container).toBeEmptyDOMElement();
  });

  it("warns about a scheduled workspace deletion", () => {
    renderWithIntl(<DeletionNotices workspace={{ name: "Home", deletion_scheduled_for: "2026-10-15T12:00:00Z" }} />);
    expect(screen.getByRole("status")).toHaveTextContent("Home will be deleted on October 15, 2026.");
    expect(screen.getByRole("link", { name: "Review" })).toHaveAttribute("href", "/settings/workspace");
  });

  it("warns about a scheduled account deletion first", () => {
    renderWithIntl(
      <DeletionNotices accountDeletion="2026-10-16T12:00:00Z" workspace={{ name: "Home", deletion_scheduled_for: "2026-10-15T12:00:00Z" }} />,
    );
    const [account, workspace] = screen.getAllByRole("status");
    expect(account).toHaveTextContent("Your account will be deleted on October 16, 2026.");
    expect(workspace).toHaveTextContent("Home will be deleted");
    expect(screen.getAllByRole("link", { name: "Review" })[0]).toHaveAttribute("href", "/settings/security");
  });

  it("is translated", () => {
    renderWithIntl(<DeletionNotices accountDeletion="2026-10-16T12:00:00Z" />, { locale: "es" });
    expect(screen.getByRole("status")).toHaveTextContent("Tu cuenta se eliminará el 16 de octubre de 2026.");
  });
});
