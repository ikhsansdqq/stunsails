import { Fragment, useState, type ReactNode } from "react";
import {
  columnFacetingFeature,
  columnFilteringFeature,
  columnVisibilityFeature,
  createColumnHelper,
  createFacetedRowModel,
  createFacetedUniqueValues,
  createFilteredRowModel,
  createPaginatedRowModel,
  createSortedRowModel,
  filterFn_equalsString,
  filterFn_includesString,
  globalFilteringFeature,
  rowPaginationFeature,
  rowExpandingFeature,
  rowSelectionFeature,
  rowSortingFeature,
  sortFn_alphanumeric,
  sortFn_basic,
  sortFn_datetime,
  sortFn_text,
  tableFeatures,
  useTable,
  type ColumnDef,
  type OnChangeFn,
  type PaginationState,
  type RowData,
  type RowSelectionState,
  type SortingState,
} from "@tanstack/react-table";
import { ArrowDown, ArrowUp, ChevronDown, ChevronFirst, ChevronLast, ChevronLeft, ChevronRight, ChevronsUpDown, Search } from "@/components/ui/lucide";
import { cn } from "@/lib/utils";
import { Select } from "@/components/ui/select";
import { TableRowsSkeleton } from "@/components/ui/skeleton";

export interface ColumnMeta {
  /** Right-align numbers and money. */
  align?: "right";
  /** Extra classes for this column's cells. */
  className?: string;
  /** Offer a select filter for this column; labels map raw values to display text. */
  filter?: { label: string; labels?: Record<string, string> };
}

const features = tableFeatures({
  rowSortingFeature,
  columnFilteringFeature,
  globalFilteringFeature,
  columnFacetingFeature,
  columnVisibilityFeature,
  rowPaginationFeature,
  rowSelectionFeature,
  rowExpandingFeature,
  sortedRowModel: createSortedRowModel(),
  filteredRowModel: createFilteredRowModel(),
  facetedRowModel: createFacetedRowModel(),
  facetedUniqueValues: createFacetedUniqueValues(),
  paginatedRowModel: createPaginatedRowModel(),
  sortFns: { alphanumeric: sortFn_alphanumeric, basic: sortFn_basic, datetime: sortFn_datetime, text: sortFn_text },
  filterFns: { includesString: filterFn_includesString, equalsString: filterFn_equalsString },
  columnMeta: {} as ColumnMeta,
});

export type Features = typeof features;

const ALL = "__all__";

/** Server mode: the page of rows comes from the API, which sorts, searches and pages them. */
export interface ServerState {
  rowCount: number;
  sorting: SortingState;
  onSortingChange: OnChangeFn<SortingState>;
  pagination: PaginationState;
  onPaginationChange: OnChangeFn<PaginationState>;
  onSearch: (q: string) => void;
  /** Offer these page sizes in the pager. */
  pageSizes?: number[];
}

/** Sections of rows under a heading row each; the table's sort applies inside every section. */
export interface Grouping<T> {
  by: (row: T) => string;
  label: (key: string, rows: T[]) => ReactNode;
  /** Section order; the default is alphabetical. */
  order?: (a: string, b: string) => number;
}

export type { RowSelectionState, SortingState, PaginationState };
export type Column<T extends RowData> = ColumnDef<Features, T, any>;

/** Typed column helper for DataTable: `const col = columnsFor<Job>()`. */
export const columnsFor = <T extends RowData>() => createColumnHelper<Features, T>();

/**
 * The app's table (TanStack Table): click headers to sort, search across text
 * columns, filter by any column whose meta has `filter`, and page long lists.
 */
