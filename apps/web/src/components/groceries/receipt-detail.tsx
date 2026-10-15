"use client";

import { ArrowLeftIcon, CheckIcon, TriangleAlertIcon } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useLocale, useTranslations } from "next-intl";
import { useState, useTransition } from "react";
import { toast } from "sonner";

import { deleteReceipt, unassignReceiptLine } from "@/app/actions/groceries";
import { ConfirmAction } from "@/components/confirm-action";
import { Money } from "@/components/money";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { formatDate } from "@/lib/dates";
import {
  formatQuantity,
  lineStatus,
  minorUnitsOf,
  transactionHref,
  variantLabel,
  type CatalogEntry,
  type Receipt,
  type ReceiptLine,
  type ReceiptWarning,
} from "@/lib/groceries";
import { intlLocale, isLocale } from "@/i18n/locales";
import { formatMoney } from "@/lib/money";

import { AssignLineForm, type GroupOption } from "./assign-line-form";

function WarningText({ warning, currency }: { warning: ReceiptWarning; currency: string }) {
  const t = useTranslations("groceries.receipt");
  const current = useLocale();
  const locale = intlLocale(isLocale(current) ? current : "en");
  const money = (amount = 0) => formatMoney(amount, currency, minorUnitsOf(currency), locale);

  return warning.code === "total_mismatch"
    ? t("warningTotalMismatch", { receipt: money(warning.receipt_total), transaction: money(warning.transaction_amount) })
    : t("warningLinesMismatch", { lines: money(warning.lines_total), receipt: money(warning.receipt_total) });
}

function LineRow({
  receipt,
  line,
  open,
  catalog,
  groups,
  onOpen,
  onClose,
}: {
  receipt: Receipt;
  line: ReceiptLine;
  open: boolean;
  catalog: CatalogEntry[];
  groups: GroupOption[];
  onOpen: () => void;
  onClose: () => void;
}) {
  const t = useTranslations("groceries.receipt");
  const current = useLocale();
  const locale = intlLocale(isLocale(current) ? current : "en");
  const [pending, startTransition] = useTransition();
  const minorUnits = minorUnitsOf(receipt.currency);
  const assigned = lineStatus(line) === "assigned";

  function unassign() {
    startTransition(async () => {
      const result = await unassignReceiptLine(receipt.id, line.id);
      if (result.ok) {
        toast.success(t("unassignedToast"));
      } else {
        toast.error(result.message);
      }
    });
  }

  return (
    <li className="grid gap-2 py-3" data-testid="receipt-line" data-status={assigned ? "assigned" : "unassigned"}>
      <div className="flex items-baseline justify-between gap-3 font-mono text-[13px] leading-snug">
        <span className="min-w-0 break-words">
          {line.quantity !== 1 && <span className="text-muted-foreground">{formatQuantity(line.quantity)} × </span>}
          {line.raw_text}
        </span>
        <span className="shrink-0 text-right">
          <Money amount={line.amount} currency={receipt.currency} minorUnits={minorUnits} />
          {line.discount > 0 && (
            <span className="block text-[11px] text-muted-foreground">
              {t("discount", { amount: formatMoney(line.discount, receipt.currency, minorUnits, locale) })}
            </span>
          )}
        </span>
      </div>

      <div className="flex flex-wrap items-center gap-2">
        {assigned ? (
          <Badge variant="secondary" className="h-7 max-w-full gap-1.5 bg-income/12 px-2.5 text-income">
            <CheckIcon aria-hidden />
            <span className="truncate">
              {line.product_name}
              {variantLabel(line) && <span className="font-normal opacity-80"> · {variantLabel(line)}</span>}
            </span>
          </Badge>
        ) : (
          <Badge variant="secondary" className="h-7 bg-amber-500/15 px-2.5 text-amber-800 dark:text-amber-300">
            {t("unassignedChip")}
          </Badge>
        )}
        {!open && (
          <Button type="button" variant={assigned ? "ghost" : "secondary"} size="sm" onClick={onOpen} className="min-h-9">
            {assigned ? t("change") : t("assign")}
          </Button>
        )}
        {assigned && !open && (
          <Button type="button" variant="ghost" size="sm" onClick={unassign} disabled={pending} className="min-h-9 text-muted-foreground">
            {t("unassign")}
          </Button>
        )}
      </div>

      {open && (
        <AssignLineForm
          receiptId={receipt.id}
          lineId={line.id}
          catalog={catalog}
          groups={groups}
          onDone={() => {
            toast.success(t("assignedToast"));
            onClose();
          }}
          onCancel={onClose}
        />
      )}
    </li>
  );
}

