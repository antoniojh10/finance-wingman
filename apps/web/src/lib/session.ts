import "server-only";

import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import { cache } from "react";

import { ApiError, createApiClient, unwrap, type ApiClient, type User } from "@/lib/api/client";

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

/** The signed-in user, fetched once per request. */
export const getCurrentUser = cache(async (): Promise<User> => {
  const api = await authedApi();
  return expectData(await api.GET("/api/v1/auth/me"));
});

export { ApiError };
