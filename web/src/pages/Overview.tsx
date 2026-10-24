import { useQuery } from "@tanstack/react-query";
import { Link } from "wouter";
import { api, type Container, type HostSample, type Overview, type Range } from "@/lib/api";
import { formatBytes, formatDuration, formatRate, pct } from "@/lib/format";
import { cn } from "@/lib/utils";
import { PageHeader } from "@/components/Layout";
import { Notice } from "@/components/Notice";
import { Section } from "@/components/ui/card";
import { Block } from "@/components/ui/block";
import { OptionSelect } from "@/components/ui/option-select";
import { SimpleTable } from "@/components/ui/simple-table";
import { Stat, StatRow } from "@/components/ui/stat";
import { Loading, SectionSkeleton, StatRowSkeleton } from "@/components/ui/skeleton";
import { Bar, Meter, SparkFigure } from "@/components/charts";
import { ActivityList } from "@/components/ActivityList";
import { StackChip } from "@/components/states";
import { rangeOptions, rangeRefetch, useRange } from "@/hooks/use-range";

export function OverviewPage() {
  const overview = useQuery({ queryKey: ["overview"], queryFn: api.overview, refetchInterval: 10_000 });
  const [range, setRange] = useRange();
  const o = overview.data;
  return (
    <>
      <PageHeader
        title="Overview"
        sub={o ? `${o.info.hostname} · ${o.info.os || "Linux"} · up ${formatDuration(o.info.uptime_sec)}` : "How the server and its containers are doing right now."}
      >
        <OptionSelect label="Chart range" value={range} onChange={setRange} options={rangeOptions} />
      </PageHeader>
      <div className="space-y-4">
        {overview.isPending && (
          <Loading label="Reading the server" className="space-y-4">
            <StatRowSkeleton />
            <SectionSkeleton lines={4} />
          </Loading>
        )}
        {overview.error && <p className="text-sm text-critical">{overview.error.message}</p>}
        {o && (
          <>
            {o.docker_error && (
              <Notice tone="red" title="Docker isn't answering">
                {o.docker_error}
              </Notice>
            )}
            <Problems o={o} />
            <Vitals o={o} />
            <HostCharts range={range} />
            <div className="grid gap-4 xl:grid-cols-[3fr_2fr]">
              <Busiest containers={o.containers} memTotal={o.host?.mem_total ?? 0} />
              <Stacks o={o} />
            </div>
            <Block title="Recent activity" sub="Container starts, stops and crashes, sign-ins to the server, and what was done here." actions={<Link href="/activity" className="text-sm text-muted-foreground hover:text-primary">See all</Link>}>
              <ActivityList items={o.activity} />
            </Block>
          </>
        )}
      </div>
    </>
  );
}

function Vitals({ o }: { o: Overview }) {
  const h = o.host;
  const disk = o.info.disks[0];
  const cpu = Math.round(h?.cpu ?? 0);
  const mem = h ? pct(h.mem_used, h.mem_total) : 0;
  const running = o.counts.running ?? 0;
  const bad = (o.counts.unhealthy ?? 0) + (o.counts.looping ?? 0) + (o.counts.dead ?? 0);
  return (
    <StatRow>
      <Meter label="CPU" value={h ? `${cpu}%` : "…"} percent={cpu} hint={`Load ${o.info.load[0].toFixed(2)} on ${o.info.cpus} cores`} />
      <Meter
        label="Memory"
        value={h ? `${mem}%` : "…"}
        percent={mem}
        hint={h ? `${formatBytes(h.mem_used)} of ${formatBytes(h.mem_total)}${h.swap_used > 0 ? `, ${formatBytes(h.swap_used)} swap` : ""}` : "Measuring"}
      />
      {disk ? (
        <Meter
          label="Disk"
          value={`${pct(disk.used, disk.total)}%`}
          percent={pct(disk.used, disk.total)}
          tone={diskTone(pct(disk.used, disk.total), o.disk_forecast_days)}
          hint={
            o.disk_forecast_days >= 0 && o.disk_forecast_days < 365
              ? `${formatBytes(disk.free)} free, full in ~${Math.max(1, Math.round(o.disk_forecast_days))} days`
              : `${formatBytes(disk.free)} free of ${formatBytes(disk.total)}`
          }
        />
      ) : (
        <Meter label="Disk" value="Unknown" percent={0} hint="The root disk couldn't be read." />
      )}
      <Stat
        label="Containers"
        href="/containers"
        value={
          <>
            {running}
            <span className="text-base font-normal text-muted-foreground"> of {o.counts.total ?? 0}</span>
          </>
        }
        tone={bad > 0 ? "critical" : undefined}
        aside={h ? `↓ ${formatRate(h.net_rx)} ↑ ${formatRate(h.net_tx)}` : undefined}
        note={bad > 0 ? `${bad} unhealthy or in a restart loop` : `Running, across ${o.stacks.length} stacks`}
      />
    </StatRow>
  );
}

