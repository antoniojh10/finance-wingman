"use client";

import { BanIcon, PauseIcon, PencilIcon, PlayIcon, RotateCcwIcon } from "lucide-react";
import { useLocale, useTranslations } from "next-intl";
import { useRouter } from "next/navigation";
import { useState, useTransition } from "react";
import { toast } from "sonner";

import { setRecurringStatus } from "@/app/actions/recurring";
import { CategoryPill } from "@/components/categories/category-pill";
import { ConfirmAction } from "@/components/confirm-action";
import { Money } from "@/components/money";
import { NativeSelect } from "@/components/native-select";
import { RowActions } from "@/components/row-actions";
import type { AccountOption, CategoryOption } from "@/components/transactions/types";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { formatDate } from "@/lib/dates";
import type { RecurringStatus, RecurringType } from "@/lib/recurring";
import { type StatusFilter, recurringFiltersHref, statusFilters } from "@/lib/recurring-filters";
import { cn } from "@/lib/utils";

import { PaymentStatusBadge } from "./payment-status-badge";
import { RecurringDialog, type RecurringRow } from "./recurring-dialog";
import { RegisterPaymentDialog } from "./register-payment-dialog";

type TypeFilter = "all" | RecurringType;
const typeFilters: TypeFilter[] = ["all", "expense", "income"];
type Sort = "due" | "amount" | "name";
const sorts: Sort[] = ["due", "amount", "name"];
const statusOrder: Record<RecurringStatus, number> = { active: 0, paused: 1, cancelled: 2 };

/** Orders items by status (active first) and then by the chosen sort. */
export function sortItems(items: RecurringRow[], sort: Sort, locale: string): RecurringRow[] {
  const compare: Record<Sort, (a: RecurringRow, b: RecurringRow) => number> = {
    // Items without a next due date go last.
    due: (a, b) => (a.next_due_on ?? "9999-12-31").localeCompare(b.next_due_on ?? "9999-12-31"),
    amount: (a, b) => b.monthly_amount - a.monthly_amount,
    name: (a, b) => a.name.localeCompare(b.name, locale),
  };
  return [...items].sort((a, b) => statusOrder[a.status] - statusOrder[b.status] || compare[sort](a, b));
}

export function RecurringList({
  items,
  accounts,
  categories,
  defaultDate,
  status: statusFromUrl = "active",
}: {
  items: RecurringRow[];
  accounts: AccountOption[];
  categories: CategoryOption[];
  defaultDate: string;
  /** Status filter read from the URL by the page. */
  status?: StatusFilter;
}) {
  const t = useTranslations("subscriptions");
  const locale = useLocale();
  const router = useRouter();
  const [status, setStatus] = useState(statusFromUrl);
  const [lastUrlStatus, setLastUrlStatus] = useState(statusFromUrl);
  const [type, setType] = useState<TypeFilter>("all");
  const [sort, setSort] = useState<Sort>("due");

  // Follow the URL when it changes from outside (e.g. a link to ?status=paused).
  if (statusFromUrl !== lastUrlStatus) {
    setLastUrlStatus(statusFromUrl);
    setStatus(statusFromUrl);
  }

  function changeStatus(next: StatusFilter) {
    setStatus(next);
    router.replace(recurringFiltersHref({ status: next }), { scroll: false });
  }

  if (items.length === 0) {
    return <p className="rounded-3xl bg-card py-10 text-center text-sm text-muted-foreground">{t("empty")}</p>;
  }

  const counts: Record<StatusFilter, number> = {
    active: items.filter((i) => i.status === "active").length,
    paused: items.filter((i) => i.status === "paused").length,
    cancelled: items.filter((i) => i.status === "cancelled").length,
    all: items.length,
  };
  const ofType = items.filter((item) => type === "all" || item.type === type);
  const visible = sortItems(
    ofType.filter((item) => status === "all" || item.status === status),
    sort,
    locale,
  );
  const hidden = status === "active" ? ofType.filter((item) => item.status !== "active").length : 0;

  return (
    <div className="grid grid-cols-1 gap-3">
      <div className="flex flex-wrap items-center justify-between gap-x-3.5 gap-y-2.5">
        <div
          role="group"
          aria-label={t("filterLabel")}
          className="inline-flex flex-wrap gap-0.5 rounded-[14px] border border-border bg-card p-[3px]"
        >
          {statusFilters.map((value) => (
            <button
              key={value}
              type="button"
              aria-pressed={status === value}
              onClick={() => changeStatus(value)}
              className={cn(
                "inline-flex min-h-9 items-center gap-1.5 rounded-[11px] px-3 text-sm font-semibold transition-colors",
                status === value ? "bg-primary text-primary-foreground" : "text-muted-foreground hover:text-foreground",
              )}
            >
              {value === "all" ? t("statusAll") : t(`status.${value}`)}
              {" "}
              <span className="text-xs font-bold opacity-75">{counts[value]}</span>
            </button>
          ))}
        </div>
        <div className="flex flex-wrap gap-2">
          <NativeSelect aria-label={t("type")} value={type} onChange={(e) => setType(e.target.value as TypeFilter)} className="h-9 text-sm">
            {typeFilters.map((value) => (
              <option key={value} value={value}>
                {t(`filters.${value}`)}
              </option>
            ))}
          </NativeSelect>
          <NativeSelect aria-label={t("sortLabel")} value={sort} onChange={(e) => setSort(e.target.value as Sort)} className="h-9 text-sm">
            {sorts.map((value) => (
              <option key={value} value={value}>
                {t(`sort.${value}`)}
              </option>
            ))}
          </NativeSelect>
        </div>
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
      {hidden > 0 && (
        <p className="flex flex-wrap items-center justify-center gap-1.5 text-sm text-muted-foreground" data-testid="hidden-note">
          {t("hiddenNote", { count: hidden })}
          <Button variant="link" size="sm" onClick={() => changeStatus("all")}>
            {t("showAll")}
          </Button>
        </p>
      )}
    </div>
  );
}

