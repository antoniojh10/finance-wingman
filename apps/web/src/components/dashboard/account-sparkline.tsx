export type SparklinePoint = { date: string; balance: number };

const WIDTH = 120;
const HEIGHT = 32;
const PAD = 3;

/** SVG coordinates for each point, scaled to the box; a flat series sits mid-height. */
export function sparklinePoints(points: SparklinePoint[]): { x: number; y: number }[] {
  const values = points.map((p) => p.balance);
  const min = Math.min(...values);
  const max = Math.max(...values);
  const span = max - min;
  return points.map((_, i) => ({
    x: PAD + (i / (points.length - 1)) * (WIDTH - 2 * PAD),
    y: span === 0 ? HEIGHT / 2 : PAD + (1 - (values[i] - min) / span) * (HEIGHT - 2 * PAD),
  }));
}

/**
 * A tiny trend line with a light area under it and a dot on the last
 * point. Needs at least two points; renders nothing otherwise.
 */
export function AccountSparkline({ points, label, className }: { points: SparklinePoint[]; label: string; className?: string }) {
  if (points.length < 2) {
    return null;
  }
  const coords = sparklinePoints(points);
  const line = coords.map((c, i) => `${i === 0 ? "M" : "L"}${c.x.toFixed(1)} ${c.y.toFixed(1)}`).join(" ");
  const last = coords[coords.length - 1];
  const area = `${line} L${last.x.toFixed(1)} ${HEIGHT} L${coords[0].x.toFixed(1)} ${HEIGHT} Z`;
  return (
    <svg
      role="img"
      aria-label={label}
      data-testid="account-sparkline"
      viewBox={`0 0 ${WIDTH} ${HEIGHT}`}
      preserveAspectRatio="none"
      className={className}
    >
      <path d={area} fill="currentColor" fillOpacity={0.14} stroke="none" />
      <path d={line} fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" vectorEffect="non-scaling-stroke" />
      {/* A zero-length round-capped stroke stays a circle however the box is stretched. */}
      <path d={`M${last.x.toFixed(1)} ${last.y.toFixed(1)}h0`} stroke="currentColor" strokeWidth={6} strokeLinecap="round" vectorEffect="non-scaling-stroke" />
    </svg>
  );
}
