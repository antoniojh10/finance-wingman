"use client";

import { BanIcon, PauseIcon, PencilIcon, PlayIcon, RotateCcwIcon } from "lucide-react";
import { useLocale, useTranslations } from "next-intl";
import { useState, useTransition } from "react";
import { toast } from "sonner";

import { setRecurringStatus } from "@/app/actions/recurring";
import { ConfirmAction } from "@/components/confirm-action";
import { Money } from "@/components/money";
import { RowActions } from "@/components/row-actions";
import type { AccountOption, CategoryOption } from "@/components/transactions/types";
import { Badge } from "@/components/ui/badge";
import { formatDate } from "@/lib/dates";
import type { RecurringStatus, RecurringType } from "@/lib/recurring";
import { cn } from "@/lib/utils";

import { RecurringDialog, type RecurringRow } from "./recurring-dialog";

type Filter = "all" | RecurringType;
const filters: Filter[] = ["all", "expense", "income"];
const statusOrder: Record<RecurringStatus, number> = { active: 0, paused: 1, cancelled: 2 };

/** Active items first, then paused, then cancelled; keeps the API order within each. */
export function sortByStatus(items: RecurringRow[]): RecurringRow[] {
  return [...items].sort((a, b) => statusOrder[a.status] - statusOrder[b.status]);
}

export function RecurringList({
  items,
  accounts,
  categories,
  defaultDate,
}: {
  items: RecurringRow[];
  accounts: AccountOption[];
  categories: CategoryOption[];
  defaultDate: string;
}) {
  const t = useTranslations("subscriptions");
  const [filter, setFilter] = useState<Filter>("all");

  if (items.length === 0) {
    return <p className="rounded-3xl bg-card py-10 text-center text-sm text-muted-foreground">{t("empty")}</p>;
  }

  const visible = sortByStatus(items).filter((item) => filter === "all" || item.type === filter);

  return (
    <div className="grid grid-cols-1 gap-3">
      <div role="group" aria-label={t("filterLabel")} className="flex flex-wrap gap-2">
        {filters.map((value) => (
          <button
            key={value}
            type="button"
            aria-pressed={filter === value}
            onClick={() => setFilter(value)}
            className={cn(
              "min-h-11 rounded-[14px] border-2 px-3.5 text-sm font-semibold transition-colors",
              filter === value ? "border-primary bg-primary/8" : "border-border bg-card hover:border-foreground/25",
            )}
          >
            {t(`filters.${value}`)}
          </button>
        ))}
      </div>
      {visible.length === 0 ? (
        <p className="rounded-3xl bg-card py-10 text-center text-sm text-muted-foreground">{t("emptyFilter")}</p>
      ) : (
        <ul className="divide-y divide-border/70 rounded-3xl bg-card px-4 ring-1 ring-foreground/5">
          {visible.map((item) => (
            <RecurringItemRow key={item.id} item={item} accounts={accounts} categories={categories} defaultDate={defaultDate} />
          ))}
        </ul>
      )}
    </div>
  );
}

function RecurringItemRow({
  item,
  accounts,
  categories,
  defaultDate,
}: {
  item: RecurringRow;
  accounts: AccountOption[];
  categories: CategoryOption[];
  defaultDate: string;
}) {
  const t = useTranslations("subscriptions");
  const tc = useTranslations("common");
  const locale = useLocale();
  const [pending, startTransition] = useTransition();
  const [editing, setEditing] = useState(false);
  const [cancelling, setCancelling] = useState(false);

  function changeStatus(status: RecurringStatus, message: string) {
    startTransition(async () => {
      const result = await setRecurringStatus(item.id, status);
      if (result.ok) {
        toast.success(message);
      } else {
        toast.error(result.message);
      }
    });
  }

  const actions = [
    { label: tc("edit"), icon: PencilIcon, onSelect: () => setEditing(true) },
    ...(item.status === "active"
      ? [{ label: t("pause"), icon: PauseIcon, onSelect: () => changeStatus("paused", t("pausedToast")), disabled: pending }]
      : []),
    ...(item.status === "paused"
      ? [{ label: t("resume"), icon: PlayIcon, onSelect: () => changeStatus("active", t("resumedToast")), disabled: pending }]
      : []),
    ...(item.status === "cancelled"
      ? [{ label: t("reactivate"), icon: RotateCcwIcon, onSelect: () => changeStatus("active", t("reactivatedToast")), disabled: pending }]
      : [{ label: t("cancel"), icon: BanIcon, onSelect: () => setCancelling(true), destructive: true }]),
  ];

  const details = [
    t(`frequency.${item.interval_unit}`, { count: item.interval_count }),
    item.account_name,
    item.category_name,
  ].filter(Boolean);

  return (
    <li className={cn("flex min-h-17.5 items-center gap-3 py-2.5", item.status !== "active" && "opacity-70")} data-testid="recurring-row">
      <div className="min-w-0 flex-1">
        <p className="flex items-center gap-2 text-[15.5px] font-semibold">
          <span className="min-w-0 truncate">{item.name}</span>
          {item.status !== "active" && <Badge variant="secondary">{t(`status.${item.status}`)}</Badge>}
        </p>
        <p className="truncate text-[12.5px] text-muted-foreground">{details.join(" · ")}</p>
        <p className="text-[12.5px] text-muted-foreground">
          {item.status === "active" && item.next_due_on
            ? t("nextDue", { date: formatDate(item.next_due_on, locale) })
            : t("noNextDue")}
        </p>
      </div>
      <Money
        amount={item.amount}
        currency={item.currency}
        minorUnits={item.minor_units}
        tone={item.type}
        signed
        className={cn("text-[15.5px] font-bold whitespace-nowrap", item.type === "expense" && "text-expense")}
      />
      <RowActions className="-mr-2 text-muted-foreground" actions={actions} />
      <RecurringDialog
        item={item}
        accounts={accounts}
        categories={categories}
        defaultDate={defaultDate}
        open={editing}
        onOpenChange={setEditing}
      />
      <ConfirmAction
        open={cancelling}
        onOpenChange={setCancelling}
        title={t("cancelTitle")}
        description={t("cancelDescription")}
        confirmLabel={t("cancel")}
        successMessage={t("cancelledToast")}
        action={() => setRecurringStatus(item.id, "cancelled")}
      />
    </li>
  );
}
