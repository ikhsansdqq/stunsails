import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

/**
 * A small set of mutually exclusive views on one grey track, the selected one raised
 * on the card colour. "tabs" switches what the page shows; "radio" filters it.
 */
export function Segmented<T extends string>({
  value,
  onChange,
  options,
  label,
  kind = "radio",
  className,
}: {
  value: T;
  onChange: (v: T) => void;
  options: { id: T; label: ReactNode }[];
  label: string;
  kind?: "radio" | "tabs";
  className?: string;
}) {
  return (
    <div role={kind === "tabs" ? "tablist" : "radiogroup"} aria-label={label} className={cn("inline-flex rounded-lg bg-muted p-0.5 dark:bg-muted/60", className)}>
      {options.map((o) => {
        const on = o.id === value;
        return (
          <button
            key={o.id}
            type="button"
            role={kind === "tabs" ? "tab" : "radio"}
            aria-selected={kind === "tabs" ? on : undefined}
            aria-checked={kind === "radio" ? on : undefined}
            onClick={() => onChange(o.id)}
            className={cn(
              "h-7 rounded-md px-3 text-sm whitespace-nowrap outline-none transition-colors focus-visible:ring-[3px] focus-visible:ring-ring/50",
              on
                ? "bg-card font-medium text-foreground shadow-xs ring-1 ring-border dark:bg-sidebar-active dark:ring-0"
                : "text-muted-foreground hover:text-foreground",
            )}
          >
            {o.label}
          </button>
        );
      })}
    </div>
  );
}
