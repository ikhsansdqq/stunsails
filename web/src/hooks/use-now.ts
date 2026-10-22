import { useSyncExternalStore } from "react";

// One shared clock for every elapsed timer and "x min ago": reading Date.now()
// during render makes output depend on when React happens to render.
const listeners = new Set<() => void>();
let now = Date.now();
let timer: ReturnType<typeof setInterval> | undefined;

function subscribe(listener: () => void) {
  listeners.add(listener);
  if (!timer) {
    now = Date.now();
    timer = setInterval(() => {
      now = Date.now();
      for (const l of listeners) l();
    }, 1000);
  }
  return () => {
    listeners.delete(listener);
    if (listeners.size === 0 && timer) {
      clearInterval(timer);
      timer = undefined;
    }
  };
}

/**
 * The current time in milliseconds, rounded down to resolution. A component
 * re-renders only when the rounded value changes, so a page that shows
 * "x days left" can ask for minutes and skip the per-second renders.
 */
export function useNow(resolution = 1000): number {
  return useSyncExternalStore(subscribe, () => Math.floor(now / resolution) * resolution);
}
