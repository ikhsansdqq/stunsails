import type { ReactNode } from "react";
import type { RowData } from "@tanstack/react-table";
import { X } from "@/components/ui/lucide";
import { Checkbox } from "@/components/ui/checkbox";
import type { columnsFor } from "@/components/ui/data-table";

/** The checkbox column for bulk actions; pair it with DataTable's selection prop. */
export function selectColumn<T extends RowData>(col: ReturnType<typeof columnsFor<T>>, noun: (row: T) => string) {
  return col.display({
    id: "select",
    header: ({ table }) => (
      <Checkbox
        aria-label="Select every row on this page"
        checked={table.getIsAllPageRowsSelected()}
        indeterminate={table.getIsSomePageRowsSelected() && !table.getIsAllPageRowsSelected()}
        onChange={table.getToggleAllPageRowsSelectedHandler()}
      />
    ),
    cell: ({ row }) => <Checkbox aria-label={`Select ${noun(row.original)}`} checked={row.getIsSelected()} onChange={row.getToggleSelectedHandler()} />,
    meta: { className: "w-10 pr-0" },
  });
}

/** Floats at the bottom while rows are selected: the count, the actions, and a way out. */
export function BulkBar({ count, noun, onClear, children }: { count: number; noun: string; onClear: () => void; children: ReactNode }) {
  if (count === 0) return null;
  return (
    <div role="toolbar" aria-label={`Actions for ${count} selected ${noun}`} className="fixed inset-x-0 bottom-4 z-30 flex justify-center px-4 motion-safe:animate-rise">
      <div className="flex max-w-full flex-wrap items-center gap-2 rounded-full border bg-card py-1.5 pr-1.5 pl-4 shadow-lg">
        <span className="num text-sm font-medium whitespace-nowrap">
          {count} {noun} selected
        </span>
        <span className="mx-1 h-5 w-px bg-border" aria-hidden />
        {children}
        <button type="button" onClick={onClear} aria-label="Clear the selection" className="grid size-8 place-items-center rounded-full text-muted-foreground hover:bg-accent hover:text-foreground">
          <X size={16} aria-hidden />
        </button>
      </div>
    </div>
  );
}
