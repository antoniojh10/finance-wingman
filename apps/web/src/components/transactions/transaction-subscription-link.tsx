"use client";

import { RepeatIcon } from "lucide-react";
import { useLocale, useTranslations } from "next-intl";
import { useState, useTransition } from "react";
import { toast } from "sonner";

import { linkTransactionRecurring, unlinkTransactionRecurring } from "@/app/actions/transactions";
import { NativeSelect } from "@/components/native-select";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { formatDate } from "@/lib/dates";
import type { FormState } from "@/lib/forms";

import type { RecurringOption, TransactionRow } from "./types";

type PeriodOption = { value: string; label: string };

/**
 * Shows which subscription an existing transaction pays and lets the user
 * link or unlink it. Acts immediately (outside the edit form) and follows the
 * live `transaction`, which is refreshed when the link changes.
 */
export function TransactionSubscriptionLink({
  transaction,
  recurringItems,
}: {
  transaction: TransactionRow;
  recurringItems: RecurringOption[];
}) {
  const t = useTranslations("transactions");
  const locale = useLocale();
  const [pending, startTransition] = useTransition();
  const [error, setError] = useState<string>();
  const [selectedId, setSelectedId] = useState("");
  const [period, setPeriod] = useState("");

  if (transaction.type === "transfer") {
    return null;
  }

  function run(action: () => Promise<FormState>, successMessage: string) {
    setError(undefined);
    startTransition(async () => {
      const result = await action();
      if (result.ok) {
        toast.success(successMessage);
      } else {
        setError(result.message);
      }
    });
  }

  if (transaction.recurring_id) {
    const linked = recurringItems.find((item) => item.id === transaction.recurring_id);
    return (
      <div className="grid grid-cols-1 gap-2 rounded-2xl bg-muted px-4 py-3" data-testid="subscription-link">
        <div className="flex items-center gap-3">
          <RepeatIcon className="size-4 shrink-0 text-muted-foreground" aria-hidden />
          <p className="min-w-0 flex-1 truncate text-sm font-semibold">
            {linked ? t("linkedTo", { name: linked.name }) : t("linkedToUnknown")}
          </p>
          <Button
            type="button"
            variant="outline"
            size="sm"
            disabled={pending}
            onClick={() => run(() => unlinkTransactionRecurring(transaction.id), t("unlinked"))}
          >
            {t("unlinkSubscription")}
          </Button>
        </div>
        {error && (
          <p className="text-sm text-destructive" role="alert">
            {error}
          </p>
        )}
      </div>
    );
  }

  const candidates = recurringItems.filter(
    (item) => item.account_id === transaction.account_id && item.type === transaction.type && item.status !== "cancelled",
  );
  if (candidates.length === 0) {
    return null;
  }
  const selected = candidates.find((item) => item.id === selectedId) ?? candidates[0];
  const periods: PeriodOption[] = [];
  if (selected.current_due_on) {
    periods.push({ value: selected.current_due_on, label: t("periodCurrent", { date: formatDate(selected.current_due_on, locale) }) });
  }
  if (selected.next_due_on && selected.next_due_on !== selected.current_due_on) {
    periods.push({ value: selected.next_due_on, label: t("periodNext", { date: formatDate(selected.next_due_on, locale) }) });
  }

  return (
    <div className="grid grid-cols-1 gap-3 rounded-2xl bg-muted px-4 py-3" data-testid="subscription-link">
      <div className="grid grid-cols-1 gap-1.5">
        <Label htmlFor="link-subscription">{t("subscriptionLabel")}</Label>
        <NativeSelect
          id="link-subscription"
          value={selected.id}
          onChange={(event) => {
            setSelectedId(event.target.value);
            setPeriod("");
          }}
        >
          {candidates.map((item) => (
            <option key={item.id} value={item.id}>
              {item.name}
            </option>
          ))}
        </NativeSelect>
      </div>
      <div className="grid grid-cols-1 gap-1.5">
        <Label htmlFor="link-period">{t("period")}</Label>
        <NativeSelect id="link-period" value={period} onChange={(event) => setPeriod(event.target.value)}>
          <option value="">{t("periodClosest")}</option>
          {periods.map((option) => (
            <option key={option.value} value={option.value}>
              {option.label}
            </option>
          ))}
        </NativeSelect>
      </div>
      {error && (
        <p className="text-sm text-destructive" role="alert">
          {error}
        </p>
      )}
      <Button
        type="button"
        variant="outline"
        disabled={pending}
        onClick={() =>
          run(() => linkTransactionRecurring(transaction.id, selected.id, period || undefined), t("linked", { name: selected.name }))
        }
      >
        {t("linkSubscription")}
      </Button>
    </div>
  );
}
