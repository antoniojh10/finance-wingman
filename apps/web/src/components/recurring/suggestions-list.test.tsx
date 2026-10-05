import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { renderWithIntl } from "@/test/render";

import { SuggestionsList, type SuggestionRow } from "./suggestions-list";

const acceptSuggestion = vi.fn();
const dismissSuggestion = vi.fn();
const toastSuccess = vi.fn();
const toastError = vi.fn();
vi.mock("sonner", () => ({
  toast: { success: (...args: unknown[]) => toastSuccess(...args), error: (...args: unknown[]) => toastError(...args) },
}));
vi.mock("@/app/actions/recurring", () => ({
  acceptSuggestion: (...args: unknown[]) => acceptSuggestion(...args),
  dismissSuggestion: (...args: unknown[]) => dismissSuggestion(...args),
}));

const suggestion = (key: string, name: string, overrides: Partial<SuggestionRow> = {}): SuggestionRow => ({
  key,
  name,
  type: "expense",
  account_name: "Checking",
  category_name: "Fun",
  currency: "MXN",
  minor_units: 2,
  amount: 19900,
  interval_unit: "month",
  interval_count: 1,
  transaction_ids: ["t1", "t2", "t3"],
  ...overrides,
});

const list = [suggestion("k1", "Spotify"), suggestion("k2", "Gym", { transaction_ids: ["t4"], amount: 50000 })];

beforeEach(() => vi.clearAllMocks());

describe("SuggestionsList", () => {
  it("renders nothing without suggestions", () => {
    renderWithIntl(<SuggestionsList suggestions={[]} />);
    expect(screen.queryByTestId("suggestions")).not.toBeInTheDocument();
  });

  it("shows each suggestion with its cadence, amount and match count", () => {
    renderWithIntl(<SuggestionsList suggestions={list} />);
    const [spotify, gym] = screen.getAllByTestId("suggestion-row");
    expect(within(spotify).getByText("Spotify")).toBeInTheDocument();
    expect(within(spotify).getByText("−MX$199.00")).toBeInTheDocument();
    expect(within(spotify).getByText(/Every month · Checking · Fun · 3 matching transactions/)).toBeInTheDocument();
    expect(within(gym).getByText(/1 matching transaction$/)).toBeInTheDocument();
  });

  it("opens a dialog prefilled with the suggested name and amount", async () => {
    const user = userEvent.setup();
    renderWithIntl(<SuggestionsList suggestions={list} />);
    await user.click(within(screen.getAllByTestId("suggestion-row")[1]).getByRole("button", { name: "Add" }));
    expect(await screen.findByRole("dialog", { name: "Add subscription: Gym" })).toBeInTheDocument();
    expect(screen.getByLabelText("Name")).toHaveValue("Gym");
    expect(screen.getByLabelText("Amount (MXN)")).toHaveValue("500.00");
  });

  it("submits the edited values and keeps the dialog until the row is gone", async () => {
    const user = userEvent.setup();
    const view = renderWithIntl(<SuggestionsList suggestions={list} />);
    // The revalidated list (without the accepted suggestion) arrives with the result.
    acceptSuggestion.mockImplementation(async () => {
      view.rerender(<SuggestionsList suggestions={[list[1]]} />);
      return { ok: true, nonce: 1 };
    });
    await user.click(within(screen.getAllByTestId("suggestion-row")[0]).getByRole("button", { name: "Add" }));
    await user.clear(await screen.findByLabelText("Name"));
    await user.type(screen.getByLabelText("Name"), "Spotify Premium");
    await user.clear(screen.getByLabelText("Amount (MXN)"));
    await user.type(screen.getByLabelText("Amount (MXN)"), "210.5");
    await user.click(screen.getByRole("button", { name: "Add subscription" }));

    await waitFor(() => expect(toastSuccess).toHaveBeenCalledWith("Subscription added"));
    const sent = acceptSuggestion.mock.calls[0][1] as FormData;
    expect(Object.fromEntries(sent.entries())).toEqual({ key: "k1", name: "Spotify Premium", amount: "210.5" });
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    expect(screen.getAllByTestId("suggestion-row")).toHaveLength(1);
  });

  it("shows the API message and keeps the dialog open on failure", async () => {
    acceptSuggestion.mockResolvedValue({ ok: false, message: "name already in use" });
    const user = userEvent.setup();
    renderWithIntl(<SuggestionsList suggestions={list} />);
    await user.click(within(screen.getAllByTestId("suggestion-row")[0]).getByRole("button", { name: "Add" }));
    await user.click(await screen.findByRole("button", { name: "Add subscription" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("name already in use");
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    expect(toastSuccess).not.toHaveBeenCalled();
  });

  it("dismisses a suggestion", async () => {
    dismissSuggestion.mockResolvedValue({ ok: true, nonce: 1 });
    const user = userEvent.setup();
    renderWithIntl(<SuggestionsList suggestions={list} />);
    await user.click(within(screen.getAllByTestId("suggestion-row")[1]).getByRole("button", { name: "Dismiss" }));
    await waitFor(() => expect(dismissSuggestion).toHaveBeenCalledWith("k2"));
    await waitFor(() => expect(toastSuccess).toHaveBeenCalledWith("Suggestion dismissed"));
  });

  it("shows an error toast when dismissing fails", async () => {
    dismissSuggestion.mockResolvedValue({ ok: false, message: "This item no longer exists." });
    const user = userEvent.setup();
    renderWithIntl(<SuggestionsList suggestions={list} />);
    await user.click(within(screen.getAllByTestId("suggestion-row")[0]).getByRole("button", { name: "Dismiss" }));
    await waitFor(() => expect(toastError).toHaveBeenCalledWith("This item no longer exists."));
  });
});
