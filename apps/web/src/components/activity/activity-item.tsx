import { BotIcon, GlobeIcon } from "lucide-react";
import { useFormatter, useTranslations } from "next-intl";

import { Badge } from "@/components/ui/badge";
import { actionLabel, entryNames, fieldLabel, isMonth, type AccountNames, type CategoryNames, type Translate } from "@/lib/activity";
import type { ActivityEntry } from "@/lib/api/client";
import { cn } from "@/lib/utils";

/**
 * One line of the feed: who did what, through which channel and when. MCP
 * changes are highlighted and name the client. The log keeps ids only, so
 * account and category names come from the maps and are left out when unknown.
 */
export function ActivityItem({
  entry,
  accountNames,
  categoryNames,
}: {
  entry: ActivityEntry;
  accountNames: AccountNames;
  categoryNames: CategoryNames;
}) {
  const t = useTranslations("activity") as unknown as Translate;
  const format = useFormatter();
  const isMcp = entry.channel === "mcp";
  const actorName = entry.actor ? entry.actor.name.trim() || entry.actor.email : t("formerMember");
  const { account, destination, category } = entryNames(entry, accountNames, categoryNames);
  const { type, changed, currency, month, format: fileFormat } = entry.details;
  const typeLabel = type && t.has(`types.${type}`) ? t(`types.${type}`) : undefined;
  const accountText = account && destination ? `${account} → ${destination}` : account;
  const monthText = month && isMonth(month) ? format.dateTime(new Date(`${month}-01T00:00:00Z`), { month: "long", year: "numeric", timeZone: "UTC" }) : undefined;
  const facts = [typeLabel, accountText, category, currency, monthText, fileFormat?.toUpperCase()].filter(Boolean).join(" · ");
  const ChannelIcon = isMcp ? BotIcon : GlobeIcon;

  return (
    <li
      data-testid="activity-item"
      data-channel={entry.channel}
      className={cn("grid gap-1.5 rounded-2xl px-4 py-3 ring-1", isMcp ? "bg-primary/10 ring-primary/25" : "bg-card ring-foreground/5")}
    >
      <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
        <ChannelIcon className="size-4 shrink-0 text-muted-foreground" aria-hidden />
        <p className="min-w-0 flex-1 text-sm">
          <span className="font-bold">{actorName}</span> {actionLabel(t, entry.action, entry.entity_type)}
        </p>
        {isMcp && (
          <Badge data-testid="activity-mcp-badge">{entry.client_name ? t("viaClient", { client: entry.client_name }) : t("viaMcp")}</Badge>
        )}
      </div>
      {facts && <p className="text-sm text-muted-foreground">{facts}</p>}
      {changed && changed.length > 0 && (
        <p className="text-sm text-muted-foreground">{t("changedFields", { fields: changed.map((f) => fieldLabel(t, f)).join(", ") })}</p>
      )}
      <time dateTime={entry.created_at} className="text-xs text-muted-foreground">
        {format.dateTime(new Date(entry.created_at), { dateStyle: "medium", timeStyle: "short" })}
      </time>
    </li>
  );
}
