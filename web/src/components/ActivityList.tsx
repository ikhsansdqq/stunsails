import { Link } from "wouter";
import { Bell, Box, Globe, HardDrive, Image as ImageIcon, KeyRound, Network, Server, ShieldAlert, Terminal, Wrench, type LucideIcon } from "@/components/ui/lucide";
import type { Activity } from "@/lib/api";
import { cn } from "@/lib/utils";
import { ago, formatFullDateTime } from "@/lib/format";
import { useNow } from "@/hooks/use-now";

/** The icon for each activity type; alerts use Bell. */
export const icons: Record<string, LucideIcon> = {
  container: Box,
  image: ImageIcon,
  volume: HardDrive,
  network: Network,
  login: KeyRound,
  action: Wrench,
  ssh: Terminal,
  sudo: ShieldAlert,
  unit: Server,
  site: Globe,
};

/** Docker's verbs, as people say them. */
const verbs: Record<string, string> = {
  create: "created",
  start: "started",
  stop: "stopped",
  die: "exited",
  kill: "was sent a signal",
  oom: "ran out of memory",
  restart: "restarted",
  destroy: "removed",
  pause: "paused",
  unpause: "resumed",
  rename: "renamed",
  update: "updated",
  pull: "pulled",
  delete: "deleted",
  tag: "tagged",
  untag: "untagged",
  import: "imported",
  load: "loaded",
  save: "saved",
};

export function describe(a: Activity): string {
  if (a.kind === "alert") return a.ok ? `Alert sent: ${a.name}` : `Alert not delivered: ${a.name}`;
  if (a.kind === "host") {
    if (a.type === "ssh") return a.action === "signed in" ? `${a.name} signed in over SSH` : a.name;
    if (a.type === "sudo") return `${a.name} ran sudo`;
    if (a.type === "unit") return `Service ${a.name} ${a.action}`;
    if (a.type === "site") return `${a.name} is ${a.action}`;
  }
  if (a.kind === "audit") {
    if (a.type === "login") return a.ok ? `Sign-in: ${a.action}` : `Failed ${a.action}`;
    return `${a.action.charAt(0).toUpperCase()}${a.action.slice(1)}${a.name ? ` ${a.name}` : ""}`;
  }
  if (a.action.startsWith("health_status")) return `${a.name} is ${a.action.split(":")[1]?.trim() ?? "checked"}`;
  return `${a.type === "container" ? "" : a.type.charAt(0).toUpperCase() + a.type.slice(1) + " "}${a.name} ${verbs[a.action] ?? a.action}`.trim();
}

/** The activity feed: Docker's events and what people did here, newest first. */
export function ActivityList({ items, compact = false, empty = "Nothing has happened yet." }: { items: Activity[]; compact?: boolean; empty?: string }) {
  const now = useNow(30_000);
  if (items.length === 0) return <p className="rounded-md border border-dashed px-4 py-6 text-center text-sm text-muted-foreground">{empty}</p>;
  return (
    <ol className="divide-y rounded-md border">
      {items.map((a) => {
        const Icon = a.kind === "alert" ? Bell : (icons[a.type] ?? Box);
        const linkable = a.kind === "docker" && a.type === "container" && a.action !== "destroy";
        const text = describe(a);
        return (
          <li key={a.id} className="flex min-h-10 items-start gap-3 px-3 py-2 text-sm">
            <span className={cn("mt-0.5 grid size-6 shrink-0 place-items-center rounded-md", a.ok ? "bg-muted text-muted-foreground" : "bg-critical/10 text-critical")} aria-hidden>
              <Icon size={14} />
            </span>
            <div className="min-w-0 flex-1">
              <p className={cn("break-words", !a.ok && "text-critical")}>
                {linkable ? (
                  <Link href={`/containers/${a.target}`} className="hover:text-primary hover:underline">
                    {text}
                  </Link>
                ) : (
                  text
                )}
              </p>
              {!compact && (a.detail || a.ip) && (
                <p className="truncate text-xs text-muted-foreground">
                  {[a.detail, a.ip && `from ${a.ip}`].filter(Boolean).join(" · ")}
                </p>
              )}
            </div>
            <time className="num shrink-0 pt-0.5 text-xs whitespace-nowrap text-muted-foreground" dateTime={new Date(a.at * 1000).toISOString()} title={formatFullDateTime(a.at * 1000)}>
              {ago(a.at, now)}
            </time>
          </li>
        );
      })}
    </ol>
  );
}
