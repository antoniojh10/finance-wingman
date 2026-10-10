import { useLocale, useTranslations } from "next-intl";

import { intlLocale, isLocale } from "@/i18n/locales";
import type { PointDetail } from "@/lib/reports";
import { cn } from "@/lib/utils";

const WIDTH = 220;

/** Detail of one chart segment: amount, month, share and the difference against the category average. */
export function ChartTooltip({
  detail,
  x,
  y,
  containerWidth,
  money,
  monthLabel,
}: {
  detail: PointDetail;
  /** Anchor point, relative to the chart box. */
  x: number;
  y: number;
  containerWidth: number;
  money: (amount: number) => string;
  monthLabel: string;
}) {
  const t = useTranslations("reports");
  const appLocale = useLocale();
  const percent = new Intl.NumberFormat(intlLocale(isLocale(appLocale) ? appLocale : "en"), { style: "percent", maximumFractionDigits: 0 });
  const { series, delta } = detail;

  let left = x + 14;
  if (left + WIDTH > containerWidth) {
    left = x - WIDTH - 14;
  }
  left = Math.max(0, left);
  const top = y > 150 ? y - 12 : y + 16;

  return (
    <div
      role="tooltip"
      style={{ left, top, width: WIDTH, transform: y > 150 ? "translateY(-100%)" : undefined }}
      className="pointer-events-none absolute z-10 grid gap-1.5 rounded-xl border bg-card p-3 text-[13px] text-foreground shadow-lg"
    >
      <p className="font-mono text-[17px] font-medium tabular-nums">{money(detail.amount)}</p>
      <p className="flex items-center gap-2">
        <i
          aria-hidden
          className="block h-[3px] w-3 shrink-0 rounded-sm"
          style={{ background: series.other ? "var(--muted-foreground)" : (series.color ?? "var(--muted-foreground)") }}
        />
        <span>
          {series.name} · {monthLabel}
          {detail.partial && ` (${t("inProgressShort")})`}
        </span>
      </p>
      <Row label={t("tipShare")} value={percent.format(detail.share)} />
      <Row label={t("tipAverage")} value={money(detail.average)} />
      {delta !== null && (
        <Row
          label={t("tipVsAverage")}
          value={`${delta >= 0 ? "+" : "−"}${percent.format(Math.abs(delta))}`}
          className={delta > 0.1 ? "text-expense" : delta < -0.1 ? "text-income" : undefined}
        />
      )}
      {series.other && <p className="text-muted-foreground">{series.members.join(", ")}</p>}
    </div>
  );
}

function Row({ label, value, className }: { label: string; value: string; className?: string }) {
  return (
    <p className="flex justify-between gap-3.5 text-muted-foreground">
      <span>{label}</span>
      <b className={cn("font-mono font-medium text-foreground tabular-nums", className)}>{value}</b>
    </p>
  );
}
