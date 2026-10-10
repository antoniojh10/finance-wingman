"use client";

import { useEffect, useId, useRef, useState } from "react";
import { useLocale, useTranslations } from "next-intl";

import { intlLocale, isLocale } from "@/i18n/locales";
import { formatMonth } from "@/lib/dates";
import { formatMoney } from "@/lib/money";
import {
  buildReport,
  computeView,
  describePoint,
  formatCompact,
  formatShortMonth,
  niceMax,
  type MonthlyCurrency,
  type PointDetail,
  type ReportsScale,
  type Series,
} from "@/lib/reports";
import { cn } from "@/lib/utils";

import { ChartTooltip } from "./chart-tooltip";

const FALLBACK_COLOR = "var(--muted-foreground)";
const GAP = 2;
const RADIUS = 4;

/** A bar segment with a rounded top, drawn as a path. */
function roundedTop(x: number, y: number, w: number, h: number, r: number): string {
  const radius = Math.min(r, h, w / 2);
  return `M${x},${y + h}V${y + radius}Q${x},${y} ${x + radius},${y}H${x + w - radius}Q${x + w},${y} ${x + w},${y + radius}V${y + h}Z`;
}

/** The width of an element, following resizes; `fallback` where it cannot be measured. */
function useWidth<T extends HTMLElement>(fallback: number): [React.RefObject<T | null>, number] {
  const ref = useRef<T>(null);
  const [width, setWidth] = useState(fallback);
  useEffect(() => {
    const node = ref.current;
    if (!node) {
      return;
    }
    const measure = () => setWidth(node.clientWidth || fallback);
    measure();
    if (typeof ResizeObserver === "undefined") {
      return;
    }
    const observer = new ResizeObserver(measure);
    observer.observe(node);
    return () => observer.disconnect();
  }, [fallback]);
  return [ref, width];
}

type Hover = { key: string; index: number; x: number; y: number };

/**
 * Stacked bars of the monthly expenses by category, for one currency: a
 * legend whose chips hide categories, the chart itself and a tooltip that
 * follows the pointer or the keyboard focus. Drawn as plain SVG.
 */
