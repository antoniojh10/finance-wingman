import Link from "next/link";
import { useTranslations } from "next-intl";

import { OwnerFilter } from "@/components/owner-filter";
import type { OwnerOption } from "@/lib/owners";
import { PERIODS, reportsHref, type ReportsQuery } from "@/lib/reports";
import { cn } from "@/lib/utils";

type Option = { key: string; label: string; href: string; active: boolean };

/** A pill switch of links, so every choice lives in the URL. */
function SegmentedLinks({ label, options }: { label: string; options: Option[] }) {
  return (
    <nav aria-label={label} className="grid gap-1.5 max-md:w-full">
      <p aria-hidden className="text-[13px] font-bold text-muted-foreground">
        {label}
      </p>
      <div className="flex gap-1 rounded-2xl bg-muted p-1">
        {options.map((option) => (
          <Link
            key={option.key}
            href={option.href}
            aria-current={option.active ? "page" : undefined}
            className={cn(
              "flex min-h-11 min-w-0 flex-1 items-center justify-center rounded-xl px-3 text-sm font-bold whitespace-nowrap text-muted-foreground transition-colors hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none",
              option.active && "bg-card text-foreground shadow-sm",
            )}
          >
            {option.label}
          </Link>
        ))}
      </div>
    </nav>
  );
}

/**
 * Period, currency, scale, current-month and owner filters in one row.
 * The currency choice is hidden when there is only one.
 */
export function ReportsFilters({
  query,
  currencies,
  activeCurrency,
  owners,
  userId,
}: {
  query: ReportsQuery;
  currencies: string[];
  /** The currency on screen, which may differ from the requested one. */
  activeCurrency?: string;
  owners: OwnerOption[];
  userId: string;
}) {
  const t = useTranslations("reports");

  return (
    <div className="grid gap-4">
      <div className="flex flex-wrap items-end gap-x-5 gap-y-3">
        <SegmentedLinks
          label={t("period")}
          options={PERIODS.map((months) => ({
            key: String(months),
            label: t("periodOption", { count: months }),
            href: reportsHref(query, { months }),
            active: query.months === months,
          }))}
        />
        {currencies.length > 1 && (
          <SegmentedLinks
            label={t("currency")}
            options={currencies.map((currency) => ({
              key: currency,
              label: currency,
              href: reportsHref(query, { currency }),
              active: activeCurrency === currency,
            }))}
          />
        )}
        <SegmentedLinks
          label={t("scale")}
          options={[
            { key: "amount", label: t("scaleAmount"), href: reportsHref(query, { scale: "amount" }), active: query.scale === "amount" },
            { key: "share", label: t("scaleShare"), href: reportsHref(query, { scale: "share" }), active: query.scale === "share" },
          ]}
        />
        <Link
          href={reportsHref(query, { includeCurrent: !query.includeCurrent })}
          role="switch"
          aria-checked={query.includeCurrent}
          className="flex min-h-11 items-center gap-2.5 text-sm font-semibold focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none"
        >
          <span
            aria-hidden
            className={cn("flex h-6 w-10 shrink-0 items-center rounded-full p-0.5 transition-colors", query.includeCurrent ? "bg-primary" : "bg-muted-foreground/40")}
          >
            <span className={cn("size-5 rounded-full bg-white shadow transition-transform", query.includeCurrent && "translate-x-4")} />
          </span>
          {t("includeCurrent")}
        </Link>
      </div>
      <OwnerFilter owners={owners} userId={userId} value={query.owner} hrefFor={(owner) => reportsHref(query, { owner })} />
    </div>
  );
}
