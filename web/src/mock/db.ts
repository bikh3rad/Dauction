/* ============================================================
   Mock backend — a small in-memory store seeded from the prototype, shaped to
   match the REAL backend DTOs (USDC cents, MONOSPACE_UPPERCASE enums, UUID-ish
   ids, language-neutral string titles). It is stateful: buying credits, placing
   bids, reserving, funding etc. mutate it so flows feel real while offline.

   NOTE: in this mock a lot's id doubles as its auction id and trade id, because
   the gallery only knows lot ids. A real integration would carry an explicit
   `auctionId` on the lot DTO; the service layer isolates that assumption.
   ============================================================ */

import type {
  Account, BidPackage, DutchAuction, Lot, PassiveAuction, Trade,
  VaultObject, Wallet,
} from "@/types";

const c = (dollars: number) => Math.round(dollars * 100); // dollars → USDC cents
const iso = (msFromNow: number) => new Date(Date.now() + msFromNow).toISOString();
const SELLER = "0b11c000-0000-4000-8000-0000000b11c0";
export const WEEK = "2026-W23";

// ---- the signed-in member ----
export const account: Account = {
  id: "0a7a4e00-0000-4000-8000-00000000a74e",
  tier: "MEMBER",
  kycStatus: "APPROVED",
  eligible: true,
  createdAt: "2025-11-02T09:00:00Z",
  updatedAt: "2026-06-01T09:00:00Z",
};

// ---- lots (catalog) ----
export const lots: Lot[] = [
  {
    id: "lot-07", objectId: "obj-07", sellerAccountId: SELLER,
    title: "Patek Philippe — Nautilus 5711/1A — Tiffany Blue Dial",
    description:
      "The discontinued steel Nautilus with the Tiffany & Co. co-signed dial — arguably the most coveted reference of the decade. Delivered full set, unworn, with a single recorded owner.",
    atype: "DUTCH", durationDays: null,
    reserveCents: c(620000), appraisedValueCents: c(1180000),
    state: "SCHEDULED", isoWeek: WEEK, createdAt: "2026-06-01T08:00:00Z", scheduledAt: iso(-60000),
  },
  {
    id: "lot-08", objectId: "obj-08", sellerAccountId: SELLER,
    title: "Hermès — Birkin 25 Himalaya — Niloticus, Diamond Hardware",
    description:
      "The grail of collectible handbags: matte Niloticus crocodile graduated to evoke a snow-capped peak, with 18k white-gold and diamond hardware. Store fresh, with CITES papers.",
    atype: "DUTCH", durationDays: null,
    reserveCents: c(285000), appraisedValueCents: c(520000),
    state: "SCHEDULED", isoWeek: WEEK, createdAt: "2026-06-01T08:00:00Z", scheduledAt: iso(1820_000),
  },
  {
    id: "lot-09", objectId: "obj-09", sellerAccountId: SELLER,
    title: "Richard Mille — RM 11-03 Flyback Chronograph — Titanium",
    description:
      "A skeletonised automatic flyback chronograph in grade-5 titanium — featherlight on the wrist, engineered like a racing chassis. Recently serviced by an authorised watchmaker.",
    atype: "DUTCH", durationDays: null,
    reserveCents: c(198000), appraisedValueCents: c(395000),
    state: "CERTIFIED", isoWeek: WEEK, createdAt: "2026-06-01T08:00:00Z", scheduledAt: iso(5400_000),
  },
  {
    id: "lot-10", objectId: "obj-10", sellerAccountId: SELLER,
    title: "Yayoi Kusama — Pumpkin (Yellow) — Screenprint, ed. 47/120",
    description:
      "Kusama's signature motif in her instantly recognisable polka-dot language — a hand-pulled screenprint, numbered 47 of 120, archival-framed with gallery certificate.",
    atype: "DUTCH", durationDays: null,
    reserveCents: c(96000), appraisedValueCents: c(185000),
    state: "SCHEDULED", isoWeek: WEEK, createdAt: "2026-06-01T08:00:00Z", scheduledAt: iso(9000_000),
  },
  {
    id: "lot-11", objectId: "obj-11", sellerAccountId: SELLER,
    title: "Nike × Dior — Air Jordan 1 High — 1 of 8,500, EU exclusive",
    description:
      "The Dior-collaboration Air Jordan 1, numbered 1 of 8,500 from the European allocation. Deadstock, unworn, with the original box, carrier bag and receipt.",
    atype: "DUTCH", durationDays: null,
    reserveCents: c(11000), appraisedValueCents: c(27000),
    state: "SCHEDULED", isoWeek: WEEK, createdAt: "2026-06-01T08:00:00Z", scheduledAt: iso(14400_000),
  },
  {
    id: "lot-12", objectId: "obj-12", sellerAccountId: SELLER,
    title: "Clive Christian — No. 1 Imperial Majesty — Baccarat Flacon",
    description:
      "One of only ten flacons produced: a hand-blown Baccarat crystal bottle with a five-carat diamond collar and 24k gold neck. Sealed, in its presentation case.",
    atype: "DUTCH", durationDays: null,
    reserveCents: c(42000), appraisedValueCents: c(96000),
    state: "SCHEDULED", isoWeek: WEEK, createdAt: "2026-06-01T08:00:00Z", scheduledAt: iso(21600_000),
  },
  {
    id: "lot-13", objectId: "obj-13", sellerAccountId: SELLER,
    title: "Estate Commission — Crimson Atrium — Oil on Linen, signed",
    description:
      "A large signed oil from a private estate — deep crimson and gilt, gallery-framed. Offered as a sealed Vickrey auction: submit one hidden bid; the second-highest price wins, paid at that price.",
    atype: "VICKREY", durationDays: 5,
    reserveCents: c(54000), appraisedValueCents: c(72000),
    state: "SCHEDULED", isoWeek: WEEK, createdAt: "2026-06-01T08:00:00Z", scheduledAt: iso(-86400_000),
  },
  {
    id: "lot-14", objectId: "obj-14", sellerAccountId: SELLER,
    title: "Hermès — Kelly Sellier 25 — Rouge Casaque",
    description:
      "A coveted Kelly in Rouge Casaque epsom with gold hardware. Offered as UniqBid: place as many unique prices as you like — the lowest price that no one else has chosen wins.",
    atype: "UNIQBID", durationDays: 7,
    reserveCents: c(38000), appraisedValueCents: c(50000),
    state: "SCHEDULED", isoWeek: WEEK, createdAt: "2026-06-01T08:00:00Z", scheduledAt: iso(-43200_000),
  },
];

