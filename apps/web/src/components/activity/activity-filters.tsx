import Link from "next/link";
import { useTranslations } from "next-intl";

import { ACTIVITY_CHANNELS, ACTIVITY_ENTITY_TYPES, activityHref, entityTypeLabel, type ActivityQuery, type Translate } from "@/lib/activity";
import type { OwnerOption } from "@/lib/owners";
import { cn } from "@/lib/utils";

type Option = { key: string; label: string; href: string; active: boolean };

function Chips({ label, options }: { label: string; options: Option[] }) {
  return (
    <nav aria-label={label} className="-mx-4 flex gap-2 overflow-x-auto px-4 pb-0.5 md:mx-0 md:px-0">
      {options.map((option) => (
        <Link
          key={option.key}
          href={option.href}
          aria-current={option.active ? "page" : undefined}
          className={cn(
            "flex h-10 shrink-0 items-center rounded-full border-[1.5px] px-4 text-sm font-bold transition-colors",
            option.active ? "border-foreground bg-foreground text-background" : "border-border bg-card hover:border-foreground/30",
          )}
        >
          {option.label}
        </Link>
      ))}
    </nav>
  );
}

/** Member, channel and entity-type filters as links, so they live in the URL. */
export function ActivityFilters({ query, members }: { query: ActivityQuery; members: OwnerOption[] }) {
  const t = useTranslations("activity");
  const translate = t as unknown as Translate;
  const all = t("all");

  return (
    <div className="grid gap-2.5">
      {members.length > 1 && (
        <Chips
          label={t("filterMember")}
          options={[
            { key: "all", label: all, href: activityHref(query, { actor_id: undefined }), active: !query.actor_id },
            ...members.map((m) => ({ key: m.id, label: m.name, href: activityHref(query, { actor_id: m.id }), active: query.actor_id === m.id })),
          ]}
        />
      )}
      <Chips
        label={t("filterChannel")}
        options={[
          { key: "all", label: all, href: activityHref(query, { channel: undefined }), active: !query.channel },
          ...ACTIVITY_CHANNELS.map((c) => ({ key: c, label: t(`channels.${c}`), href: activityHref(query, { channel: c }), active: query.channel === c })),
        ]}
      />
      <Chips
        label={t("filterEntity")}
        options={[
          { key: "all", label: all, href: activityHref(query, { entity_type: undefined }), active: !query.entity_type },
          ...ACTIVITY_ENTITY_TYPES.map((e) => ({
            key: e,
            label: entityTypeLabel(translate, e),
            href: activityHref(query, { entity_type: e }),
            active: query.entity_type === e,
          })),
        ]}
      />
    </div>
  );
}
