import Link from "next/link";
import { useTranslations } from "next-intl";

import { BudgetBar } from "@/components/budgets/budget-bar";
import { BudgetStateBadge } from "@/components/budgets/budget-state-badge";
import { ColorTile } from "@/components/color-tile";
import { Money } from "@/components/money";
import { Button } from "@/components/ui/button";
import { Card, CardAction, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { attentionLines, hasBudgets, type BudgetCurrencyData } from "@/lib/budgets";

/**
 * Budgets at a glance: the categories of the month that are near or over
 * their budget (spent plus committed recurring), per currency. Without any
 * budget it invites to set them up; with all of them on track it says so.
 */
export function BudgetsCard({ currencies, href }: { currencies: BudgetCurrencyData[]; href: string }) {
  const t = useTranslations("dashboard");
  const configured = hasBudgets(currencies);
  const attention = attentionLines(currencies);

  return (
    <Card data-testid="budgets-card">
      <CardHeader>
        <CardTitle>
          <h2>{t("budgets")}</h2>
        </CardTitle>
        {configured && (
          <CardAction>
            <Link href={href} className="flex min-h-9 items-center text-sm font-bold text-primary">
              {t("viewAll")}
            </Link>
          </CardAction>
        )}
      </CardHeader>
      <CardContent className="grid gap-4">
        {!configured ? (
          <div className="flex flex-wrap items-center justify-between gap-3">
            <p className="text-sm text-muted-foreground">{t("budgetsSetUp")}</p>
            <Button variant="secondary" nativeButton={false} render={<Link href={href} />}>
              {t("budgetsSetUpAction")}
            </Button>
          </div>
        ) : attention.length === 0 ? (
          <p className="py-2 text-sm text-muted-foreground" data-testid="budgets-on-track">
            {t("budgetsAllGood")}
          </p>
        ) : (
          attention.map(({ currency, minorUnits, lines }) => (
            <section key={currency} className="grid gap-1" aria-label={currency} data-testid={`budgets-attention-${currency}`}>
              <h3 className="text-[13px] font-bold text-muted-foreground">{`${t("budgetsAttention")} · ${currency}`}</h3>
              <ul className="divide-y divide-border/70">
                {lines.map((line) => {
                  const name = line.category_name ?? "";
                  const state = line.state === "over" ? "over" : "near";
                  return (
                    <li key={line.category_id} className="grid gap-2 py-3" data-testid="budget-attention-row">
                      <div className="flex items-center gap-3">
                        <ColorTile color={line.category_color} label={name} className="size-9 rounded-[12px]" />
                        <p className="min-w-0 flex-1 truncate text-[15px] font-semibold">{name}</p>
                        <BudgetStateBadge state={state} />
                      </div>
                      {line.amount !== null && (
                        <BudgetBar amount={line.amount} spent={line.spent} committed={line.committed} state={state} />
                      )}
                      {line.amount !== null && (
                        <p className="text-[13px] text-muted-foreground">
                          <Money amount={line.spent + line.committed} currency={currency} minorUnits={minorUnits} /> /{" "}
                          <Money amount={line.amount} currency={currency} minorUnits={minorUnits} />
                        </p>
                      )}
                    </li>
                  );
                })}
              </ul>
            </section>
          ))
        )}
      </CardContent>
    </Card>
  );
}