/** A receipt as printed: raw lines in monospace, each with its product mapping. */
export function ReceiptDetail({ receipt, catalog, groups }: { receipt: Receipt; catalog: CatalogEntry[]; groups: GroupOption[] }) {
  const t = useTranslations("groceries.receipt");
  const locale = useLocale();
  const router = useRouter();
  const [openLine, setOpenLine] = useState<string | null>(null);
  const minorUnits = minorUnitsOf(receipt.currency);
  const date = formatDate(receipt.purchased_on, intlLocale(isLocale(locale) ? locale : "en"));

  return (
    <div className="grid gap-5">
      <div>
        <Link href="/groceries/receipts" className="inline-flex min-h-11 items-center gap-1.5 text-sm font-bold text-muted-foreground hover:text-foreground">
          <ArrowLeftIcon className="size-4" aria-hidden />
          {t("back")}
        </Link>
        <h2 className="text-2xl font-bold tracking-tight">{t("title", { store: receipt.store_name })}</h2>
        <p className="text-sm text-muted-foreground">
          {date} · {t(`source.${receipt.source}`)}
        </p>
      </div>

      {receipt.warnings.length > 0 && (
        <div role="alert" className="grid gap-1.5 rounded-2xl bg-amber-500/12 p-4 text-sm text-amber-900 dark:text-amber-200">
          <p className="flex items-center gap-2 font-bold">
            <TriangleAlertIcon className="size-4" aria-hidden />
            {t("warnings")}
          </p>
          {receipt.warnings.map((warning) => (
            <p key={warning.code}>
              <WarningText warning={warning} currency={receipt.currency} />
            </p>
          ))}
        </div>
      )}

      <section aria-labelledby="receipt-lines-heading" className="rounded-3xl bg-card p-5 ring-1 ring-foreground/5 sm:p-6">
        <h3 id="receipt-lines-heading" className="mb-1 text-center font-mono text-sm font-medium tracking-wide uppercase">
          {receipt.store_name}
        </h3>
        <p className="mb-3 text-center font-mono text-xs text-muted-foreground">{date}</p>
        <ul className="divide-y divide-dashed divide-border border-y border-dashed border-border" aria-label={t("linesHeading")}>
          {receipt.lines.map((line) => (
            <LineRow
              key={line.id}
              receipt={receipt}
              line={line}
              open={openLine === line.id}
              catalog={catalog}
              groups={groups}
              onOpen={() => setOpenLine(line.id)}
              onClose={() => setOpenLine(null)}
            />
          ))}
        </ul>
        <dl className="mt-3 grid gap-1 font-mono text-sm">
          <div className="flex justify-between font-medium">
            <dt>{t("total")}</dt>
            <dd>
              <Money amount={receipt.total} currency={receipt.currency} minorUnits={minorUnits} />
            </dd>
          </div>
          {receipt.lines_total !== receipt.total && (
            <div className="flex justify-between text-muted-foreground">
              <dt>{t("linesTotal")}</dt>
              <dd>
                <Money amount={receipt.lines_total} currency={receipt.currency} minorUnits={minorUnits} />
              </dd>
            </div>
          )}
        </dl>
        <p className="mt-3 text-center text-xs text-muted-foreground">
          {receipt.unassigned_count > 0 ? t("unassignedSummary", { count: receipt.unassigned_count }) : t("allAssigned")}
        </p>
      </section>

      <div className="flex flex-col-reverse gap-2 sm:flex-row sm:items-center sm:justify-between">
        <ConfirmAction
          trigger={
            <Button type="button" variant="destructive" size="lg">
              {t("delete")}
            </Button>
          }
          title={t("deleteTitle")}
          description={t("deleteDescription")}
          confirmLabel={t("deleteConfirm")}
          successMessage={t("deletedToast")}
          action={async () => {
            const result = await deleteReceipt(receipt.id);
            if (result.ok) {
              router.push("/groceries/receipts");
            }
            return result;
          }}
        />
        {receipt.transaction_id ? (
          <Button variant="secondary" size="lg" nativeButton={false} render={<Link href={transactionHref(receipt.purchased_on)} />}>
            {t("linkedExpense")}
          </Button>
        ) : (
          <p className="text-sm text-muted-foreground">{t("noExpense")}</p>
        )}
      </div>
    </div>
  );
}
