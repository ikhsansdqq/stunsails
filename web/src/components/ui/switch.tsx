import { cn } from "@/lib/utils";

/** An on/off toggle (role="switch"), for settings that apply as soon as they're saved. */
export function Switch({
  id,
  checked,
  onChange,
  label,
  disabled,
  className,
}: {
  id?: string;
  checked: boolean;
  onChange: (checked: boolean) => void;
  /** The accessible name, when no <label htmlFor> points at the switch. */
  label?: string;
  disabled?: boolean;
  className?: string;
}) {
  return (
    <button
      id={id}
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={label}
      disabled={disabled}
      onClick={() => onChange(!checked)}
      className={cn(
        "relative inline-flex h-5 w-9 shrink-0 items-center rounded-full outline-none transition-colors focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:opacity-60",
        checked ? "bg-primary" : "bg-input dark:bg-muted",
        className,
      )}
    >
      <span
        aria-hidden
        className={cn("size-4 rounded-full bg-card shadow-xs transition-transform motion-reduce:transition-none", checked ? "translate-x-[1.125rem]" : "translate-x-0.5")}
      />
    </button>
  );
}
