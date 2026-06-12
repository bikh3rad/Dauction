/* ============================================================
   Platform settings — the editable "variables" of the platform. Seeded from the
   resolved feature set (membership levels, bid packages, categories, escrow /
   auction economics) and persisted to localStorage so the admin panel can edit
   everything and the rest of the app reads the live values.
   ============================================================ */
import { useSyncExternalStore } from "react";
import type { Category } from "@/types";

export interface LevelCfg { level: number; name: string; priceCentsYear: number; premiumBps: number; perks: string[]; }
export interface PackageCfg { id: string; credits: number; priceCents: number; bestValue: boolean; }
export interface CategoryCfg { key: Category; label: string; active: boolean; }
export interface Economics {
  weeklyCap: number;        // lots admitted per ISO week
  premiumBps: number;       // buyer's premium (basis points)
  buybackCashPct: number;   // instant buyback, cash
  buybackCreditPct: number; // instant buyback, vault credit
  depositPct: number;       // Dutch reservation deposit
  fundingHours: number;     // winner funding window
  otpTtlSecs: number;       // OTP lifetime
}
export interface PlatformSettings {
  levels: LevelCfg[];
  packages: PackageCfg[];
  categories: CategoryCfg[];
  economics: Economics;
}

const c = (d: number) => Math.round(d * 100);

export const DEFAULT_SETTINGS: PlatformSettings = {
  levels: [
    { level: 1, name: "Member", priceCentsYear: 0, premiumBps: 1200, perks: ["perk_bid", "perk_vault", "perk_verified"] },
    { level: 2, name: "Gold", priceCentsYear: c(290), premiumBps: 1000, perks: ["perk_premium10", "perk_early", "perk_credit"] },
    { level: 3, name: "Platinum", priceCentsYear: c(1900), premiumBps: 800, perks: ["perk_premium8", "perk_priority", "perk_concierge"] },
  ],
  packages: [
    { id: "PKG_100", credits: 100, priceCents: c(80), bestValue: true },
    { id: "PKG_50", credits: 50, priceCents: c(45), bestValue: false },
    { id: "PKG_20", credits: 20, priceCents: c(20), bestValue: false },
  ],
  categories: [
    { key: "horology", label: "Horology", active: true },
    { key: "bag", label: "Haute Maroquinerie", active: true },
    { key: "sneaker", label: "Grail Sneaker", active: true },
    { key: "perfume", label: "Rare Perfume", active: true },
    { key: "art", label: "Blue-Chip Art", active: true },
    { key: "painting", label: "Fine Painting", active: true },
    { key: "jewel", label: "Haute Joaillerie", active: true },
  ],
  economics: {
    weeklyCap: 32, premiumBps: 1000, buybackCashPct: 50, buybackCreditPct: 85,
    depositPct: 10, fundingHours: 24, otpTtlSecs: 300,
  },
};

const KEY = "dauction.settings";
let cache: PlatformSettings | null = null;
const listeners = new Set<() => void>();

function load(): PlatformSettings {
  try {
    const raw = localStorage.getItem(KEY);
    if (raw) return { ...DEFAULT_SETTINGS, ...JSON.parse(raw) };
  } catch { /* ignore */ }
  return DEFAULT_SETTINGS;
}

export function getSettings(): PlatformSettings {
  if (!cache) cache = load();
  return cache;
}

export function saveSettings(next: PlatformSettings): void {
  cache = next;
  try { localStorage.setItem(KEY, JSON.stringify(next)); } catch { /* ignore */ }
  listeners.forEach((l) => l());
}

export function updateSettings(patch: Partial<PlatformSettings>): void {
  saveSettings({ ...getSettings(), ...patch });
}

export function resetSettings(): void {
  saveSettings(DEFAULT_SETTINGS);
}

// React binding: components re-render when settings change.
export function useSettings(): PlatformSettings {
  return useSyncExternalStore(
    (cb) => { listeners.add(cb); return () => listeners.delete(cb); },
    getSettings,
    getSettings,
  );
}
