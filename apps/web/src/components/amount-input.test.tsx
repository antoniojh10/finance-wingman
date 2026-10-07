import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";

import { AmountInput } from "./amount-input";

function setup(props: Parameters<typeof AmountInput>[0] = {}) {
  render(<AmountInput aria-label="Amount" {...props} />);
  return screen.getByLabelText("Amount") as HTMLInputElement;
}

describe("AmountInput", () => {
  it("groups thousands with spaces while typing", async () => {
    const input = setup();
    await userEvent.type(input, "1234567.89");
    expect(input).toHaveValue("1 234 567.89");
  });

  it("formats the default value", () => {
    expect(setup({ defaultValue: "25000.00" })).toHaveValue("25 000.00");
  });

  it("drops characters that are not part of an amount", async () => {
    const input = setup();
    await userEvent.type(input, "1a0b00");
    expect(input).toHaveValue("1 000");
  });

  it("keeps the caret after the edited digit", async () => {
    const input = setup({ defaultValue: "1000" });
    // Caret right after the first "1", in "1| 000".
    await userEvent.type(input, "5", { initialSelectionStart: 1, initialSelectionEnd: 1 });
    expect(input).toHaveValue("15 000");
    expect(input.selectionStart).toBe(2);
  });

  it("deletes the digit before a group separator on backspace", async () => {
    const input = setup({ defaultValue: "12345" });
    // Caret right after the space, in "12 |345".
    await userEvent.type(input, "{Backspace}", { initialSelectionStart: 3, initialSelectionEnd: 3 });
    expect(input).toHaveValue("1 345");
  });

  it("deletes the digit after a group separator on delete", async () => {
    const input = setup({ defaultValue: "12345" });
    // Caret right before the space, in "12| 345".
    await userEvent.type(input, "{Delete}", { initialSelectionStart: 2, initialSelectionEnd: 2 });
    expect(input).toHaveValue("1 245");
  });

  it("drops a minus sign unless signed", async () => {
    const input = setup();
    await userEvent.type(input, "-5000");
    expect(input).toHaveValue("5 000");
  });

  it("keeps a minus sign when signed", async () => {
    const input = setup({ signed: true });
    await userEvent.type(input, "-5000");
    expect(input).toHaveValue("-5 000");
  });
});
