import { useCallback, useState } from "react";

// Storage can throw (private mode, blocked site data); every caller falls back to its default.
export function readStored(key: string): string | null {
  try {
    return localStorage.getItem(key);
  } catch {
    return null;
  }
}

export function writeStored(key: string, value: string | null) {
  try {
    if (value === null) localStorage.removeItem(key);
    else localStorage.setItem(key, value);
  } catch {
    // not persisted; the in-memory value still applies
  }
}

/** useState backed by localStorage, for per-browser conveniences (a collapsed panel, a last choice). */
export function useLocalStorage(key: string, initial: string): [string, (v: string) => void] {
  const [value, setValue] = useState(() => readStored(key) ?? initial);
  const set = useCallback(
    (v: string) => {
      setValue(v);
      writeStored(key, v);
    },
    [key],
  );
  return [value, set];
}
