import type { ReactNode } from "react";
import { cn } from "@/lib/utils";
import { cardTitle } from "@/components/ui/card";

/**
 * A titled group for a table or a bordered list. Never put a table inside a card
 * (web/DESIGN.md): the table carries its own border, so the title sits above it on
 * the page itself. Cards (Section) are for stats, charts and detail grids.
 */
export function Block({ title, sub, actions, children, className }: { title: ReactNode; sub?: ReactNode; actions?: ReactNode; children: ReactNode; className?: string }) {
  return (
    <section className={cn("flex flex-col gap-3", className)}>
      <div className="flex flex-wrap items-end justify-between gap-2">
        <div className="min-w-0">
          <h2 className={cardTitle}>{title}</h2>
          {sub && <p className="mt-0.5 text-[0.8125rem] text-muted-foreground">{sub}</p>}
        </div>
        {actions && <div className="flex flex-wrap items-center gap-2">{actions}</div>}
      </div>
      {children}
    </section>
  );
}
