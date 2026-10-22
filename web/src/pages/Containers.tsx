import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "wouter";
import { api, type Container } from "@/lib/api";
import { formatDuration, formatRate, shortImage } from "@/lib/format";
import { cn } from "@/lib/utils";
import { PageHeader } from "@/components/Layout";
import { Button } from "@/components/ui/button";
import { Chip } from "@/components/ui/chip";
import { DataTable, columnsFor, type Grouping, type RowSelectionState } from "@/components/ui/data-table";
import { Loading, SectionSkeleton } from "@/components/ui/skeleton";
import { HealthChip, Ports, StateChip } from "@/components/states";
import { ContainerMenu, items, useBulkContainerAction } from "@/components/ContainerActions";
import { BulkBar, selectColumn } from "@/components/bulk";
import type { ContainerAction } from "@/lib/api";
import { MemCell } from "@/pages/Overview";
import { useNow } from "@/hooks/use-now";

const col = columnsFor<Container>();

/** One section per compose stack, the stack's name linking to it; loose containers last. */
const byStack: Grouping<Container> = {
  by: (c) => c.project || "",
  label: (stack) =>
    stack ? (
      <Link href={`/stacks/${stack}`} className="hover:text-primary">
        {stack}
      </Link>
    ) : (
      <span className="text-muted-foreground">No stack</span>
    ),
  order: (a, b) => (a === "" ? 1 : b === "" ? -1 : a.localeCompare(b)),
};

export function ContainersPage() {
  const containers = useQuery({ queryKey: ["containers"], queryFn: api.containers, refetchInterval: 10_000 });
  const overview = useQuery({ queryKey: ["overview"], queryFn: api.overview, refetchInterval: 10_000 });
  const memTotal = overview.data?.host?.mem_total ?? 0;
  const now = useNow(60_000);
  const list = containers.data ?? [];
  const [selected, setSelected] = useState<RowSelectionState>({});
  const bulk = useBulkContainerAction();
  const picked = useMemo(() => list.filter((c) => selected[c.id]), [list, selected]);
  // Start goes to the stopped ones, restart and stop to the running ones, remove to all.
  const targets = (a: ContainerAction) => picked.filter((c) => (a === "start" ? c.state !== "running" : a === "remove" ? true : c.state === "running"));
  const doBulk = async (a: ContainerAction) => {
    const r = await bulk(
      targets(a).map((c) => ({ id: c.id, name: c.name })),
      a,
    );
    if (r) setSelected({});
  };

  const columns = [
    selectColumn(col, (c) => c.name),
    col.accessor("name", {
      header: "Container",
      sortFn: "text",
      cell: ({ row: { original: c } }) => (
        <Link href={`/containers/${c.id}`} className="block min-w-0 hover:text-primary">
          <span className="block font-medium break-all">{c.name}</span>
          <span className="block max-w-[36ch] truncate text-xs text-muted-foreground" title={c.image}>
            {shortImage(c.image)}
          </span>
        </Link>
      ),
    }),
    col.accessor("state", {
      header: "State",
      filterFn: "equalsString",
      meta: { filter: { label: "State" } },
      cell: ({ row: { original: c } }) => (
        <span className="flex flex-wrap gap-1">
          <StateChip c={c} />
          <HealthChip health={c.health} />
          {c.looping && (
            <Chip tone="critical" title="Died 3 or more times in 15 minutes">
              restart loop
            </Chip>
          )}
          <span className="block w-full text-xs whitespace-nowrap text-muted-foreground">
            {c.state === "running" ? c.status.replace(/ \((healthy|unhealthy|health: starting)\)/, "") : c.status || `Created ${formatDuration(now / 1000 - c.created)} ago`}
          </span>
        </span>
      ),
    }),
    col.accessor((c) => c.live?.cpu ?? -1, {
      id: "cpu",
      header: "CPU",
      sortFn: "basic",
      meta: { align: "right" },
      cell: ({ getValue }) => (getValue() < 0 ? <span className="text-muted-foreground">—</span> : `${getValue().toFixed(1)}%`),
    }),
    col.accessor((c) => c.live?.mem ?? -1, {
      id: "mem",
      header: "Memory",
      sortFn: "basic",
      cell: ({ row: { original: c } }) => <MemCell c={c} memTotal={memTotal} />,
    }),
    col.accessor((c) => (c.live ? c.live.io_read + c.live.io_write : -1), {
      id: "io",
      header: "Disk I/O",
      sortFn: "basic",
      meta: { align: "right", className: "whitespace-nowrap text-muted-foreground" },
      cell: ({ row: { original: c } }) => (c.live ? formatRate(c.live.io_read + c.live.io_write) : "—"),
    }),
    col.accessor((c) => c.ports.filter((p) => p.public).length, {
      id: "ports",
      header: "Ports",
      enableSorting: false,
      cell: ({ row: { original: c } }) => <Ports ports={c.ports} />,
    }),
    col.display({
      id: "actions",
      header: () => <span className="sr-only">Actions</span>,
      meta: { align: "right" },
      cell: ({ row: { original: c } }) => <ContainerMenu id={c.id} name={c.name} state={c.state} />,
    }),
  ];

  return (
    <>
      <PageHeader title="Containers" />
      {containers.isPending && (
        <Loading label="Listing containers">
          <SectionSkeleton lines={8} />
        </Loading>
      )}
      {containers.error && <p className="text-sm text-critical">{containers.error.message}</p>}
      {containers.data && (
        <DataTable
          data={list}
          columns={columns}
          getRowId={(c) => c.id}
          searchPlaceholder="Search containers"
          initialSort={{ id: "cpu", desc: true }}
          minWidth="56rem"
          group={byStack}
          empty="No containers on this server."
          rowClassName={(c) => cn(c.state !== "running" && "text-muted-foreground")}
          selection={{ state: selected, onChange: setSelected }}
        />
      )}
      <BulkBar count={picked.length} noun={picked.length === 1 ? "container" : "containers"} onClear={() => setSelected({})}>
        {(["start", "restart", "stop", "remove"] as const).map((a) => {
          const it = items[a];
          const n = targets(a).length;
          return (
            <Button key={a} size="sm" variant={a === "remove" ? "destructive" : "outline"} icon={it.icon} disabled={n === 0} onClick={() => void doBulk(a)}>
              {it.label}
              {n > 0 && n !== picked.length && <span className="num text-muted-foreground">{n}</span>}
            </Button>
          );
        })}
      </BulkBar>
    </>
  );
}
