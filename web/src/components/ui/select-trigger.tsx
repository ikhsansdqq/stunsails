import { ChevronsUpDown } from "@/components/ui/lucide";

/** The one trigger look every select shares (Select, SearchSelect, OptionSelect): label left, chevron right, both centred. */
export const selectTriggerClass =
  "flex h-9 w-full min-w-0 items-center gap-2 rounded-md border border-input bg-card px-3 text-left text-sm leading-none shadow-xs outline-none transition-[color,box-shadow] focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:opacity-60 data-[popup-open]:border-ring";

/** The label slot of a select trigger. */
export const selectValueClass = "min-w-0 flex-1 truncate py-0.5";

/** The chevron at the right of a select trigger: a flex box, so it centres on the label instead of sitting on a text baseline. */
export function SelectChevron() {
  return (
    <span className="flex shrink-0 items-center text-muted-foreground" aria-hidden>
      <ChevronsUpDown className="size-4" />
    </span>
  );
}
