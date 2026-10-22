import { useId, type ReactNode } from "react";
import { Link } from "wouter";
import { cn } from "@/lib/utils";

/**
 * A headline number after DOSS: a plain label, a large figure, one muted line under it,
 * and an optional sparkline. With href the whole card links to the page behind it.
 */
export function Stat({
  label,
  value,
  note,
  tone,
  href,
  chart,
  aside,
}: {
  label: ReactNode;
  value: ReactNode;
  note?: ReactNode;
  tone?: "critical" | "warning" | "good";
  href?: string;
  chart?: ReactNode;
  /** Small text top-right, such as a change against last period. */
  aside?: ReactNode;
}) {
  const body = (
    <>
      <div className="flex items-baseline justify-between gap-2">
        <dt className="truncate text-sm">{label}</dt>
        {aside && <span className="shrink-0 text-xs text-muted-foreground">{aside}</span>}
      </div>
      <dd
        className={cn(
          "num mt-1.5 text-[1.75rem] leading-none font-semibold tracking-[-0.02em]",
          tone === "critical" && "text-critical",
          tone === "warning" && "text-warning",
          tone === "good" && "text-good",
        )}
      >
        {value}
      </dd>
      {chart && <dd className="mt-3">{chart}</dd>}
      {note && <dd className="mt-auto truncate pt-2 text-[0.8125rem] text-muted-foreground">{note}</dd>}
    </>
  );
  const box = "flex flex-col rounded-lg border bg-card p-4";
  return href ? (
    <Link href={href} className={cn(box, "outline-none transition-colors hover:border-foreground/20 focus-visible:ring-[3px] focus-visible:ring-ring/50")}>
      {body}
    </Link>
  ) : (
    <div className={box}>{body}</div>
  );
}

/** A row of Stats: two across on phones, up to four on wide screens. */
export function StatRow({ children, className }: { children: ReactNode; className?: string }) {
  return <dl className={cn("grid grid-cols-2 gap-3 lg:grid-cols-4", className)}>{children}</dl>;
}

/**
 * A small area line for a Stat, after Vercel's usage cards: a thin line over a gradient
 * that fades out, and a dot on the latest value. Values are drawn oldest first. Hidden
 * from screen readers, so the Stat's note must say what it shows.
 */
export function Sparkline({ values, className }: { values: number[]; className?: string }) {
  const id = useId();
  if (values.length < 2) return null;
  const max = Math.max(...values);
  const min = Math.min(...values, 0);
  const span = max - min || 1;
  const pts = values.map((v, i) => [(i / (values.length - 1)) * 100, 30 - ((v - min) / span) * 26 - 2] as const);
  const line = pts.map(([x, y], i) => `${i ? "L" : "M"}${x.toFixed(2)},${y.toFixed(2)}`).join(" ");
  const last = pts.at(-1)!;
  return (
    <span className={cn("relative block h-9 w-full", className)} aria-hidden>
      <svg viewBox="0 0 100 30" preserveAspectRatio="none" className="absolute inset-0 h-full w-full overflow-visible">
        <defs>
          <linearGradient id={id} x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" style={{ stopColor: "var(--chart)", stopOpacity: 0.22 }} />
            <stop offset="100%" style={{ stopColor: "var(--chart)", stopOpacity: 0 }} />
          </linearGradient>
        </defs>
        <path d={`${line} L100,30 L0,30 Z`} fill={`url(#${id})`} />
        <path d={line} className="fill-none stroke-chart" strokeWidth={1.5} vectorEffect="non-scaling-stroke" strokeLinejoin="round" strokeLinecap="round" />
      </svg>
      <span className="absolute size-1.5 -translate-x-1/2 -translate-y-1/2 rounded-full bg-primary ring-2 ring-card" style={{ left: `${last[0]}%`, top: `${(last[1] / 30) * 100}%` }} />
    </span>
  );
}
