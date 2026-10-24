import { useEffect, useRef, useState, type ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import { Link, useLocation, useSearch } from "wouter";
import { api, type Detail, type Live } from "@/lib/api";
import { formatBytes, formatDuration, formatFullDateTime, formatRate, pct, shortId } from "@/lib/format";
import { PageHeader } from "@/components/Layout";
import { Notice } from "@/components/Notice";
import { Section } from "@/components/ui/card";
import { Block } from "@/components/ui/block";
import { Chip } from "@/components/ui/chip";
import { Segmented } from "@/components/ui/segmented";
import { OptionSelect } from "@/components/ui/option-select";
import { SimpleTable } from "@/components/ui/simple-table";
import { Stat, StatRow } from "@/components/ui/stat";
import { Loading, SectionSkeleton, StatRowSkeleton } from "@/components/ui/skeleton";
import { Bar, SparkFigure } from "@/components/charts";
import { HealthChip, StateChip } from "@/components/states";
import { ContainerButtons } from "@/components/ContainerActions";
import { LogViewer } from "@/components/LogViewer";
import { ActivityList } from "@/components/ActivityList";
import { rangeOptions, rangeRefetch, useRange } from "@/hooks/use-range";
import { useNow } from "@/hooks/use-now";

type Tab = "overview" | "resources" | "logs" | "events";
const tabs: { id: Tab; label: string }[] = [
  { id: "overview", label: "Overview" },
  { id: "resources", label: "Resources" },
  { id: "logs", label: "Logs" },
  { id: "events", label: "Events" },
];

export function ContainerPage({ id }: { id: string }) {
  const search = useSearch();
  const [, navigate] = useLocation();
  const tab = (new URLSearchParams(search).get("tab") as Tab) || "overview";
  const setTab = (t: Tab) => navigate(`/containers/${id}${t === "overview" ? "" : `?tab=${t}`}`, { replace: true });
  const q = useQuery({ queryKey: ["container", id], queryFn: () => api.container(id), refetchInterval: 5_000 });
  const d = q.data?.detail;
  const now = useNow(10_000);

  if (q.isPending)
    return (
      <Loading label="Reading the container" className="space-y-4">
        <StatRowSkeleton />
        <SectionSkeleton lines={6} />
      </Loading>
    );
  if (q.error || !d)
    return (
      <Notice tone="amber" title="This container isn't here">
        {q.error?.message} <Link href="/containers" className="font-medium underline underline-offset-4">Back to containers</Link>
      </Notice>
    );

  const started = Date.parse(d.state.started_at);
  const finished = Date.parse(d.state.finished_at);
  return (
    <>
      <PageHeader
        title={d.name}
        sub={
          <span className="flex flex-wrap items-center gap-x-2 gap-y-1">
            <StateChip c={{ state: d.state.status, status: d.state.status === "exited" ? `Exited (${d.state.exit_code})` : "" }} />
            {d.state.health && <HealthChip health={d.state.health.status as "healthy"} />}
            {d.state.oom_killed && <Chip tone="critical">out of memory</Chip>}
            <span className="break-all">{d.image}</span>
            {d.project && (
              <>
                <span aria-hidden>·</span>
                <Link href={`/stacks/${d.project}`} className="hover:text-primary">
                  {d.project}
                  {d.service && ` / ${d.service}`}
                </Link>
              </>
            )}
          </span>
        }
      >
        <ContainerButtons id={d.id} name={d.name} state={d.state.status} onRemoved={() => navigate("/containers")} />
      </PageHeader>
      <div className="space-y-4">
        <StatRow>
          <Stat label="CPU" value={q.data.live ? `${q.data.live.cpu.toFixed(1)}%` : "—"} note={d.cpu_limit > 0 ? `Limited to ${d.cpu_limit} cores` : "No CPU limit"} />
          <MemStat live={q.data.live} d={d} />
          <Stat
            label={d.state.running ? "Up for" : "Stopped"}
            value={d.state.running && started ? formatDuration((now - started) / 1000) : finished > 0 ? formatDuration((now - finished) / 1000) : "—"}
            note={d.state.running ? `Since ${formatFullDateTime(started)}` : finished > 0 ? `ago, exit code ${d.state.exit_code}` : "Never started"}
            tone={!d.state.running && d.state.exit_code !== 0 ? "critical" : undefined}
          />
          <Stat label="Restarts" value={d.restart_count} note={`Policy: ${d.restart_policy || "no"}`} tone={d.restart_count > 5 ? "warning" : undefined} />
        </StatRow>
        {d.state.error && (
          <Notice tone="red" title="Docker reported an error">
            {d.state.error}
          </Notice>
        )}
        <Segmented kind="tabs" label="Container views" value={tab} onChange={setTab} options={tabs} />
        {tab === "overview" && <OverviewTab d={d} />}
        {tab === "resources" && <ResourcesTab d={d} live={q.data.live} />}
        {tab === "logs" && (
          <Block title="Logs" sub="stdout and stderr; stderr lines are red.">
            <LogViewer id={d.id} name={d.name} />
          </Block>
        )}
        {tab === "events" && <EventsTab d={d} />}
      </div>
    </>
  );
}

function MemStat({ live, d }: { live: Live | null; d: Detail }) {
  const limit = live?.mem_limit || d.memory_limit;
  const p = live && limit ? pct(live.mem, limit) : 0;
  return (
    <Stat
      label="Memory"
      value={live ? formatBytes(live.mem) : "—"}
      tone={p >= 90 ? "critical" : p >= 75 ? "warning" : undefined}
      chart={limit ? <Bar percent={p} label="Memory of its limit" /> : undefined}
      note={limit ? `${p}% of its ${formatBytes(limit)} limit` : "No memory limit"}
    />
  );
}

function Rows({ rows }: { rows: [string, ReactNode][] }) {
  return (
    <dl className="grid gap-x-6 gap-y-3 text-sm sm:grid-cols-2 lg:grid-cols-3">
      {rows.map(([k, v]) => (
        <div key={k} className="min-w-0">
          <dt className="text-[0.8125rem] text-muted-foreground">{k}</dt>
          <dd className="mt-0.5 break-words">{v || <span className="text-muted-foreground">—</span>}</dd>
        </div>
      ))}
    </dl>
  );
}

const code = (s: string) => <code className="literal rounded bg-muted px-1 py-0.5 text-[0.8125rem] break-all">{s}</code>;

function OverviewTab({ d }: { d: Detail }) {
  const [showLabels, setShowLabels] = useState(false);
  const labels = Object.entries(d.labels).sort(([a], [b]) => a.localeCompare(b));
  return (
    <>
      <Section title="Details">
        <Rows
          rows={[
            ["Image", d.image],
            ["Image ID", code(shortId(d.image_id))],
            ["Container ID", code(shortId(d.id))],
            ["Command", d.command.length ? code(d.command.join(" ")) : ""],
            ["Created", formatFullDateTime(d.created)],
            ["Restart policy", d.restart_policy || "no"],
            ["Network mode", d.network_mode],
            ["User", d.user || "root (image default)"],
            ["Working folder", d.working_dir],
            ["PID", d.state.pid ? String(d.state.pid) : ""],
            ["Privileged", d.privileged ? <Chip tone="warning">yes</Chip> : "no"],
            ["Healthcheck", d.healthcheck.length ? code(d.healthcheck.filter((x) => x !== "CMD" && x !== "CMD-SHELL").join(" ")) : "None"],
          ]}
        />
      </Section>
      <div className="grid gap-4 xl:grid-cols-2">
        <Block title="Ports" sub="Published ports reach the container from the host.">
          <SimpleTable
            label="Ports"
            rows={d.ports}
            rowKey={(p) => `${p.container}-${p.host_ip}-${p.host_port}`}
            empty="No ports exposed."
            columns={[
              { header: "Container", cell: (p) => <span className="num">{p.container}</span> },
              { header: "Host", cell: (p) => (p.host_port ? <span className="num">{`${p.host_ip && p.host_ip !== "0.0.0.0" ? p.host_ip : "all addresses"}:${p.host_port}`}</span> : <span className="text-muted-foreground">Not published</span>) },
            ]}
          />
        </Block>
        <Block title="Networks">
          <SimpleTable
            label="Networks"
            rows={d.networks}
            rowKey={(n) => n.name}
            empty="Not on any network."
            columns={[
              { header: "Network", cell: (n) => <span className="font-medium">{n.name}</span> },
              { header: "Address", cell: (n) => <span className="num">{n.ip || "—"}</span> },
              { header: "Aliases", cell: (n) => <span className="text-muted-foreground">{n.aliases.join(", ") || "—"}</span> },
            ]}
          />
        </Block>
      </div>
      <Block title="Mounts" sub="Folders and volumes the container can see.">
        <SimpleTable
          label="Mounts"
          rows={d.mounts}
          rowKey={(m) => m.destination}
          empty="No mounts."
          columns={[
            { header: "Inside", cell: (m) => <span className="num break-all">{m.destination}</span> },
            { header: "Kind", cell: (m) => m.type },
            { header: "Source", cell: (m) => <span className="break-all text-muted-foreground">{m.name || m.source}</span> },
            { header: "Access", cell: (m) => (m.rw ? "read-write" : <Chip tone="neutral">read-only</Chip>) },
          ]}
        />
      </Block>
      {d.state.health && (
        <Block title="Health checks" sub={`Last ${d.state.health.log.length} probes. ${d.state.health.failing_streak} failing in a row.`}>
          <SimpleTable
            label="Health checks"
            rows={[...d.state.health.log].reverse()}
            rowKey={(h) => h.start}
            empty="No probes yet."
            columns={[
              { header: "When", cell: (h) => <span className="whitespace-nowrap">{formatFullDateTime(h.start)}</span> },
              { header: "Result", cell: (h) => <Chip tone={h.exit_code === 0 ? "good" : "critical"}>{h.exit_code === 0 ? "passed" : `exit ${h.exit_code}`}</Chip> },
              { header: "Output", cell: (h) => <span className="line-clamp-2 break-all text-muted-foreground">{h.output || "—"}</span> },
            ]}
          />
        </Block>
      )}
      <Section title="Environment" sub="Variable names only. Values often hold secrets, so StunSails never sends them to the browser.">
        {d.env_keys.length ? (
          <div className="flex flex-wrap gap-1.5">
            {d.env_keys.map((k) => (
              <code key={k} className="literal rounded bg-muted px-1.5 py-0.5 text-xs">
                {k}
              </code>
            ))}
          </div>
        ) : (
          <p className="text-sm text-muted-foreground">No variables.</p>
        )}
      </Section>
      <Block
        title="Labels"
        sub={`${labels.length} labels.`}
        actions={
          labels.length > 0 && (
            <button type="button" className="text-sm text-muted-foreground hover:text-primary" onClick={() => setShowLabels((v) => !v)}>
              {showLabels ? "Hide" : "Show"}
            </button>
          )
        }
      >
        {showLabels ? (
          <SimpleTable
            label="Labels"
            rows={labels}
            rowKey={([k]) => k}
            empty="No labels."
            columns={[
              { header: "Label", cell: ([k]) => <span className="num break-all">{k}</span> },
              { header: "Value", cell: ([, v]) => <span className="break-all text-muted-foreground">{v}</span> },
            ]}
          />
        ) : null}
      </Block>
    </>
  );
}

/** Network rates measured in the browser from Docker's counters, every 5 s while open. */
function useNetRates(id: string, running: boolean) {
  const [points, setPoints] = useState<{ at: number; rx: number; tx: number }[]>([]);
  const prev = useRef<{ rx: number; tx: number; at: number } | null>(null);
  const net = useQuery({ queryKey: ["container-net", id], queryFn: () => api.containerNet(id), refetchInterval: 5_000, enabled: running });
  useEffect(() => {
    const n = net.data;
    if (!n) return;
    const p = prev.current;
    prev.current = n;
    if (!p || n.at <= p.at) return;
    const dt = (n.at - p.at) / 1000;
    setPoints((xs) => [...xs.slice(-119), { at: Math.floor(n.at / 1000), rx: Math.max(0, (n.rx - p.rx) / dt), tx: Math.max(0, (n.tx - p.tx) / dt) }]);
  }, [net.data]);
  return { points, totals: net.data };
}

function ResourcesTab({ d, live }: { d: Detail; live: Live | null }) {
  const [range, setRange] = useRange();
  const history = useQuery({ queryKey: ["container-history", d.id, range], queryFn: () => api.containerHistory(d.id, range), refetchInterval: rangeRefetch(range) });
  const rows = history.data ?? [];
  const times = rows.map((r) => r.at);
  const limit = live?.mem_limit || d.memory_limit;
  const { points, totals } = useNetRates(d.id, d.state.running);
  const cpu = rows.map((r) => r.cpu);
  const mem = rows.map((r) => r.mem);
  return (
    <>
      <Section title="CPU and memory" sub={rows.length < 2 ? "Readings appear once the container has run for a little while." : "CPU is per core: 100% is one full core."} actions={<OptionSelect label="Chart range" value={range} onChange={setRange} options={rangeOptions} />}>
        <div className="grid gap-x-6 gap-y-5 md:grid-cols-2">
          <SparkFigure label="CPU" values={cpu} times={times} max={Math.max(100, ...cpu)} format={(v) => `${v.toFixed(1)}%`} height={96} />
          <SparkFigure label={limit ? `Memory (limit ${formatBytes(limit)})` : "Memory"} values={mem} times={times} max={limit || Math.max(...mem, 1)} format={formatBytes} height={96} />
        </div>
      </Section>
      <Section title="Network and disk" sub={d.state.running ? "Network measured live while this page is open; totals since the container started." : "The container isn't running."}>
        <div className="grid gap-x-6 gap-y-5 md:grid-cols-2">
          <SparkFigure label="Received" values={points.map((p) => p.rx)} times={points.map((p) => p.at)} max={Math.max(...points.map((p) => p.rx), 1)} format={formatRate} />
          <SparkFigure label="Sent" values={points.map((p) => p.tx)} times={points.map((p) => p.at)} max={Math.max(...points.map((p) => p.tx), 1)} format={formatRate} />
        </div>
        <dl className="mt-5 grid grid-cols-2 gap-4 text-sm lg:grid-cols-5">
          {(
            [
              ["Received in total", totals ? formatBytes(totals.rx) : "—"],
              ["Sent in total", totals ? formatBytes(totals.tx) : "—"],
              ["Disk read", live ? formatRate(live.io_read) : "—"],
              ["Disk write", live ? formatRate(live.io_write) : "—"],
              ["Processes", live ? String(live.pids) : "—"],
            ] as const
          ).map(([k, v]) => (
            <div key={k}>
              <dt className="text-[0.8125rem] text-muted-foreground">{k}</dt>
              <dd className="num mt-0.5 text-lg font-semibold tracking-[-0.01em]">{v}</dd>
            </div>
          ))}
        </dl>
      </Section>
    </>
  );
}

function EventsTab({ d }: { d: Detail }) {
  const events = useQuery({ queryKey: ["activity", { target: d.id }], queryFn: () => api.activity({ target: d.id, limit: 100 }), refetchInterval: 10_000 });
  return (
    <Block title="Events" sub="What Docker and people did to this container, newest first. Kept for 30 days.">
      {events.data ? <ActivityList items={events.data} empty="Nothing recorded for this container yet." /> : <SectionSkeleton lines={4} />}
    </Block>
  );
}
