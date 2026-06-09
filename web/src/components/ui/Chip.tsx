import type { ReactNode } from "react";

// Maps a domain state to a chip color band. Covers both backend enums
// (UPPERCASE) and the lowercase prototype state keys.
const STATE_MAP: Record<string, string> = {
  // generic
  in_closet: "neut", IN_VAULT: "neut", proposed: "neut", DRAFT: "neut", SCHEDULED: "neut",
  appraising: "warn", APPRAISING: "warn", PENDING: "warn", pending: "warn",
  in_auction: "active", IN_AUCTION: "active", OPEN: "live", CLOSING: "warn",
  live: "live", LIVE: "live", HAMMER: "live",
  funded: "active", FUNDED: "active", HELD: "active", FULL_LOCKED: "active",
  DEPOSIT_LOCKED: "active", in_transit: "active", IN_TRANSIT: "active", delivered: "active",
  completed: "good", COMPLETED: "good", RELEASED: "good", certified: "good", CERTIFIED: "good",
  APPROVED: "good", approved: "good", PASS: "good", RESOLVED: "good",
  sold: "neut", SOLD: "neut", BOUGHT_BACK: "neut", redeemed: "neut", REDEEMED: "neut",
  disputed: "bad", DISPUTED: "bad", FORFEITED: "bad", REJECTED: "bad", flagged: "bad",
  FLAGGED: "bad", REFUNDED: "warn", ABORTED: "bad", FAIL: "bad",
  active: "good", ISSUED: "good",
};

export interface ChipProps {
  state?: string;
  label?: string;
  pulse?: boolean;
  children?: ReactNode;
}

export function Chip({ state, label, pulse, children }: ChipProps) {
  const st = (state && STATE_MAP[state]) || "neut";
  const txt = label || (state ? state.toUpperCase().replace(/ /g, "_") : children);
  return (
    <span className={"chip" + (pulse ? " live-pulse" : "")} data-st={st}>
      {pulse && <span className="dot" />}
      {txt || children}
    </span>
  );
}
