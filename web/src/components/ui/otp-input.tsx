import { useEffect, useRef, useState, type ReactNode } from "react";
import { OTPInput, REGEXP_ONLY_DIGITS, type SlotProps } from "input-otp";
import { CircleCheck, LoaderCircle, Minus } from "@/components/ui/lucide";
import { cn } from "@/lib/utils";

type State = "idle" | "checking" | "wrong" | "right";

/**
 * Six digit boxes for an authenticator code, built like shadcn's InputOTP: two
 * groups of three joined slots with a dash between, the active slot ringed.
 * It checks the code by itself on the sixth digit: a wrong code turns the boxes
 * red, shakes them, clears them and keeps the focus for the next try; a right one
 * turns them green. `verify` resolves to an error message, or null when right.
 */
export function OtpInput({
  verify,
  onVerified,
  id,
  autoFocus = true,
  size = "default",
  "aria-describedby": describedBy,
}: {
  verify: (code: string) => Promise<string | null>;
  onVerified?: () => void;
  id?: string;
  autoFocus?: boolean;
  size?: "default" | "large";
  "aria-describedby"?: string;
}) {
  const [value, setValue] = useState("");
  const [state, setState] = useState<State>("idle");
  const [error, setError] = useState("");
  const input = useRef<HTMLInputElement>(null);
  const errId = (id ?? "otp") + "-error";

  useEffect(() => {
    if (state === "wrong") {
      const t = setTimeout(() => {
        setValue("");
        setState("idle");
        input.current?.focus();
      }, 650);
      return () => clearTimeout(t);
    }
  }, [state]);

  async function complete(code: string) {
    setState("checking");
    setError("");
    const err = await verify(code).catch((e: unknown) => (e instanceof Error ? e.message : "That didn't work."));
    if (err) {
      setError(err);
      setState("wrong");
      return;
    }
    setState("right");
    onVerified?.();
  }

  const large = size === "large";
  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-center gap-3">
        <OTPInput
          ref={input}
          id={id}
          maxLength={6}
          pattern={REGEXP_ONLY_DIGITS}
          value={value}
          onChange={(v) => {
            setValue(v);
            if (state === "wrong") setState("idle");
          }}
          onComplete={(v: string) => void complete(v)}
          disabled={state === "checking" || state === "right"}
          autoFocus={autoFocus}
          inputMode="numeric"
          autoComplete="one-time-code"
          placeholder="······"
          pasteTransformer={(p) => p.replace(/\D/g, "")}
          noScriptCSSFallback={null}
          aria-label="6-digit authenticator code"
          aria-invalid={state === "wrong" || undefined}
          aria-describedby={[error && errId, describedBy].filter(Boolean).join(" ") || undefined}
          containerClassName={cn("flex items-center gap-2 has-disabled:opacity-100", state === "wrong" && "motion-safe:animate-[otp-shake_0.4s_ease-in-out]")}
          render={({ slots }) => (
            <>
              <Group>
                {slots.slice(0, 3).map((s, i) => (
                  <Slot key={i} {...s} state={state} large={large} />
                ))}
              </Group>
              <div role="separator" className="text-muted-foreground">
                <Minus className="size-4" aria-hidden />
              </div>
              <Group>
                {slots.slice(3).map((s, i) => (
                  <Slot key={i} {...s} state={state} large={large} />
                ))}
              </Group>
            </>
          )}
        />
        {state === "checking" && <LoaderCircle className="size-5 animate-spin text-muted-foreground" aria-label="Checking" />}
        {state === "right" && <CircleCheck className="size-5 text-good" aria-label="Code accepted" />}
      </div>
      <p id={errId} role={error ? "alert" : undefined} className={cn("min-h-5 text-sm", error ? "text-critical" : "text-muted-foreground")}>
        {error || (state === "checking" ? "Checking…" : "It checks as soon as the sixth digit is in.")}
      </p>
    </div>
  );
}

function Group({ children }: { children: ReactNode }) {
  return <div className="flex items-center">{children}</div>;
}

function Slot({ char, placeholderChar, hasFakeCaret, isActive, state, large }: SlotProps & { state: State; large: boolean }) {
  return (
    <div
      data-active={(isActive && state === "idle") || undefined}
      className={cn(
        "relative flex items-center justify-center border-y border-r border-input bg-card font-mono shadow-xs transition-[border-color,box-shadow,background-color,color] duration-150 outline-none first:rounded-l-md first:border-l last:rounded-r-md dark:bg-input/30",
        large ? "size-12 text-xl" : "size-10 text-base",
        "data-active:z-10 data-active:border-ring data-active:ring-[3px] data-active:ring-ring/50",
        state === "wrong" && "border-critical bg-critical/5 text-critical",
        state === "right" && "border-good bg-good/8 text-good",
      )}
    >
      {char ?? <span className="text-muted-foreground/40">{placeholderChar ?? "·"}</span>}
      {hasFakeCaret && (
        <span className="pointer-events-none absolute inset-0 flex items-center justify-center" aria-hidden>
          <span className={cn("w-px bg-foreground motion-safe:animate-[caret-blink_1s_ease-out_infinite]", large ? "h-5" : "h-4")} />
        </span>
      )}
    </div>
  );
}
