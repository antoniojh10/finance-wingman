import "server-only";

import createClient from "openapi-fetch";

import type { components, paths } from "./schema";

export type Schemas = components["schemas"];
export type Account = Schemas["Account"];
export type Category = Schemas["Category"];
export type Currency = Schemas["Currency"];
export type RecurringItem = Schemas["RecurringItem"];
export type Transaction = Schemas["Transaction"];
export type TransactionPage = Schemas["TransactionPage"];
export type Summary = Schemas["Summary"];
export type CurrencySummary = Schemas["CurrencySummary"];
export type User = Schemas["User"];
export type Session = Schemas["Session"];
export type SessionWorkspace = Schemas["SessionWorkspace"];
export type Membership = Schemas["Membership"];
export type Member = Schemas["Member"];
export type ActivityEntry = Schemas["ActivityEntry"];
export type Invitation =Schemas["Invitation"];
export type InvitationPreview = Schemas["InvitationPreview"];

export type ApiClient = ReturnType<typeof createApiClient>;

/** Base URL of the Go API, reachable from the Next.js server. */
export function apiBaseUrl(): string {
  return process.env.API_URL ?? "http://localhost:8080";
}

export function createApiClient(token?: string, extraHeaders: Record<string, string> = {}) {
  return createClient<paths>({
    baseUrl: apiBaseUrl(),
    headers: { ...extraHeaders, ...(token ? { Authorization: `Bearer ${token}` } : {}) },
    cache: "no-store",
  });
}

type ProblemDetails = Schemas["ErrorModel"];

/** An error response from the API (RFC 9457 problem details). */
export class ApiError extends Error {
  readonly status: number;
  readonly fieldErrors: Record<string, string>;

  constructor(status: number, problem?: ProblemDetails) {
    super(problem?.detail ?? `API request failed with status ${status}`);
    this.name = "ApiError";
    this.status = status;
    this.fieldErrors = {};
    for (const detail of problem?.errors ?? []) {
      if (detail.location && detail.message) {
        // Locations look like "body.amount", "amount" or "query.from".
        const field = detail.location.split(".").pop() ?? detail.location;
        this.fieldErrors[field] = detail.message;
      }
    }
  }
}

/** Returns the response data or throws an ApiError. */
export function unwrap<T>(result: { data?: T; error?: unknown; response: Response }): T {
  if (result.response.ok && result.data !== undefined) {
    return result.data;
  }
  throw new ApiError(result.response.status, result.error as ProblemDetails | undefined);
}
