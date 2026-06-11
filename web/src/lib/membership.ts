import type { Account, Tier } from "@/types";

// Membership leveling. Everyone who signs in is Member (Level 1) for free; higher
// levels are PAID upgrades that lower the buyer's premium and unlock perks. The
// tier (MEMBER/VIP) is derived from the level so the existing participation gate
// keeps working (Level >= 1 can participate; Level >= 2 maps to VIP).
export interface MembershipLevel {
  level: number;
  name: string;            // brand name (proper noun, like "VIP")
  tier: Tier;
  priceCentsYear: number;  // 0 = free
  premiumBps: number;      // buyer's premium in basis points (1200 = 12%)
  perkKeys: string[];      // i18n perk keys
  accent: string;
}

export const LEVELS: MembershipLevel[] = [
  {
    level: 1, name: "Member", tier: "MEMBER", priceCentsYear: 0, premiumBps: 1200,
    accent: "var(--line-strong)", perkKeys: ["perk_bid", "perk_vault", "perk_verified"],
  },
  {
    level: 2, name: "Gold", tier: "VIP", priceCentsYear: 29000, premiumBps: 1000,
    accent: "var(--gold)", perkKeys: ["perk_premium10", "perk_early", "perk_credit"],
  },
  {
    level: 3, name: "Platinum", tier: "VIP", priceCentsYear: 190000, premiumBps: 800,
    accent: "var(--gold-pale)", perkKeys: ["perk_premium8", "perk_priority", "perk_concierge"],
  },
];

export const MAX_LEVEL = LEVELS.length;

// levelOf resolves the current membership level (0 = guest / not signed in).
export function levelOf(account?: Account | null): number {
  if (!account || account.tier === "GUEST") return 0;
  if (account.membershipLevel && account.membershipLevel > 0) return account.membershipLevel;
  return account.tier === "VIP" ? 2 : 1; // back-compat when level isn't set
}

export function levelInfo(level: number): MembershipLevel | undefined {
  return LEVELS.find((l) => l.level === level);
}

// tierForLevel maps a level to the participation tier.
export function tierForLevel(level: number): Tier {
  if (level <= 0) return "GUEST";
  return level >= 2 ? "VIP" : "MEMBER";
}