export function MonthlyChart({
  data,
  months,
  includeCurrent,
  scale,
}: {
  data: MonthlyCurrency;
  months: number;
  includeCurrent: boolean;
  scale: ReportsScale;
}) {
  const t = useTranslations("reports");
  const common = useTranslations("common");
  const appLocale = useLocale();
  const locale = intlLocale(isLocale(appLocale) ? appLocale : "en");
  const [hidden, setHidden] = useState<ReadonlySet<string>>(new Set());
  const [hover, setHover] = useState<Hover | null>(null);
  const [boxRef, width] = useWidth<HTMLDivElement>(640);
  const patternId = useId().replace(/:/g, "");

  const shape = buildReport(data, { months, includeCurrent, labels: { uncategorized: common("uncategorized"), other: common("other") } });
  const view = computeView(shape, hidden);
  const money = (amount: number) => formatMoney(Math.round(amount), shape.currency, shape.minorUnits, locale);
  const compact = (amount: number) => formatCompact(amount, shape.currency, shape.minorUnits, locale);
  const monthName = (month: string) => formatMonth(month, locale);

  const toggle = (key: string) =>
    setHidden((current) => {
      const next = new Set(current);
      if (!next.delete(key)) {
        next.add(key);
      }
      return next;
    });

  const detail: PointDetail | null = (() => {
    const series = hover && shape.series.find((s) => s.key === hover.key);
    return hover && series ? describePoint(shape, view, series, hover.index) : null;
  })();

  const share = scale === "share";
  const height = width < 520 ? 300 : 360;
  const margin = { top: 24, right: 8, bottom: 40, left: width < 520 ? 44 : 58 };
  const innerW = width - margin.left - margin.right;
  const innerH = height - margin.top - margin.bottom;
  const yMax = share ? 1 : niceMax(Math.max(Math.max(...view.totals, 0) * 1.08, 1));
  const y = (value: number) => margin.top + innerH - (Math.min(value, yMax) / yMax) * innerH;
  const band = innerW / shape.months.length;
  const barWidth = Math.min(64, band * (shape.months.length > 6 ? 0.62 : 0.5));
  const ticks = [0, 1, 2, 3, 4].map((k) => (yMax * k) / 4);
  const percent = new Intl.NumberFormat(locale, { style: "percent", maximumFractionDigits: 0 });

  const showAt = (series: Series, index: number, event: React.PointerEvent | React.FocusEvent<SVGElement>) => {
    const box = boxRef.current?.getBoundingClientRect();
    if (!box) {
      return;
    }
    if ("clientX" in event && event.clientX !== undefined) {
      setHover({ key: series.key, index, x: event.clientX - box.left, y: event.clientY - box.top });
      return;
    }
    const rect = event.currentTarget.getBoundingClientRect();
    setHover({ key: series.key, index, x: rect.left - box.left + rect.width / 2, y: rect.top - box.top });
  };

  const stats = [
    { label: t("statTotal"), value: money(view.periodTotal) },
    { label: t("statAverage"), value: money(view.average) },
    ...(view.highest ? [{ label: t("statHighest", { month: monthName(shape.months[view.highest.index]) }), value: money(view.highest.total) }] : []),
  ];

  const first = shape.months[0];
  const last = shape.months[shape.months.length - 1];

  return (
    <div className="grid gap-4">
      <div className="flex flex-wrap items-baseline justify-between gap-x-6 gap-y-2">
        <p className="text-[13px] text-muted-foreground" data-testid="reports-subtitle">
          {t("subtitle", { from: monthName(first), to: monthName(last), currency: shape.currency })}
        </p>
        <dl className="flex flex-wrap gap-x-6 gap-y-1.5">
          {stats.map((stat) => (
            <div key={stat.label} className="grid">
              <dd className="font-mono text-lg font-medium tabular-nums">{stat.value}</dd>
              <dt className="text-xs text-muted-foreground">{stat.label}</dt>
            </div>
          ))}
        </dl>
      </div>

      <div role="group" aria-label={t("legendLabel")} className="flex flex-wrap gap-1.5">
        {shape.series.map((series) => {
          const on = !hidden.has(series.key);
          return (
            <button
              key={series.key}
              type="button"
              aria-pressed={on}
              title={series.other ? series.members.join(", ") : undefined}
              onClick={() => toggle(series.key)}
              className={cn(
                "inline-flex min-h-8 items-center gap-2 rounded-full border bg-card py-1 pr-3 pl-2 text-[13px] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary",
                !on && "text-muted-foreground line-through opacity-60",
              )}
            >
              <i
                aria-hidden
                className="block size-2.5 rounded-[3px]"
                style={on ? { background: series.other ? "var(--muted-foreground)" : (series.color ?? FALLBACK_COLOR), opacity: series.other ? 0.55 : 1 } : { outline: "1.5px solid var(--muted-foreground)", outlineOffset: -1.5 }}
              />
              <span>{series.other ? t("otherCount", { name: series.name, count: series.members.length }) : series.name}</span>
            </button>
          );
        })}
      </div>

      <div ref={boxRef} className={cn("relative w-full", hover && "[&_[data-segment]]:opacity-45")}>
        <svg
          viewBox={`0 0 ${width} ${height}`}
          role="group"
          aria-label={t("chartLabel")}
          className="block h-auto w-full overflow-visible"
          data-testid="reports-chart"
        >
          <defs>
            <pattern id={`${patternId}-hatch`} width={6} height={6} patternUnits="userSpaceOnUse" patternTransform="rotate(45)">
              <rect width={6} height={6} fill="transparent" />
              <line x1={0} y1={0} x2={0} y2={6} stroke="var(--card)" strokeWidth={2.2} strokeOpacity={0.75} />
            </pattern>
            <pattern id={`${patternId}-other`} width={5} height={5} patternUnits="userSpaceOnUse" patternTransform="rotate(135)">
              <rect width={5} height={5} fill="var(--muted-foreground)" fillOpacity={0.55} />
              <line x1={0} y1={0} x2={0} y2={5} stroke="var(--card)" strokeWidth={1.2} strokeOpacity={0.6} />
            </pattern>
          </defs>

          {ticks.map((value, k) => (
            <g key={k}>
              <line
                x1={margin.left}
                x2={width - margin.right}
                y1={y(value)}
                y2={y(value)}
                stroke={k === 0 ? "var(--muted-foreground)" : "var(--border)"}
                strokeOpacity={k === 0 ? 0.4 : 1}
              />
              <text x={margin.left - 8} y={y(value) + 4} textAnchor="end" className="fill-muted-foreground font-mono text-[11px]">
                {share ? percent.format(value) : compact(value)}
              </text>
            </g>
          ))}

          {shape.months.map((month, i) => {
            const x = margin.left + band * i + (band - barWidth) / 2;
            const denominator = share ? view.totals[i] || 1 : 1;
            const segments = view.visible.filter((s) => s.values[i] > 0);
            let acc = 0;
            return (
              <g key={month} data-month={month}>
                {segments.map((series, j) => {
                  const value = series.values[i] / denominator;
                  const bottom = y(acc);
                  const top = y(acc + value);
                  acc += value;
                  const h = Math.max(0, bottom - top - GAP);
                  if (h <= 0) {
                    return null;
                  }
                  const fill = series.other ? `url(#${patternId}-other)` : (series.color ?? FALLBACK_COLOR);
                  const isTop = j === segments.length - 1;
                  const active = hover?.key === series.key;
                  const segmentProps = {
                    fill,
                    tabIndex: 0,
                    role: "img",
                    "aria-label": `${series.name}, ${monthName(month)}: ${money(series.values[i])}`,
                    "data-segment": series.key,
                    "data-active": active ? "" : undefined,
                    className: cn("cursor-pointer transition-opacity focus:outline-none focus-visible:stroke-foreground focus-visible:stroke-2", active && "opacity-100!"),
                    onPointerEnter: (e: React.PointerEvent<SVGElement>) => showAt(series, i, e),
                    onPointerMove: (e: React.PointerEvent<SVGElement>) => showAt(series, i, e),
                    onFocus: (e: React.FocusEvent<SVGElement>) => showAt(series, i, e),
                    onPointerLeave: () => setHover(null),
                    onBlur: () => setHover(null),
                  };
                  return (
                    <g key={series.key}>
                      {isTop ? <path d={roundedTop(x, top, barWidth, h, RADIUS)} {...segmentProps} /> : <rect x={x} y={top} width={barWidth} height={h} {...segmentProps} />}
                      {i === shape.partialIndex && <rect x={x} y={top} width={barWidth} height={h} fill={`url(#${patternId}-hatch)`} pointerEvents="none" />}
                    </g>
                  );
                })}
                {!share && view.totals[i] > 0 && (
                  <text x={x + barWidth / 2} y={y(view.totals[i]) - 7} textAnchor="middle" className="fill-foreground font-mono text-[11px] font-medium">
                    {compact(view.totals[i])}
                  </text>
                )}
                <text x={x + barWidth / 2} y={height - margin.bottom + 18} textAnchor="middle" className="fill-muted-foreground font-mono text-[11px]">
                  {formatShortMonth(month, locale)}
                  {i === shape.partialIndex ? "*" : ""}
                </text>
                {i === shape.partialIndex && (
                  <text x={x + barWidth / 2} y={height - margin.bottom + 31} textAnchor="middle" className="fill-muted-foreground font-mono text-[10px]">
                    {t("inProgress")}
                  </text>
                )}
              </g>
            );
          })}

          {!share && view.average > 0 && <AverageLine y={y(view.average)} left={margin.left} right={width - margin.right} label={t("averageLine", { amount: compact(view.average) })} />}
          {!share && view.budgetLine && (
            <BudgetLine
              budgets={view.budgetLine}
              y={y}
              left={margin.left}
              band={band}
              label={(amount) => (view.budgetSeries ? t("categoryBudgetLine", { name: view.budgetSeries.name, amount: compact(amount) }) : t("budgetLine", { amount: compact(amount) }))}
            />
          )}
        </svg>
        {hover && detail && (
          <ChartTooltip detail={detail} x={hover.x} y={hover.y} containerWidth={width} money={money} monthLabel={monthName(detail.month)} />
        )}
      </div>
      <p className="text-xs text-muted-foreground">{t("hint")}</p>
    </div>
  );
}

