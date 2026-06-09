import type { Category, Lot } from "@/types";
import { accentOf, artOf, categoryOf, maisonOf } from "./enrich";
import { secondsUntil } from "./format";

export type LotStatus = "live" | "upcoming" | "timed";

export interface LotView {
  id: string;
  maison: string;
  /** Title with the maison prefix stripped (the prototype kept them separate). */
  body: string;
  category: Category;
  art: string;
  accent: string;
  isPassive: boolean;
  isLive: boolean;
  status: LotStatus;
  /** route segment to open this lot's detail/auction screen */
  route: string;
  opensInSec: number;
}

// The catalog Lot is minimal; presentation status is derived client-side:
//  - DUTCH whose schedule has arrived  → live
//  - DUTCH scheduled in the future      → upcoming
//  - VICKREY/UNIQBID                    → timed (passive)
export function toLotView(lot: Lot): LotView {
  const isPassive = lot.atype === "VICKREY" || lot.atype === "UNIQBID";
  const opensInSec = secondsUntil(lot.scheduledAt);
  const isLive = !isPassive && opensInSec <= 0;
  const status: LotStatus = isPassive ? "timed" : isLive ? "live" : "upcoming";
  const route = isPassive ? "passive" : isLive ? "auction" : "lot";
  return {
    id: lot.id,
    maison: maisonOf(lot.title),
    body: lot.title.split(/\s+[—–-]\s+/).slice(1).join(" — ") || lot.title,
    category: categoryOf(lot),
    art: artOf(lot),
    accent: accentOf(lot),
    isPassive,
    isLive,
    status,
    route,
    opensInSec,
  };
}
