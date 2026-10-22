import type { ReactNode } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { cn } from "@/lib/utils";
import { useSlowStage } from "@/hooks/use-slow";

/**
 * Placeholders shaped like the content they stand in for, so a page keeps its layout
 * while it loads. The blocks are muted fills with one slow light sweep (`.skeleton` in
 * style.css, still under reduced motion). Wrap them in Loading so screen readers hear
 * what is on its way.
 */
export function Skeleton({ className }: { className?: string }) {
  return <div aria-hidden className={cn("skeleton rounded-md", className)} />;
}

/**
 * The accessible wrapper: a polite status with a spoken label, the skeleton shown instead.
 * A wait that runs long says so (SlowNote), so a slow connection never looks like a hang.
 */
export function Loading({ label, className, onRetry, children }: { label: string; className?: string; onRetry?: () => void; children: ReactNode }) {
  return (
    <div role="status" aria-live="polite" className={className}>
      <span className="sr-only">{label}</span>
      <SlowNote onRetry={onRetry} />
      {children}
    </div>
  );
}

/**
 * Nothing for the first few seconds, then a line saying the wait is the connection,
 * then a way to try again. By default that asks again for whatever is still loading.
 */
function SlowNote({ onRetry }: { onRetry?: () => void }) {
  const stage = useSlowStage();
  const queryClient = useQueryClient();
  if (stage === "waiting") return null;
  // A first load still running is only restarted once cancelled: refetch alone waits on it.
  const retry =
    onRetry ??
    (() => {
      for (const { queryKey } of queryClient.getQueryCache().findAll({ predicate: (q) => q.state.status === "pending" && q.getObserversCount() > 0 })) {
        void queryClient.cancelQueries({ queryKey, exact: true }).then(() => queryClient.refetchQueries({ queryKey, exact: true }));
      }
    });
  return (
    <p className="flex flex-wrap items-center gap-x-2 gap-y-1 text-sm text-muted-foreground">
      <span className="size-1.5 shrink-0 animate-pulse rounded-full bg-primary" aria-hidden />
      {stage === "slow" ? "Still loading. Your connection seems slow, so this may take a little longer." : "This is taking longer than usual."}
      {stage === "stuck" && (
        <button type="button" onClick={retry} className="font-medium text-primary underline-offset-4 hover:underline">
          Try again
        </button>
      )}
    </p>
  );
}

// Varied widths, so a block of lines reads as text rather than a stack of bars.
const LINE_WIDTHS = ["w-full", "w-11/12", "w-4/5", "w-full", "w-2/3", "w-5/6"];

export function SkeletonLines({ lines = 3, className }: { lines?: number; className?: string }) {
  return (
    <div className={cn("space-y-2.5", className)}>
      {Array.from({ length: lines }, (_, i) => (
        <Skeleton key={i} className={cn("h-3.5", i === lines - 1 && lines > 1 ? "w-3/5" : LINE_WIDTHS[i % LINE_WIDTHS.length])} />
      ))}
    </div>
  );
}

/** Mirrors StatRow: label, large figure, note. */
export function StatRowSkeleton({ count = 4, className }: { count?: number; className?: string }) {
  return (
    <div className={cn("grid grid-cols-2 gap-3 lg:grid-cols-4", className)}>
      {Array.from({ length: count }, (_, i) => (
        <div key={i} className="flex flex-col rounded-lg border bg-card p-4">
          <Skeleton className="h-3.5 w-24" />
          <Skeleton className="mt-3 h-7 w-20" />
          <Skeleton className="mt-4 h-3 w-32 max-w-full" />
        </div>
      ))}
    </div>
  );
}

/** Body rows for a table whose header is already drawn (DataTable, the admin users table). */
export function TableRowsSkeleton({ rows = 6, cols }: { rows?: number; cols: number }) {
  return (
    <>
      {Array.from({ length: rows }, (_, r) => (
        <tr key={r} className="border-b last:border-0" aria-hidden>
          {Array.from({ length: cols }, (_, c) => (
            <td key={c} className="h-11 px-4 align-middle">
              <Skeleton className={cn("h-3.5", c === 0 ? (r % 2 ? "w-36" : "w-44") : c === cols - 1 ? "ml-auto w-12" : r % 3 ? "w-20" : "w-24")} />
            </td>
          ))}
        </tr>
      ))}
    </>
  );
}

