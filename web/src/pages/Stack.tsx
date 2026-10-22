import { useQuery } from "@tanstack/react-query";
import { Link, useLocation } from "wouter";
import { Trash2 } from "@/components/ui/lucide";
import { api, type Container } from "@/lib/api";
import { formatBytes } from "@/lib/format";
import { PageHeader } from "@/components/Layout";
import { Notice } from "@/components/Notice";
import { Button } from "@/components/ui/button";
import { Section } from "@/components/ui/card";
import { Block } from "@/components/ui/block";
import { SimpleTable } from "@/components/ui/simple-table";
import { Stat, StatRow } from "@/components/ui/stat";
import { Loading, SectionSkeleton, StatRowSkeleton } from "@/components/ui/skeleton";
import { ActivityList } from "@/components/ActivityList";
import { HealthChip, Ports, StackChip, StateChip } from "@/components/states";
import { ContainerMenu } from "@/components/ContainerActions";
import { useStackRun, verbIcons, verbs } from "@/components/StackRunner";
import { useRun } from "@/components/guard";
import { totals, verbsFor } from "@/pages/Stacks";

export function StackPage({ project }: { project: string }) {
  const q = useQuery({ queryKey: ["stack", project], queryFn: () => api.stack(project), refetchInterval: 5_000 });
  const activity = useQuery({ queryKey: ["activity", { q: project }], queryFn: () => api.activity({ q: project, limit: 30 }), refetchInterval: 15_000 });
  const run = useStackRun();
  const guard = useRun();
  const [, navigate] = useLocation();
  const s = q.data;

  if (q.isPending)
    return (
      <Loading label="Reading the stack" className="space-y-4">
        <StatRowSkeleton />
        <SectionSkeleton lines={5} />
      </Loading>
    );
  if (q.error || !s)
    return (
      <Notice tone="amber" title="No such stack">
        {q.error?.message} <Link href="/stacks" className="font-medium underline underline-offset-4">Back to stacks</Link>
      </Notice>
    );

  const t = totals(s);
  const rows: (Container & { svc: string })[] = s.services.flatMap((sv) => sv.containers.map((c) => ({ ...c, svc: sv.name, live: s.live[c.id] ?? null })));
  return (
    <>
      <PageHeader
        title={s.project}
        sub={
          <span className="flex flex-wrap items-center gap-2">
            <StackChip state={s.state} />
            <span className="max-w-[48ch] truncate" title={s.working_dir}>
              {s.working_dir || "Folder unknown"}
            </span>
          </span>
        }
      >
        {s.manageable &&
          verbsFor(s).map((v, i) => (
            <Button key={v} size="sm" variant={v === "down" || v === "stop" ? "destructive" : i === 0 ? "default" : "outline"} icon={verbIcons[v]} onClick={() => void run(s.project, v)}>
              {verbs[v].label}
            </Button>
          ))}
        {s.state === "down" && (
          <Button
            size="sm"
            variant="ghost"
            icon={Trash2}
            onClick={async () => {
              const ok = await guard(() => api.forgetStack(s.project), {
                confirm: { title: `Forget ${s.project}?`, description: "Harbor stops listing it. Nothing on disk changes; it comes back the next time it's started with docker compose." },
                success: `Forgot ${s.project}`,
                invalidate: [["stacks"]],
              });
              if (ok) navigate("/stacks");
            }}
          >
            Forget
          </Button>
        )}
      </PageHeader>
      <div className="space-y-4">
        {!s.manageable && (
          <Notice tone="amber" title="Harbor can't run compose for this stack">
            Its containers don't say where the compose file lives. Bring it up once with docker compose on the server, and the actions appear.
          </Notice>
        )}
        <StatRow>
          <Stat label="Services" value={s.total ? `${s.running}/${s.total}` : s.services.length || 0} note={s.total ? "Containers running" : "Nothing is running"} tone={s.state === "partial" ? "warning" : undefined} />
          <Stat label="Unhealthy" value={s.unhealthy} tone={s.unhealthy ? "critical" : "good"} note={s.unhealthy ? "Failing their health check" : "All checks passing or none set"} />
          <Stat label="CPU" value={s.running ? `${t.cpu.toFixed(1)}%` : "—"} note="All services, per core" />
          <Stat label="Memory" value={s.running ? formatBytes(t.mem) : "—"} note="All services together" />
        </StatRow>
        <Block title="Services">
          <SimpleTable
            label="Services"
            rows={rows}
            rowKey={(c) => c.id}
            empty="No containers. Bring the stack up to create them."
            columns={[
              { header: "Service", cell: (c) => <span className="font-medium">{c.svc}</span> },
              {
                header: "Container",
                cell: (c) => (
                  <Link href={`/containers/${c.id}`} className="break-all hover:text-primary">
                    {c.name}
                  </Link>
                ),
              },
              {
                header: "State",
                cell: (c) => (
                  <span className="flex flex-wrap gap-1">
                    <StateChip c={c} />
                    <HealthChip health={c.health} />
                  </span>
                ),
              },
              { header: "CPU", align: "right", cell: (c) => (c.live ? `${c.live.cpu.toFixed(1)}%` : "—") },
              { header: "Memory", align: "right", cell: (c) => (c.live ? formatBytes(c.live.mem) : "—") },
              { header: "Ports", cell: (c) => <Ports ports={c.ports} /> },
              { header: "Image", cell: (c) => <span className="block max-w-[26ch] truncate text-muted-foreground" title={c.image}>{c.image}</span> },
              { header: " ", align: "right", cell: (c) => <ContainerMenu id={c.id} name={c.name} state={c.state} /> },
            ]}
          />
        </Block>
        <Section title="Files" sub="Compose runs with these, mounted read-only into a short-lived docker CLI container.">
          <dl className="grid gap-3 text-sm sm:grid-cols-2">
            <div>
              <dt className="text-[0.8125rem] text-muted-foreground">Project folder</dt>
              <dd className="mt-0.5 break-all">{s.working_dir || "—"}</dd>
            </div>
            <div>
              <dt className="text-[0.8125rem] text-muted-foreground">Compose files</dt>
              <dd className="mt-0.5 break-all">{s.config_files.join(", ") || "—"}</dd>
            </div>
            {s.env_files.length > 0 && (
              <div>
                <dt className="text-[0.8125rem] text-muted-foreground">Env files</dt>
                <dd className="mt-0.5 break-all">{s.env_files.join(", ")}</dd>
              </div>
            )}
          </dl>
        </Section>
        <Block title="Activity" sub="Events mentioning this stack.">
          {activity.data ? <ActivityList items={activity.data} empty="Nothing recorded for this stack yet." /> : <SectionSkeleton lines={3} />}
        </Block>
      </div>
    </>
  );
}
