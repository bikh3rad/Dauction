/* ============================================================
   Mock backend — a small in-memory store seeded from the prototype, shaped to
   match the REAL backend DTOs (USDC cents, MONOSPACE_UPPERCASE enums, UUID-ish
   ids, language-neutral string titles). It is stateful: buying credits, placing
   bids, reserving, funding etc. mutate it so flows feel real while offline.

   This is the SINGLE source of truth for the demo catalog. The gallery reads it,
   the admin console derives its auction rows from it (mock/adminDb.ts), and admin
   edits (floor / high / bid-cost) write back through here — so an edit in the
   House-Operations panel is reflected in the auction every buyer sees.

   Every lot carries:
     • category   → the unique line-art icon shown on the gallery card
     • imageRefs  → the real product photos shown on the detail carousel + card

   NOTE: in this mock a lot's id doubles as its auction id and trade id, because
   the gallery only knows lot ids. A real integration would carry an explicit
   `auctionId` on the lot DTO; the service layer isolates that assumption.
   ============================================================ */

import type {
  Account, BidPackage, Category, DutchAuction, Lot, PassiveAuction, Trade,
  VaultObject, Wallet,
} from "@/types";

const c = (dollars: number) => Math.round(dollars * 100); // dollars → USDC cents
const iso = (msFromNow: number) => new Date(Date.now() + msFromNow).toISOString();
const SELLER = "0b11c000-0000-4000-8000-0000000b11c0";
export const WEEK = "2026-W24";

// Real product photography (Unsplash CDN). The carousel + card degrade to the
// category line-art if a URL ever fails to load (see LotCarousel/LotCard onError).
const img = (id: string) => `https://images.unsplash.com/${id}?auto=format&fit=crop&w=1000&q=70`;
export const PHOTOS: Record<Category, string[]> = {
  horology: ["photo-1523275335684-37898b6baf30", "photo-1614164185128-e4ec99c436d7", "photo-1612817159949-195b6eb9e31a", "photo-1547996160-81dfa63595aa"].map(img),
  bag: ["photo-1584917865442-de89df76afd3", "photo-1594223274512-ad4803739b7c", "photo-1548036328-c9fa89d128fa", "photo-1591561954557-26941169b49e"].map(img),
  sneaker: ["photo-1556906781-9a412961c28c", "photo-1600185365483-26d7a4cc7519", "photo-1552346154-21d32810aba3", "photo-1542291026-7eec264c27ff"].map(img),
  perfume: ["photo-1541643600914-78b084683601", "photo-1594035910387-fea47794261f", "photo-1592945403244-b3fbafd7f539", "photo-1588405748880-12d1d2a59f75"].map(img),
  art: ["photo-1549887534-1541e9326642", "photo-1578321272176-b7bbc0679853", "photo-1536924940846-227afb31e2a5", "photo-1577083552431-6e5fd75a9160"].map(img),
  painting: ["photo-1577083552431-6e5fd75a9160", "photo-1536924940846-227afb31e2a5", "photo-1549887534-1541e9326642", "photo-1578321272176-b7bbc0679853"].map(img),
  jewel: ["photo-1515562141207-7a88fb7ce338", "photo-1605100804763-247f67b3557e", "photo-1611652022419-a9419f74343d", "photo-1599643478518-a784e5dc4c8f"].map(img),
};
const photos = (cat: Category, n = 4): string[] => PHOTOS[cat].slice(0, n);

// ---- the signed-in member ----
export const account: Account = {
  id: "0a7a4e00-0000-4000-8000-00000000a74e",
  tier: "MEMBER",
  kycStatus: "APPROVED",
  eligible: true,
  createdAt: "2025-11-02T09:00:00Z",
  updatedAt: "2026-06-01T09:00:00Z",
};

