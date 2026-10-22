import { Children, Fragment, isValidElement, type ReactElement, type ReactNode } from "react";
import { Select as Base } from "@base-ui/react/select";
import { Check } from "@/components/ui/lucide";
import { cn } from "@/lib/utils";
import { SearchSelect, type SearchOption } from "@/components/ui/search-select";
import { SelectChevron, selectTriggerClass, selectValueClass } from "@/components/ui/select-trigger";

/** Above this many options the select becomes searchable, like the model and currency pickers. */
const SEARCH_FROM = 12;

interface Option {
  value: string;
  label: string;
  disabled?: boolean;
  group?: string;
}

export interface SelectProps {
  id?: string;
  value?: string | number;
  /** Called like a native select's change event, so `e.target.value` handlers keep working. */
  onChange?: (e: { target: { value: string } }) => void;
  /** <option> and <optgroup> elements, as for a native select. */
  children?: ReactNode;
  className?: string;
  disabled?: boolean;
  required?: boolean;
  name?: string;
  "aria-label"?: string;
  /** Forces a search box on (or off) whatever the number of options. */
  searchable?: boolean;
  /** Shown while no option is chosen. */
  placeholder?: string;
}

/**
 * The design system's select: a styled trigger and popup in place of the browser's own,
 * taking <option> children like a native one. Long lists get a search box.
 */
export function Select({ id, value, onChange, children, className, disabled, required, name, "aria-label": label, searchable, placeholder = "Choose one" }: SelectProps) {
  const options = readOptions(children);
  const current = String(value ?? "");
  const change = (v: string) => onChange?.({ target: { value: v } });

  if (searchable ?? options.length > SEARCH_FROM)
    return (
      <SearchSelect
        id={id ?? name ?? "select"}
        label={label}
        value={current}
        onChange={change}
        options={options.filter((o) => !o.disabled).map((o): SearchOption => ({ value: o.value, label: o.label, group: o.group }))}
        choose={placeholder}
        disabled={disabled}
        layout="inline"
        width="16rem"
        triggerClassName={className}
      />
    );

  const groups = new Map<string, Option[]>();
  for (const o of options) groups.set(o.group ?? "", [...(groups.get(o.group ?? "") ?? []), o]);
  return (
    <Base.Root
      id={id}
      name={name}
      // A value no option has shows the placeholder, as an "Add…" picker that resets does.
      value={options.some((o) => o.value === current) ? current : null}
      onValueChange={(v) => change(String(v ?? ""))}
      items={options.map((o) => ({ value: o.value, label: o.label }))}
      disabled={disabled}
      required={required}
    >
      <Base.Trigger
        aria-label={label}
        className={cn(selectTriggerClass, className)}
      >
        <Base.Value className={cn(selectValueClass, "data-[placeholder]:text-muted-foreground")} placeholder={placeholder} />
        {/* A flex box, like the chevron inside it, so neither sits on a text baseline. */}
        <Base.Icon className="flex shrink-0">
          <SelectChevron />
        </Base.Icon>
      </Base.Trigger>
      <Base.Portal>
        {/* Above dialogs (z-91), like the searchable picker. */}
        <Base.Positioner sideOffset={4} align="start" alignItemWithTrigger={false} className="z-[95] outline-none">
          <Base.Popup className="max-h-[min(20rem,var(--available-height))] min-w-[max(var(--anchor-width),10rem)] max-w-[calc(100vw-2rem)] origin-[var(--transform-origin)] overflow-y-auto overscroll-contain rounded-lg border bg-card p-1 shadow-md outline-none transition-[opacity,transform] duration-100 data-[ending-style]:scale-98 data-[ending-style]:opacity-0 data-[starting-style]:scale-98 data-[starting-style]:opacity-0">
            {[...groups].map(([group, list]) =>
              group ? (
                <Base.Group key={group}>
                  <Base.GroupLabel className="px-2 pt-2 pb-1 text-xs font-medium text-muted-foreground">{group}</Base.GroupLabel>
                  {list.map(item)}
                </Base.Group>
              ) : (
                <Fragment key="">{list.map(item)}</Fragment>
              ),
            )}
          </Base.Popup>
        </Base.Positioner>
      </Base.Portal>
    </Base.Root>
  );
}

function item(o: Option) {
  return (
    <Base.Item
      key={o.value}
      value={o.value}
      disabled={o.disabled}
      className="flex cursor-default items-center gap-2 rounded-md py-1.5 pr-3 pl-2 text-sm outline-none select-none data-[disabled]:opacity-50 data-[highlighted]:bg-accent data-[highlighted]:text-foreground"
    >
      <span className="grid size-4 shrink-0 place-items-center text-primary">
        <Base.ItemIndicator>
          <Check className="size-4" aria-hidden />
        </Base.ItemIndicator>
      </span>
      <Base.ItemText className="min-w-0 flex-1 truncate">{o.label}</Base.ItemText>
    </Base.Item>
  );
}

/** The options a native select's children describe, through fragments and <optgroup>s. */
function readOptions(children: ReactNode, group?: string): Option[] {
  return Children.toArray(children).flatMap((child) => {
    if (!isValidElement(child)) return [];
    const el = child as ReactElement<{ value?: string | number; label?: string; disabled?: boolean; children?: ReactNode }>;
    if (el.type === Fragment) return readOptions(el.props.children, group);
    if (el.type === "optgroup") return readOptions(el.props.children, el.props.label);
    if (el.type !== "option") return [];
    const label = textOf(el.props.children);
    return [{ value: String(el.props.value ?? label), label, disabled: el.props.disabled, group }];
  });
}

function textOf(node: ReactNode): string {
  if (node == null || typeof node === "boolean") return "";
  if (typeof node === "string" || typeof node === "number") return String(node);
  if (Array.isArray(node)) return node.map(textOf).join("");
  if (isValidElement<{ children?: ReactNode }>(node)) return textOf(node.props.children);
  return "";
}
