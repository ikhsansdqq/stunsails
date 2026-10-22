import { createContext, useCallback, useContext, useRef, useState, type ReactNode } from "react";
import { TriangleAlert } from "@/components/ui/lucide";
import { Button } from "@/components/ui/button";
import { AlertDialog, AlertDialogContent, DialogDescription, DialogFooter, DialogTitle } from "@/components/ui/dialog";
import { cn } from "@/lib/utils";

export interface ConfirmOptions {
  title: string;
  description?: ReactNode;
  confirmLabel?: string;
  cancelLabel?: string;
  /** destructive (the default) colours the action red; use default for reversible actions. */
  tone?: "destructive" | "default";
}

type Confirm = (options: ConfirmOptions) => Promise<boolean>;

const ConfirmContext = createContext<Confirm | null>(null);

/** Asks a yes/no question in the app's own dialog: `if (await confirm({ title })) …`. */
export function useConfirm() {
  const confirm = useContext(ConfirmContext);
  if (!confirm) throw new Error("useConfirm needs a ConfirmProvider");
  return confirm;
}

/** One shared dialog for the whole app; each confirm() call resolves when it closes. */
export function ConfirmProvider({ children }: { children: ReactNode }) {
  const [open, setOpen] = useState(false);
  const [options, setOptions] = useState<ConfirmOptions>({ title: "" });
  const resolver = useRef<((ok: boolean) => void) | null>(null);

  const settle = (ok: boolean) => {
    resolver.current?.(ok);
    resolver.current = null;
    setOpen(false);
  };

  const confirm = useCallback<Confirm>((next) => {
    resolver.current?.(false);
    setOptions(next);
    setOpen(true);
    return new Promise<boolean>((resolve) => {
      resolver.current = resolve;
    });
  }, []);

  const destructive = (options.tone ?? "destructive") === "destructive";
  return (
    <ConfirmContext.Provider value={confirm}>
      {children}
      <AlertDialog open={open} onOpenChange={(next) => !next && settle(false)}>
        <AlertDialogContent>
          <div className="flex gap-3.5">
            {destructive && (
              <span className="grid size-9 shrink-0 place-items-center rounded-full bg-critical/10 text-critical" aria-hidden>
                <TriangleAlert className="size-[18px]" />
              </span>
            )}
            <div className="min-w-0 pt-1">
              <DialogTitle>{options.title}</DialogTitle>
              {options.description && <DialogDescription>{options.description}</DialogDescription>}
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => settle(false)}>
              {options.cancelLabel ?? "Cancel"}
            </Button>
            <Button autoFocus className={cn(destructive && "border-transparent bg-critical text-on-tone hover:bg-critical/90")} onClick={() => settle(true)}>
              {options.confirmLabel ?? "Confirm"}
            </Button>
          </DialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </ConfirmContext.Provider>
  );
}
