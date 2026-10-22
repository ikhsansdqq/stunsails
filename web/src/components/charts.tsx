import { useId, useState, type ReactNode } from "react";
import { cn } from "@/lib/utils";
import { Stat } from "@/components/ui/stat";

export const tone = (p: number) => (p >= 90 ? "bg-critical" : p >= 75 ? "bg-warning" : "bg-primary");

/** A thin usage bar, as on the freelance server page. */
export function Bar({ percent, label, className }: { percent: number; label: string; className?: string }) {
  return (
    <div className={cn("h-1.5 overflow-hidden rounded-full bg-muted", className)} role="meter" aria-label={label} aria-valuemin={0} aria-valuemax={100} aria-valuenow={Math.round(percent)}>
      <div className={cn("h-full rounded-full", tone(percent))} style={{ width: `${Math.min(100, Math.max(percent, 1))}%` }} />
    </div>
  );
}

/** A Stat with a usage bar under the figure, toned once it runs high. */
export function Meter({ label, value, hint, percent, tone }: { label: string; value: string; hint: ReactNode; percent: number; tone?: "critical" | "warning" }) {
  return (
    <Stat
      label={label}
      value={value}
      tone={tone ?? (percent >= 90 ? "critical" : percent >= 75 ? "warning" : undefined)}
      chart={<Bar percent={percent} label={label} />}
      note={hint}
    />
  );
}

/**
 * The freelance server page's spark: a thin line over a gradient that fades out and a dot
 * on the latest reading. Hovering reads out the value under the pointer.
 */
export function Spark({
  values,
  times,
  max,
  label,
  format,
  height = 56,
}: {
  values: number[];
  times?: number[];
  max: number;
  label: string;
  format?: (v: number) => string;
  height?: number;
}) {
  const gradient = useId();
  const [hover, setHover] = useState<number | null>(null);
  const w = 240;
  const h = height;
  if (values.length < 2) {
    return <div className="mt-2 rounded-md border border-dashed" style={{ height }} role="img" aria-label={`${label}: not enough readings yet`} />;
  }
  const top = Math.max(max, 1e-9);
  const points = values.map((v, i) => [(i / (values.length - 1)) * w, h - (Math.min(v, top) / top) * (h - 4) - 2] as const);
  const line = points.map(([x, y]) => `${x.toFixed(1)},${y.toFixed(1)}`).join(" ");
  const [lx, ly] = hover !== null ? points[hover]! : points.at(-1)!;
  const move = (e: React.PointerEvent<HTMLDivElement>) => {
    const r = e.currentTarget.getBoundingClientRect();
    const i = Math.round(((e.clientX - r.left) / r.width) * (values.length - 1));
    setHover(Math.min(values.length - 1, Math.max(0, i)));
  };
  return (
    <div className="relative mt-2 w-full touch-none" style={{ height }} role="img" aria-label={label} onPointerMove={format ? move : undefined} onPointerLeave={() => setHover(null)}>
      <svg viewBox={`0 0 ${w} ${h}`} preserveAspectRatio="none" className="absolute inset-0 h-full w-full overflow-visible" aria-hidden>
        <defs>
          <linearGradient id={gradient} x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" style={{ stopColor: "var(--chart)", stopOpacity: 0.22 }} />
            <stop offset="100%" style={{ stopColor: "var(--chart)", stopOpacity: 0 }} />
          </linearGradient>
        </defs>
        <polygon points={`0,${h} ${line} ${w},${h}`} fill={`url(#${gradient})`} />
        <polyline points={line} fill="none" className="stroke-chart" strokeWidth="1.5" vectorEffect="non-scaling-stroke" strokeLinejoin="round" strokeLinecap="round" />
        {hover !== null && <line x1={lx} x2={lx} y1={0} y2={h} className="stroke-border" strokeWidth="1" vectorEffect="non-scaling-stroke" />}
      </svg>
      <span className="absolute size-1.5 -translate-x-1/2 -translate-y-1/2 rounded-full bg-primary ring-2 ring-card" style={{ left: `${(lx / w) * 100}%`, top: `${(ly / h) * 100}%` }} aria-hidden />
      {hover !== null && format && (
        <span
          className="num pointer-events-none absolute -top-7 z-10 -translate-x-1/2 rounded-md border bg-card px-1.5 py-0.5 text-xs whitespace-nowrap shadow-md"
          style={{ left: `${Math.min(85, Math.max(15, (lx / w) * 100))}%` }}
        >
          {format(values[hover]!)}
          {times?.[hover] !== undefined && <span className="text-muted-foreground"> · {new Date(times[hover]! * 1000).toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit" })}</span>}
        </span>
      )}
    </div>
  );
}

/** One captioned spark: label and the latest value on top, the peak under it. */
export function SparkFigure({
  label,
  values,
  times,
  max,
  format,
  height,
}: {
  label: string;
  values: number[];
  times?: number[];
  max: number;
  format: (v: number) => string;
  height?: number;
}) {
  const last = values.at(-1);
  const peak = values.length ? Math.max(...values) : 0;
  return (
    <figure className="min-w-0">
      <figcaption className="flex items-baseline justify-between gap-2 text-[0.8125rem]">
        <span>{label}</span>
        <span className="num text-muted-foreground">{last === undefined ? "No data" : format(last)}</span>
      </figcaption>
      <Spark values={values} times={times} max={max} label={label} format={format} height={height} />
      <p className="num mt-1 text-xs text-muted-foreground">{values.length ? `Peak ${format(peak)}` : " "}</p>
    </figure>
  );
}
