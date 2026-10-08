import type { ActivityEntry } from "@/lib/api/client";

/**
 * Entity types the activity filter offers. The API logs more kinds of records
 * over time: add the type here (and its label under `activity.entityTypes`)
 * to make it filterable. Types missing from this list still render in the feed.
 */
export const ACTIVITY_ENTITY_TYPES = [
  "transaction",
  "account",
  "category",
  "budget",
  "recurring_item",
  "recurring_suggestion",
  "workspace",
] as const;

export type ActivityEntityType = (typeof ACTIVITY_ENTITY_TYPES)[number];

export const ACTIVITY_CHANNELS = ["web", "mcp"] as const;
export type ActivityChannel = (typeof ACTIVITY_CHANNELS)[number];

export const ACTIVITY_PAGE_SIZE = 25;

const uuidPattern = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

export type ActivityQuery = {
  /** Member who made the change. */
  actor_id?: string;
  channel?: ActivityChannel;
  entity_type?: ActivityEntityType;
};

/** Sanitizes activity search params, dropping invalid values. */
export function parseActivityQuery(params: Record<string, string | string[] | undefined>): ActivityQuery {
  const get = (key: string) => {
    const value = params[key];
    return (Array.isArray(value) ? value[0] : value)?.trim() || undefined;
  };
  const channel = get("channel");
  const entityType = get("entity_type");
  const actor = get("actor_id");
  return {
    actor_id: actor && uuidPattern.test(actor) ? actor : undefined,
    channel: ACTIVITY_CHANNELS.find((c) => c === channel),
    entity_type: ACTIVITY_ENTITY_TYPES.find((e) => e === entityType),
  };
}

/** URL of the feed with some filters changed; undefined clears a filter. */
export function activityHref(query: ActivityQuery, changes: Partial<ActivityQuery>): string {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries({ ...query, ...changes })) {
    if (value) {
      params.set(key, value);
    }
  }
  const qs = params.toString();
  return qs ? `?${qs}` : "?";
}

/** Turns an identifier like "recurring_due_on" into "recurring due on". */
export function humanize(value: string): string {
  return value.replace(/[._-]+/g, " ").trim();
}

/** The minimal translator shape the label helpers need (next-intl's). */
export type Translate = {
  (key: string, values?: Record<string, string | number>): string;
  has: (key: string) => boolean;
};

/** Label of an entity type, or the humanized raw type for kinds without copy. */
export function entityTypeLabel(t: Translate, entityType: string): string {
  const key = `entityTypes.${entityType}`;
  return t.has(key) ? t(key) : humanize(entityType);
}

/**
 * Sentence for an action ("created a transaction"). Actions without copy fall
 * back to a generic sentence naming the raw action and the entity type, so new
 * kinds of events render without code changes.
 */
export function actionLabel(t: Translate, action: string, entityType: string): string {
  const key = `actions.${action}`;
  return t.has(key) ? t(key) : t("unknownAction", { action: humanize(action), entity: entityTypeLabel(t, entityType) });
}

/** Label of a changed field, or the humanized field name without copy. */
export function fieldLabel(t: Translate, field: string): string {
  const key = `fields.${field}`;
  return t.has(key) ? t(key) : humanize(field);
}

/** Account names by id, resolved by the page: the log only stores ids. */
export type AccountNames = Record<string, string>;

/** Names of the accounts an entry refers to; ids that no longer resolve are skipped. */
export function accountNamesFor(entry: ActivityEntry, names: AccountNames): { account?: string; destination?: string } {
  const { account_id: accountId, destination_account_id: destinationId } = entry.details;
  return {
    account: accountId ? names[accountId] : undefined,
    destination: destinationId ? names[destinationId] : undefined,
  };
}

/** Category names by id, resolved by the page like account names. */
export type CategoryNames = Record<string, string>;

/**
 * Names of the records an entry points at: the account(s) in its details, the
 * entity itself for accounts, and the category (budgets use the category id as
 * their entity id). Ids that no longer resolve are skipped.
 */
export function entryNames(
  entry: ActivityEntry,
  accounts: AccountNames,
  categories: CategoryNames,
): { account?: string; destination?: string; category?: string } {
  const { account, destination } = accountNamesFor(entry, accounts);
  const categoryId = entry.details.category_id ?? (entry.entity_type === "category" || entry.entity_type === "budget" ? entry.entity_id : undefined);
  return {
    account: account ?? (entry.entity_type === "account" ? accounts[entry.entity_id] : undefined),
    destination,
    category: categoryId ? categories[categoryId] : undefined,
  };
}

const monthPattern = /^\d{4}-(0[1-9]|1[0-2])$/;

/** Whether a string is a YYYY-MM month. */
export function isMonth(value: string): boolean {
  return monthPattern.test(value);
}
