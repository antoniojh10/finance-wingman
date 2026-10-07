import "server-only";

import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import { cache } from "react";

import { ApiError, createApiClient, unwrap, type ApiClient, type Membership, type Session, type User } from "@/lib/api/client";
import { toOwnerOption, type OwnerOption } from "@/lib/owners";

export const SESSION_COOKIE = "fw_session";

export async function getSessionToken(): Promise<string | undefined> {
  return (await cookies()).get(SESSION_COOKIE)?.value;
}

/** Stores the API bearer token in an HttpOnly cookie. Server Actions only. */
export async function setSession(token: string, expiresAt: string): Promise<void> {
  (await cookies()).set(SESSION_COOKIE, token, {
    httpOnly: true,
    secure: process.env.NODE_ENV === "production",
    sameSite: "lax",
    path: "/",
    expires: new Date(expiresAt),
  });
}

export async function clearSession(): Promise<void> {
  (await cookies()).delete(SESSION_COOKIE);
}

/** Returns an API client authenticated as the current user, or redirects to login. */
export async function authedApi(): Promise<ApiClient> {
  const token = await getSessionToken();
  if (!token) {
    redirect("/login");
  }
  return createApiClient(token);
}

/**
 * Unwraps an API response for a page render. An expired session sends the
 * user through sign-out (which clears the cookie) back to the login page.
 */
export function expectData<T>(result: { data?: T; error?: unknown; response: Response }): T {
  if (result.response.status === 401) {
    redirect("/auth/signout");
  }
  return unwrap(result);
}

/** The current session (user and active workspace), fetched once per request. */
export const getCurrentSession = cache(async (): Promise<Session> => {
  const api = await authedApi();
  return expectData(await api.GET("/api/v1/auth/session"));
});

/** The signed-in user, fetched once per request. */
export const getCurrentUser = cache(async (): Promise<User> => (await getCurrentSession()).user);

/** The workspaces the signed-in user belongs to, fetched once per request. */
export const getWorkspaces = cache(async (): Promise<Membership[]> => {
  const api = await authedApi();
  return expectData(await api.GET("/api/v1/workspaces")).items;
});

/**
 * Members of the active workspace, who can own accounts, fetched once per
 * request. Owner controls are only shown when there are several.
 */
export const getAccountOwners = cache(async (): Promise<OwnerOption[]> => {
  const { workspace } = await getCurrentSession();
  if (!workspace) {
    return [];
  }
  const api = await authedApi();
  return expectData(await api.GET("/api/v1/workspaces/{id}/members", { params: { path: { id: workspace.id } } })).items.map(toOwnerOption);
});

/** Whether accounts are labelled with their owner: only with several members. */
export const showAccountOwners = cache(async (): Promise<boolean> => (await getAccountOwners()).length > 1);

export { ApiError };
