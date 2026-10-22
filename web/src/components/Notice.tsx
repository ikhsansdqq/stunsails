import type { ReactNode } from "react";
import { cn } from "@/lib/utils";
import { TimeoutError } from "@/lib/api";

export function Notice({ tone, title, children }: { tone: "red" | "amber"; title: string; children?: ReactNode }) {
  return (
    <div
      role={tone === "red" ? "alert" : "status"}
      className={cn(
        "rounded-lg px-4 py-3 text-sm",
        tone === "red" ? "border border-critical/30 bg-critical/5 text-critical" : "border border-warning/30 bg-warning/8 text-warning",
      )}
    >
      <p className="font-semibold">{title}</p>
      {children && <div className="mt-0.5">{children}</div>}
    </div>
  );
}

/** Why a load failed, in words that tell a slow connection apart from a refusal. */
export function loadFailure(error: unknown): string {
  if (error instanceof TimeoutError) return "The connection timed out. Nothing is lost; try again, or wait for a better signal.";
  if (error instanceof TypeError) return "We couldn't reach Harbor. Check your connection and try again.";
  return error instanceof Error ? error.message : "Something went wrong.";
}

/** In place of content that failed to load: what happened, and a way to ask again. */
export function LoadFailed({ what, error, onRetry }: { what: string; error: unknown; onRetry: () => void }) {
  return (
    <Notice tone="amber" title={`Couldn't load ${what}.`}>
      {loadFailure(error)}{" "}
      <button type="button" onClick={onRetry} className="font-medium underline underline-offset-4">
        Try again
      </button>
    </Notice>
  );
}