function AverageLine({ y, left, right, label }: { y: number; left: number; right: number; label: string }) {
  const labelWidth = label.length * 6.6 + 10;
  return (
    <g pointerEvents="none" data-testid="average-line">
      <line x1={left} x2={right} y1={y} y2={y} stroke="var(--foreground)" strokeWidth={1.5} strokeDasharray="5 4" strokeOpacity={0.55} />
      <rect x={right - labelWidth} y={y - 18} width={labelWidth} height={15} rx={4} fill="var(--card)" fillOpacity={0.85} />
      <text x={right - 4} y={y - 7} textAnchor="end" className="fill-foreground font-mono text-[11px] font-medium">
        {label}
      </text>
    </g>
  );
}

/** The budget as a step line: one segment per month, where that month has a budget. */
function BudgetLine({
  budgets,
  y,
  left,
  band,
  label,
}: {
  budgets: (number | null)[];
  y: (value: number) => number;
  left: number;
  band: number;
  label: (amount: number) => string;
}) {
  const firstIndex = budgets.findIndex((b) => b !== null);
  const text = label(budgets[firstIndex] ?? 0);
  const labelWidth = text.length * 6.6 + 10;
  return (
    <g pointerEvents="none" data-testid="budget-line">
      {budgets.map((amount, i) =>
        amount === null ? null : <line key={i} x1={left + band * i} x2={left + band * (i + 1)} y1={y(amount)} y2={y(amount)} stroke="var(--expense)" strokeWidth={2} strokeOpacity={0.85} />,
      )}
      <rect x={left + band * firstIndex + 2} y={y(budgets[firstIndex] ?? 0) - 18} width={labelWidth} height={15} rx={4} fill="var(--card)" fillOpacity={0.85} />
      <text x={left + band * firstIndex + 6} y={y(budgets[firstIndex] ?? 0) - 7} className="fill-expense font-mono text-[11px] font-medium">
        {text}
      </text>
    </g>
  );
}
