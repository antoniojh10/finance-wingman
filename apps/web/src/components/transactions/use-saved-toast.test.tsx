import { renderHook } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { beforeEach, describe, expect, it, vi } from "vitest";

import en from "../../../messages/en.json";

import { useSavedToast } from "./use-saved-toast";

const findMatch = vi.fn();
const link = vi.fn();
const toast = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }));
vi.mock("@/app/actions/transactions", () => ({
  findRecurringMatch: (...args: unknown[]) => findMatch(...args),
  linkTransactionRecurring: (...args: unknown[]) => link(...args),
}));
vi.mock("sonner", () => ({ toast }));

function setup() {
  const wrapper = ({ children }: { children: React.ReactNode }) => (
    <NextIntlClientProvider locale="en" messages={en}>
      {children}
    </NextIntlClientProvider>
  );
  return renderHook(() => useSavedToast(), { wrapper }).result.current;
}

beforeEach(() => {
  findMatch.mockReset();
  link.mockReset();
  toast.success.mockReset();
  toast.error.mockReset();
});

describe("useSavedToast", () => {
  it("shows a plain toast for edits without looking for a match", async () => {
    await setup()(undefined);
    expect(findMatch).not.toHaveBeenCalled();
    expect(toast.success).toHaveBeenCalledWith("Transaction saved");
  });

  it("shows a plain toast when nothing matches", async () => {
    findMatch.mockResolvedValue(null);
    await setup()("tx1");
    expect(toast.success).toHaveBeenCalledWith("Transaction saved");
  });

  it("offers to link a matching subscription and links it on click", async () => {
    findMatch.mockResolvedValue({ recurringId: "r1", name: "Netflix", period: "2026-10-05" });
    link.mockResolvedValue({ ok: true, nonce: 1 });
    await setup()("tx1");

    const [message, options] = toast.success.mock.calls[0];
    expect(message).toBe("Transaction saved");
    expect(options.action.label).toBe("Link to Netflix?");

    await options.action.onClick();
    expect(link).toHaveBeenCalledWith("tx1", "r1", "2026-10-05");
    expect(toast.success).toHaveBeenLastCalledWith("Linked to Netflix");
  });

  it("shows the budget warning in the toast, with or without a subscription match", async () => {
    findMatch.mockResolvedValue(null);
    await setup()("tx1", "Food: this goes over budget by MX$10.00.");
    expect(toast.success).toHaveBeenCalledWith("Transaction saved", { description: "Food: this goes over budget by MX$10.00.", duration: 8000 });

    findMatch.mockResolvedValue({ recurringId: "r1", name: "Netflix", period: null });
    await setup()("tx2", "Food: warning");
    expect(toast.success.mock.calls[1][1]).toMatchObject({ description: "Food: warning" });
    expect(toast.success.mock.calls[1][1].action.label).toBe("Link to Netflix?");
  });

  it("reports a refused link", async () => {
    findMatch.mockResolvedValue({ recurringId: "r1", name: "Netflix", period: null });
    link.mockResolvedValue({ ok: false, message: "nope" });
    await setup()("tx1");
    await toast.success.mock.calls[0][1].action.onClick();
    expect(link).toHaveBeenCalledWith("tx1", "r1", undefined);
    expect(toast.error).toHaveBeenCalledWith("nope");
  });
});
