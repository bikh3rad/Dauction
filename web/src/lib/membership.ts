import { getSettings } from "@/mock/settings";
import type { Account, Tier } from "@/types";

// Membership leveling. Everyone who signs in is Member (Level 1) for free; higher
// levels are PAID upgrades that lower the buyer's premium and unlock perks. The
// tier (MEMBER/VIP) is derived from the level so the existing participation gate
// keeps working (Level >= 1 can participate; Level >= 2 maps to VIP). The level
// list is editable in the admin panel (see mock/settings).
export interface MembershipLevel {
  level: number;
  name: string;            // brand name (proper noun, like "VIP")
  tier: Tier;
  priceCentsYear: number;  // 0 = free
  premiumBps: number;      // buyer's premium in basis points (1200 = 12%)
  perkKeys: string[];      // i18n perk keys
  accent: string;
}

const ACCENTS = ["var(--line-strong)", "var(--gold)", "var(--gold-pale)", "var(--gold-bright)"];

// getLevels builds the membership ladder from the live platform settings.
export function getLevels(): MembershipLevel[] {
  return getSettings().levels.map((l, i) => ({
    level: l.level,
    name: l.name,
    tier: l.level >= 2 ? "VIP" : "MEMBER",
    priceCentsYear: l.priceCentsYear,
    premiumBps: l.premiumBps,
    perkKeys: l.perks,
    accent: ACCENTS[Math.min(i, ACCENTS.length - 1)],
  }));
}

export function maxLevel(): number {
  return getLevels().length;
}

// levelOf resolves the current membership level (0 = guest / not signed in).
export function levelOf(account?: Account | null): number {
  if (!account || account.tier === "GUEST") return 0;
  if (account.membershipLevel && account.membershipLevel > 0) return account.membershipLevel;
  return account.tier === "VIP" ? 2 : 1; // back-compat when level isn't set
}

export function levelInfo(level: number): MembershipLevel | undefined {
  return getLevels().find((l) => l.level === level);
}

// tierForLevel maps a level to the participation tier.
export function tierForLevel(level: number): Tier {
  if (level <= 0) return "GUEST";
  return level >= 2 ? "VIP" : "MEMBER";
}
