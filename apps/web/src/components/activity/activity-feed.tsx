"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";

import { loadMoreActivity } from "@/app/actions/activity";
import { Button } from "@/components/ui/button";
import type { AccountNames, ActivityQuery, CategoryNames } from "@/lib/activity";
import type { ActivityEntry } from "@/lib/api/client";

import { ActivityItem } from "./activity-item";

/**
 * The feed, newest first. "Load more" appends the next page through a Server
 * Action using the API cursor. Remount it (key) when the filters change.
 */
export function ActivityFeed({
  initialItems,
  initialCursor,
  query,
  accountNames,
  categoryNames,
}: {
  initialItems: ActivityEntry[];
  initialCursor: string | null;
  query: ActivityQuery;
  accountNames: AccountNames;
  categoryNames: CategoryNames;
}) {
  const t = useTranslations("activity");
  const [items, setItems] = useState(initialItems);
  const [cursor, setCursor] = useState(initialCursor);
  const [failed, setFailed] = useState(false);
  const [pending, setPending] = useState(false);

  async function loadMore() {
    if (!cursor) {
      return;
    }
    setFailed(false);
    setPending(true);
    try {
      const page = await loadMoreActivity(query, cursor);
      setItems((current) => [...current, ...page.items]);
      setCursor(page.nextCursor);
    } catch {
      setFailed(true);
    } finally {
      setPending(false);
    }
  }

  if (items.length === 0) {
    return (
      <p data-testid="activity-empty" className="rounded-2xl bg-card px-5 py-8 text-center text-sm text-muted-foreground ring-1 ring-foreground/5">
        {t("empty")}
      </p>
    );
  }

  return (
    <div className="grid gap-3">
      <ul className="grid gap-2.5">
        {items.map((entry) => (
          <ActivityItem key={entry.id} entry={entry} accountNames={accountNames} categoryNames={categoryNames} />
        ))}
      </ul>
      {failed && (
        <p role="alert" className="text-center text-sm text-destructive">
          {t("loadFailed")}
        </p>
      )}
      {cursor && (
        <Button type="button" variant="outline" size="lg" disabled={pending} onClick={loadMore} className="justify-self-center">
          {pending ? t("loading") : t("loadMore")}
        </Button>
      )}
    </div>
  );
}
