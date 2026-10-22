import { useState, type FormEvent } from "react";
import { useQueries, useQuery } from "@tanstack/react-query";
import { Copy, ExternalLink, History, Plus, Trash2 } from "@/components/ui/lucide";
import { useLocation } from "wouter";
import { api, type Site } from "@/lib/api";
import { ago } from "@/lib/format";
import { PageHeader } from "@/components/Layout";
import { Block } from "@/components/ui/block";
import { Button } from "@/components/ui/button";
import { Section } from "@/components/ui/card";
import { Chip } from "@/components/ui/chip";
import { Input } from "@/components/ui/field";
import { RowMenu } from "@/components/ui/row-menu";
import { OptionSelect } from "@/components/ui/option-select";
import { SimpleTable } from "@/components/ui/simple-table";
import { Stat, StatRow } from "@/components/ui/stat";
import { Loading, SectionSkeleton } from "@/components/ui/skeleton";
import { SparkFigure } from "@/components/charts";
import { useRun } from "@/components/guard";
import { copy } from "@/components/ContainerActions";
import { rangeOptions, rangeRefetch, useRange } from "@/hooks/use-range";
import { useNow } from "@/hooks/use-now";

const certDays = (s: Site, now: number) => (s.last?.cert_expiry ? Math.floor((s.last.cert_expiry * 1000 - now) / 86_400_000) : null);

