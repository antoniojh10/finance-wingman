import type { RecurringStatus } from "@/lib/recurring";

export type StatusFilter = "all" | RecurringStatus;

export const statusFilters: StatusFilter[] = ["active", "paused", "cancelled", "all"];

/** Filter state of the subscriptions list that lives in the URL. */
export type RecurringFilters = {
  status: StatusFilter;
};

export const defaultRecurringFilters: RecurringFilters = { status: "active" };

/** Sanitizes the subscriptions search params: a missing or invalid status falls back to active. */
export function parseRecurringFilters(params: Record<string, string | string[] | undefined>): RecurringFilters {
  const raw = params.status;
  const status = Array.isArray(raw) ? raw[0] : raw;
  return { status: statusFilters.includes(status as StatusFilter) ? (status as StatusFilter) : defaultRecurringFilters.status };
}

/** The subscriptions URL for a set of filters; defaults are left out. */
export function recurringFiltersHref(filters: RecurringFilters): string {
  const params = new URLSearchParams();
  if (filters.status !== defaultRecurringFilters.status) params.set("status", filters.status);
  const query = params.toString();
  return query ? `/subscriptions?${query}` : "/subscriptions";
}
