import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { renderWithIntl } from "@/test/render";

import { ProfileForm } from "./profile-form";

const refresh = vi.fn();
const updateProfile = vi.fn();
vi.mock("next/navigation", () => ({ useRouter: () => ({ refresh }) }));
vi.mock("@/app/actions/profile", () => ({ updateProfile: (...args: unknown[]) => updateProfile(...args) }));

describe("ProfileForm", () => {
  it("saves the name and language, then refreshes the page", async () => {
    updateProfile.mockResolvedValue({ ok: true, nonce: 1 });
    const user = userEvent.setup();
    renderWithIntl(<ProfileForm name="Ana" email="ana@example.com" locale="en" />);

    expect(screen.getByLabelText("Email")).toBeDisabled();
    expect(screen.getByRole("radio", { name: "English" })).toBeChecked();
    await user.clear(screen.getByLabelText("Name"));
    await user.type(screen.getByLabelText("Name"), "Ana María");
    await user.click(screen.getByRole("radio", { name: "Español" }));
    await user.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() => expect(refresh).toHaveBeenCalled());
    const data = updateProfile.mock.calls[0][1] as FormData;
    expect(Object.fromEntries(data.entries())).toEqual({ name: "Ana María", locale: "es" });
  });
});
