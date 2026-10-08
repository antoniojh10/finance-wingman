import { createHash, randomBytes } from "node:crypto";

import { devices, type Page } from "@playwright/test";

import { expect, test } from "./fixtures";
import { codeFrom, invitationLinkFrom, latestEmail, signIn } from "./helpers";

const apiUrl = "http://localhost:8081";

/** Invites a new person to the owner's workspace and returns the invitation link. */
async function invite(page: Page, email: string): Promise<string> {
  await page.goto("/settings/workspace");
  await expect(page.getByRole("heading", { name: "Invite someone" })).toBeVisible();
  const since = new Date(Date.now() - 1000);
  await page.getByLabel("Email").fill(email);
  await page.getByRole("button", { name: "Invite" }).click();
  await expect(page.getByText("Invitation sent")).toBeVisible();
  return invitationLinkFrom(await latestEmail(email, since));
}

type Tokens = { access_token: string; refresh_token: string };

/**
 * Connects an app the way an MCP host does: dynamic registration, the
 * authorization page with an emailed code, and the token exchange.
 */
async function connectApp(name: string, email: string): Promise<{ clientId: string; tokens: Tokens }> {
  const redirectUri = "http://127.0.0.1:9/callback";
  const registered = await fetch(`${apiUrl}/oauth/register`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ client_name: name, redirect_uris: [redirectUri], token_endpoint_auth_method: "none" }),
  });
  expect(registered.status).toBe(201);
  const { client_id: clientId } = (await registered.json()) as { client_id: string };

  const verifier = randomBytes(32).toString("base64url");
  const authorize = new URL(`${apiUrl}/oauth/authorize`);
  authorize.search = new URLSearchParams({
    response_type: "code",
    client_id: clientId,
    redirect_uri: redirectUri,
    code_challenge: createHash("sha256").update(verifier).digest("base64url"),
    code_challenge_method: "S256",
    state: "e2e",
  }).toString();
  const page = await (await fetch(authorize)).text();
  const requestId = /name="request_id" value="([0-9a-f-]{36})"/.exec(page)?.[1];
  expect(requestId).toBeTruthy();

  const submit = (fields: Record<string, string>) =>
    fetch(`${apiUrl}/oauth/authorize`, {
      method: "POST",
      headers: { "Content-Type": "application/x-www-form-urlencoded" },
      body: new URLSearchParams({ request_id: requestId!, ...fields }),
      redirect: "manual",
    });
  const since = new Date(Date.now() - 1000);
  expect((await submit({ action: "send_code", email })).status).toBe(200);
  const approved = await submit({ action: "verify", code: codeFrom(await latestEmail(email, since)) });
  expect(approved.status).toBe(302);
  const code = new URL(approved.headers.get("location")!).searchParams.get("code")!;

  const tokens = await token({ grant_type: "authorization_code", client_id: clientId, code, redirect_uri: redirectUri, code_verifier: verifier });
  expect(tokens.status).toBe(200);
  return { clientId, tokens: (await tokens.json()) as Tokens };
}

function token(fields: Record<string, string>) {
  return fetch(`${apiUrl}/oauth/token`, {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body: new URLSearchParams(fields),
  });
}

async function openSecurity(page: Page) {
  await page.goto("/settings");
  await page.getByRole("link", { name: /security/i }).click();
  await expect(page.getByRole("heading", { name: "Where you're signed in" })).toBeVisible();
}

// Everything happens as a freshly invited person, so signing out their
// other sessions never touches the session saved by auth.setup.ts.
test("signs out other sessions and disconnects apps", async ({ page, browser }, testInfo) => {
  const suffix = `${testInfo.project.name}-${Date.now()}`;
  const person = `security-${suffix}@example.com`;
  const link = await invite(page, person);
  const workspace = (await page.getByLabel("Name").inputValue()).trim();

  // The person accepts the invitation on their laptop...
  const laptopContext = await browser.newContext({ ...testInfo.project.use, storageState: undefined });
  const laptop = await laptopContext.newPage();
  await laptop.goto(link.replace(/^https?:\/\/[^/]+/, ""));
  await laptop.getByRole("button", { name: "Accept invitation" }).click();
  await expect(laptop.getByRole("heading", { name: "Dashboard" })).toBeVisible();

  // ...and signs in on a phone and on another computer.
  const phoneContext = await browser.newContext({ ...devices["iPhone 13"], storageState: undefined });
  const phone = await phoneContext.newPage();
  await signIn(phone, person);
  const otherContext = await browser.newContext({ ...testInfo.project.use, storageState: undefined });
  const other = await otherContext.newPage();
  await signIn(other, person);

  await openSecurity(laptop);
  const sessions = laptop.getByTestId("session-row");
  await expect(sessions).toHaveCount(3);
  await expect(sessions.filter({ hasText: "This device" })).toHaveCount(1);
  await expect(laptop.getByText("No apps are connected.")).toBeVisible();

  // Sign out the phone only.
  await laptop.getByRole("button", { name: "Sign out Safari on iOS" }).click();
  await laptop.getByRole("alertdialog").getByRole("button", { name: "Sign out" }).click();
  await expect(laptop.getByText("Session signed out")).toBeVisible();
  await expect(sessions).toHaveCount(2);
  await phone.goto("/");
  await expect(phone).toHaveURL(/\/login/);
  await other.goto("/");
  await expect(other.getByRole("heading", { name: "Dashboard" })).toBeVisible();

  // Sign out everywhere else.
  await laptop.getByRole("button", { name: "Sign out everywhere else" }).click();
  await laptop.getByRole("alertdialog").getByRole("button", { name: "Sign out everywhere else" }).click();
  await expect(laptop.getByText("Signed out everywhere else")).toBeVisible();
  await expect(sessions).toHaveCount(1);
  await expect(laptop.getByRole("button", { name: "Sign out everywhere else" })).toHaveCount(0);
  await other.goto("/");
  await expect(other).toHaveURL(/\/login/);

  // Connect an AI assistant, then disconnect it.
  const appName = `E2E Assistant ${suffix}`;
  const { clientId, tokens } = await connectApp(appName, person);
  await laptop.reload();
  const connection = laptop.getByTestId("connection-row").filter({ hasText: appName });
  await expect(connection).toContainText(workspace);
  await connection.getByRole("button", { name: `Disconnect ${appName}` }).click();
  await laptop.getByRole("alertdialog").getByRole("button", { name: "Disconnect" }).click();
  await expect(laptop.getByText("App disconnected")).toBeVisible();
  await expect(laptop.getByText("No apps are connected.")).toBeVisible();

  // The app can neither refresh its tokens nor call the MCP endpoint.
  const refreshed = await token({ grant_type: "refresh_token", client_id: clientId, refresh_token: tokens.refresh_token });
  expect(refreshed.status).toBe(400);
  const mcp = await fetch(`${apiUrl}/mcp`, {
    method: "POST",
    headers: {
      Authorization: `Bearer ${tokens.access_token}`,
      "Content-Type": "application/json",
      Accept: "application/json, text/event-stream",
    },
    body: JSON.stringify({ jsonrpc: "2.0", id: 1, method: "tools/list" }),
  });
  expect(mcp.status).toBe(401);

  // The laptop is still signed in.
  await laptop.goto("/");
  await expect(laptop.getByRole("heading", { name: "Dashboard" })).toBeVisible();
  await Promise.all([laptopContext.close(), phoneContext.close(), otherContext.close()]);
});