// ---- lots (catalog) — fresh sample data, each with a category icon + real photos ----
export const lots: Lot[] = [
  {
    id: "lot-07", objectId: "obj-07", sellerAccountId: SELLER,
    title: "Audemars Piguet — Royal Oak ‘Jumbo’ 15202ST — Blue Dial",
    description:
      "The discontinued extra-thin Royal Oak in steel with the ‘Petite Tapisserie’ blue dial — the reference that defined the luxury sports watch. Full set, unworn, single owner.",
    atype: "DUTCH", durationDays: null, category: "horology",
    reserveCents: c(56000), appraisedValueCents: c(92000), imageRefs: photos("horology"),
    state: "SCHEDULED", isoWeek: WEEK, createdAt: "2026-06-08T08:00:00Z", scheduledAt: iso(-60000),
  },
  {
    id: "lot-08", objectId: "obj-08", sellerAccountId: SELLER,
    title: "Hermès — Birkin 30 — Gold Togo, Palladium Hardware",
    description:
      "The definitive Birkin: Gold Togo calfskin with palladium hardware, the most requested combination in the world. Store fresh with raincoat, clochette, lock and keys.",
    atype: "DUTCH", durationDays: null, category: "bag",
    reserveCents: c(26000), appraisedValueCents: c(48000), imageRefs: photos("bag"),
    state: "SCHEDULED", isoWeek: WEEK, createdAt: "2026-06-08T08:00:00Z", scheduledAt: iso(1820_000),
  },
  {
    id: "lot-09", objectId: "obj-09", sellerAccountId: SELLER,
    title: "Nike — Air Jordan 1 Retro High OG ‘Chicago’ — 1985 OG",
    description:
      "A pair of the original 1985 ‘Chicago’ Air Jordan 1 in collector-grade condition. The most important sneaker ever made, with original box and documented provenance.",
    atype: "DUTCH", durationDays: null, category: "sneaker",
    reserveCents: c(5500), appraisedValueCents: c(12000), imageRefs: photos("sneaker"),
    state: "CERTIFIED", isoWeek: WEEK, createdAt: "2026-06-08T08:00:00Z", scheduledAt: iso(5400_000),
  },
  {
    id: "lot-10", objectId: "obj-10", sellerAccountId: SELLER,
    title: "Roja Parfums — Haute Luxe — Crystal Flacon, sealed",
    description:
      "An opulent oud-and-saffron extrait in a Swarovski-set crystal flacon — one of the most expensive fragrances ever bottled. Sealed, in its presentation case with certificate.",
    atype: "DUTCH", durationDays: null, category: "perfume",
    reserveCents: c(4200), appraisedValueCents: c(8500), imageRefs: photos("perfume"),
    state: "SCHEDULED", isoWeek: WEEK, createdAt: "2026-06-08T08:00:00Z", scheduledAt: iso(9000_000),
  },
  {
    id: "lot-11", objectId: "obj-11", sellerAccountId: SELLER,
    title: "Tiffany & Co. — Diamond Solitaire — 3.01ct, D / IF",
    description:
      "A round-brilliant solitaire of 3.01 carats, graded D colour / Internally Flawless, in a platinum Tiffany setting. Accompanied by the original GIA report and Blue Book papers.",
    atype: "DUTCH", durationDays: null, category: "jewel",
    reserveCents: c(38000), appraisedValueCents: c(64000), imageRefs: photos("jewel"),
    state: "SCHEDULED", isoWeek: WEEK, createdAt: "2026-06-08T08:00:00Z", scheduledAt: iso(14400_000),
  },
  {
    id: "lot-12", objectId: "obj-12", sellerAccountId: SELLER,
    title: "Contemporary — ‘Vermillion Field’ — Acrylic on Canvas, signed",
    description:
      "A large signed contemporary canvas in saturated vermillion and gilt — gallery-framed, exhibition-ready. Offered as a live descending Dutch auction from its appraised ceiling.",
    atype: "DUTCH", durationDays: null, category: "painting",
    reserveCents: c(11000), appraisedValueCents: c(21000), imageRefs: photos("painting"),
    state: "SCHEDULED", isoWeek: WEEK, createdAt: "2026-06-08T08:00:00Z", scheduledAt: iso(21600_000),
  },
  {
    id: "lot-13", objectId: "obj-13", sellerAccountId: SELLER,
    title: "Estate Commission — ‘Crimson Atrium’ — Oil on Linen, signed",
    description:
      "A large signed oil from a private estate — deep crimson and gilt, gallery-framed. Offered as a sealed Vickrey auction: submit one hidden bid; the second-highest price wins, paid at that price.",
    atype: "VICKREY", durationDays: 5, category: "art",
    reserveCents: c(54000), appraisedValueCents: c(72000), imageRefs: photos("art"),
    state: "SCHEDULED", isoWeek: WEEK, createdAt: "2026-06-08T08:00:00Z", scheduledAt: iso(-86400_000),
  },
  {
    id: "lot-14", objectId: "obj-14", sellerAccountId: SELLER,
    title: "Chanel — Classic Flap Medium — Black Caviar, Gold Hardware",
    description:
      "The timeless medium Classic Flap in black caviar with gold hardware. Offered as UniqBid: place as many unique prices as you like — the lowest price that no one else has chosen wins.",
    atype: "UNIQBID", durationDays: 7, category: "bag",
    reserveCents: c(9500), appraisedValueCents: c(14000), imageRefs: PHOTOS.bag.slice(1, 4),
    state: "SCHEDULED", isoWeek: WEEK, createdAt: "2026-06-08T08:00:00Z", scheduledAt: iso(-43200_000),
  },
];