export function RecurringItemRow({
  item,
  accounts,
  categories,
  defaultDate,
  showCategory = true,
}: {
  item: RecurringRow;
  accounts: AccountOption[];
  categories: CategoryOption[];
  defaultDate: string;
  /** Shows the category pill; hidden when a heading already names the category. */
  showCategory?: boolean;
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

  const isMonthly = item.interval_unit === "month" && item.interval_count === 1;
  const nextDue =
    item.status === "active" && item.next_due_on
      ? t("nextDue", { date: formatDate(item.next_due_on, locale) })
      : t("noNextDue");
  const details = [
    t(`frequency.${item.interval_unit}`, { count: item.interval_count }),
    item.account_name,
    nextDue + (item.last_payment ? ` · ${t("lastPaid", { date: formatDate(item.last_payment.date, locale) })}` : ""),
  ];

  return (
    <li className={cn("flex min-h-17.5 items-center gap-3 py-2.5", item.status !== "active" && "opacity-70")} data-testid="recurring-row">
      <div className="min-w-0 flex-1">
        <p className="flex items-center gap-2 text-[15.5px] font-semibold">
          <span className="min-w-0 truncate">{item.name}</span>
          {item.status !== "active" && <Badge variant="secondary">{t(`status.${item.status}`)}</Badge>}
          {item.status === "active" && item.current_period && <PaymentStatusBadge status={item.current_period.status} />}
        </p>
        <div className="mt-0.5 flex flex-wrap items-center gap-x-2 gap-y-1 text-[12.5px] text-muted-foreground">
          {showCategory && item.category_name && <CategoryPill name={item.category_name} color={item.category_color} />}
          <span>{details.join(" · ")}</span>
        </div>
        {item.status === "active" && item.current_period && (
          // Stays mounted once paid so the dialog outlives the revalidation and can toast and close.
          <RegisterPaymentDialog
            defaultDate={defaultDate}
            target={{
              id: item.id,
              name: item.name,
              amount: item.amount,
              currency: item.currency,
              minor_units: item.minor_units,
              period: item.current_period.due_on,
            }}
            trigger={
              item.current_period.status === "paid" ? undefined : (
                <Button variant="outline" size="sm" className="mt-1.5">
                  {t("register")}
                </Button>
              )
            }
          />
        )}
      </div>
      <div className="text-right whitespace-nowrap">
        <p className={cn("text-[15.5px] font-bold", item.type === "expense" && "text-expense")}>
          <Money amount={item.amount} currency={item.currency} minorUnits={item.minor_units} tone={item.type} signed />
        </p>
        {!isMonthly && (
          <p className="text-xs font-medium text-muted-foreground" data-testid="recurring-monthly">
            <Money amount={item.monthly_amount} currency={item.currency} minorUnits={item.minor_units} /> {t("monthlyEquivalent")}
          </p>
        )}
      </div>
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
