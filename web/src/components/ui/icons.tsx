import { useSyncExternalStore } from "react";

/*
 * The animated icon set, loaded off the startup path (it pulls in motion): when the
 * browser is idle after start-up, or on the first hover, whichever comes first.
 * ./lucide.tsx draws the plain glyph until then.
 */

type Animated = typeof import("./icons-animated");
export type AnimatedName = keyof Animated;

let lib: Animated | null = null;
let loading: Promise<void> | null = null;
const listeners = new Set<() => void>();

export function loadAnimatedIcons() {
  loading ??= import("./icons-animated").then(
    (m) => {
      lib = m;
      for (const l of listeners) l();
    },
    () => {
      // Offline or a stale deploy: keep the static icons, try again later.
      loading = null;
    },
  );
  return loading;
}

if (typeof window !== "undefined") {
  const idle = window.requestIdleCallback ?? ((cb: () => void) => window.setTimeout(cb, 2000));
  idle(() => void loadAnimatedIcons(), { timeout: 5000 });
}

const subscribe = (l: () => void) => {
  listeners.add(l);
  return () => listeners.delete(l);
};
const ready = () => lib !== null;

/** The animated set once it has loaded (null before). */
export function useAnimatedIcons() {
  const loaded = useSyncExternalStore(subscribe, ready, () => false);
  return loaded ? lib : null;
}
