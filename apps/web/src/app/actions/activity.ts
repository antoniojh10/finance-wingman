"use server";

import { ACTIVITY_PAGE_SIZE, parseActivityQuery, type ActivityQuery } from "@/lib/activity";
import { unwrap, type ActivityEntry } from "@/lib/api/client";
import { authedApi } from "@/lib/session";

export type ActivityPageResult = { items: ActivityEntry[]; nextCursor: string | null };

/** Fetches one page of the activity feed. Used by the page and by "Load more". */
export async function fetchActivityPage(query: ActivityQuery, cursor?: string): Promise<ActivityPageResult> {
  const api = await authedApi();
  const page = unwrap(
    await api.GET("/api/v1/activity", {
      // The generated enum only lists the entity types the API knows today.
      params: { query: { ...query, entity_type: query.entity_type as "transaction" | undefined, cursor, limit: ACTIVITY_PAGE_SIZE } },
    }),
  );
  return { items: page.items, nextCursor: page.next_cursor };
}

/** Server Action behind "Load more": re-validates the filters sent by the browser. */
export async function loadMoreActivity(query: ActivityQuery, cursor: string): Promise<ActivityPageResult> {
  return fetchActivityPage(parseActivityQuery({ ...query }), cursor);
}
