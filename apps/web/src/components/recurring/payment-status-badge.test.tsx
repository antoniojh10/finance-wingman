import { screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { renderWithIntl } from "@/test/render";

import { PaymentStatusBadge } from "./payment-status-badge";

describe("PaymentStatusBadge", () => {
  it.each([
    ["paid", "Paid"],
    ["pending", "Pending"],
    ["overdue", "Overdue"],
  ] as const)("labels %s", (status, label) => {
    renderWithIntl(<PaymentStatusBadge status={status} />);
    expect(screen.getByText(label)).toHaveAttribute("data-status", status);
  });

  it("is localized", () => {
    renderWithIntl(<PaymentStatusBadge status="overdue" />, { locale: "es" });
    expect(screen.getByText("Vencido")).toBeInTheDocument();
  });
});
