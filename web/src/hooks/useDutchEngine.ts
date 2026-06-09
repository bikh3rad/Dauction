import { useMemo, useState } from "react";
import type { DutchAuction } from "@/types";
import { useInterval } from "./useInterval";

// Client-side descending-price renderer for a Dutch auction. Prices are
// server-authoritative: we seed from the server's params (ceiling/floor/step/
// interval/openAt) and the server's currentPriceCents, then animate locally
// between reads. The buy action is always re-validated server-side.
//
// current_price(now) = max(floor, ceiling − step·⌊(now − openAt)/interval⌋)
export function useDutchEngine(auction: DutchAuction | undefined, running = true) {
  const [now, setNow] = useState(() => Date.now());
  useInterval(() => setNow(Date.now()), 100, running && !!auction);

  return useMemo(() => {
    if (!auction || !auction.openAt) {
      return {
        priceCents: auction?.currentPriceCents ?? 0,
        nextInSec: 0, atFloor: false, progress: 0, drops: 0,
        intervalSec: auction?.dropIntervalSeconds ?? 1, open: false,
      };
    }
    const openAt = Date.parse(auction.openAt);
    const interval = Math.max(1, auction.dropIntervalSeconds);
    const open = now >= openAt && auction.state === "OPEN";
    const elapsed = Math.max(0, (now - openAt) / 1000);
    const drops = open ? Math.floor(elapsed / interval) : 0;
    const priceCents = Math.max(auction.floorCents, auction.ceilingCents - auction.dropStepCents * drops);
    const atFloor = priceCents <= auction.floorCents;
    const nextInSec = !open || atFloor ? 0 : interval - (elapsed % interval);
    const span = auction.ceilingCents - auction.floorCents;
    const progress = span > 0 ? 1 - (priceCents - auction.floorCents) / span : 1;
    return { priceCents, nextInSec, atFloor, progress, drops, intervalSec: interval, open };
  }, [auction, now]);
}
