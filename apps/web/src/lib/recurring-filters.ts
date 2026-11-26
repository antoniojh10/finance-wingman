import type { RecurringStatus } from "@/lib/recurring";

export type StatusFilter = "all" | RecurringStatus;

export const statusFilters: StatusFilter[] = ["active", "paused", "cancelled", "all"];

/** Value of the category filter that matches items without a category. */
export const uncategorizedFilter = "none";

/** Filter state of the subscriptions list that lives in the URL. */
export type RecurringFilters = {
  status: StatusFilter;
  /** A category id, or `none` for uncategorized items; undefined shows every category. */
  category?: string;
  /** Groups the list by category (on by default). */
  group: boolean;
};

export const defaultRecurringFilters: RecurringFilters = { status: "active", group: true };

function first(value: string | string[] | undefined): string | undefined {
  return Array.isArray(value) ? value[0] : value;
}

/**
 * Sanitizes the subscriptions search params: a missing or invalid status falls
 * back to active, an empty category is ignored and only `group=0` turns the grouping off.
 */
export function parseRecurringFilters(params: Record<string, string | string[] | undefined>): RecurringFilters {
  const status = first(params.status);
  const category = first(params.category)?.trim();
  return {
    status: statusFilters.includes(status as StatusFilter) ? (status as StatusFilter) : defaultRecurringFilters.status,
    ...(category ? { category } : {}),
    group: first(params.group) !== "0",
  };
}

/** The subscriptions URL for a set of filters; defaults are left out. */
export function recurringFiltersHref(filters: Partial<RecurringFilters>): string {
  const params = new URLSearchParams();
  if (filters.status && filters.status !== defaultRecurringFilters.status) params.set("status", filters.status);
  if (filters.category) params.set("category", filters.category);
  if (filters.group === false) params.set("group", "0");
  const query = params.toString();
  return query ? `/subscriptions?${query}` : "/subscriptions";
}
