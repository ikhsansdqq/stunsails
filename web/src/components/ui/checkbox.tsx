import { useEffect, useRef, type InputHTMLAttributes } from "react";
import { Check, Minus } from "@/components/ui/lucide";
import { cn } from "@/lib/utils";

/**
 * The design system's checkbox: a real <input type="checkbox"> kept for labels, forms and
 * keyboards, drawn as a rounded box in the primary colour. `indeterminate` shows "some selected".
 */
export function Checkbox({
  className,
  indeterminate = false,
  ...props
}: Omit<InputHTMLAttributes<HTMLInputElement>, "type"> & { indeterminate?: boolean }) {
  const ref = useRef<HTMLInputElement>(null);
  useEffect(() => {
    if (ref.current) ref.current.indeterminate = indeterminate;
  }, [indeterminate]);
  const Mark = indeterminate ? Minus : Check;
  return (
    <span className={cn("relative inline-grid size-4 shrink-0 place-items-center align-middle", className)}>
      <input
        ref={ref}
        type="checkbox"
        className="peer absolute inset-0 m-0 size-full cursor-pointer appearance-none rounded-[5px] border border-input bg-card shadow-xs outline-none transition-[background-color,border-color,box-shadow] checked:border-primary checked:bg-primary indeterminate:border-primary indeterminate:bg-primary focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-50"
        {...props}
      />
      <Mark
        aria-hidden
        strokeWidth={3}
        className={cn(
          "pointer-events-none relative size-3 text-primary-foreground opacity-0 transition-opacity",
          indeterminate ? "peer-indeterminate:opacity-100" : "peer-checked:opacity-100",
        )}
      />
    </span>
  );
}
