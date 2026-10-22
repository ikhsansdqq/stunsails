import { Menu } from "@base-ui/react/menu";
import { EllipsisVertical, type LucideIcon } from "@/components/ui/lucide";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";

export interface RowMenuItem {
  label: string;
  icon: LucideIcon;
  onSelect: () => void;
  /** Destructive items are red; keep them in the last group. */
  danger?: boolean;
  disabled?: boolean;
}

/**
 * A row's "…" menu: groups of icon-led items split by separators, view actions
 * first and destructive ones last. Each icon plays as its item is highlighted.
 */
export function RowMenu({ label, groups, className }: { label: string; groups: RowMenuItem[][]; className?: string }) {
  const filled = groups.filter((g) => g.length > 0);
  const row =
    "flex cursor-default items-center gap-2.5 rounded-md px-2.5 py-1.5 text-sm outline-none select-none data-[highlighted]:bg-sidebar-active data-[disabled]:opacity-50";
  return (
    <Menu.Root>
      <Menu.Trigger render={<Button variant="ghost" size="icon-sm" aria-label={label} className={className} />}>
        <EllipsisVertical aria-hidden />
      </Menu.Trigger>
      <Menu.Portal>
        <Menu.Positioner sideOffset={4} align="end" className="z-50">
          <Menu.Popup className="min-w-48 origin-[var(--transform-origin)] rounded-lg border bg-card p-1 shadow-lg outline-none transition-[opacity,scale] duration-150 data-[ending-style]:scale-98 data-[ending-style]:opacity-0 data-[starting-style]:scale-98 data-[starting-style]:opacity-0">
            {filled.map((g, gi) => (
              <div key={gi} role="group">
                {gi > 0 && <Menu.Separator className="my-1 h-px bg-border" />}
                {g.map((it) => {
                  const Icon = it.icon;
                  return (
                    <Menu.Item key={it.label} disabled={it.disabled} className={cn(row, it.danger && "text-critical")} onClick={it.onSelect}>
                      <Icon size={15} className={cn("shrink-0", !it.danger && "text-muted-foreground")} aria-hidden />
                      {it.label}
                    </Menu.Item>
                  );
                })}
              </div>
            ))}
          </Menu.Popup>
        </Menu.Positioner>
      </Menu.Portal>
    </Menu.Root>
  );
}
