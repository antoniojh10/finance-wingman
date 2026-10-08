"use client";

import { useTranslations } from "next-intl";
import { useCallback } from "react";
import { toast } from "sonner";

import { findRecurringMatch, linkTransactionRecurring } from "@/app/actions/transactions";

/**
 * Returns a function that announces a saved transaction. For a newly created
 * one it also looks for a matching active subscription and, when found, adds
 * a "Link to <name>?" action to the toast. The lookup is best effort: any
 * failure just shows the plain confirmation. A budget warning (the expense
 * left its category near or over budget) is shown as the toast description.
 */
export function useSavedToast() {
  const t = useTranslations("transactions");

  return useCallback(
    async (createdId?: string, budgetWarning?: string) => {
      const match = createdId ? await findRecurringMatch(createdId) : null;
      if (!createdId || !match) {
        if (budgetWarning) {
          toast.success(t("saved"), { description: budgetWarning, duration: 8000 });
        } else {
          toast.success(t("saved"));
        }
        return;
      }
      toast.success(t("saved"), {
        description: budgetWarning,
        duration: 8000,
        action: {
          label: t("linkSuggestion", { name: match.name }),
          onClick: async () => {
            const result = await linkTransactionRecurring(createdId, match.recurringId, match.period ?? undefined);
            if (result.ok) {
              toast.success(t("linked", { name: match.name }));
            } else {
              toast.error(result.message ?? t("saved"));
            }
          },
        },
      });
    },
    [t],
  );
}