// ---- dutch auction engine params (keyed by lot/auction id) ----
interface DutchSeed { ceiling: number; floor: number; step: number; interval: number; openOffsetMs: number; }
const dutchSeed: Record<string, DutchSeed> = {
  "lot-07": { ceiling: c(1180000), floor: c(620000), step: c(2500), interval: 22, openOffsetMs: -180_000 },
  "lot-08": { ceiling: c(520000), floor: c(285000), step: c(1500), interval: 24, openOffsetMs: 1820_000 },
  "lot-09": { ceiling: c(395000), floor: c(198000), step: c(1000), interval: 20, openOffsetMs: 5400_000 },
  "lot-10": { ceiling: c(185000), floor: c(96000), step: c(500), interval: 26, openOffsetMs: 9000_000 },
  "lot-11": { ceiling: c(27000), floor: c(11000), step: c(150), interval: 18, openOffsetMs: 14400_000 },
  "lot-12": { ceiling: c(96000), floor: c(42000), step: c(400), interval: 22, openOffsetMs: 21600_000 },
};

export function dutchAuction(id: string): DutchAuction | undefined {
  const lot = lots.find((l) => l.id === id);
  const seed = dutchSeed[id];
  if (!lot || !seed) return undefined;
  const openAtMs = Date.now() + seed.openOffsetMs;
  const isOpen = seed.openOffsetMs <= 0;
  // server-authoritative current price
  let current = seed.ceiling;
  if (isOpen) {
    const elapsed = (Date.now() - openAtMs) / 1000;
    const drops = Math.floor(elapsed / seed.interval);
    current = Math.max(seed.floor, seed.ceiling - seed.step * drops);
  }
  return {
    id, lotId: id, state: isOpen ? "OPEN" : "SCHEDULED",
    ceilingCents: seed.ceiling, floorCents: seed.floor,
    dropStepCents: seed.step, dropIntervalSeconds: seed.interval,
    currentPriceCents: current,
    openAt: iso(seed.openOffsetMs),
    createdAt: "2026-06-01T08:00:00Z",
  };
}

// ---- passive auctions (keyed by lot/auction id) ----
const passiveSeed: Record<string, { closesInMs: number; participants: number }> = {
  "lot-13": { closesInMs: 5 * 86400_000 - 86400_000, participants: 41 },
  "lot-14": { closesInMs: 7 * 86400_000 - 43200_000, participants: 184 },
};
export function passiveAuction(id: string): PassiveAuction | undefined {
  const lot = lots.find((l) => l.id === id);
  const seed = passiveSeed[id];
  if (!lot || !seed || (lot.atype !== "VICKREY" && lot.atype !== "UNIQBID")) return undefined;
  return {
    id, lotId: id, atype: lot.atype, state: "OPEN",
    closesAt: iso(seed.closesInMs), reserveCents: lot.reserveCents,
    participantCount: seed.participants,
  };
}

