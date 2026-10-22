import { useQuery } from "@tanstack/react-query";
import { useLocation } from "wouter";
import { Copy, FileSearch, History } from "@/components/ui/lucide";
import { api, type Stack, type StackVerb } from "@/lib/api";
import { ago, formatBytes } from "@/lib/format";
import { PageHeader } from "@/components/Layout";
import { Block } from "@/components/ui/block";
import { RowMenu } from "@/components/ui/row-menu";
import { SimpleTable } from "@/components/ui/simple-table";
import { Loading, SectionSkeleton } from "@/components/ui/skeleton";
import { StackChip } from "@/components/states";
import { useStackRun, verbIcons, verbs } from "@/components/StackRunner";
import { copy } from "@/components/ContainerActions";

/** What a stack can do in its state, most likely first. */
export function verbsFor(s: Pick<Stack, "state">): StackVerb[] {
  switch (s.state) {
    case "running":
      return ["restart", "pull", "up", "stop", "down"];
    case "partial":
      return ["up", "restart", "pull", "stop", "down"];
    case "stopped":
      return ["start", "up", "pull", "down"];
    default:
      return ["up", "pull"];
  }
}

export function totals(s: Stack) {
  let cpu = 0;
  let mem = 0;
  for (const l of Object.values(s.live)) {
    cpu += l.cpu;
    mem += l.mem;
  }
  return { cpu, mem };
}

export function StackMenu({ s }: { s: Stack }) {
  const run = useStackRun();
  const [, navigate] = useLocation();
  const vs = verbsFor(s);
  const item = (v: StackVerb) => ({ label: verbs[v].label, icon: verbIcons[v], danger: v === "down", onSelect: () => void run(s.project, v) });
  return (
    <RowMenu
      label={`Actions for ${s.project}`}
      groups={[
        [
          { label: "Open", icon: FileSearch, onSelect: () => navigate(`/stacks/${s.project}`) },
          { label: "Activity", icon: History, onSelect: () => navigate(`/activity?q=${encodeURIComponent(s.project)}`) },
          ...(s.working_dir ? [{ label: "Copy folder path", icon: Copy, onSelect: () => copy(s.working_dir, "the folder path") }] : []),
        ],
        s.manageable ? vs.filter((v) => v !== "down").map(item) : [],
        s.manageable && vs.includes("down") ? [item("down")] : [],
      ]}
    />
  );
}

export function StacksPage() {
  const stacks = useQuery({ queryKey: ["stacks"], queryFn: api.stacks, refetchInterval: 10_000 });
  const list = stacks.data ?? [];
  const up = list.filter((s) => s.state === "running").length;
  return (
    <>
      <PageHeader
        title="Stacks"
        sub={stacks.data ? `${up} of ${list.length} compose projects fully running.` : "Every docker compose project on the server."}
      />
      {stacks.isPending && (
        <Loading label="Listing stacks">
          <SectionSkeleton lines={6} />
        </Loading>
      )}
      {stacks.error && <p className="text-sm text-critical">{stacks.error.message}</p>}
      {stacks.data && (
        <Block title="Compose projects" sub="Stacks taken down stay listed, so they can be brought back up. Actions run docker compose with the stack's own files.">
          <SimpleTable
            label="Compose projects"
            rows={list}
            rowKey={(s) => s.project}
            href={(s) => `/stacks/${s.project}`}
            empty="No compose stacks yet. Start one with docker compose up and it appears here."
            columns={[
              { header: "Stack", cell: (s) => s.project },
              {
                header: "State",
                cell: (s) => (
                  <span className="flex items-center gap-1.5">
                    <StackChip state={s.state} />
                    {s.unhealthy > 0 && <span className="text-xs text-critical">{s.unhealthy} unhealthy</span>}
                  </span>
                ),
              },
              { header: "Services", align: "right", cell: (s) => (s.total ? `${s.running}/${s.total}` : `${s.services.length || "—"}`) },
              { header: "CPU", align: "right", cell: (s) => (s.running ? `${totals(s).cpu.toFixed(1)}%` : "—") },
              { header: "Memory", align: "right", cell: (s) => (s.running ? formatBytes(totals(s).mem) : "—") },
              {
                header: "Folder",
                cell: (s) => (
                  <span className="block max-w-[28ch] truncate text-muted-foreground" title={s.working_dir}>
                    {s.working_dir || "Unknown"}
                    {s.state === "down" && s.last_seen > 0 && ` · last seen ${ago(s.last_seen)}`}
                  </span>
                ),
              },
              { header: " ", align: "right", cell: (s) => <StackMenu s={s} /> },
            ]}
          />
        </Block>
      )}
    </>
  );
}
