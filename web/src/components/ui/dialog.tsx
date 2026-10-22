import type { ComponentProps, ReactNode } from "react";
import { AlertDialog as AlertPrimitive } from "@base-ui/react/alert-dialog";
import { Dialog as DialogPrimitive } from "@base-ui/react/dialog";
import { cn } from "@/lib/utils";

// shadcn-style dialog parts on Base UI. Dialog closes on backdrop click; AlertDialog
// (for actions that need an answer) only closes through its buttons or Escape.

const backdrop =
  "fixed inset-0 z-[90] bg-black/40 transition-opacity duration-150 data-[ending-style]:opacity-0 data-[starting-style]:opacity-0";
const popup =
  "fixed top-1/2 left-1/2 z-[91] w-[calc(100vw-2rem)] max-w-md -translate-x-1/2 -translate-y-1/2 rounded-xl border bg-card p-5 text-card-foreground shadow-lg outline-none transition-[opacity,scale] duration-150 data-[ending-style]:scale-95 data-[ending-style]:opacity-0 data-[starting-style]:scale-95 data-[starting-style]:opacity-0";

function Content({ kind, className, children }: { kind: "dialog" | "alert"; className?: string; children: ReactNode }) {
  const P = kind === "alert" ? AlertPrimitive : DialogPrimitive;
  return (
    <P.Portal>
      <P.Backdrop className={backdrop} />
      <P.Popup className={cn(popup, className)}>{children}</P.Popup>
    </P.Portal>
  );
}

export const Dialog = DialogPrimitive.Root;
export const DialogClose = DialogPrimitive.Close;
export function DialogContent(props: { className?: string; children: ReactNode }) {
  return <Content kind="dialog" {...props} />;
}

const sheet =
  "fixed inset-y-0 right-0 z-[91] flex w-full max-w-2xl flex-col border-l bg-card text-card-foreground shadow-xl outline-none transition-transform duration-200 ease-out data-[ending-style]:translate-x-full data-[starting-style]:translate-x-full";

/** A dialog that slides in from the right, full height, for a record's details. */
export function SheetContent({ className, children }: { className?: string; children: ReactNode }) {
  return (
    <DialogPrimitive.Portal>
      <DialogPrimitive.Backdrop className={backdrop} />
      <DialogPrimitive.Popup className={cn(sheet, className)}>{children}</DialogPrimitive.Popup>
    </DialogPrimitive.Portal>
  );
}

export const AlertDialog = AlertPrimitive.Root;
export const AlertDialogClose = AlertPrimitive.Close;
export function AlertDialogContent(props: { className?: string; children: ReactNode }) {
  return <Content kind="alert" {...props} />;
}

export function DialogTitle({ className, ...props }: ComponentProps<typeof DialogPrimitive.Title>) {
  return <DialogPrimitive.Title className={cn("text-base font-semibold tracking-tight", className)} {...props} />;
}

export function DialogDescription({ className, ...props }: ComponentProps<typeof DialogPrimitive.Description>) {
  return <DialogPrimitive.Description className={cn("mt-1.5 text-sm text-muted-foreground", className)} {...props} />;
}

export function DialogFooter({ className, ...props }: ComponentProps<"div">) {
  return <div className={cn("mt-5 flex flex-col-reverse gap-2 sm:flex-row sm:justify-end", className)} {...props} />;
}
