import { expect, type Page } from "@playwright/test";

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
