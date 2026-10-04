import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { renderWithIntl } from "@/test/render";

import { LoginForm } from "./login-form";

const requestLogin = vi.fn();
const verifyCode = vi.fn();
vi.mock("@/app/actions/auth", () => ({
  requestLogin: (...args: unknown[]) => requestLogin(...args),
  verifyCode: (...args: unknown[]) => verifyCode(...args),
}));

beforeEach(() => {
  requestLogin.mockReset();
  verifyCode.mockReset();
});

describe("LoginForm", () => {
  it("moves from email to code entry", async () => {
    requestLogin.mockResolvedValue({ step: "code", email: "ana@example.com" });
    verifyCode.mockResolvedValue({ step: "code", email: "ana@example.com", error: "That code is invalid or has expired." });
    const user = userEvent.setup();
    renderWithIntl(<LoginForm />);

    await user.type(screen.getByLabelText("Email"), "ana@example.com");
    await user.click(screen.getByRole("button", { name: "Email me a sign-in link" }));

    expect(await screen.findByText("Check your email")).toBeInTheDocument();
    expect(screen.getByText(/If ana@example.com has access/)).toBeInTheDocument();

    await user.type(screen.getByLabelText("6-digit code"), "123456");
    await user.click(screen.getByRole("button", { name: "Sign in" }));
    expect(await screen.findByText("That code is invalid or has expired.")).toBeInTheDocument();
    const data = verifyCode.mock.calls[0][1] as FormData;
    expect(data.get("email")).toBe("ana@example.com");
    expect(data.get("code")).toBe("123456");

    await user.click(screen.getByRole("button", { name: "Use a different email" }));
    expect(screen.getByLabelText("Email")).toHaveValue("ana@example.com");
  });

  it("blocks invalid emails in the browser", async () => {
    const user = userEvent.setup();
    renderWithIntl(<LoginForm />);
    await user.type(screen.getByLabelText("Email"), "nope");
    await user.click(screen.getByRole("button", { name: "Email me a sign-in link" }));
    expect(requestLogin).not.toHaveBeenCalled();
  });

  it("shows request errors on the email step", async () => {
    // "ana@example" passes the browser's email check but not the server's.
    requestLogin.mockResolvedValue({ step: "email", email: "ana@example", error: "Escribe un correo electrónico válido." });
    const user = userEvent.setup();
    renderWithIntl(<LoginForm />, { locale: "es" });

    await user.type(screen.getByLabelText("Correo electrónico"), "ana@example");
    await user.click(screen.getByRole("button", { name: "Enviarme un enlace de acceso" }));
    expect(await screen.findByText("Escribe un correo electrónico válido.")).toBeInTheDocument();
    expect(requestLogin).toHaveBeenCalledTimes(1);
  });
});
