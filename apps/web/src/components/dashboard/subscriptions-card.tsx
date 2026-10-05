import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";

import { Money } from "@/components/money";
import { PaymentStatusBadge } from "@/components/recurring/payment-status-badge";
import { RegisterPaymentDialog } from "@/components/recurring/register-payment-dialog";
import { Button } from "@/components/ui/button";
import { Card, CardAction, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { formatDate } from "@/lib/dates";
import type { PaymentStatus, RecurringType } from "@/lib/recurring";
import { cn } from "@/lib/utils";

export type UpcomingPayment = {
  due_on: string;
  status: PaymentStatus;
  item: {
    id: string;
    name: string;
    type: RecurringType;
    amount: number;
    currency: string;
    minor_units: number;
    account_name: string;
  };
};

export type CommittedCost = { currency: string; minor_units: number; expense: number };

/**
 * Subscriptions at a glance: what is overdue or due in the next days (with a
 * shortcut to register the payment) and the committed monthly cost per currency.
 */
export function SubscriptionsCard({
  upcoming,
  committed,
  defaultDate,
}: {
  upcoming: UpcomingPayment[];
  committed: CommittedCost[];
  /** Today's date, the default payment date. */
  defaultDate: string;
}) {
  const t = useTranslations();
  const locale = useLocale();
  const costs = committed.filter((c) => c.expense > 0);

  return (
    <Card data-testid="subscriptions-card">
      <CardHeader>
        <CardTitle>
          <h2>{t("dashboard.subscriptions")}</h2>
        </CardTitle>
        <CardAction>
          <Link href="/subscriptions" className="flex min-h-9 items-center text-sm font-bold text-primary">
            {t("dashboard.viewAll")}
          </Link>
        </CardAction>
      </CardHeader>
      <CardContent className="grid grid-cols-1 gap-4">
        {upcoming.length === 0 ? (
          <p className="py-2 text-sm text-muted-foreground">{t("dashboard.subscriptionsNothingDue")}</p>
        ) : (
          <ul className="divide-y divide-border/70">
            {upcoming.map(({ item, due_on, status }) => (
              <li
                key={`${item.id}-${due_on}`}
                className="flex flex-wrap items-center gap-x-3 gap-y-2 py-3"
                data-testid="upcoming-row"
              >
                <div className="min-w-0 flex-1 basis-40">
                  <p className="flex items-center gap-2 text-[15px] font-semibold">
                    <span className="min-w-0 truncate">{item.name}</span>
                    <PaymentStatusBadge status={status} />
                  </p>
                  <p className="truncate text-[12.5px] text-muted-foreground">
                    {t("subscriptions.dueOn", { date: formatDate(due_on, locale) })} · {item.account_name}
                  </p>
                </div>
                <Money
                  amount={item.amount}
                  currency={item.currency}
                  minorUnits={item.minor_units}
                  tone={item.type}
                  signed
                  className={cn("text-[15px] font-bold whitespace-nowrap", item.type === "expense" && "text-expense")}
                />
                {/* Stays mounted once paid so the dialog outlives the revalidation and can toast and close. */}
                <RegisterPaymentDialog
                    defaultDate={defaultDate}
                    target={{
                      id: item.id,
                      name: item.name,
                      amount: item.amount,
                      currency: item.currency,
                      minor_units: item.minor_units,
                      period: due_on,
                    }}
                    trigger={
                      status === "paid" ? undefined : (
                        <Button variant="outline" size="sm">
                          {t("subscriptions.register")}
                        </Button>
                      )
                    }
                  />
              </li>
            ))}
          </ul>
        )}
        {costs.length > 0 && (
          <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1 border-t border-border/70 pt-3">
            <span className="text-sm text-muted-foreground">{t("dashboard.subscriptionsCommitted")}</span>
            <span className="flex flex-wrap gap-x-3 text-[15px] font-bold" data-testid="committed-costs">
              {costs.map((c) => (
                <Money key={c.currency} amount={c.expense} currency={c.currency} minorUnits={c.minor_units} />
              ))}
            </span>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
