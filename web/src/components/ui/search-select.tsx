import { useMemo, useState, type ReactNode } from "react";
import { Popover } from "@base-ui/react/popover";
import { Command } from "cmdk";
import { Check, Search } from "@/components/ui/lucide";
import { cn } from "@/lib/utils";
import { Skeleton } from "@/components/ui/skeleton";
import { SelectChevron, selectTriggerClass, selectValueClass } from "@/components/ui/select-trigger";

export interface SearchOption {
  value: string;
  label: string;
  /** A second line (stacked) or muted text after the label (inline). */
  note?: string;
  group?: string;
  /** Extra words that should find this option. */
  keywords?: string[];
  /** Right-aligned detail, such as a price or a symbol. */
  aside?: string;
}

/**
 * A searchable select (shadcn's Popover + Command combobox): the model, currency,
 * deck tone and language pickers are all this. Options are grouped by `group`
 * in the order they come; `custom` lets any typed value be used.
 */
export function SearchSelect({
  id,
  value,
  onChange,
  options,
  placeholder = "Search",
  empty = "Nothing matches.",
  choose = "Choose one",
  lead,
  custom,
  loading,
  invalid,
  layout = "stacked",
  match = "fuzzy",
  trigger,
  width = "22rem",
  triggerClassName,
  label,
  disabled,
}: {
  id: string;
  /** The accessible name, when no <label htmlFor> points at the trigger. */
  label?: string;
  value: string;
  onChange: (value: string) => void;
  options: SearchOption[];
  placeholder?: string;
  empty?: string;
  /** Shown in the trigger while nothing is chosen. */
  choose?: string;
  /** A first choice above the groups, such as "Auto" (its value is usually ""). */
  lead?: SearchOption;
  /** When set, a typed value that matches no option can be used; this is its verb ("Use", "Write in"). */
  custom?: string;
  loading?: boolean;
  invalid?: boolean;
  layout?: "stacked" | "inline";
  /** "substring" for codes and names, where fuzzy matching finds nonsense. */
  match?: "fuzzy" | "substring";
  /** Replaces the trigger's text for the current option. */
  trigger?: (current: SearchOption | undefined, value: string) => ReactNode;
  width?: string;
  /** Restyles the trigger, such as a chip in a prompt bar. */
  triggerClassName?: string;
  disabled?: boolean;
}) {
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState("");
  const current = lead?.value === value ? lead : options.find((o) => o.value === value);
  const groups = useMemo(() => {
    const out = new Map<string, SearchOption[]>();
    for (const o of options) out.set(o.group ?? "", [...(out.get(o.group ?? "") ?? []), o]);
    return [...out];
  }, [options]);
  const typed = search.trim();
  const canUseTyped = !!custom && typed !== "" && !options.some((o) => o.value.toLowerCase() === typed.toLowerCase());

  function pick(next: string) {
    onChange(next);
    setOpen(false);
    setSearch("");
  }

  const item = (o: SearchOption) => (
    <Command.Item key={o.value || "lead"} value={o.value || `lead ${o.label}`} keywords={[o.label, ...(o.keywords ?? [])]} onSelect={() => pick(o.value)} className={itemClass}>
      <Check className={cn("size-4 shrink-0 text-primary", o.value !== value && "invisible")} aria-hidden />
      {layout === "stacked" ? (
        <span className="grid min-w-0 flex-1">
          <span className="truncate">{o.label}</span>
          {o.note && <span className="truncate text-xs text-muted-foreground">{o.note}</span>}
        </span>
      ) : (
        <span className="flex min-w-0 flex-1 items-baseline gap-2">
          <span className="shrink-0 font-medium">{o.label}</span>
          {o.note && <span className="truncate text-muted-foreground">{o.note}</span>}
        </span>
      )}
      {o.aside && <span className="num shrink-0 text-xs text-muted-foreground">{o.aside}</span>}
    </Command.Item>
  );

  return (
    <Popover.Root
      open={open}
      onOpenChange={(next) => {
        setOpen(next);
        if (!next) setSearch("");
      }}
    >
      <Popover.Trigger
        id={id}
        disabled={disabled}
        aria-label={label ? `${label}: ${current?.label ?? (value || choose)}` : undefined}
        aria-haspopup="listbox"
        aria-invalid={invalid || undefined}
        className={cn(selectTriggerClass, "aria-invalid:border-critical", triggerClassName)}
      >
        <span className={selectValueClass}>
          {trigger ? trigger(current, value) : current ? current.label : value || <span className="text-muted-foreground">{choose}</span>}
        </span>
        <SelectChevron />
      </Popover.Trigger>
      <Popover.Portal>
        {/* Above dialogs (z-91), so the picker works inside one. */}
        <Popover.Positioner sideOffset={4} align="start" className="z-[95]">
          <Popover.Popup
            style={{ width: `max(var(--anchor-width), ${width})` }}
            className="max-w-[calc(100vw-2rem)] origin-[var(--transform-origin)] overflow-hidden rounded-lg border bg-card shadow-md outline-none transition-[opacity,transform] duration-100 data-[ending-style]:scale-98 data-[ending-style]:opacity-0 data-[starting-style]:scale-98 data-[starting-style]:opacity-0"
          >
            <Command
              loop
              className="flex flex-col"
              filter={match === "substring" ? (v, s, keywords) => ([v, ...(keywords ?? [])].join(" ").toLowerCase().includes(s.trim().toLowerCase()) ? 1 : 0) : undefined}
            >
              <div className="flex items-center gap-2 border-b px-3">
                <Search className="size-4 shrink-0 text-muted-foreground" aria-hidden />
                <Command.Input value={search} onValueChange={setSearch} placeholder={placeholder} aria-label={placeholder} className="h-10 w-full bg-transparent text-sm outline-none placeholder:text-muted-foreground" />
              </div>
              <Command.List className="max-h-80 scroll-py-1 overflow-y-auto overscroll-contain p-1">
                {loading && (
                  <Command.Loading label="Loading options">
                    <div className="space-y-1.5 p-1" aria-hidden>
                      {["w-4/5", "w-3/5", "w-2/3", "w-1/2"].map((w) => (
                        <Skeleton key={w} className={`h-6 ${w}`} />
                      ))}
                    </div>
                  </Command.Loading>
                )}
                {!loading && <Command.Empty className="px-2 py-6 text-center text-sm text-muted-foreground">{empty}</Command.Empty>}
                {lead && !typed && <Command.Group>{item(lead)}</Command.Group>}
                {groups.map(([group, list]) => (
                  <Command.Group key={group || "all"} heading={group || undefined} className={groupClass}>
                    {list.map(item)}
                  </Command.Group>
                ))}
                {/* Last, so Enter picks the best match and the typed value is the fallback. */}
                {canUseTyped && (
                  <Command.Group forceMount>
                    <Command.Item forceMount value={`custom ${typed}`} onSelect={() => pick(typed)} className={itemClass}>
                      {custom} <span className="font-medium break-all">{typed}</span>
                    </Command.Item>
                  </Command.Group>
                )}
              </Command.List>
            </Command>
          </Popover.Popup>
        </Popover.Positioner>
      </Popover.Portal>
    </Popover.Root>
  );
}

const groupClass =
  "[&_[cmdk-group-heading]]:px-2 [&_[cmdk-group-heading]]:pt-2 [&_[cmdk-group-heading]]:pb-1 [&_[cmdk-group-heading]]:text-xs [&_[cmdk-group-heading]]:font-medium [&_[cmdk-group-heading]]:text-muted-foreground";
const itemClass = "flex cursor-default items-center gap-2 rounded-md px-2 py-1.5 text-sm outline-none select-none data-[selected=true]:bg-accent data-[selected=true]:text-foreground";
