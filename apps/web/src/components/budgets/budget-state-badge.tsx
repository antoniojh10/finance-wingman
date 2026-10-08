import { useTranslations } from "next-intl";

import { Badge } from "@/components/ui/badge";
import type { BudgetState } from "@/lib/budgets";
import { cn } from "@/lib/utils";

/** State of a budget line: on track, near the limit or over it. */
export function BudgetStateBadge({ state, className }: { state: Exclude<BudgetState, "none">; className?: string }) {
  const t = useTranslations("budgets.state");
  return (
    <Badge
      variant={state === "over" ? "destructive" : "secondary"}
      className={cn(
        state === "ok" && "bg-income/15 text-income",
        state === "near" && "bg-amber-500/15 text-amber-700 dark:text-amber-400",
        className,
      )}
      data-state={state}
    >
      {t(state)}
    </Badge>
  );
}