export function DataTable<T extends RowData>({
  data,
  columns,
  searchPlaceholder,
  initialSort,
  pageSize = 25,
  minWidth = "40rem",
  empty,
  loading = false,
  rowClassName,
  toolbar,
  getRowId,
  server,
  selection,
  expand,
  group,
  onSearch,
  initialSearch = "",
}: {
  data: T[];
  columns: Column<T>[];
  searchPlaceholder?: string;
  initialSort?: { id: string; desc?: boolean };
  pageSize?: number;
  minWidth?: string;
  empty?: ReactNode;
  /** Draws skeleton rows under the real header while the data is on its way. */
  loading?: boolean;
  rowClassName?: (row: T) => string | undefined;
  toolbar?: ReactNode;
  getRowId?: (row: T) => string;
  /** Let the API sort, search and page; filters then belong in `toolbar`. */
  server?: ServerState;
  /** Row checkboxes; pair with getRowId so a selection survives paging. */
  selection?: { state: RowSelectionState; onChange: OnChangeFn<RowSelectionState> };
  /**
   * A detail panel under a row, shown while the row is expanded. Columns toggle it
   * with `row.getToggleExpandedHandler()`; pair with getRowId so it stays open on refetch.
   */
  expand?: (row: T) => ReactNode;
  /** Split the rows into collapsible sections; paging is off so a section never splits. */
  group?: Grouping<T>;
  /** Search through the API instead of the loaded rows; sorting and filters stay local. */
  onSearch?: (q: string) => void;
  /** The search box's starting text, such as a ?q= from a link. */
  initialSearch?: string;
}) {
  const paged = !group && pageSize > 0;
  const table = useTable({
    features,
    columns,
    data,
    getRowId: getRowId ? (row: T) => getRowId(row) : undefined,
    globalFilterFn: "includesString",
    enableRowSelection: !!selection,
    getRowCanExpand: () => !!expand,
    autoResetExpanded: false,
    manualSorting: !!server,
    manualPagination: !!server,
    manualFiltering: !!server,
    rowCount: server?.rowCount,
    state: {
      ...(server ? { sorting: server.sorting, pagination: server.pagination } : {}),
      ...(selection ? { rowSelection: selection.state } : {}),
    },
    // Only set handlers that exist: an explicit undefined replaces the table's own updater,
    // and sorting, paging or selection then silently do nothing.
    ...(server ? { onSortingChange: server.onSortingChange, onPaginationChange: server.onPaginationChange } : {}),
    ...(selection ? { onRowSelectionChange: selection.onChange } : {}),
    initialState: {
      sorting: initialSort ? [{ id: initialSort.id, desc: initialSort.desc ?? false }] : [],
      pagination: { pageIndex: 0, pageSize: paged ? pageSize : Number.MAX_SAFE_INTEGER },
    },
  });
  const filterable = server ? [] : table.getAllLeafColumns().filter((c) => c.columnDef.meta?.filter);
  const rows = table.getRowModel().rows;
  const total = server ? server.rowCount : table.getPrePaginatedRowModel().rows.length;
  const pages = table.getPageCount();
  const [search, setSearch] = useState(initialSearch);
  const [collapsed, setCollapsed] = useState<Set<string>>(() => new Set());
  const cols = table.getVisibleLeafColumns().length;
  const sections = group ? groupRows(rows, group) : [{ key: "", rows }];
  const showToolbar = searchPlaceholder || filterable.length > 0 || toolbar;

  return (
    <div className="overflow-hidden rounded-lg border bg-card">
      {showToolbar && (
        <div className="flex flex-wrap items-center gap-2 border-b px-3 py-2.5">
          {searchPlaceholder && (
            <label className="relative min-w-[12rem] flex-1 sm:max-w-xs">
              <span className="sr-only">{searchPlaceholder}</span>
              <Search className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" aria-hidden />
              <input
                type="search"
                value={search}
                onChange={(e) => {
                  setSearch(e.target.value);
                  if (server) server.onSearch(e.target.value);
                  else if (onSearch) onSearch(e.target.value);
                  else table.setGlobalFilter(e.target.value);
                }}
                placeholder={searchPlaceholder}
                className="h-8 w-full rounded-md border border-input bg-card pr-2 pl-8 text-sm outline-none placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50"
              />
            </label>
          )}
          {filterable.map((column) => {
            const meta = column.columnDef.meta!.filter!;
            const values = [...column.getFacetedUniqueValues().keys()].filter((v) => v !== "" && v != null).sort();
            // Base UI's select treats "" as no selection, so "all" has a value of its own.
            const value = (column.getFilterValue() as string | undefined) ?? ALL;
            return (
              <Select
                key={column.id}
                aria-label={meta.label}
                value={value}
                onChange={(e) => column.setFilterValue(e.target.value === ALL ? undefined : e.target.value)}
                className={cn("h-8 w-auto pl-2.5", value !== ALL && "border-primary/40 bg-accent text-primary")}
              >
                <option value={ALL}>{meta.label}: all</option>
                {values.map((v) => (
                  <option key={String(v)} value={String(v)}>
                    {meta.labels?.[String(v)] ?? String(v)}
                  </option>
                ))}
              </Select>
            );
          })}
          {toolbar && <div className="ml-auto flex items-center gap-2">{toolbar}</div>}
        </div>
      )}
      {/* relative, so screen-reader-only text in a cell scrolls with the table instead of widening the page. */}
      <div className="relative overflow-x-auto">
        <table className="w-full text-sm" style={{ minWidth }} aria-busy={loading || undefined}>
          <thead>
            {table.getHeaderGroups().map((group) => (
              <tr key={group.id} className={cn("border-b bg-muted/60 text-left text-[0.8125rem] text-muted-foreground dark:bg-muted/40")}>
                {group.headers.map((header) => {
                  const meta = header.column.columnDef.meta;
                  const sorted = header.column.getIsSorted();
                  const Icon = sorted === "asc" ? ArrowUp : sorted === "desc" ? ArrowDown : ChevronsUpDown;
                  return (
                    <th
                      key={header.id}
                      className={cn("h-10 px-4 font-normal whitespace-nowrap", meta?.align === "right" && "text-right")}
                      aria-sort={sorted === "asc" ? "ascending" : sorted === "desc" ? "descending" : undefined}
                    >
                      {header.isPlaceholder ? null : header.column.getCanSort() ? (
                        <button
                          type="button"
                          onClick={header.column.getToggleSortingHandler()}
                          className={cn(
                            "-mx-1 inline-flex items-center gap-1 rounded px-1 hover:text-foreground focus-visible:ring-[3px] focus-visible:ring-ring/50 focus-visible:outline-none",
                            sorted && "text-foreground",
                          )}
                        >
                          <table.FlexRender header={header} />
                          <Icon className={cn("size-3.5", !sorted && "opacity-40")} aria-hidden />
                        </button>
                      ) : (
                        <table.FlexRender header={header} />
                      )}
                    </th>
                  );
                })}
              </tr>
            ))}
          </thead>
          <tbody>
            {loading && rows.length === 0 && <TableRowsSkeleton cols={cols} />}
            {!loading && rows.length === 0 && (
              <tr>
                <td colSpan={cols} className="px-4 py-8 text-center text-muted-foreground">
                  {total === 0 && (data.length > 0 || ((server || onSearch) && search)) ? "Nothing matches these filters." : (empty ?? "Nothing here yet.")}
                </td>
              </tr>
            )}
            {sections.map(({ key, rows }) => {
              const shut = !!group && collapsed.has(key);
              return (
                <Fragment key={key}>
                  {group && (
                    <tr className="border-b bg-muted/40 dark:bg-muted/25">
                      <th scope="colgroup" colSpan={cols} className="px-4 py-2 text-left font-medium">
                        <span className="flex items-center gap-2">
                          <button
                            type="button"
                            aria-expanded={!shut}
                            aria-label={shut ? "Show section" : "Hide section"}
                            onClick={() =>
                              setCollapsed((prev) => {
                                const next = new Set(prev);
                                if (!next.delete(key)) next.add(key);
                                return next;
                              })
                            }
                            className="grid size-6 place-items-center rounded text-muted-foreground hover:text-foreground focus-visible:ring-[3px] focus-visible:ring-ring/50 focus-visible:outline-none"
                          >
                            <ChevronDown className={cn("size-4 transition-transform", shut && "-rotate-90")} aria-hidden />
                          </button>
                          {group.label(key, rows.map((r) => r.original))}
                          <span className="num text-xs font-normal text-muted-foreground">{rows.length}</span>
                        </span>
                      </th>
                    </tr>
                  )}
                  {!shut && rows.map((row) => {
              const open = !!expand && row.getIsExpanded();
              return (
                <Fragment key={row.id}>
                  <tr
                    data-selected={row.getIsSelected() || undefined}
                    data-expanded={open || undefined}
                    // A click anywhere on the row opens its panel; links and buttons in it keep their own job.
                    // Keyboard users toggle it with the row's own button.
                    onClick={
                      expand
                        ? (e) => {
                            if ((e.target as HTMLElement).closest("a, button, input, label, select") || getSelection()?.toString()) return;
                            row.toggleExpanded();
                          }
                        : undefined
                    }
                    className={cn(
                      "border-b transition-colors last:border-0 hover:bg-muted/30 data-selected:bg-accent/60 data-expanded:border-b-0 data-expanded:bg-muted/30",
                      expand && "cursor-pointer",
                      rowClassName?.(row.original),
                    )}
                  >
                    {row.getVisibleCells().map((cell) => {
                      const meta = cell.column.columnDef.meta;
                      return (
                        <td key={cell.id} className={cn("px-4 py-2.5 align-middle", meta?.align === "right" && "num text-right", meta?.className)}>
                          <table.FlexRender cell={cell} />
                        </td>
                      );
                    })}
                  </tr>
                  {open && (
                    <tr id={`row-detail-${row.id}`} className="border-b bg-muted/30 last:border-0">
                      <td colSpan={row.getVisibleCells().length} className="px-4 pt-1 pb-4">
                        {/* On phones the table scrolls sideways; the panel stays in view. */}
                        <div className="sticky left-4 max-w-[calc(100vw-4rem)] md:static md:max-w-none">{expand(row.original)}</div>
                      </td>
                    </tr>
                  )}
                </Fragment>
              );
            })}
                </Fragment>
              );
            })}
          </tbody>
        </table>
      </div>
      {paged && (pages > 1 || (server?.pageSizes && total > (server.pageSizes[0] ?? 0))) && (
        <div className="flex flex-wrap items-center justify-between gap-3 border-t px-4 py-2 text-xs text-muted-foreground">
          <span className="num">
            {total === 0 ? 0 : table.state.pagination.pageIndex * table.state.pagination.pageSize + 1}–
            {Math.min(total, (table.state.pagination.pageIndex + 1) * table.state.pagination.pageSize)} of {total.toLocaleString()}
          </span>
          <span className="flex items-center gap-3">
            {server?.pageSizes && (
              <label className="flex items-center gap-2">
                Rows per page
                <Select
                  aria-label="Rows per page"
                  value={String(table.state.pagination.pageSize)}
                  onChange={(e) => table.setPagination({ pageIndex: 0, pageSize: Number(e.target.value) })}
                  className="h-7 w-auto pl-2 text-xs"
                >
                  {server.pageSizes.map((n) => (
                    <option key={n} value={String(n)}>
                      {n}
                    </option>
                  ))}
                </Select>
              </label>
            )}
            {server && (
              <span className="num">
                Page {table.state.pagination.pageIndex + 1} of {Math.max(pages, 1)}
              </span>
            )}
            <span className="flex gap-1">
              {server && (
                <PageButton label="First page" disabled={!table.getCanPreviousPage()} onClick={() => table.firstPage()}>
                  <ChevronFirst className="size-4" />
                </PageButton>
              )}
              <PageButton label="Previous page" disabled={!table.getCanPreviousPage()} onClick={() => table.previousPage()}>
                <ChevronLeft className="size-4" />
              </PageButton>
              <PageButton label="Next page" disabled={!table.getCanNextPage()} onClick={() => table.nextPage()}>
                <ChevronRight className="size-4" />
              </PageButton>
              {server && (
                <PageButton label="Last page" disabled={!table.getCanNextPage()} onClick={() => table.lastPage()}>
                  <ChevronLast className="size-4" />
                </PageButton>
              )}
            </span>
          </span>
        </div>
      )}
    </div>
  );
}

function PageButton({ label, disabled, onClick, children }: { label: string; disabled: boolean; onClick: () => void; children: ReactNode }) {
  return (
    <button
      type="button"
      aria-label={label}
      disabled={disabled}
      onClick={onClick}
      className="grid size-7 place-items-center rounded-md border bg-card hover:text-primary focus-visible:ring-[3px] focus-visible:ring-ring/50 focus-visible:outline-none disabled:opacity-40"
    >
      {children}
    </button>
  );
}

/** Rows split by `group.by`, each section keeping the table's sort order. */
function groupRows<R extends { original: T }, T>(rows: R[], group: Grouping<T>) {
  const map = new Map<string, R[]>();
  for (const r of rows) {
    const k = group.by(r.original);
    map.set(k, [...(map.get(k) ?? []), r]);
  }
  return [...map.keys()].sort(group.order ?? ((a, b) => a.localeCompare(b))).map((key) => ({ key, rows: map.get(key)! }));
}
