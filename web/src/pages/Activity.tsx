import { useEffect, useState } from "react";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { Link, useSearch } from "wouter";
import { Bell, Box, ChevronRight } from "@/components/ui/lucide";
import { api, type Activity } from "@/lib/api";
import { ago, formatFullDateTime } from "@/lib/format";
import { cn } from "@/lib/utils";
import { PageHeader } from "@/components/Layout";
import { Block } from "@/components/ui/block";
import { DataTable, columnsFor, type PaginationState, type SortingState } from "@/components/ui/data-table";
import { ActivityDetail } from "@/components/ActivityDetail";
import { OptionSelect } from "@/components/ui/option-select";
import { describe, icons } from "@/components/ActivityList";
import { useDebounced } from "@/hooks/use-debounced";
import { useNow } from "@/hooks/use-now";

type Kind = "" | "docker" | "audit" | "host" | "alert";
type Type = "" | "container" | "image" | "volume" | "network" | "login" | "action" | "ssh" | "sudo" | "unit" | "site";

const PAGE_SIZES = [25, 50, 100];
const FIRST_SIZE = 25;

const kinds: { id: Kind; label: string }[] = [
  { id: "", label: "Everything" },
  { id: "docker", label: "Docker" },
  { id: "host", label: "Server" },
  { id: "audit", label: "Harbor" },
  { id: "alert", label: "Alerts" },
];

const kindLabel = Object.fromEntries(kinds.map((k) => [k.id, k.label])) as Record<Kind, string>;

const types: Record<"docker" | "host" | "audit", { id: Type; label: string }[]> = {
  host: [
    { id: "", label: "All kinds" },
    { id: "ssh", label: "SSH" },
    { id: "sudo", label: "sudo" },
    { id: "unit", label: "Services" },
    { id: "site", label: "Sites" },
  ],
  audit: [
    { id: "", label: "All kinds" },
    { id: "login", label: "Sign-ins" },
    { id: "action", label: "Changes" },
  ],
  docker: [
    { id: "", label: "All kinds" },
    { id: "container", label: "Containers" },
    { id: "image", label: "Images" },
    { id: "volume", label: "Volumes" },
    { id: "network", label: "Networks" },
  ],
};

const col = columnsFor<Activity>();

export function ActivityPage() {
  const [kind, setKind] = useState<Kind>("");
  const [type, setType] = useState<Type>("");
  const search = useSearch();
  const [initial] = useState(() => new URLSearchParams(search).get("q") ?? "");
  const [text, setText] = useState(initial);
  const q = useDebounced(text.trim(), 300);
  const now = useNow(30_000);
  const [pagination, setPagination] = useState<PaginationState>({ pageIndex: 0, pageSize: FIRST_SIZE });
  const [sorting, setSorting] = useState<SortingState>([{ id: "at", desc: true }]);
  const order = sorting[0]?.desc === false ? "asc" : "desc";
  // A new filter or search starts again from the first page.
  useEffect(() => setPagination((p) => ({ ...p, pageIndex: 0 })), [kind, type, q]);
  const page = useQuery({
    queryKey: ["activity", "page", { kind, type, q, order, ...pagination }],
    queryFn: () => api.activityPage({ kind, type, q, order, offset: pagination.pageIndex * pagination.pageSize, limit: pagination.pageSize }),
    placeholderData: keepPreviousData,
    refetchInterval: 15_000,
  });
  const items = page.data?.items ?? [];
  const total = page.data?.total ?? 0;

  const columns = [
    col.accessor((a) => describe(a), {
      id: "event",
      header: "Event",
      enableSorting: false,
      cell: ({ row, getValue }) => {
        const a = row.original;
        const Icon = a.kind === "alert" ? Bell : (icons[a.type] ?? Box);
        const linkable = a.kind === "docker" && a.type === "container" && a.action !== "destroy";
        const open = row.getIsExpanded();
        return (
          <span className="flex min-w-0 items-center gap-2">
            <button
              type="button"
              onClick={row.getToggleExpandedHandler()}
              aria-expanded={open}
              aria-controls={open ? `row-detail-${row.id}` : undefined}
              aria-label={open ? "Hide details" : "Show details"}
              className="-ml-1.5 grid size-6 shrink-0 place-items-center rounded text-muted-foreground hover:text-foreground focus-visible:ring-[3px] focus-visible:ring-ring/50 focus-visible:outline-none"
            >
              <ChevronRight className={cn("size-4 transition-transform motion-reduce:transition-none", open && "rotate-90")} aria-hidden />
            </button>
            <span className={cn("grid size-6 shrink-0 place-items-center rounded-md", a.ok ? "bg-muted text-muted-foreground" : "bg-critical/10 text-critical")} aria-hidden>
              <Icon size={14} />
            </span>
            {linkable ? (
              <Link href={`/containers/${a.target}`} className="break-words hover:text-primary hover:underline">
                {getValue()}
              </Link>
            ) : (
              <span className="break-words">{getValue()}</span>
            )}
          </span>
        );
      },
    }),
    col.accessor("kind", {
      header: "Source",
      enableSorting: false,
      meta: { className: "whitespace-nowrap text-muted-foreground" },
      cell: ({ getValue }) => kindLabel[getValue()],
    }),
    col.accessor((a) => a.ip ?? "", {
      id: "ip",
      header: "From",
      enableSorting: false,
      meta: { className: "num whitespace-nowrap text-muted-foreground" },
      cell: ({ row: { original: a }, getValue }) => getValue() || (a.kind === "host" && a.action === "failed logins" ? "Several" : "—"),
    }),
    col.accessor("at", {
      header: "Date",
      meta: { align: "right", className: "whitespace-nowrap" },
      cell: ({ getValue }) => (
        <time dateTime={new Date(getValue() * 1000).toISOString()} className="flex flex-col items-end leading-tight">
          <span>{formatFullDateTime(getValue() * 1000)}</span>
          <span className="text-xs text-muted-foreground">{ago(getValue(), now)}</span>
        </time>
      ),
    }),
  ];

  return (
    <>
      <PageHeader title="Activity" sub="What Docker reported, who signed in to the server, what was done through Harbor, and every alert sent. Kept for 30 days." />
      <Block title="Feed" sub={page.data ? `${total.toLocaleString()} ${total === 1 ? "entry" : "entries"}. Open one for its addresses and where they are.` : undefined}>
        {page.error ? (
          <p className="text-sm text-critical">{page.error.message}</p>
        ) : (
          <DataTable
            data={items}
            columns={columns}
            getRowId={(a) => String(a.id)}
            searchPlaceholder="Search names, actions, addresses, commands"
            initialSearch={initial}
            pageSize={pagination.pageSize}
            minWidth="44rem"
            loading={page.isPending}
            empty={q || kind ? "Nothing matches." : "Nothing has happened yet."}
            rowClassName={(a) => cn(!a.ok && "text-critical")}
            expand={(a) => <ActivityDetail entry={a} />}
            server={{
              rowCount: total,
              sorting,
              onSortingChange: setSorting,
              pagination,
              onPaginationChange: setPagination,
              onSearch: setText,
              pageSizes: PAGE_SIZES,
            }}
            toolbar={
              <>
                <OptionSelect
                  label="Source"
                  value={kind}
                  onChange={(k) => {
                    setKind(k);
                    setType("");
                  }}
                  options={kinds}
                />
                {kind !== "" && kind !== "alert" && <OptionSelect label="Kind" value={type} onChange={setType} options={types[kind]} />}
              </>
            }
          />
        )}
      </Block>
    </>
  );
}
