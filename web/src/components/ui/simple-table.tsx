import type { ReactNode } from "react";
import { Link } from "wouter";
import { cn } from "@/lib/utils";

export interface SimpleColumn<T> {
  header: string;
  cell: (row: T) => ReactNode;
  align?: "right";
  className?: string;
}

/**
 * A short table inside a card, after DOSS's "Sales order queue": its own rounded box,
 * a grey header row, hairline rows. For long, sortable lists use DataTable instead.
 * With href, the first cell becomes a link and the row highlights on hover.
 */
export function SimpleTable<T>({
  columns,
  rows,
  rowKey,
  href,
  empty,
  label,
}: {
  columns: SimpleColumn<T>[];
  rows: T[];
  rowKey: (row: T) => string | number;
  href?: (row: T) => string;
  empty: ReactNode;
  label: string;
}) {
  if (rows.length === 0) return <p className="rounded-md border border-dashed px-4 py-6 text-center text-sm text-muted-foreground">{empty}</p>;
  return (
    <div className="relative overflow-x-auto rounded-md border">
      <table className="w-full text-sm">
        <caption className="sr-only">{label}</caption>
        <thead>
          <tr className={cn("bg-muted/60 text-left text-[0.8125rem] text-muted-foreground dark:bg-muted/40")}>
            {columns.map((c) => (
              <th key={c.header} scope="col" className={cn("h-9 px-3 font-normal whitespace-nowrap", c.align === "right" && "text-right")}>
                {c.header}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((row) => (
            <tr key={rowKey(row)} className={cn("border-t", href && "transition-colors hover:bg-muted/30")}>
              {columns.map((c, i) => (
                <td key={c.header} className={cn("h-10 px-3 align-middle", c.align === "right" && "num text-right", c.className)}>
                  {i === 0 && href ? (
                    <Link href={href(row)} className="font-medium hover:text-primary">
                      {c.cell(row)}
                    </Link>
                  ) : (
                    c.cell(row)
                  )}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