// ---- dutch auction engine params (keyed by lot/auction id) ----
interface DutchSeed { ceiling: number; floor: number; step: number; interval: number; openOffsetMs: number; }
const dutchSeed: Record<string, DutchSeed> = {
  "lot-07": { ceiling: c(92000), floor: c(56000), step: c(220), interval: 22, openOffsetMs: -180_000 },
  "lot-08": { ceiling: c(48000), floor: c(26000), step: c(160), interval: 24, openOffsetMs: 1820_000 },
  "lot-09": { ceiling: c(12000), floor: c(5500), step: c(40), interval: 20, openOffsetMs: 5400_000 },
  "lot-10": { ceiling: c(8500), floor: c(4200), step: c(30), interval: 26, openOffsetMs: 9000_000 },
  "lot-11": { ceiling: c(64000), floor: c(38000), step: c(200), interval: 18, openOffsetMs: 14400_000 },
  "lot-12": { ceiling: c(21000), floor: c(11000), step: c(80), interval: 22, openOffsetMs: 21600_000 },
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
    createdAt: "2026-06-08T08:00:00Z",
  };
}

// ---- passive auctions (keyed by lot/auction id) ----
const passiveSeed: Record<string, { closesInMs: number; participants: number; bidCost: number }> = {
  "lot-13": { closesInMs: 5 * 86400_000 - 86400_000, participants: 41, bidCost: 1 },
  // a premium timed auction where each bid costs 40 credits (per-auction custom).
  "lot-14": { closesInMs: 7 * 86400_000 - 43200_000, participants: 184, bidCost: 40 },
};
export function passiveAuction(id: string): PassiveAuction | undefined {
  const lot = lots.find((l) => l.id === id);
  const seed = passiveSeed[id];
  if (!lot || !seed || (lot.atype !== "VICKREY" && lot.atype !== "UNIQBID")) return undefined;
  return {
    id, lotId: id, atype: lot.atype, state: "OPEN",
    closesAt: iso(seed.closesInMs),
    reserveCents: lot.reserveCents,          // low — minimum bid
    highCents: lot.appraisedValueCents,      // high — appraised value
    participantCount: seed.participants, bidCostCredits: seed.bidCost,
  };
}

// floorOf returns a passive auction's low price (the minimum allowable bid).
export function floorOf(id: string): number {
  const lot = lots.find((l) => l.id === id);
  return lot?.reserveCents ?? 0;
}

// bidCostOf returns the per-bid credit cost for a passive auction (default 1).
export function bidCostOf(id: string): number {
  return passiveSeed[id]?.bidCost ?? 1;
}

// setBidCost lets the admin retune a live timed auction's per-bid credit cost.
export function setBidCost(id: string, cost: number): void {
  if (passiveSeed[id]) passiveSeed[id].bidCost = Math.max(1, Math.round(cost));
}

// setLotReserve retunes a lot's floor / low price (admin price edit). For Dutch
// this is the descending floor; for passive it is the minimum allowable bid.
export function setLotReserve(id: string, reserveCents: number): void {
  const lot = lots.find((l) => l.id === id);
  if (lot) lot.reserveCents = Math.max(0, Math.round(reserveCents));
  if (dutchSeed[id]) dutchSeed[id].floor = Math.max(0, Math.round(reserveCents));
}

