import { useState, type FormEvent } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { CornerDownLeft, Eye, EyeOff, LoaderCircle } from "@/components/ui/lucide";
import { OtpInput } from "@/components/ui/otp-input";
import { api } from "@/lib/api";
import { HarborMark } from "@/components/Layout";
import { useNow } from "@/hooks/use-now";
import "./login.css";

// Sign-in as a console session: one centred prompt that takes the password, then
// the code from Google Authenticator. It follows the system's light or dark theme
// with Harbor's own tokens; the status bar along the foot is the only ornament.

export function Login({ configured }: { configured: boolean }) {
  const queryClient = useQueryClient();
  const [step, setStep] = useState<"password" | "code">("password");
  const [password, setPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [caps, setCaps] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (busy || step !== "password" || !password) return;
    setBusy(true);
    setError("");
    try {
      await api.login(password);
      setPassword("");
      setStep("code");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not sign in. Try again.");
    }
    setBusy(false);
  }

  function backToPassword() {
    setStep("password");
    setError("");
  }

  return (
    <div className="tl">
      <main className="tl-main">
        <div className="tl-col">
          <p className="tl-brand">
            <HarborMark className="tl-mark" />
            <span>Harbor</span>
            <span className="tl-host">{location.hostname}</span>
          </p>

          {step === "password" ? (
            <form onSubmit={submit} className="tl-step" noValidate>
              <h1 className="tl-prompt">
                <span className="tl-sigil" aria-hidden>
                  $
                </span>{" "}
                sign in<span className="sr-only"> to Harbor</span>
                <span className="tl-cursor" aria-hidden />
              </h1>
              <p className="tl-help">Enter your Harbor password. Your authenticator code comes next.</p>

              {!configured && (
                <p className="tl-error" role="alert">
                  Harbor isn't set up yet. On the server run <code>sudo server-monitor passwd</code> and <code>sudo server-monitor enroll</code>, then restart it.
                </p>
              )}

              <div className="tl-field" data-invalid={Boolean(error) || undefined}>
                <span className="tl-chevron" aria-hidden>
                  ›
                </span>
                <input
                  id="login-password"
                  type={showPassword ? "text" : "password"}
                  autoComplete="current-password"
                  placeholder="password"
                  aria-label="Password"
                  autoFocus
                  required
                  spellCheck={false}
                  value={password}
                  onChange={(e) => {
                    setPassword(e.target.value);
                    if (error) setError("");
                  }}
                  disabled={busy || !configured}
                  onKeyDown={(e) => setCaps(e.getModifierState("CapsLock"))}
                  onKeyUp={(e) => setCaps(e.getModifierState("CapsLock"))}
                  onBlur={() => setCaps(false)}
                  aria-invalid={Boolean(error) || undefined}
                  aria-describedby={[error && "login-error", caps && "login-caps"].filter(Boolean).join(" ") || undefined}
                />
                <button type="button" className="tl-icon" onClick={() => setShowPassword((v) => !v)} aria-label={showPassword ? "Hide password" : "Show password"} aria-pressed={showPassword}>
                  {showPassword ? <EyeOff className="size-4" aria-hidden /> : <Eye className="size-4" aria-hidden />}
                </button>
                <button type="submit" className="tl-enter" disabled={busy || !configured || !password} aria-label="Continue">
                  {busy ? <LoaderCircle className="size-4 animate-spin" aria-hidden /> : <CornerDownLeft className="size-4" aria-hidden />}
                </button>
              </div>

              <p className="tl-line" aria-live="polite">
                {error ? (
                  <span id="login-error" role="alert" className="tl-error-text">
                    {error}
                  </span>
                ) : busy ? (
                  "Checking…"
                ) : caps ? (
                  <span id="login-caps">Caps Lock is on.</span>
                ) : (
                  "Press Enter to continue."
                )}
              </p>
            </form>
          ) : (
            <div className="tl-step">
              <h1 className="tl-prompt">
                <span className="tl-sigil" aria-hidden>
                  $
                </span>{" "}
                verify authenticator code
              </h1>
              <p className="tl-help">Enter the 6-digit code Google Authenticator shows for Harbor.</p>
              <div className="tl-otp">
                <OtpInput
                  id="login-code"
                  size="large"
                  verify={async (c) => {
                    try {
                      await api.loginCode(c);
                      return null;
                    } catch (err) {
                      const msg = err instanceof Error ? err.message : "Could not sign in. Try again.";
                      // The password step expired: back to it.
                      if (/password/i.test(msg)) {
                        setStep("password");
                        setError(msg);
                      }
                      return msg;
                    }
                  }}
                  onVerified={() => {
                    toast.success("Signed in");
                    setTimeout(() => void queryClient.invalidateQueries({ queryKey: ["session"] }), 300);
                  }}
                />
              </div>
              <p className="tl-help">
                Lost the phone? On the server run <code>sudo server-monitor enroll</code> to pair a new one.{" "}
                <button type="button" className="tl-link" onClick={backToPassword}>
                  Back to password
                </button>
              </p>
            </div>
          )}
        </div>
      </main>
      <StatusBar step={step} />
    </div>
  );
}

/** A tmux-like status line: where you are, how sign-in works, the time here. */
function StatusBar({ step }: { step: "password" | "code" }) {
  const now = useNow(15_000);
  const time = new Date(now).toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit", timeZoneName: "short" });
  return (
    <footer className="tl-status">
      <span className="tl-band" aria-hidden>
        <i />
        <i />
        <i />
        <i />
        <i />
      </span>
      <span className="tl-seg tl-seg-strong">harbor</span>
      <span className="tl-seg">{step === "password" ? "1/2 password" : "2/2 authenticator"}</span>
      <span className="tl-seg tl-wide">{location.protocol === "https:" ? "encrypted connection" : "local connection"}</span>
      <span className="tl-seg tl-time num">{time}</span>
    </footer>
  );
}
