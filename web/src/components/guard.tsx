import { createContext, useCallback, useContext, useRef, useState, type ReactNode } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { KeyRound } from "@/components/ui/lucide";
import { OtpInput } from "@/components/ui/otp-input";
import { api, ApiError } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { AlertDialog, AlertDialogContent, DialogDescription, DialogFooter, DialogTitle } from "@/components/ui/dialog";
import { useConfirm, type ConfirmOptions } from "@/components/confirm";

type AskCode = () => Promise<boolean>;
const StepUpContext = createContext<AskCode | null>(null);

/**
 * Changes need an authenticator code from the last five minutes. When the server
 * answers "stepup_required", this asks for one and the change is retried. The code
 * boxes check themselves on the sixth digit, so there is no Confirm button.
 */
export function StepUpProvider({ children }: { children: ReactNode }) {
  const [open, setOpen] = useState(false);
  const [round, setRound] = useState(0);
  const resolver = useRef<((ok: boolean) => void) | null>(null);
  const queryClient = useQueryClient();

  const settle = (ok: boolean) => {
    resolver.current?.(ok);
    resolver.current = null;
    setOpen(false);
  };

  const ask = useCallback<AskCode>(() => {
    resolver.current?.(false);
    setRound((r) => r + 1);
    setOpen(true);
    return new Promise<boolean>((resolve) => {
      resolver.current = resolve;
    });
  }, []);

  return (
    <StepUpContext.Provider value={ask}>
      {children}
      <AlertDialog open={open} onOpenChange={(next) => !next && settle(false)}>
        <AlertDialogContent>
          <div className="flex gap-3.5">
            <span className="grid size-9 shrink-0 place-items-center rounded-full bg-muted text-foreground" aria-hidden>
              <KeyRound className="size-[18px]" />
            </span>
            <div className="min-w-0 flex-1 pt-1">
              <DialogTitle>Confirm with your authenticator</DialogTitle>
              <DialogDescription id="stepup-desc">Enter the 6-digit code from Google Authenticator. It unlocks changes for five minutes.</DialogDescription>
              <div className="mt-4">
                <OtpInput
                  key={round}
                  id="stepup-code"
                  aria-describedby="stepup-desc"
                  verify={async (code) => {
                    try {
                      await api.stepUp(code);
                      return null;
                    } catch (err) {
                      return err instanceof Error ? err.message : "That didn't work.";
                    }
                  }}
                  onVerified={() => {
                    void queryClient.invalidateQueries({ queryKey: ["session"] });
                    toast.success("Changes unlocked for 5 minutes");
                    setTimeout(() => settle(true), 350);
                  }}
                />
              </div>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => settle(false)}>
              Cancel
            </Button>
          </DialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </StepUpContext.Provider>
  );
}

export interface RunOptions<T> {
  confirm?: ConfirmOptions;
  /** Toast on success; a function gets the result. */
  success?: string | ((r: T) => string);
  /** Query keys to refetch afterwards. */
  invalidate?: unknown[][];
}

/**
 * Runs a change the way every StunSails button does: confirm when asked, prompt for a
 * fresh code if the server wants one, toast the outcome, refetch what it touched.
 * Resolves to the result, or undefined if cancelled or failed.
 */
export function useRun() {
  const ask = useContext(StepUpContext);
  const confirm = useConfirm();
  const queryClient = useQueryClient();
  if (!ask) throw new Error("useRun needs a StepUpProvider");
  return useCallback(
    async <T,>(fn: () => Promise<T>, opts: RunOptions<T> = {}): Promise<T | undefined> => {
      if (opts.confirm && !(await confirm(opts.confirm))) return undefined;
      for (let attempt = 0; ; attempt++) {
        try {
          const r = await fn();
          if (opts.success) toast.success(typeof opts.success === "function" ? opts.success(r) : opts.success);
          return r;
        } catch (err) {
          if (err instanceof ApiError && err.code === "stepup_required" && attempt === 0) {
            if (await ask()) continue;
            return undefined;
          }
          toast.error(err instanceof Error ? err.message : "That didn't work.");
          return undefined;
        } finally {
          for (const key of opts.invalidate ?? []) void queryClient.invalidateQueries({ queryKey: key });
        }
      }
    },
    [ask, confirm, queryClient],
  );
}

/** Make sure changes are unlocked before something that streams (compose runs). */
export function useEnsureStepUp() {
  const ask = useContext(StepUpContext);
  const queryClient = useQueryClient();
  return useCallback(async () => {
    const s = await queryClient.fetchQuery({ queryKey: ["session"], queryFn: api.session, staleTime: 0 });
    if (s.stepup_until && s.now && s.stepup_until - s.now > 15) return true;
    return ask ? ask() : false;
  }, [ask, queryClient]);
}
