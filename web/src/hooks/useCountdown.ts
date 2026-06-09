import { useState } from "react";
import { breakdown, secondsUntil } from "@/lib/format";
import { useInterval } from "./useInterval";

// Counts down to an ISO deadline, ticking once a second. Server time is truth;
// this only renders the remaining interval between fetches.
export function useCountdown(closesAt: string | undefined, on = true) {
  const [, setNow] = useState(Date.now());
  useInterval(() => setNow(Date.now()), 1000, on);
  return breakdown(secondsUntil(closesAt));
}