// setLotAppraised retunes a lot's high / appraised price (admin price edit). For
// Dutch this is the descending ceiling the auction opens from.
export function setLotAppraised(id: string, appraisedCents: number): void {
  const v = Math.max(0, Math.round(appraisedCents));
  const lot = lots.find((l) => l.id === id);
  if (lot) lot.appraisedValueCents = v;
  if (dutchSeed[id]) dutchSeed[id].ceiling = v;
}

// setLotTitle / setLotState — admin edits that write through to the gallery lot.
export function setLotTitle(id: string, title: string): void {
  const lot = lots.find((l) => l.id === id);
  if (lot && title.trim()) lot.title = title.trim();
}

// listFromObject turns a just-listed vault object into a live gallery lot: it
// pushes a SCHEDULED lot (so it shows in the weekly gallery, carrying the owner's
// category icon + photos) and registers the matching auction seed so the lot's
// auction page renders. Re-listing the same object replaces its prior lot.
export function listFromObject(obj: VaultObject, atype: Lot["atype"], durationDays?: number): Lot {
  const lotId = `lot-${obj.id}`;
  const stale = lots.findIndex((l) => l.objectId === obj.id);
  if (stale >= 0) lots.splice(stale, 1);

  const floor = Math.max(c(1), Math.round(obj.appraisedValueCents * 0.6));
  const lot: Lot = {
    id: lotId, objectId: obj.id, sellerAccountId: account.id,
    title: obj.title, description: obj.description || obj.title,
    atype, durationDays: atype === "DUTCH" ? null : (durationDays ?? 5),
    reserveCents: floor, appraisedValueCents: obj.appraisedValueCents,
    state: "SCHEDULED", isoWeek: WEEK, createdAt: iso(0), scheduledAt: iso(-1000),
    category: obj.category, imageRefs: obj.imageRefs,
    certified: true, // the object was Inspector-verified before listing
  };
  lots.unshift(lot);

  if (atype === "DUTCH") {
    dutchSeed[lotId] = {
      ceiling: obj.appraisedValueCents, floor,
      step: Math.max(c(10), Math.round(obj.appraisedValueCents / 400)),
      interval: 22, openOffsetMs: -1000, // already open (live)
    };
  } else {
    passiveSeed[lotId] = { closesInMs: (durationDays ?? 5) * 86400_000, participants: 1, bidCost: 1 };
  }
  return lot;
}

// ---- inspector queue: objects awaiting authenticity verification (§3.5) ----
// An object added to a vault must be verified by an Inspector before its owner
// can list it for auction or sell it to the house.
export interface PendingInspection {
  id: string;
  objectId: string;
  ownerHandle: string;
  title: string;          // full "Maison — Body" title
  category?: Category;
  imageRefs?: string[];
  valueCents: number;
  submittedAt: string;
}

export const inspections: PendingInspection[] = [
  { id: "insp-1", objectId: "obj-seed-1", ownerHandle: "@noor.auh", title: "Rolex — Submariner ‘Hulk’ 116610LV", category: "horology", imageRefs: photos("horology", 3), valueCents: c(28000), submittedAt: iso(-3600_000) },
  { id: "insp-2", objectId: "obj-seed-2", ownerHandle: "@sterling.ldn", title: "Hermès — Constance 18 — Rouge Casaque", category: "bag", imageRefs: photos("bag", 3), valueCents: c(19500), submittedAt: iso(-7200_000) },
  // a pending object in YOUR vault (v2) so the verify → unlock flow is demoable.
  { id: "insp-v2", objectId: "v2", ownerHandle: "@you", title: "Louis Vuitton — Capucines MM — Taurillon", category: "bag", imageRefs: photos("bag", 3), valueCents: c(12400), submittedAt: iso(-1800_000) },
];

// Items the Inspector has already decided (history). Newest first.
export interface DecidedInspection extends PendingInspection {
  verdict: "APPROVED" | "REJECTED";
  decidedAt: string;
}
export const decidedInspections: DecidedInspection[] = [
  { id: "insp-d1", objectId: "obj-d1", ownerHandle: "@khalid.vip", title: "Patek Philippe — Nautilus 5712/1A", category: "horology", imageRefs: photos("horology", 3), valueCents: c(180000), submittedAt: iso(-90000_000), verdict: "APPROVED", decidedAt: iso(-82800_000) },
  { id: "insp-d2", objectId: "obj-d2", ownerHandle: "@dana.doh", title: "Atelier Copy — ‘Royal Oak’ Homage", category: "horology", imageRefs: photos("horology", 2), valueCents: c(8000), submittedAt: iso(-96000_000), verdict: "REJECTED", decidedAt: iso(-90000_000) },
];

