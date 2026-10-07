import { expect, test } from "@playwright/test";

import { e2eUser } from "../playwright.config";
import { codeFrom, linkFrom, navigate, requestLogin, signIn } from "./helpers";

// These flows start signed out, and signing out ends the session, so they
// don't use the session saved by auth.setup.ts.
test.use({ storageState: { cookies: [], origins: [] } });

test("protected pages redirect to sign-in", async ({ page }) => {
  await page.goto("/transactions");
  await expect(page).toHaveURL(/\/login/);
});

test("rejects a wrong code", async ({ page }) => {
  const text = await requestLogin(page, e2eUser);
  const wrong = codeFrom(text) === "000000" ? "111111" : "000000";
  await page.getByLabel(/6-digit code/i).fill(wrong);
  await page.getByRole("button", { name: /^sign in$/i }).click();
  await expect(page.getByText(/that code is invalid or has expired/i)).toBeVisible();
});

test("signs in with the magic link", async ({ page }) => {
  const text = await requestLogin(page, e2eUser);
  await page.goto(linkFrom(text).replace(/^https?:\/\/[^/]+/, ""));
  await page.getByRole("button", { name: /continue/i }).click();
  await expect(page).toHaveURL(/\/$/);
  await expect(page.getByRole("heading", { name: "Dashboard" })).toBeVisible();
});

test("switches language and signs out", async ({ page }) => {
  await signIn(page, e2eUser);
  await navigate(page, /settings/i);
  await expect(page.getByTestId("mcp-url")).toHaveText("http://localhost:8081/mcp");

  await page.locator("label", { hasText: "Español" }).click();
  await page.getByRole("button", { name: "Save" }).click();
  await expect(page.getByRole("heading", { name: "Ajustes" })).toBeVisible();
  await expect(page.getByRole("navigation", { name: "Main" }).first()).toContainText("Movimientos");

  // Restore English for other tests.
  await page.locator("label", { hasText: "English" }).click();
  await page.getByRole("button", { name: "Guardar" }).click();
  await expect(page.getByRole("heading", { name: "Settings" })).toBeVisible();

  await page.getByRole("main").getByRole("button", { name: "Sign out" }).click();
  await expect(page).toHaveURL(/\/login/);
  await page.goto("/");
  await expect(page).toHaveURL(/\/login/);
});
