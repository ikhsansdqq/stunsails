import { useState } from "react";
import { readStored, writeStored } from "@/hooks/use-local-storage";
import type { Range } from "@/lib/api";

export const rangeOptions: { id: Range; label: string }[] = [
  { id: "1h", label: "1 hour" },
  { id: "24h", label: "24 hours" },
  { id: "7d", label: "7 days" },
];

/** The chart range, remembered across pages. */
export function useRange(): [Range, (r: Range) => void] {
  const [r, set] = useState<Range>(() => {
    const v = readStored("stunsails:range");
    return v === "24h" || v === "7d" ? v : "1h";
  });
  return [
    r,
    (next) => {
      writeStored("stunsails:range", next);
      set(next);
    },
  ];
}

/** How often a range's chart refetches: live hour every 10 s, rollups each minute. */
export const rangeRefetch = (r: Range) => (r === "1h" ? 10_000 : 60_000);
