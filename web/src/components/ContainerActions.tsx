import { useLocation } from "wouter";
import { toast } from "sonner";
import { CirclePause, CirclePlay, CircleStop, Copy, FileSearch, Gauge, History, RotateCw, ScrollText, Trash2, Zap, type LucideIcon } from "@/components/ui/lucide";
import { api, ApiError, type ContainerAction, type ContainerState } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { RowMenu } from "@/components/ui/row-menu";
import { useRun } from "@/components/guard";
import type { ConfirmOptions } from "@/components/confirm";

interface Item {
  action: ContainerAction;
  label: string;
  icon: LucideIcon;
  done: string;
  danger?: boolean;
  confirm?: (name: string) => ConfirmOptions;
}

export const items: Record<ContainerAction, Item> = {
  start: { action: "start", label: "Start", icon: CirclePlay, done: "Started" },
  stop: {
    action: "stop",
    label: "Stop",
    icon: CircleStop,
    done: "Stopped",
    confirm: (n) => ({ title: `Stop ${n}?`, description: "It gets 10 seconds to shut down cleanly before Docker kills it. Whatever it serves goes offline.", confirmLabel: "Stop" }),
  },
  restart: {
    action: "restart",
    label: "Restart",
    icon: RotateCw,
    done: "Restarted",
    confirm: (n) => ({ title: `Restart ${n}?`, description: "It stops (10 seconds to shut down cleanly) and starts again. Anything it serves is briefly down.", confirmLabel: "Restart", tone: "default" }),
  },
  pause: {
    action: "pause",
    label: "Pause",
    icon: CirclePause,
    done: "Paused",
    confirm: (n) => ({ title: `Pause ${n}?`, description: "Its processes freeze until you resume it; requests to it hang meanwhile.", confirmLabel: "Pause", tone: "default" }),
  },
  unpause: { action: "unpause", label: "Resume", icon: CirclePlay, done: "Resumed" },
  kill: {
    action: "kill",
    label: "Kill",
    icon: Zap,
    done: "Killed",
    danger: true,
    confirm: (n) => ({ title: `Kill ${n}?`, description: "Sends SIGKILL right away, with no time to save anything. Use Stop unless it's stuck.", confirmLabel: "Kill" }),
  },
  remove: {
    action: "remove",
    label: "Remove",
    icon: Trash2,
    done: "Removed",
    danger: true,
    confirm: (n) => ({
      title: `Remove ${n}?`,
      description: "The container is stopped if needed and deleted, with anything written inside it. Named volumes stay. A compose stack recreates it on its next up.",
      confirmLabel: "Remove",
    }),
  },
};

/** What can be done to a container in its current state, most likely first. */
export function actionsFor(state: ContainerState): ContainerAction[] {
  switch (state) {
    case "running":
      return ["restart", "stop", "pause", "kill", "remove"];
    case "paused":
      return ["unpause", "stop", "kill", "remove"];
    case "restarting":
      return ["stop", "kill", "remove"];
    default:
      return ["start", "remove"];
  }
}

const invalidate = (id?: string) => [["containers"], ...(id ? [["container", id]] : []), ["overview"], ["stacks"], ["storage"]];

export function useContainerAction() {
  const run = useRun();
  return (id: string, name: string, action: ContainerAction) => {
    const it = items[action];
    return run(() => api.containerAction(id, action), {
      confirm: it.confirm?.(name),
      success: `${it.done} ${name}`,
      invalidate: invalidate(id),
    });
  };
}

/** One action on many containers: one question, one code, then each in turn, and a summary. */
export function useBulkContainerAction() {
  const run = useRun();
  return async (targets: { id: string; name: string }[], action: ContainerAction) => {
    const it = items[action];
    const names = targets.map((t) => t.name);
    const list = names.length > 4 ? `${names.slice(0, 4).join(", ")} and ${names.length - 4} more` : names.join(", ");
    const r = await run(
      async () => {
        let ok = 0;
        const failed: string[] = [];
        for (const t of targets) {
          try {
            await api.containerAction(t.id, action);
            ok++;
          } catch (err) {
            // Changes needing a fresh code stop the batch so useRun can ask and retry it.
            if (err instanceof ApiError && err.code === "stepup_required" && ok === 0) throw err;
            failed.push(t.name);
          }
        }
        return { ok, failed };
      },
      {
        confirm: {
          title: `${it.label} ${targets.length} containers?`,
          description: `${list}. ${it.confirm?.("each").description ?? ""}`.trim(),
          confirmLabel: `${it.label} ${targets.length}`,
          tone: it.danger || action === "stop" ? "destructive" : "default",
        },
        invalidate: invalidate(),
      },
    );
    if (r) {
      if (r.failed.length) toast.warning(`${it.done} ${r.ok}, ${r.failed.length} failed`, { description: r.failed.join(", ") });
      else toast.success(`${it.done} ${r.ok} containers`);
    }
    return r;
  };
}

export function copy(text: string, what: string) {
  navigator.clipboard.writeText(text).then(
    () => toast.success(`Copied ${what}`),
    () => toast.error("Couldn't copy: the browser blocked the clipboard"),
  );
}

/** The row menu in tables: look first, then lifecycle, then destructive. */
export function ContainerMenu({ id, name, state }: { id: string; name: string; state: ContainerState }) {
  const act = useContainerAction();
  const [, navigate] = useLocation();
  const list = actionsFor(state);
  const life = list.filter((a) => !items[a].danger);
  const danger = list.filter((a) => items[a].danger);
  return (
    <RowMenu
      label={`Actions for ${name}`}
      groups={[
        [
          { label: "Open details", icon: FileSearch, onSelect: () => navigate(`/containers/${id}`) },
          { label: "Logs", icon: ScrollText, onSelect: () => navigate(`/containers/${id}?tab=logs`) },
          { label: "Resources", icon: Gauge, onSelect: () => navigate(`/containers/${id}?tab=resources`) },
          { label: "Events", icon: History, onSelect: () => navigate(`/containers/${id}?tab=events`) },
          { label: "Copy ID", icon: Copy, onSelect: () => copy(id, "the container ID") },
        ],
        life.map((a) => ({ label: items[a].label, icon: items[a].icon, onSelect: () => void act(id, name, a) })),
        danger.map((a) => ({ label: items[a].label, icon: items[a].icon, danger: true, onSelect: () => void act(id, name, a) })),
      ]}
    />
  );
}

/** The detail page's buttons: the main actions out front. */
export function ContainerButtons({ id, name, state, onRemoved }: { id: string; name: string; state: ContainerState; onRemoved: () => void }) {
  const act = useContainerAction();
  return (
    <>
      {actionsFor(state).map((a) => {
        const it = items[a];
        return (
          <Button
            key={a}
            size="sm"
            variant={it.danger ? "destructive" : a === "start" || a === "unpause" ? "default" : "outline"}
            icon={it.icon}
            onClick={async () => {
              const r = await act(id, name, a);
              if (r && a === "remove") onRemoved();
            }}
          >
            {it.label}
          </Button>
        );
      })}
    </>
  );
}
