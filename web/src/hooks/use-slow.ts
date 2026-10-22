import { useEffect, useState } from "react";

/** After this long, a wait gets a word of explanation. */
export const SLOW_AFTER_MS = 3_000;
/** After this long, it also offers to try again. */
export const STUCK_AFTER_MS = 12_000;

/**
 * How long something has been waiting, in stages: "waiting" at first, then "slow",
 * then "stuck". Counts from mount, or from when `active` turns true; resets when it
 * turns false.
 */
export function useSlowStage(active = true): "waiting" | "slow" | "stuck" {
  const [stage, setStage] = useState<"waiting" | "slow" | "stuck">("waiting");
  useEffect(() => {
    if (!active) return;
    const slow = setTimeout(() => setStage("slow"), SLOW_AFTER_MS);
    const stuck = setTimeout(() => setStage("stuck"), STUCK_AFTER_MS);
    return () => {
      clearTimeout(slow);
      clearTimeout(stuck);
      setStage("waiting");
    };
  }, [active]);
  return active ? stage : "waiting";
}