let inspSeq = 100;

// submitForInspection enqueues a newly added object for authenticity verification.
export function submitForInspection(obj: VaultObject): PendingInspection {
  const stale = inspections.findIndex((i) => i.objectId === obj.id);
  if (stale >= 0) inspections.splice(stale, 1);
  const p: PendingInspection = {
    id: `insp-${inspSeq++}`,
    objectId: obj.id,
    ownerHandle: `@${account.handle ?? "you"}`,
    title: obj.title,
    category: obj.category,
    imageRefs: obj.imageRefs,
    valueCents: obj.appraisedValueCents,
    submittedAt: iso(0),
  };
  inspections.unshift(p);
  return p;
}

// approveInspection marks the object verified (IN_VAULT) — it does NOT publish an
// auction. The owner then chooses to list it or sell it to the house. The item
// moves to the Inspector's decided history.
export function approveInspection(id: string): void {
  decide(id, "APPROVED");
}

// rejectInspection marks the object REJECTED (authenticity not confirmed).
export function rejectInspection(id: string): void {
  decide(id, "REJECTED");
}

function decide(id: string, verdict: "APPROVED" | "REJECTED"): void {
  const idx = inspections.findIndex((i) => i.id === id);
  if (idx < 0) return;
  const p = inspections[idx];
  const owned = vault.objects.find((o) => o.id === p.objectId);
  if (owned) { owned.state = verdict === "APPROVED" ? "IN_VAULT" : "REJECTED"; owned.updatedAt = iso(0); }
  decidedInspections.unshift({ ...p, verdict, decidedAt: iso(0) });
  inspections.splice(idx, 1);
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

// ---- vault — the signed-in member's own objects (real photos + category icon) ----
export const vault: { objects: VaultObject[]; creditBalanceCents: number } = {
  creditBalanceCents: c(34850),
  objects: [
    { id: "v1", title: "Rolex — Daytona 116500LN — Panda Dial", description: "Steel Daytona, panda dial, 2021, full set.", appraisedValueCents: c(38500), state: "IN_VAULT", category: "horology", imageRefs: PHOTOS.horology.slice(1, 4), createdAt: "2025-12-01T00:00:00Z", updatedAt: "2026-05-01T00:00:00Z" },
    { id: "v2", title: "Louis Vuitton — Capucines MM — Taurillon", description: "Capucines MM in Taurillon leather, 2023.", appraisedValueCents: c(12400), state: "PENDING_INSPECTION", category: "bag", imageRefs: photos("bag", 3), createdAt: "2026-01-01T00:00:00Z", updatedAt: "2026-05-20T00:00:00Z" },
    { id: "v3", title: "Andy Warhol — Flowers (1970) — Screenprint", description: "Screenprint, 1970, archival framed.", appraisedValueCents: c(96000), state: "IN_AUCTION", category: "art", imageRefs: photos("art", 3), createdAt: "2025-10-01T00:00:00Z", updatedAt: "2026-06-01T00:00:00Z" },
    { id: "v4", title: "Cartier — Love Bracelet — Pavé Diamond, Gold", description: "Pavé diamond Love bracelet, gold, 2020.", appraisedValueCents: c(28900), state: "SOLD", category: "jewel", imageRefs: photos("jewel", 3), createdAt: "2025-09-01T00:00:00Z", updatedAt: "2026-04-01T00:00:00Z" },
    { id: "v5", title: "Private Collection — Gilded Horizon — Oil", description: "Oil on canvas, 2009, home collection.", appraisedValueCents: c(46000), state: "IN_VAULT", category: "painting", imageRefs: photos("painting", 3), createdAt: "2026-02-01T00:00:00Z", updatedAt: "2026-05-15T00:00:00Z" },
  ],
};

// ---- escrow trades (keyed by id) ----
export function tradeFor(id: string, opts?: { priceCents?: number }): Trade {
  const lot = lots.find((l) => l.id === id);
  const price = opts?.priceCents ?? lot?.reserveCents ?? c(56000);
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
