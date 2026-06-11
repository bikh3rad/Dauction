import { createContext, useCallback, useContext, useMemo, useState } from "react";
import type { ReactNode } from "react";
import { useAccount } from "./queries";
import { levelOf } from "@/lib/membership";
import type { Account, Tier } from "@/types";

// Per-lot participation is not exposed by a backend read model, so the client
// tracks it for the session (mirrors the prototype's `parts` map).
type Participation = "REQUESTED" | "LOCKED_FULL" | "WON";

interface SessionValue {
  account?: Account;
  tier: Tier;
  /** Paid membership level: 0 = guest, 1 = free Member, 2+ = purchased. */
  level: number;
  /** MEMBER/VIP AND kyc APPROVED — the participation gate (root CLAUDE.md §1). */
  canParticipate: boolean;
  isGuest: boolean;
  loading: boolean;
  parts: Record<string, Participation>;
  setPart: (lotId: string, state: Participation) => void;
}

const SessionContext = createContext<SessionValue | null>(null);

export function SessionProvider({ children }: { children: ReactNode }) {
  const { data: account, isLoading } = useAccount();
  const [parts, setParts] = useState<Record<string, Participation>>({});

  const setPart = useCallback((lotId: string, state: Participation) => {
    setParts((p) => ({ ...p, [lotId]: state }));
  }, []);

  const value = useMemo<SessionValue>(() => {
    const tier: Tier = account?.tier ?? "GUEST";
    return {
      account,
      tier,
      level: levelOf(account),
      isGuest: tier === "GUEST",
      canParticipate: (tier === "MEMBER" || tier === "VIP") && account?.kycStatus === "APPROVED",
      loading: isLoading,
      parts,
      setPart,
    };
  }, [account, isLoading, parts, setPart]);

  return <SessionContext.Provider value={value}>{children}</SessionContext.Provider>;
}

export function useSession(): SessionValue {
  const ctx = useContext(SessionContext);
  if (!ctx) throw new Error("useSession must be used within <SessionProvider>");
  return ctx;
}
