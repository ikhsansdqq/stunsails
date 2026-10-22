import { Chip, type ChipTone } from "@/components/ui/chip";
import type { Container, StackState } from "@/lib/api";

const ctrTone: Record<string, ChipTone> = {
  running: "good",
  restarting: "warning",
  paused: "warning",
  created: "neutral",
  exited: "neutral",
  dead: "critical",
  removing: "warning",
};

/** A container's state; a crash (non-zero exit) reads as critical. */
export function StateChip({ c }: { c: Pick<Container, "state" | "status"> }) {
  const crashed = c.state === "exited" && /Exited \((?!0\))\d+\)/.test(c.status);
  return (
    <Chip tone={crashed ? "critical" : (ctrTone[c.state] ?? "neutral")} title={c.status}>
      {crashed ? "crashed" : c.state}
    </Chip>
  );
}

export function HealthChip({ health }: { health: Container["health"] }) {
  if (!health) return null;
  const t: ChipTone = health === "healthy" ? "good" : health === "unhealthy" ? "critical" : "warning";
  return <Chip tone={t}>{health}</Chip>;
}

const stackTone: Record<StackState, ChipTone> = { running: "good", partial: "warning", stopped: "neutral", down: "neutral" };

export function StackChip({ state }: { state: StackState }) {
  return <Chip tone={stackTone[state]}>{state}</Chip>;
}

export function Ports({ ports }: { ports: Container["ports"] }) {
  const published = ports.filter((p) => p.public);
  if (!published.length) return <span className="text-muted-foreground">—</span>;
  return (
    <span className="num flex flex-col text-[0.8125rem] leading-tight whitespace-nowrap">
      {published.map((p) => (
        <span key={`${p.ip}-${p.public}-${p.private}`} title={`${p.ip || "all addresses"}:${p.public} → ${p.private}/${p.type}`}>
          {p.public}→{p.private}
          {p.ip === "127.0.0.1" || p.ip === "::1" ? <span className="text-muted-foreground"> local</span> : p.ip ? <span className="text-muted-foreground"> {p.ip}</span> : null}
        </span>
      ))}
    </span>
  );
}