/** A whole table card when the real one can't be drawn yet: search bar, header, rows. */
export function TableSkeleton({ rows = 6, cols = 4, toolbar = true, className }: { rows?: number; cols?: number; toolbar?: boolean; className?: string }) {
  return (
    <div className={cn("overflow-hidden rounded-lg border bg-card", className)}>
      {toolbar && (
        <div className="border-b p-3">
          <Skeleton className="h-8 w-full max-w-64" />
        </div>
      )}
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b bg-muted/60 dark:bg-muted/40" aria-hidden>
            {Array.from({ length: cols }, (_, c) => (
              <th key={c} className="h-10 px-4">
                <Skeleton className={cn("h-3 bg-foreground/[0.07]", c === cols - 1 ? "ml-auto w-10" : "w-16")} />
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          <TableRowsSkeleton rows={rows} cols={cols} />
        </tbody>
      </table>
    </div>
  );
}

/** Mirrors Section: a titled card with a few lines in it. */
export function SectionSkeleton({ lines = 3, className, children }: { lines?: number; className?: string; children?: ReactNode }) {
  return (
    <div className={cn("flex flex-col gap-4 rounded-lg border bg-card p-4", className)}>
      <div className="space-y-2">
        <Skeleton className="h-4 w-40" />
        <Skeleton className="h-3 w-64 max-w-full" />
      </div>
      {children ?? <SkeletonLines lines={lines} />}
    </div>
  );
}

/** A plain list: a round mark and two lines per row (notifications, sessions, backups). */
export function ListSkeleton({ rows = 5, mark = true, className }: { rows?: number; mark?: boolean; className?: string }) {
  return (
    <ul className={cn("divide-y", className)} aria-hidden>
      {Array.from({ length: rows }, (_, i) => (
        <li key={i} className="flex items-start gap-3 px-4 py-3">
          {mark && <Skeleton className="mt-0.5 size-7 shrink-0 rounded-full" />}
          <div className="min-w-0 flex-1 space-y-2">
            <Skeleton className={cn("h-3.5", i % 2 ? "w-3/5" : "w-4/5")} />
            <Skeleton className="h-3 w-24" />
          </div>
        </li>
      ))}
    </ul>
  );
}

/** Label-and-input pairs, for forms that wait on saved values. */
export function FormSkeleton({ fields = 4, className }: { fields?: number; className?: string }) {
  return (
    <div className={cn("space-y-5", className)}>
      {Array.from({ length: fields }, (_, i) => (
        <div key={i} className="space-y-2">
          <Skeleton className="h-3.5 w-28" />
          <Skeleton className="h-9 w-full max-w-md" />
        </div>
      ))}
    </div>
  );
}

/** A detail page: title, a row of chips, then a wide column of sections beside a narrow one. */
export function DetailSkeleton({ className }: { className?: string }) {
  return (
    <div className={className}>
      <div className="mb-6 space-y-3">
        <Skeleton className="h-3.5 w-20" />
        <Skeleton className="h-8 w-72 max-w-full" />
        <div className="flex gap-2">
          <Skeleton className="h-6 w-20 rounded-full" />
          <Skeleton className="h-6 w-24 rounded-full" />
          <Skeleton className="h-6 w-16 rounded-full" />
        </div>
      </div>
      <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_20rem]">
        <div className="space-y-4">
          <SectionSkeleton lines={4} />
          <SectionSkeleton lines={3} />
        </div>
        <SectionSkeleton lines={5} className="self-start" />
      </div>
    </div>
  );
}

/** The page shell while a route's code is still downloading. A retry starts the download again. */
export function PageSkeleton() {
  return (
    <Loading label="Loading the page" className="space-y-6" onRetry={() => location.reload()}>
      <div className="space-y-2.5">
        <Skeleton className="h-8 w-56" />
        <Skeleton className="h-4 w-96 max-w-full" />
      </div>
      <StatRowSkeleton />
      <TableSkeleton />
    </Loading>
  );
}
