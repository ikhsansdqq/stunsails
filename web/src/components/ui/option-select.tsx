import { Select } from "@/components/ui/select";
import { cn } from "@/lib/utils";

// Base UI's select treats "" as no selection, so an empty id travels under a value of its own.
const EMPTY = "__empty__";

/**
 * A compact select over a few fixed choices: filters, ranges and preferences.
 * Takes the same props as Segmented, so a button group swaps for it in place.
 */
export function OptionSelect<T extends string>({
  value,
  onChange,
  options,
  label,
  className,
}: {
  value: T;
  onChange: (v: T) => void;
  options: { id: T; label: string }[];
  label: string;
  className?: string;
}) {
  return (
    <Select
      aria-label={label}
      value={value === "" ? EMPTY : value}
      onChange={(e) => onChange((e.target.value === EMPTY ? "" : e.target.value) as T)}
      className={cn("h-8 w-auto pl-2.5", className)}
      searchable={false}
    >
      {options.map((o) => (
        <option key={o.id || EMPTY} value={o.id || EMPTY}>
          {o.label}
        </option>
      ))}
    </Select>
  );
}
