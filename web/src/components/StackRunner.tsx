import { createContext, useCallback, useContext, useEffect, useRef, useState, type ReactNode } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { ArrowDownToLine, CircleAlert, CircleCheck, CirclePlay, CircleStop, CloudDownload, LoaderCircle, Power, RotateCw, X, type LucideIcon } from "@/components/ui/lucide";
import { ApiError, streamStack, type StackVerb } from "@/lib/api";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Dialog, DialogClose, DialogTitle, SheetContent } from "@/components/ui/dialog";
import { useConfirm, type ConfirmOptions } from "@/components/confirm";
import { useEnsureStepUp } from "@/components/guard";

export const verbIcons: Record<StackVerb, LucideIcon> = { up: ArrowDownToLine, pull: CloudDownload, restart: RotateCw, start: CirclePlay, stop: CircleStop, down: Power };

export const verbs: Record<StackVerb, { label: string; doing: string; confirm?: (p: string) => ConfirmOptions }> = {
  up: {
    label: "Up",
    doing: "Bringing up",
    confirm: (p) => ({ title: `Bring ${p} up?`, description: "Runs docker compose up -d: creates what's missing and recreates containers whose settings or image changed.", confirmLabel: "Bring up", tone: "default" }),
  },
  pull: {
    label: "Pull and update",
    doing: "Updating",
    confirm: (p) => ({ title: `Update ${p}?`, description: "Pulls newer images for every service, then runs up -d, recreating the containers that changed. They're briefly down while that happens.", confirmLabel: "Pull and update", tone: "default" }),
  },
  restart: {
    label: "Restart",
    doing: "Restarting",
    confirm: (p) => ({ title: `Restart ${p}?`, description: "Restarts every service in the stack. What it serves is briefly down.", confirmLabel: "Restart", tone: "default" }),
  },
  start: {
    label: "Start",
    doing: "Starting",
    confirm: (p) => ({ title: `Start ${p}?`, description: "Starts the stack's stopped containers as they were.", confirmLabel: "Start", tone: "default" }),
  },
  stop: {
    label: "Stop",
    doing: "Stopping",
    confirm: (p) => ({ title: `Stop ${p}?`, description: "Stops every container in the stack. They stay in place, so Start brings them back as they were.", confirmLabel: "Stop" }),
  },
  down: {
    label: "Down",
    doing: "Taking down",
    confirm: (p) => ({
      title: `Take ${p} down?`,
      description: "Runs docker compose down: stops and removes the stack's containers and networks. Named volumes and images stay, and Up brings it back.",
      confirmLabel: "Take down",
    }),
  },
};

type Run = (project: string, verb: StackVerb) => Promise<boolean>;
const RunnerContext = createContext<Run | null>(null);

export function useStackRun() {
  const run = useContext(RunnerContext);
  if (!run) throw new Error("useStackRun needs a StackRunnerProvider");
  return run;
}

/** Runs compose actions with their output in a side sheet, one at a time. */
export function StackRunnerProvider({ children }: { children: ReactNode }) {
  const [open, setOpen] = useState(false);
  const [job, setJob] = useState<{ project: string; verb: StackVerb; lines: string[]; state: "running" | "ok" | "failed"; error?: string } | null>(null);
  const confirm = useConfirm();
  const ensure = useEnsureStepUp();
  const queryClient = useQueryClient();
  const box = useRef<HTMLPreElement>(null);

  useEffect(() => {
    if (box.current) box.current.scrollTop = box.current.scrollHeight;
  }, [job?.lines.length]);

  const run = useCallback<Run>(
    async (project, verb) => {
      if (job?.state === "running") {
        setOpen(true);
        return false;
      }
      const v = verbs[verb];
      if (v.confirm && !(await confirm(v.confirm(project)))) return false;
      if (!(await ensure())) return false;
      setJob({ project, verb, lines: [], state: "running" });
      setOpen(true);
      const add = (l: string) => setJob((j) => (j ? { ...j, lines: [...j.lines.slice(-3000), l] } : j));
      try {
        await streamStack(project, verb, add);
        setJob((j) => (j ? { ...j, state: "ok" } : j));
        toast.success(`${v.label}: ${project} done`);
        return true;
      } catch (err) {
        const msg = err instanceof ApiError && err.code === "stepup_required" ? "Your authenticator code expired. Try again." : err instanceof Error ? err.message : "The action failed.";
        setJob((j) => (j ? { ...j, state: "failed", error: msg } : j));
        toast.error(`${v.label}: ${project} failed`, { description: msg });
        return false;
      } finally {
        for (const k of [["stacks"], ["stack", project], ["containers"], ["overview"]]) void queryClient.invalidateQueries({ queryKey: k });
      }
    },
    [confirm, ensure, job?.state, queryClient],
  );

  return (
    <RunnerContext.Provider value={run}>
      {children}
      <Dialog open={open} onOpenChange={setOpen}>
        <SheetContent>
          {job && (
            <>
              <div className="flex h-14 shrink-0 items-center gap-3 border-b px-5">
                {job.state === "running" ? (
                  <LoaderCircle className="size-4 animate-spin text-muted-foreground" aria-hidden />
                ) : job.state === "ok" ? (
                  <CircleCheck className="size-4 text-good" aria-hidden />
                ) : (
                  <CircleAlert className="size-4 text-critical" aria-hidden />
                )}
                <DialogTitle className="min-w-0 flex-1 truncate">
                  {verbs[job.verb].doing} {job.project}
                  {job.state === "ok" ? ": done" : job.state === "failed" ? ": failed" : "…"}
                </DialogTitle>
                <DialogClose render={<Button variant="ghost" size="icon-sm" aria-label="Close" />}>
                  <X aria-hidden />
                </DialogClose>
              </div>
              <pre ref={box} className="min-h-0 flex-1 overflow-auto bg-[#141412] p-4 font-mono text-[12px] leading-[1.55] whitespace-pre-wrap break-all text-[#e8e6df]">
                {job.lines.length ? job.lines.join("\n") : "Starting…"}
              </pre>
              <div className={cn("shrink-0 border-t px-5 py-3 text-sm", job.state === "failed" ? "text-critical" : "text-muted-foreground")}>
                {job.state === "running"
                  ? "Closing this keeps the action running on the server."
                  : job.state === "failed"
                    ? job.error
                    : "Finished. The stack's page shows the new state."}
              </div>
            </>
          )}
        </SheetContent>
      </Dialog>
    </RunnerContext.Provider>
  );
}
