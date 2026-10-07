"use client";

import type { ChangeEvent, ComponentProps, KeyboardEvent } from "react";

import { Input } from "@/components/ui/input";
import { formatAmountInput } from "@/lib/money";

const significant = /[\d.,-]/;

/**
 * Event handlers that keep an uncontrolled amount input grouped in thousands
 * ("1 000.50") while typing, preserving the caret position.
 */
export function amountInputHandlers(signed = false) {
  return {
    onChange(event: ChangeEvent<HTMLInputElement>) {
      const input = event.currentTarget;
      const caret = input.selectionStart ?? input.value.length;
      // Characters other than spaces before the caret, to restore it after
      // the spaces move.
      const before = [...input.value.slice(0, caret)].filter((c) => significant.test(c)).length;
      const formatted = formatAmountInput(input.value, signed);
      if (formatted === input.value) {
        return;
      }
      input.value = formatted;
      let position = 0;
      for (let seen = 0; position < formatted.length && seen < before; position++) {
        if (significant.test(formatted[position])) {
          seen++;
        }
      }
      input.setSelectionRange(position, position);
    },
    onKeyDown(event: KeyboardEvent<HTMLInputElement>) {
      // Deleting next to a group separator removes the digit beyond it
      // instead of the space, which formatting would put back.
      const input = event.currentTarget;
      const { selectionStart: start, selectionEnd: end } = input;
      if (start === null || start !== end) {
        return;
      }
      if (event.key === "Backspace" && input.value[start - 1] === " ") {
        input.setSelectionRange(start - 1, start - 1);
      } else if (event.key === "Delete" && input.value[start] === " ") {
        input.setSelectionRange(start + 1, start + 1);
      }
    },
  };
}

type AmountInputProps = Omit<ComponentProps<typeof Input>, "defaultValue" | "onChange" | "onKeyDown"> & {
  defaultValue?: string;
  /** Allows a leading minus sign, e.g. for an opening balance. */
  signed?: boolean;
};

/** Text input for decimal amounts that groups thousands with spaces. */
export function AmountInput({ defaultValue = "", signed = false, ...props }: AmountInputProps) {
  return (
    <Input
      inputMode="decimal"
      autoComplete="off"
      defaultValue={formatAmountInput(defaultValue, signed)}
      {...amountInputHandlers(signed)}
      {...props}
    />
  );
}
