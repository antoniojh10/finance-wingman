"use client";

import { useId, useState } from "react";
import { useLocale, useTranslations } from "next-intl";

import { useWidth } from "@/components/reports/monthly-chart";
import { intlLocale, isLocale } from "@/i18n/locales";
import { areaPath, buildTrend, crossesZero, linePath, nearestIndex, xTickIndexes, yScale, type DailyCurrency, type TrendRecurring } from "@/lib/balance-trend";
import { formatDate } from "@/lib/dates";
import { formatMoney } from "@/lib/money";
import { formatCompact } from "@/lib/reports";

type Hover = { index: number; recurring: TrendRecurring | null };

const TOOLTIP_WIDTH = 210;

/**
 * Daily balance of one currency for the month in view: the real balance as
 * a line with a soft fill, then a dashed projection to the month end with
 * hollow dots on the pending recurring items. Plain SVG; a crosshair and
 * tooltip follow the pointer, and the arrow keys move it by day.
 */
export function BalanceTrendChart({ data }: { data: DailyCurrency }) {
  const t = useTranslations("balanceTrend");
  const appLocale = useLocale();
  const locale = intlLocale(isLocale(appLocale) ? appLocale : "en");
  const [boxRef, width] = useWidth<HTMLDivElement>(640);
  const [hover, setHover] = useState<Hover | null>(null);
  const gradientId = useId().replace(/:/g, "");

  const trend = buildTrend(data);
  const { points, todayIndex, projectionStart } = trend;
  const money = (amount: number) => formatMoney(amount, data.currency, data.minor_units, locale);
  const compact = (amount: number) => formatCompact(amount, data.currency, data.minor_units, locale);

  const narrow = width < 520;
  const height = narrow ? 220 : 260;
  const margin = { top: 28, right: narrow ? 12 : 16, bottom: 28, left: narrow ? 48 : 58 };
  const innerW = width - margin.left - margin.right;
  const values = points.map((p) => p.balance);
  const scale = yScale(values);
  const x = (i: number) => (points.length === 1 ? margin.left + innerW / 2 : margin.left + (i / (points.length - 1)) * innerW);
  const y = (value: number) => margin.top + (1 - (value - scale.min) / (scale.max - scale.min)) * (height - margin.top - margin.bottom);
  const baseline = height - margin.bottom;
  const coords = points.map((p, i) => ({ x: x(i), y: y(p.balance) }));

  const realCoords = todayIndex === null ? [] : coords.slice(0, todayIndex + 1);
  // The projection continues from today's point; a future month has no such point.
  const projectedCoords = projectionStart === null ? [] : coords.slice(Math.max(projectionStart - 1, 0));
  const showToday = todayIndex !== null && projectionStart !== null;

  const move = (event: React.PointerEvent<SVGRectElement>) => {
    const rect = event.currentTarget.getBoundingClientRect();
    const scaleX = rect.width ? width / rect.width : 1;
    setHover({ index: nearestIndex((event.clientX - rect.left) * scaleX, margin.left, width - margin.right, points.length), recurring: null });
  };
  const onKeyDown = (event: React.KeyboardEvent) => {
    if (event.key !== "ArrowLeft" && event.key !== "ArrowRight") {
      return;
    }
    event.preventDefault();
    const step = event.key === "ArrowRight" ? 1 : -1;
    setHover((current) => ({ index: Math.min(points.length - 1, Math.max(0, (current?.index ?? (step > 0 ? -1 : points.length)) + step)), recurring: null }));
  };

  const active = hover ? points[hover.index] : null;
  const anchor = hover ? { x: hover.recurring ? x(hover.recurring.index) : x(hover.index), y: y(points[hover.recurring ? hover.recurring.index : hover.index].balance) } : null;
  let tipLeft = anchor ? anchor.x + 14 : 0;
  if (anchor && tipLeft + TOOLTIP_WIDTH > width) {
    tipLeft = anchor.x - TOOLTIP_WIDTH - 14;
  }
  tipLeft = Math.max(0, tipLeft);

  const endIndex = points.length - 1;
  const showEnd = trend.endBalance !== null;

  return (
    <div ref={boxRef} className="relative w-full" data-testid="balance-trend-chart">
      <svg viewBox={`0 0 ${width} ${height}`} role="group" aria-label={t("chartLabel")} className="block h-auto w-full overflow-visible">
        <defs>
          <linearGradient id={gradientId} x1="0" x2="0" y1="0" y2="1">
            <stop offset="0%" stopColor="var(--primary)" stopOpacity={0.22} />
            <stop offset="100%" stopColor="var(--primary)" stopOpacity={0.02} />
          </linearGradient>
        </defs>

        {scale.ticks.map((value) => (
          <g key={value}>
            <line x1={margin.left} x2={width - margin.right} y1={y(value)} y2={y(value)} stroke="var(--border)" />
            <text x={margin.left - 8} y={y(value) + 4} textAnchor="end" className="fill-muted-foreground font-mono text-[11px]">
              {compact(value)}
            </text>
          </g>
        ))}
        {crossesZero(values) && (
          <line x1={margin.left} x2={width - margin.right} y1={y(0)} y2={y(0)} stroke="var(--muted-foreground)" strokeDasharray="4 4" strokeOpacity={0.7} data-testid="zero-line" />
        )}

        {xTickIndexes(points.length, narrow ? 4 : 5).map((i) => (
          <text key={i} x={x(i)} y={height - 8} textAnchor={i === 0 ? "start" : i === endIndex ? "end" : "middle"} className="fill-muted-foreground font-mono text-[11px]">
            {new Intl.DateTimeFormat(locale, { day: "numeric", month: "short", timeZone: "UTC" }).format(new Date(`${points[i].date}T00:00:00Z`))}
          </text>
        ))}

        {realCoords.length > 1 && <path d={areaPath(realCoords, baseline)} fill={`url(#${gradientId})`} data-testid="balance-area" />}
        {realCoords.length > 1 && <path d={linePath(realCoords)} fill="none" stroke="var(--primary)" strokeWidth={2} strokeLinejoin="round" strokeLinecap="round" data-testid="balance-line" />}
        {realCoords.length === 1 && <circle cx={realCoords[0].x} cy={realCoords[0].y} r={3} fill="var(--primary)" />}
        {projectedCoords.length > 1 && (
          <path d={linePath(projectedCoords)} fill="none" stroke="var(--primary)" strokeWidth={2} strokeDasharray="5 4" strokeLinejoin="round" data-testid="projection-line" />
        )}

        {showToday && (
          <g data-testid="today-marker" pointerEvents="none">
            <line x1={x(todayIndex)} x2={x(todayIndex)} y1={margin.top - 6} y2={baseline} stroke="var(--muted-foreground)" strokeOpacity={0.6} strokeDasharray="2 3" />
            <text x={x(todayIndex)} y={margin.top - 12} textAnchor="middle" className="fill-muted-foreground text-[11px] font-medium">
              {t("today")}
            </text>
            <circle cx={x(todayIndex)} cy={y(points[todayIndex].balance)} r={4} fill="var(--primary)" stroke="var(--card)" strokeWidth={2} />
          </g>
        )}

        {trend.low && (
          <text x={Math.min(x(trend.low.index), width - margin.right - 40)} y={y(trend.low.balance) + 18} textAnchor="middle" className="fill-expense font-mono text-[11px] font-medium" data-testid="low-label">
            {t("low", { amount: compact(trend.low.balance) })}
          </text>
        )}
        {showEnd && (
          <text x={x(endIndex)} y={y(points[endIndex].balance) - 10} textAnchor="end" className="fill-foreground font-mono text-[11px] font-medium" data-testid="end-label">
            {compact(trend.endBalance!)}
          </text>
        )}

        <rect
          x={margin.left}
          y={margin.top - 6}
          width={Math.max(innerW, 0)}
          height={baseline - margin.top + 6}
          fill="transparent"
          tabIndex={0}
          role="img"
          aria-label={t("chartHint")}
          data-testid="balance-trend-overlay"
          className="focus:outline-none focus-visible:stroke-primary"
          onPointerMove={move}
          onPointerDown={move}
          onPointerLeave={() => setHover(null)}
          onKeyDown={onKeyDown}
          onBlur={() => setHover(null)}
        />

        {trend.recurring.map((r, k) => (
          <circle
            key={`${r.recurringId}-${k}`}
            cx={x(r.index)}
            cy={y(points[r.index].balance)}
            r={4.5}
            fill="var(--card)"
            stroke="var(--primary)"
            strokeWidth={2}
            tabIndex={0}
            role="img"
            aria-label={`${r.name}, ${formatDate(r.date, locale)}: ${money(r.amount)}`}
            data-testid="recurring-dot"
            className="cursor-pointer focus:outline-none focus-visible:stroke-foreground"
            onPointerEnter={() => setHover({ index: r.index, recurring: r })}
            onPointerLeave={() => setHover(null)}
            onFocus={() => setHover({ index: r.index, recurring: r })}
            onBlur={() => setHover(null)}
          />
        ))}

        {hover && anchor && !hover.recurring && (
          <g pointerEvents="none" data-testid="crosshair">
            <line x1={anchor.x} x2={anchor.x} y1={margin.top} y2={baseline} stroke="var(--foreground)" strokeOpacity={0.35} />
            <circle cx={anchor.x} cy={anchor.y} r={4} fill="var(--card)" stroke="var(--primary)" strokeWidth={2} />
          </g>
        )}
      </svg>

      {hover && active && anchor && (
        <div
          role="tooltip"
          style={{ left: tipLeft, top: anchor.y > 130 ? anchor.y - 12 : anchor.y + 16, width: TOOLTIP_WIDTH, transform: anchor.y > 130 ? "translateY(-100%)" : undefined }}
          className="pointer-events-none absolute z-10 grid gap-1.5 rounded-xl border bg-card p-3 text-[13px] text-foreground shadow-lg"
        >
          {hover.recurring ? (
            <>
              <p className="font-medium">{hover.recurring.name}</p>
              <p className="font-mono text-[17px] font-medium tabular-nums">{money(hover.recurring.amount)}</p>
              <p className="text-muted-foreground">{formatDate(hover.recurring.date, locale)}</p>
            </>
          ) : (
            <>
              <p className="text-muted-foreground">{formatDate(active.date, locale)}</p>
              <p className="font-mono text-[17px] font-medium tabular-nums">{money(active.balance)}</p>
              <p className="text-muted-foreground">{active.projected ? t("projectedBalance") : t("balance")}</p>
              {active.net !== null && (
                <p className="flex justify-between gap-3.5 text-muted-foreground">
                  <span>{t("dayNet")}</span>
                  <b className="font-mono font-medium text-foreground tabular-nums">{money(active.net)}</b>
                </p>
              )}
            </>
          )}
        </div>
      )}
    </div>
  );
}