export function SitesPage() {
  const sites = useQuery({ queryKey: ["sites"], queryFn: api.sites, refetchInterval: 30_000 });
  const [range, setRange] = useRange();
  const now = useNow(60_000);
  const run = useRun();
  const [, navigate] = useLocation();
  const [host, setHost] = useState("");
  const list = sites.data ?? [];
  const checked = list.filter((s) => s.last);
  const down = checked.filter((s) => !s.last!.up);
  const soonest = list.map((s) => certDays(s, now)).filter((d): d is number => d !== null).sort((a, b) => a - b)[0];
  const histories = useQueries({
    queries: list.map((s) => ({ queryKey: ["site-history", s.host, range], queryFn: () => api.siteHistory(s.host, range), refetchInterval: rangeRefetch(range) })),
  });

  async function add(e: FormEvent) {
    e.preventDefault();
    const h = host.trim();
    if (!h) return;
    const ok = await run(() => api.addSite(h), { success: `Checking ${h} every minute`, invalidate: [["sites"]] });
    if (ok) setHost("");
  }

  return (
    <>
      <PageHeader title="Sites" sub="Every site Caddy serves, fetched over HTTPS once a minute as a visitor would.">
        <OptionSelect label="Chart range" value={range} onChange={setRange} options={rangeOptions} />
      </PageHeader>
      {sites.isPending && (
        <Loading label="Checking sites">
          <SectionSkeleton lines={5} />
        </Loading>
      )}
      {sites.error && <p className="text-sm text-critical">{sites.error.message}</p>}
      {sites.data && (
        <div className="space-y-6">
          <StatRow>
            <Stat label="Up" value={`${checked.length - down.length} of ${list.length}`} tone={down.length ? "critical" : "good"} note={down.length ? `${down.map((s) => s.host).join(", ")} down` : "Every site answers"} />
            <Stat
              label="Slowest answer"
              value={checked.length ? `${Math.max(...checked.map((s) => s.last!.ms))} ms` : "—"}
              note={checked.length ? checked.reduce((a, b) => (a.last!.ms > b.last!.ms ? a : b)).host : "No checks yet"}
            />
            <Stat
              label="Uptime, 24 h"
              value={list.some((s) => s.uptime_24h >= 0) ? `${(list.filter((s) => s.uptime_24h >= 0).reduce((n, s) => n + s.uptime_24h, 0) / list.filter((s) => s.uptime_24h >= 0).length).toFixed(2)}%` : "—"}
              note="Average across sites"
            />
            <Stat label="Next certificate expiry" value={soonest !== undefined ? `${soonest} days` : "—"} tone={soonest !== undefined && soonest < 14 ? "warning" : undefined} note="Caddy renews them a month ahead" />
          </StatRow>

          <Block
            title="All sites"
            sub="401 or 404 still counts as up: the site answered. A 5xx or no answer is down; two in a row sends an alert."
            actions={
              <form onSubmit={add} className="flex items-center gap-2">
                <Input value={host} onChange={(e) => setHost(e.target.value)} placeholder="shop.example.com" aria-label="Hostname to check" className="h-8 w-56" />
                <Button type="submit" size="sm" variant="outline" icon={Plus} disabled={!host.trim()}>
                  Add site
                </Button>
              </form>
            }
          >
            <SimpleTable
              label="Sites"
              rows={list}
              rowKey={(s) => s.host}
              empty="No sites yet. Caddy's sites appear here by themselves; add others above."
              columns={[
                {
                  header: "Site",
                  cell: (s) => (
                    <span className="flex items-center gap-2">
                      <a href={`https://${s.host}`} target="_blank" rel="noreferrer" className="font-medium hover:text-primary">
                        {s.host}
                      </a>
                      {s.source === "manual" && <Chip tone="neutral">added</Chip>}
                    </span>
                  ),
                },
                {
                  header: "State",
                  cell: (s) =>
                    !s.last ? (
                      <Chip tone="neutral">checking</Chip>
                    ) : s.last.up ? (
                      <Chip tone="good">up · {s.last.status}</Chip>
                    ) : (
                      <Chip tone="critical" title={s.last.error}>
                        down{s.last.status ? ` · ${s.last.status}` : ""}
                      </Chip>
                    ),
                },
                { header: "Response", align: "right", cell: (s) => (s.last ? `${s.last.ms} ms` : "—") },
                { header: "Uptime 24 h", align: "right", cell: (s) => (s.uptime_24h >= 0 ? `${s.uptime_24h.toFixed(s.uptime_24h === 100 ? 0 : 2)}%` : "—") },
                {
                  header: "Certificate",
                  cell: (s) => {
                    const d = certDays(s, now);
                    return d === null ? <span className="text-muted-foreground">—</span> : <span className={d < 14 ? "font-medium text-warning" : "text-muted-foreground"}>{d} days left</span>;
                  },
                },
                { header: "Checked", cell: (s) => <span className="whitespace-nowrap text-muted-foreground">{s.last ? ago(s.last.at, now) : "—"}</span> },
                {
                  header: " ",
                  align: "right",
                  cell: (s) => (
                    <RowMenu
                      label={`Actions for ${s.host}`}
                      groups={[
                        [
                          { label: "Open site", icon: ExternalLink, onSelect: () => window.open(`https://${s.host}`, "_blank", "noopener") },
                          { label: "Its activity", icon: History, onSelect: () => navigate(`/activity?q=${encodeURIComponent(s.host)}`) },
                          { label: "Copy address", icon: Copy, onSelect: () => copy(`https://${s.host}`, "the address") },
                        ],
                        s.source === "manual"
                          ? [
                              {
                                label: "Stop checking",
                                icon: Trash2,
                                danger: true,
                                onSelect: () =>
                                  void run(() => api.removeSite(s.host), {
                                    confirm: { title: `Stop checking ${s.host}?`, description: "Its history stays for 7 days; no more alerts for it.", confirmLabel: "Stop checking" },
                                    success: `Stopped checking ${s.host}`,
                                    invalidate: [["sites"]],
                                  }),
                              },
                            ]
                          : [],
                      ]}
                    />
                  ),
                },
              ]}
            />
          </Block>

          {list.length > 0 && (
            <Section title="Response time" sub="Average time to the first full answer, from this server through Cloudflare and back.">
              <div className="grid gap-x-6 gap-y-5 sm:grid-cols-2 lg:grid-cols-3">
                {list.map((s, i) => {
                  const h = histories[i]?.data ?? [];
                  const ms = h.map((c) => c.ms);
                  return <SparkFigure key={s.host} label={s.host} values={ms} times={h.map((c) => c.at)} max={Math.max(...ms, 200)} format={(v) => `${Math.round(v)} ms`} />;
                })}
              </div>
            </Section>
          )}
        </div>
      )}
    </>
  );
}
