import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

export type ChipTone = "good" | "warning" | "critical" | "info" | "neutral";

// DOSS marks a row's state with a small solid chip. Light mode keeps it solid; in dark
// mode a solid chip glares, so it becomes a tint with the tone as text.
const tones: Record<ChipTone, string> = {
  good: "bg-good text-white dark:bg-good/18 dark:text-good",
  warning: "bg-warning text-[oklch(0.2_0.03_62)] dark:bg-warning/18 dark:text-warning",
  critical: "bg-critical text-white dark:bg-critical/18 dark:text-critical",
  info: "bg-primary text-primary-foreground dark:bg-primary/18 dark:text-primary",
  neutral: "bg-muted text-muted-foreground ring-1 ring-border ring-inset",
};

export function Chip({ tone, icon, children, className, title }: { tone: ChipTone; icon?: ReactNode; children: ReactNode; className?: string; title?: string }) {
  return (
    <span title={title} className={cn("inline-flex h-5 items-center gap-1 rounded-[5px] px-1.5 text-xs font-medium whitespace-nowrap [&_svg]:size-3", tones[tone], className)}>
      {icon}
      {children}
    </span>
  );
}
