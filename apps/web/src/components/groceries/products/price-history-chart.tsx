"use client";

import { useLocale, useTranslations } from "next-intl";
import { useState } from "react";

import { NativeSelect } from "@/components/native-select";
import { useWidth } from "@/components/reports/monthly-chart";
import { intlLocale, isLocale } from "@/i18n/locales";
import { minorUnitsOf } from "@/lib/groceries";
import { priceAxis, type HistorySeries } from "@/lib/groceries-products";
import { formatMoney } from "@/lib/money";
import { formatShortMonth } from "@/lib/reports";

/** "Hacendado · 1 L @ Mercadona". */
export function seriesLabel(series: Pick<HistorySeries, "brand" | "packLabel" | "storeName">): string {
  return `${[series.brand, series.packLabel].filter(Boolean).join(" · ")} @ ${series.storeName}`;
}

function monthIndex(month: string): number {
  const [year, m] = month.split("-").map(Number);
  return year * 12 + m - 1;
}

/**
 * Monthly average unit price of one variant at one store, as a plain SVG
 * line in the style of the reports charts. A selector switches between the
 * variants and stores that have history.
 */
export function PriceHistoryChart({ series, initialKey, unitLabel }: { series: HistorySeries[]; initialKey?: string; unitLabel: string }) {
  const t = useTranslations("groceries.products.history");
  const appLocale = useLocale();
  const locale = intlLocale(isLocale(appLocale) ? appLocale : "en");
  const [selected, setSelected] = useState(initialKey ?? series[0]?.key);
  const [boxRef, width] = useWidth<HTMLDivElement>(560);

  const current = series.find((item) => item.key === selected) ?? series[0];
  if (!current) {
    return <p className="text-sm text-muted-foreground">{t("empty")}</p>;
  }

  const minorUnits = minorUnitsOf(current.currency);
  const money = (value: number) => formatMoney(Math.round(value), current.currency, minorUnits, locale);
  const first = monthIndex(current.points[0].month);
  const last = monthIndex(current.points[current.points.length - 1].month);
  const span = Math.max(last - first, 1);
  const axis = priceAxis(current.points.map((point) => point.value));

  const height = width < 520 ? 220 : 260;
  const margin = { top: 20, right: 16, bottom: 30, left: width < 520 ? 52 : 62 };
  const innerW = width - margin.left - margin.right;
  const innerH = height - margin.top - margin.bottom;
  const x = (month: string) => margin.left + (current.points.length === 1 ? innerW / 2 : ((monthIndex(month) - first) / span) * innerW);
  const y = (value: number) => margin.top + innerH - ((value - axis.min) / (axis.max - axis.min || 1)) * innerH;
  const path = current.points.map((point, i) => `${i === 0 ? "M" : "L"}${x(point.month).toFixed(1)},${y(point.value).toFixed(1)}`).join("");
  // Label only the months present, thinned so they never overlap.
  const perLabel = Math.ceil(current.points.length / Math.max(Math.floor(innerW / 56), 1));

  return (
    <div className="grid min-w-0 gap-3">
      {series.length > 1 ? (
        <NativeSelect aria-label={t("selector")} value={current.key} onChange={(event) => setSelected(event.target.value)}>
          {series.map((item) => (
            <option key={item.key} value={item.key}>
              {seriesLabel(item)}
            </option>
          ))}
        </NativeSelect>
      ) : (
        <p className="text-sm font-medium">{seriesLabel(current)}</p>
      )}
      <div ref={boxRef} className="w-full">
        <svg viewBox={`0 0 ${width} ${height}`} role="img" aria-label={t("chartLabel", { name: seriesLabel(current), unit: unitLabel })} className="block h-auto w-full overflow-visible" data-testid="price-history-chart">
          {axis.ticks.map((tick, k) => (
            <g key={tick}>
              <line x1={margin.left} x2={width - margin.right} y1={y(tick)} y2={y(tick)} stroke={k === 0 ? "var(--muted-foreground)" : "var(--border)"} strokeOpacity={k === 0 ? 0.4 : 1} />
              <text x={margin.left - 8} y={y(tick) + 4} textAnchor="end" className="fill-muted-foreground font-mono text-[11px]">
                {money(tick)}
              </text>
            </g>
          ))}
          {current.points.length > 1 && <path d={path} fill="none" stroke="var(--chart-1)" strokeWidth={2.5} strokeLinejoin="round" strokeLinecap="round" />}
          {current.points.map((point, i) => (
            <g key={point.month}>
              <circle cx={x(point.month)} cy={y(point.value)} r={4} fill="var(--card)" stroke="var(--chart-1)" strokeWidth={2.5} tabIndex={0} role="img" aria-label={`${formatShortMonth(point.month, locale)} ${point.month.slice(0, 4)}: ${money(point.value)}`}>
                <title>{`${point.month}: ${money(point.value)} (${t("observations", { count: point.observations })})`}</title>
              </circle>
              {(current.points.length - 1 - i) % perLabel === 0 && (
                <text x={x(point.month)} y={height - margin.bottom + 18} textAnchor="middle" className="fill-muted-foreground font-mono text-[11px]">
                  {formatShortMonth(point.month, locale)}
                </text>
              )}
              {i === current.points.length - 1 && (
                // The latest price sits above the point, or below it when the line comes down to it.
                <text x={Math.min(x(point.month), width - margin.right - 24)} y={y(point.value) + (i > 0 && current.points[i - 1].value > point.value ? 20 : -11)} textAnchor="middle" className="fill-foreground font-mono text-[11px] font-medium">
                  {money(point.value)}
                </text>
              )}
            </g>
          ))}
        </svg>
      </div>
      <p className="text-xs text-muted-foreground">{t("note", { unit: unitLabel })}</p>
    </div>
  );
}
