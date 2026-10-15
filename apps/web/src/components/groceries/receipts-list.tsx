import { ChevronRightIcon } from "lucide-react";
import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";

import { MonthPicker } from "@/components/dashboard/month-picker";
import { Money } from "@/components/money";
import { Badge } from "@/components/ui/badge";
import { buttonVariants } from "@/components/ui/button";
import { formatDate } from "@/lib/dates";
import { ALL_MONTHS, minorUnitsOf, receiptsHref, type ReceiptSummary, type ReceiptsQuery } from "@/lib/groceries";
import { intlLocale, isLocale } from "@/i18n/locales";
import { cn } from "@/lib/utils";

type StoreOption = { id: string; name: string };

function Chip({ href, active, children }: { href: string; active: boolean; children: React.ReactNode }) {
  return (
    <Link
      href={href}
      aria-current={active ? "page" : undefined}
      className={cn(
        "flex min-h-11 items-center rounded-xl px-3.5 text-sm font-bold whitespace-nowrap text-muted-foreground transition-colors hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none",
        active && "bg-card text-foreground shadow-sm",
      )}
    >
      {children}
    </Link>
  );
}

/** Month navigation plus the store filter; both live in the URL. */
export function ReceiptsFilters({
  query,
  month,
  currentMonth,
  stores,
}: {
  query: ReceiptsQuery;
  /** The month on screen, or "all". */
  month: string;
  currentMonth: string;
  stores: StoreOption[];
}) {
  const t = useTranslations("groceries.receipts");
  const all = month === ALL_MONTHS;

  return (
    <div className="grid gap-4">
      <div className="flex flex-wrap items-center gap-x-4 gap-y-3 max-md:justify-between">
        {!all && <MonthPicker month={month} basePath="/groceries/receipts" keep={{ store: query.storeId }} />}
        <div className="flex gap-1 rounded-2xl bg-muted p-1">
          <Chip href={receiptsHref(query, { month: currentMonth })} active={!all}>
            {t("byMonth")}
          </Chip>
          <Chip href={receiptsHref(query, { month: ALL_MONTHS })} active={all}>
            {t("allMonths")}
          </Chip>
        </div>
      </div>
      {stores.length > 1 && (
        <nav aria-label={t("filterStore")} className="flex flex-wrap gap-1 self-start rounded-2xl bg-muted p-1">
          <Chip href={receiptsHref({ ...query, month }, { storeId: undefined })} active={!query.storeId}>
            {t("allStores")}
          </Chip>
          {stores.map((store) => (
            <Chip key={store.id} href={receiptsHref({ ...query, month }, { storeId: store.id })} active={query.storeId === store.id}>
              {store.name}
            </Chip>
          ))}
        </nav>
      )}
    </div>
  );
}

/** One receipt: store, date, total and what still needs attention. */
function ReceiptRow({ receipt }: { receipt: ReceiptSummary }) {
  const t = useTranslations("groceries.receipts");
  const locale = useLocale();
  const date = formatDate(receipt.purchased_on, intlLocale(isLocale(locale) ? locale : "en"));
  const minorUnits = minorUnitsOf(receipt.currency);

  return (
    <li>
      <Link
        href={`/groceries/receipts/${receipt.id}`}
        aria-label={t("openReceipt", { store: receipt.store_name, date })}
        className="flex items-center gap-3 rounded-3xl bg-card px-5 py-4 ring-1 ring-foreground/5 transition-colors hover:bg-accent focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none"
      >
        <span className="grid min-w-0 flex-1 gap-1.5">
          <span className="flex items-baseline justify-between gap-3">
            <span className="truncate font-heading text-base font-bold">{receipt.store_name}</span>
            <Money amount={receipt.total} currency={receipt.currency} minorUnits={minorUnits} className="shrink-0 text-base font-bold" />
          </span>
          <span className="text-[13px] text-muted-foreground">
            {date} · {t("lines", { count: receipt.line_count })}
          </span>
          <span className="flex flex-wrap gap-1.5 empty:hidden">
            {receipt.unassigned_count > 0 && (
              <Badge variant="secondary" className="h-6 bg-amber-500/15 px-2.5 text-amber-800 dark:text-amber-300">
                {t("unassigned", { count: receipt.unassigned_count })}
              </Badge>
            )}
            {receipt.lines_total !== receipt.total && (
              <Badge variant="destructive" className="h-6 px-2.5">
                {t("linesMismatch")}
              </Badge>
            )}
            <Badge variant="outline" className="h-6 px-2.5 text-muted-foreground">
              {receipt.transaction_id ? t("linked") : t("noExpense")}
            </Badge>
          </span>
        </span>
        <ChevronRightIcon className="size-5 shrink-0 text-muted-foreground" aria-hidden />
      </Link>
    </li>
  );
}

/** The receipts tab body: filters, then the list or an empty state. */
export function ReceiptsList({
  receipts,
  stores,
  query,
  month,
  currentMonth,
}: {
  receipts: ReceiptSummary[];
  stores: StoreOption[];
  query: ReceiptsQuery;
  /** The month on screen (YYYY-MM) or "all". */
  month: string;
  currentMonth: string;
}) {
  const t = useTranslations("groceries.receipts");
  const filtered = month !== ALL_MONTHS || Boolean(query.storeId);

  return (
    <div className="grid gap-5">
      <ReceiptsFilters query={query} month={month} currentMonth={currentMonth} stores={stores} />
      {receipts.length > 0 ? (
        <ul className="grid gap-3" data-testid="receipts-list">
          {receipts.map((receipt) => (
            <ReceiptRow key={receipt.id} receipt={receipt} />
          ))}
        </ul>
      ) : (
        <div className="grid justify-items-center gap-3 rounded-3xl bg-card px-6 py-12 text-center ring-1 ring-foreground/5" data-testid="receipts-empty">
          <p className="max-w-md text-sm text-muted-foreground">{filtered ? t("emptyMonth") : t("empty")}</p>
          {filtered && (
            <Link href="/groceries/receipts?month=all" className={buttonVariants({ variant: "secondary" })}>
              {t("showAll")}
            </Link>
          )}
        </div>
      )}
    </div>
  );
}
