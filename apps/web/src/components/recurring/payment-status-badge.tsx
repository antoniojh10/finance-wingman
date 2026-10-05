import { useTranslations } from "next-intl";

import { Badge } from "@/components/ui/badge";
import type { PaymentStatus } from "@/lib/recurring";
import { cn } from "@/lib/utils";

/** Whether the current period of a subscription is paid, pending or overdue. */
export function PaymentStatusBadge({ status, className }: { status: PaymentStatus; className?: string }) {
  const t = useTranslations("subscriptions.paymentStatus");
  return (
    <Badge
      variant={status === "overdue" ? "destructive" : "secondary"}
      className={cn(status === "paid" && "bg-income/15 text-income", className)}
      data-status={status}
    >
      {t(status)}
    </Badge>
  );
}
