import type { HTMLAttributes, ReactNode } from "react";
import { cn } from "@/lib/utils";

export function Card({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return <div className={cn("rounded-lg border bg-card", className)} {...props} />;
}

/** A Section or Block title. */
export const cardTitle = "text-[15px] font-semibold tracking-[-0.01em]";

/** A titled card, after DOSS: a semibold title over a muted line, flat on a hairline border. */
export function Section({
  title,
  sub,
  actions,
  children,
  className,
}: {
  title: ReactNode;
  sub?: ReactNode;
  actions?: ReactNode;
  children: ReactNode;
  className?: string;
}) {
  return (
    <Card className={cn("flex flex-col gap-4 py-4", className)}>
      <div className="flex flex-wrap items-start justify-between gap-2 px-4">
        <div>
          <h2 className={cardTitle}>{title}</h2>
          {sub && <p className="mt-0.5 text-[0.8125rem] text-muted-foreground">{sub}</p>}
        </div>
        {actions}
      </div>
      <div className="px-4">{children}</div>
    </Card>
  );
}