/** The worse of how full the disk is and how soon it fills. */
function diskTone(used: number, days: number): "critical" | "warning" | undefined {
  const soon = days >= 0;
  if (used >= 90 || (soon && days < 7)) return "critical";
  if (used >= 75 || (soon && days < 30)) return "warning";
  return undefined;
}

const charts: { label: string; value: (s: HostSample) => number; max?: (xs: number[]) => number; format: (v: number) => string }[] = [
  { label: "CPU", value: (s) => s.cpu, max: () => 100, format: (v) => `${Math.round(v)}%` },
  { label: "Memory", value: (s) => pct(s.mem_used, s.mem_total), max: () => 100, format: (v) => `${Math.round(v)}%` },
  { label: "Network in", value: (s) => s.net_rx, format: formatRate },
  { label: "Network out", value: (s) => s.net_tx, format: formatRate },
  { label: "Disk read", value: (s) => s.disk_read, format: formatRate },
  { label: "Disk write", value: (s) => s.disk_write, format: formatRate },
];

const rangeSub: Record<Range, string> = {
  "1h": "The last hour, one reading every 10 seconds.",
  "24h": "The last day, averaged over 5 minutes.",
  "7d": "The last week, averaged over 30 minutes.",
};

export function HostCharts({ range }: { range: Range }) {
  const history = useQuery({ queryKey: ["host-history", range], queryFn: () => api.hostHistory(range), refetchInterval: rangeRefetch(range) });
  const rows = history.data ?? [];
  const times = rows.map((r) => r.at);
  return (
    <Section title="Server" sub={rows.length < 2 && !history.isPending ? "Readings start when StunSails starts; the charts fill in over the next minute." : rangeSub[range]}>
      <div className="grid gap-x-6 gap-y-5 sm:grid-cols-2 lg:grid-cols-3">
        {charts.map((c) => {
          const values = rows.map(c.value);
          return <SparkFigure key={c.label} label={c.label} values={values} times={times} max={c.max ? c.max(values) : Math.max(...values, 1)} format={c.format} />;
        })}
      </div>
    </Section>
  );
}

function Busiest({ containers, memTotal }: { containers: Container[]; memTotal: number }) {
  const rows = [...containers].sort((a, b) => (b.live?.cpu ?? 0) - (a.live?.cpu ?? 0) || (b.live?.mem ?? 0) - (a.live?.mem ?? 0)).slice(0, 8);
  return (
    <Block title="Busiest containers" sub="Running now, by CPU. 100% is one full core." actions={<Link href="/containers" className="text-sm text-muted-foreground hover:text-primary">All containers</Link>}>
      <SimpleTable
        label="Busiest containers"
        rows={rows}
        rowKey={(c) => c.id}
        href={(c) => `/containers/${c.id}`}
        empty="No container is running."
        columns={[
          { header: "Container", cell: (c) => <span className="whitespace-nowrap">{c.name}</span> },
          { header: "Stack", cell: (c) => <span className="whitespace-nowrap text-muted-foreground">{c.project || "—"}</span> },
          { header: "CPU", align: "right", cell: (c) => `${(c.live?.cpu ?? 0).toFixed(1)}%` },
          { header: "Memory", cell: (c) => <MemCell c={c} memTotal={memTotal} /> },
        ]}
      />
    </Block>
  );
}

