import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import { readStored, writeStored } from "@/hooks/use-local-storage";

// After ems-dashboard's use-theme: light, dark or follow the system; the choice lives in localStorage.
// public/theme-init.js applies it before first paint.
type Theme = "light" | "dark" | "system";
interface Ctx {
  theme: Theme;
  resolved: "light" | "dark";
  setTheme: (t: Theme) => void;
}
const ThemeCtx = createContext<Ctx>({ theme: "system", resolved: "light", setTheme: () => {} });
const KEY = "stunsails-theme";

function read(): Theme {
  return (readStored(KEY) as Theme | null) || "system";
}

/** Runs a colour swap with transitions off, or every transition fires at once and the switch smears. */
function swap(change: () => void) {
  const off = document.createElement("style");
  off.append("*,*::before,*::after{transition:none!important}");
  document.head.append(off);
  change();
  void document.body.offsetHeight; // commit the new styles while transitions are off
  requestAnimationFrame(() => requestAnimationFrame(() => off.remove()));
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setTheme] = useState<Theme>(read);
  const [systemDark, setSystemDark] = useState(() => window.matchMedia("(prefers-color-scheme: dark)").matches);
  useEffect(() => {
    const mq = window.matchMedia("(prefers-color-scheme: dark)");
    const on = (e: MediaQueryListEvent) => setSystemDark(e.matches);
    mq.addEventListener("change", on);
    return () => mq.removeEventListener("change", on);
  }, []);
  const resolved = theme === "system" ? (systemDark ? "dark" : "light") : theme;
  useEffect(() => {
    const root = document.documentElement;
    if (root.classList.contains("dark") !== (resolved === "dark")) swap(() => root.classList.toggle("dark", resolved === "dark"));
    writeStored(KEY, theme);
  }, [resolved, theme]);
  const value = useMemo<Ctx>(() => ({ theme, resolved, setTheme }), [theme, resolved]);
  return <ThemeCtx.Provider value={value}>{children}</ThemeCtx.Provider>;
}

export function useTheme() {
  return useContext(ThemeCtx);
}
