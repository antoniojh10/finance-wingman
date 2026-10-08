import { useTranslations } from "next-intl";

import { barSegments, type BudgetState } from "@/lib/budgets";
import { cn } from "@/lib/utils";

const stateColor: Record<Exclude<BudgetState, "none">, string> = {
  ok: "text-primary",
  near: "text-amber-500",
  over: "text-destructive",
};

/**
 * Progress of a budget: the spent share is solid and the committed share
 * (unpaid recurring expenses due this month) is a lighter striped segment
 * after it. Both amounts are minor units of the same currency.
 */
export function BudgetBar({
  amount,
  spent,
  committed,
  state,
  className,
}: {
  amount: number;
  spent: number;
  committed: number;
  state: Exclude<BudgetState, "none">;
  className?: string;
}) {
  const t = useTranslations("budgets");
  const segments = barSegments(amount, spent, committed);
  const used = amount > 0 ? Math.round(((spent + committed) / amount) * 100) : spent + committed > 0 ? 100 : 0;

  return (
    <div
      role="progressbar"
      aria-label={t("progress")}
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={Math.min(used, 100)}
      aria-valuetext={t("used", { percent: used })}
      data-state={state}
      className={cn("flex h-2.5 overflow-hidden rounded-full bg-muted", stateColor[state], className)}
    >
      <div data-segment="spent" className="h-full bg-current" style={{ width: `${segments.spent}%` }} />
      <div
        data-segment="committed"
        className="h-full opacity-45"
        style={{
          width: `${segments.committed}%`,
          backgroundImage: "repeating-linear-gradient(135deg, currentColor 0 3px, transparent 3px 6px)",
        }}
      />
    </div>
  );
}