export function MemCell({ c, memTotal }: { c: Container; memTotal: number }) {
  if (!c.live) return <span className="text-muted-foreground">—</span>;
  const limited = c.live.mem_limit > 0 && (memTotal === 0 || c.live.mem_limit < memTotal);
  const p = pct(c.live.mem, limited ? c.live.mem_limit : memTotal);
  return (
    <div className="flex items-center gap-2">
      <Bar percent={p} label={`${c.name} memory`} className="w-16 shrink-0" />
      <span className="num whitespace-nowrap">
        {formatBytes(c.live.mem)}
        {limited && <span className="text-muted-foreground"> of {formatBytes(c.live.mem_limit)}</span>}
      </span>
    </div>
  );
}

function Stacks({ o }: { o: Overview }) {
  return (
    <Block title="Stacks" sub="Compose projects on this server." actions={<Link href="/stacks" className="text-sm text-muted-foreground hover:text-primary">Manage</Link>}>
      {o.stacks.length === 0 ? (
        <p className="rounded-md border border-dashed px-4 py-6 text-center text-sm text-muted-foreground">No compose stacks yet.</p>
      ) : (
        <ul className="divide-y rounded-md border">
          {o.stacks.map((s) => (
            <li key={s.project}>
              <Link href={`/stacks/${s.project}`} className="flex h-10 items-center gap-3 px-3 text-sm transition-colors hover:bg-muted/30">
                <span className="min-w-0 flex-1 truncate font-medium">{s.project}</span>
                {s.unhealthy > 0 && <span className="text-xs text-critical">{s.unhealthy} unhealthy</span>}
                <span className={cn("num text-xs text-muted-foreground", s.total === 0 && "invisible")}>
                  {s.running}/{s.total}
                </span>
                <StackChip state={s.state} />
              </Link>
            </li>
          ))}
        </ul>
      )}
    </Block>
  );
}

/** Whatever needs the owner now: restart loops, failed services, sites down, open alerts. */
function Problems({ o }: { o: Overview }) {
  const down = o.sites?.down ?? [];
  const looping = o.looping ?? [];
  const units = o.failed_units ?? [];
  return (
    <>
      {looping.length > 0 && (
        <Notice tone="red" title={`${looping.length === 1 ? "A container keeps" : `${looping.length} containers keep`} restarting`}>
          {looping.map((c, i) => (
            <span key={c.id}>
              {i > 0 && ", "}
              <Link href={`/containers/${c.id}?tab=logs`} className="font-medium underline underline-offset-4">
                {c.name}
              </Link>
            </span>
          ))}
          . Each died 3 or more times in 15 minutes; their logs usually say why.
        </Notice>
      )}
      {units.length > 0 && (
        <Notice tone="red" title={`${units.length === 1 ? "A system service has" : `${units.length} system services have`} failed`}>
          {units.map((u) => u.unit).join(", ")}. Check with <code className="rounded bg-card px-1 py-0.5 text-xs">journalctl -u {units[0]!.unit}</code>.
        </Notice>
      )}
      {down.length > 0 && (
        <Notice tone="red" title={`${down.length === 1 ? "A site is" : `${down.length} sites are`} down`}>
          <Link href="/sites" className="font-medium underline underline-offset-4">
            {down.join(", ")}
          </Link>{" "}
          didn't answer two checks in a row.
        </Notice>
      )}
    </>
  );
}