// deterministic competing prices (cents) for a UniqBid/Vickrey lot, so standing feels alive
export function simTakenCents(id: string): number[] {
  const lot = lots.find((l) => l.id === id);
  const base = lot ? Number(lot.id.replace(/\D/g, "")) : 7;
  let seed = base * 97 + 13;
  const rnd = () => { seed = (seed * 1103515245 + 12345) & 0x7fffffff; return seed / 0x7fffffff; };
  const out = new Set<number>();
  const n = id === "lot-14" ? 70 : 40;
  for (let i = 0; i < n; i++) out.add(c(1 + Math.floor(rnd() * 480)));
  return [...out];
}

// ---- bid wallet ----
export const wallet: Wallet = {
  accountId: account.id,
  balanceCredits: 18,
  updatedAt: iso(0),
  purchases: [],
  debits: [],
};
export const packages: BidPackage[] = [
  { id: "PKG_100", credits: 100, priceCents: c(80), bestValue: true },
  { id: "PKG_50", credits: 50, priceCents: c(45), bestValue: false },
  { id: "PKG_20", credits: 20, priceCents: c(20), bestValue: false },
];

// per-lot member activity (mock-side session state)
export const sealedBids: Record<string, number> = {}; // lotId -> priceCents (Vickrey)
export const placedBids: Record<string, number[]> = {}; // lotId -> [priceCents] (UniqBid)
export const reservations: Record<string, "REQUESTED" | "LOCKED" | "FULL"> = {};

// ---- vault ----
export const vault: { objects: VaultObject[]; creditBalanceCents: number } = {
  creditBalanceCents: c(34850),
  objects: [
    { id: "v1", title: "Rolex — Daytona 116500LN — Panda Dial", description: "Steel Daytona, panda dial, 2021.", appraisedValueCents: c(38500), state: "IN_VAULT", createdAt: "2025-12-01T00:00:00Z", updatedAt: "2026-05-01T00:00:00Z" },
    { id: "v2", title: "Chanel — Classic Flap Medium — Black Caviar", description: "Medium classic flap, black caviar, 2022.", appraisedValueCents: c(12400), state: "APPRAISING", createdAt: "2026-01-01T00:00:00Z", updatedAt: "2026-05-20T00:00:00Z" },
    { id: "v3", title: "Andy Warhol — Flowers (1970) — Screenprint", description: "Screenprint, 1970.", appraisedValueCents: c(96000), state: "IN_AUCTION", createdAt: "2025-10-01T00:00:00Z", updatedAt: "2026-06-01T00:00:00Z" },
    { id: "v4", title: "Cartier — Love Bracelet — Pavé Diamond, Gold", description: "Pavé diamond Love bracelet, gold, 2020.", appraisedValueCents: c(28900), state: "SOLD", createdAt: "2025-09-01T00:00:00Z", updatedAt: "2026-04-01T00:00:00Z" },
    { id: "v5", title: "Private Collection — Gilded Horizon — Oil, home collection", description: "Oil on canvas, 2009, home collection.", appraisedValueCents: c(46000), state: "IN_VAULT", createdAt: "2026-02-01T00:00:00Z", updatedAt: "2026-05-15T00:00:00Z" },
  ],
};

// ---- escrow trades (keyed by id) ----
export function tradeFor(id: string, opts?: { priceCents?: number }): Trade {
  const lot = lots.find((l) => l.id === id);
  const price = opts?.priceCents ?? lot?.reserveCents ?? c(620000);
  const premium = Math.round(price * 0.10);
  const fee = Math.round(price * 0.02);
  const inspectorFee = c(1500);
  const obligation = price + premium;
  const existing = trades[id];
  if (existing) return existing;
  const t: Trade = {
    id, lotId: id, buyerId: account.id, sellerId: SELLER,
    kind: lot && (lot.atype === "VICKREY" || lot.atype === "UNIQBID") ? "PASSIVE" : "DUTCH",
    state: "FULL_LOCKED",
    priceCents: price, premiumCents: premium, feeCents: fee, inspectorFeeCents: inspectorFee,
    obligationCents: obligation,
    fundingDeadline: iso(24 * 3600_000),
    createdAt: iso(0), updatedAt: iso(0),
    balances: [{ participantId: account.id, balanceCents: Math.round(price * 0.10) }],
    conservation: { inflowsCents: Math.round(price * 0.10), disbursedCents: 0, balanced: false },
  };
  trades[id] = t;
  return t;
}
export const trades: Record<string, Trade> = {};
