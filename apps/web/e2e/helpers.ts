import { execFileSync } from "node:child_process";
import path from "node:path";

import { expect, type Browser, type BrowserContext, type Cookie, type Page } from "@playwright/test";

import { apiPort, databaseUrl, webPort } from "../playwright.config";

const mailpitUrl = process.env.MAILPIT_URL ?? "http://localhost:8025";

type MailpitMessage = { ID: string; Created: string };

/** Waits for a sign-in email newer than `since` and returns its text body. */
export async function latestEmail(to: string, since: Date): Promise<string> {
  for (let attempt = 0; attempt < 40; attempt++) {
    const res = await fetch(`${mailpitUrl}/api/v1/search?query=${encodeURIComponent(`to:${to}`)}`);
    const { messages } = (await res.json()) as { messages: MailpitMessage[] };
    const fresh = messages.find((m) => new Date(m.Created) >= since);
    if (fresh) {
      const message = (await (await fetch(`${mailpitUrl}/api/v1/message/${fresh.ID}`)).json()) as { Text: string };
      return message.Text;
    }
    await new Promise((resolve) => setTimeout(resolve, 250));
  }
  throw new Error(`no email for ${to} in Mailpit`);
}

export function codeFrom(text: string): string {
  const match = /\b(\d{6})\b/.exec(text);
  if (!match) throw new Error(`no code in email:\n${text}`);
  return match[1];
}

export function linkFrom(text: string): string {
  const match = /(https?:\/\/\S+\/auth\/verify\?token=\S+)/.exec(text);
  if (!match) throw new Error(`no link in email:\n${text}`);
  return match[1];
}

/** Requests a sign-in email and returns its text. */
export async function requestLogin(page: Page, email: string): Promise<string> {
  // Mailpit timestamps have second precision.
  const since = new Date(Date.now() - 1000);
  await page.goto("/login");
  await page.getByLabel(/email|correo/i).fill(email);
  await page.getByRole("button", { name: /sign-in link|enlace de acceso/i }).click();
  await expect(page.getByLabel(/6-digit code|código de 6 dígitos/i)).toBeVisible();
  return latestEmail(email, since);
}

export async function signIn(page: Page, email: string): Promise<void> {
  const text = await requestLogin(page, email);
  await page.getByLabel(/6-digit code|código de 6 dígitos/i).fill(codeFrom(text));
  await page.getByRole("button", { name: /^(sign in|entrar)$/i }).click();
  await expect(page).toHaveURL(/\/$/);
}

type SessionOptions = { name?: string; workspace?: string };

/**
 * Opens a session for `email` straight in the e2e database, with the API's
 * `sessions create` command (no email, no Mailpit), and returns the session
 * cookie. The user is created if needed. Pass `workspace` to act on a
 * workspace owned by the user (created if missing).
 */
export function mintSession(email: string, options: SessionOptions = {}): Cookie {
  const args = ["run", "./cmd/api", "sessions", "create", "--email", email];
  if (options.name) args.push("--name", options.name);
  if (options.workspace) args.push("--workspace", options.workspace);
  const output = execFileSync("go", args, {
    cwd: path.join(__dirname, "../../api"),
    env: { ...process.env, DATABASE_URL: databaseUrl, PORT: String(apiPort) },
    encoding: "utf8",
  });
  const [name, value] = output.trim().split("=");
  if (!name || !value) throw new Error(`unexpected sessions create output: ${output}`);
  return {
    name,
    value,
    domain: "localhost",
    path: "/",
    // The API session lasts longer; this only has to outlive the run.
    expires: Math.floor(Date.now() / 1000) + 24 * 60 * 60,
    httpOnly: true,
    secure: false,
    sameSite: "Lax",
  };
}

/** A browser context already signed in as `email`, for tests that need another user. */
export async function signedInContext(browser: Browser, email: string, options: SessionOptions = {}): Promise<BrowserContext> {
  const context = await browser.newContext({ baseURL: `http://localhost:${webPort}` });
  await context.addCookies([mintSession(email, options)]);
  return context;
}

/**
 * Navigates using the visible links: the sidebar on desktop; the bottom tabs
 * and the header avatar (settings) on mobile.
 */
export async function navigate(page: Page, name: RegExp): Promise<void> {
  await page.getByRole("link", { name }).filter({ visible: true }).first().click();
}

/** Opens the new transaction page from the sidebar button or the tab bar. */
export async function openNewTransaction(page: Page): Promise<void> {
  await page.getByRole("link", { name: "Add transaction" }).filter({ visible: true }).first().click();
  await expect(page.getByRole("heading", { name: "New transaction" })).toBeVisible();
}

/**
 * Opens a section that is not in the mobile tab bar: from the sidebar on
 * desktop, and through the settings page on mobile.
 */
async function openFromSettings(page: Page, name: RegExp): Promise<void> {
  const link = page.getByRole("link", { name }).filter({ visible: true });
  if ((await link.count()) === 0) {
    await page.getByRole("link", { name: /settings/i }).filter({ visible: true }).first().click();
    await expect(page.getByRole("heading", { name: "Settings" })).toBeVisible();
  }
  await link.first().click();
}

export const openCategories = (page: Page) => openFromSettings(page, /categories/i);

export const openSubscriptions = (page: Page) => openFromSettings(page, /subscriptions/i);

export function invitationLinkFrom(text: string): string {
  const match = /(https?:\/\/\S+\/invite\?token=\S+)/.exec(text);
  if (!match) throw new Error(`no invitation link in email:\n${text}`);
  return match[1];
}

/** Opens the workspace menu (sidebar on desktop, header on mobile). */
export async function openWorkspaceMenu(page: Page): Promise<void> {
  await page.getByRole("button", { name: "Switch workspace" }).filter({ visible: true }).first().click();
}

/** Asserts the workspace shown by the switcher. */
export async function expectWorkspace(page: Page, name: string): Promise<void> {
  await expect(page.getByRole("button", { name: "Switch workspace" }).filter({ visible: true }).first()).toContainText(name);
}
