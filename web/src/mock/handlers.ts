// Mock implementations of every service method, operating on the in-memory db.
// Each mirrors the real endpoint's response shape so the UI cannot tell the
// difference. Mutations persist for the session.

import * as db from "./db";
import { currentAccount, setSession } from "@/auth/session";
import type {
  Account, BidPackage, BuyBidsResp, BuybackResp, ConfirmReq, DutchAuction,
  KycSubmission, LotDetail, PassiveAuction, RedeemInviteResp, Reservation,
  Standing, StartKycResp, Trade, TradeState, VaultView, WeeklyGallery,
  Wallet, BidResp, BuybackMode, ReleaseMode, AType, RequestOtpResp, SessionResp,
  OAuthProvider, VaultObject,
} from "@/types";

const uid = (p: string) => `${p}-${Math.random().toString(16).slice(2, 10)}`;

// ---- identity ----
// me() reflects the client-side session: a logged-in account or the GUEST visitor.
export function me(): Account {
  return { ...currentAccount() };
}

// ---- auth: mobile OTP + social OAuth (replaces invite redemption) ----
// In dev/demo there is no SMS provider, so requestOtp returns a fixed devCode.
export function requestOtp(_mobile: string, _purpose?: string): RequestOtpResp {
  return { expiresInSecs: 300, devCode: "000000" };
}

// verifyOtp signs the user in by mobile number. In this product, verifying the
// SMS code IS the identity check — the account is verified (MEMBER) immediately;
// there is no separate document/KYC step.
// Usernames are unique. A few are reserved (the seeded admin accounts) so demo
// users never collide with them; mkUsername slugifies a base and appends a
// counter until free.
const usedUsernames = new Set<string>(["aurelia.dxb", "noor.auh", "khalid.vip", "admin"]);
function mkUsername(base: string): string {
  const slug = (base || "member").toLowerCase().trim()
    .replace(/[^a-z0-9._]+/g, "_").replace(/^[._]+|[._]+$/g, "").slice(0, 20) || "member";
  let name = slug, i = 1;
  while (usedUsernames.has(name)) name = `${slug}_${i++}`;
  usedUsernames.add(name);
  return name;
}

// avatarFor returns a deterministic profile image (an inline SVG data URL with a
// gradient + initial). In a real integration the Google/Facebook userinfo call
// returns the provider's picture URL; here we synthesise one so it works offline.
function avatarFor(seed: string, provider?: OAuthProvider): string {
  const initial = (seed.trim()[0] || "U").toUpperCase();
  const h = [...seed].reduce((a, ch) => a + ch.charCodeAt(0), provider === "FACEBOOK" ? 210 : provider === "GOOGLE" ? 8 : 0) % 360;
  const svg =
    `<svg xmlns='http://www.w3.org/2000/svg' width='120' height='120'>` +
    `<defs><linearGradient id='g' x1='0' y1='0' x2='1' y2='1'>` +
    `<stop offset='0' stop-color='hsl(${h},58%,46%)'/><stop offset='1' stop-color='hsl(${(h + 38) % 360},58%,30%)'/>` +
    `</linearGradient></defs><rect width='120' height='120' rx='16' fill='url(#g)'/>` +
    `<text x='50%' y='55%' font-size='58' fill='white' font-family='sans-serif' font-weight='600' text-anchor='middle' dominant-baseline='middle'>${initial}</text></svg>`;
  return `data:image/svg+xml;utf8,${encodeURIComponent(svg)}`;
}

function newMember(mobile: string, usernameBase: string, avatarUrl?: string): Account {
  const handle = mkUsername(usernameBase);
  return {
    id: uid("acc"),
    handle,
    avatarUrl: avatarUrl ?? avatarFor(handle),
    tier: "MEMBER",
    kycStatus: "APPROVED",
    eligible: true,
    roles: [],
    status: "ACTIVE",
    mobileE164: mobile,
    membershipLevel: 1, // signing in grants Member · Level 1 (free)
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
  };
}

// checkOtp validates a code WITHOUT creating the account — used by step 1 of the
// registration wizard (mobile verification before connecting a social account).
export function checkOtp(_mobile: string, code: string): { ok: boolean } {
  if (!code || code.length < 4) throw { message: "invalid code", code: "RESOURCE_INVALID" };
  return { ok: true };
}

export function verifyOtp(mobile: string, code: string, handle = ""): SessionResp {
  if (!code || code.length < 4) throw { message: "invalid code", code: "RESOURCE_INVALID" };
  // Register supplies a name -> username; bare login derives one from the number.
  const account = newMember(mobile, handle || `user_${mobile.replace(/\D/g, "").slice(-4)}`);
  setSession(account);
  return { token: account.id, created: true, account };
}

// oauthLogin signs the user in via a social provider. The provider supplies the
// profile image. In the registration wizard, opts carries the already-verified
// mobile + chosen name so the new account is fully populated.
export function oauthLogin(provider: OAuthProvider, opts?: { mobile?: string; name?: string }): SessionResp {
  const base = opts?.name?.trim() || `${provider.toLowerCase()}_user`;
  const account = newMember(opts?.mobile ?? "", base, avatarFor(opts?.name?.trim() || provider, provider));
  setSession(account);
  return { token: account.id, created: true, account };
}

// updateAvatar sets an uploaded profile image on the session account.
export function updateAvatar(dataUrl: string): Account {
  const acc = currentAccount();
  if (acc.id === "guest") throw { message: "sign in first", code: "ACCESS_DENIED" };
  const next = { ...acc, avatarUrl: dataUrl, updatedAt: new Date().toISOString() };
  setSession(next);
  return next;
}

// demoLogin signs in as a ready-made profile so the demo can be explored at any
// membership level or as an inspector/auditor, without an SMS/OAuth round-trip.
export function demoLogin(profile: string): SessionResp {
  const cfg: Record<string, Partial<Account>> = {
    gold: { handle: "gold_demo", membershipLevel: 2, tier: "VIP" },
    platinum: { handle: "platinum_demo", membershipLevel: 3, tier: "VIP" },
    inspector: { handle: "inspector_demo", roles: ["INSPECTOR"] },
    member: { handle: "member_demo" },
  };
  const patch = cfg[profile] ?? cfg.member;
  const acc = newMember("+10000000000", patch.handle!, avatarFor(patch.handle!));
  Object.assign(acc, patch);
  setSession(acc);
  return { token: acc.id, created: false, account: acc };
}

// upgradeMembership raises the paid membership level (after payment). Level >= 2
// maps to VIP for participation. Persisted on the session.
export function upgradeMembership(level: number): Account {
  const acc = currentAccount();
  if (acc.id === "guest") throw { message: "sign in first", code: "ACCESS_DENIED" };
  const next: Account = {
    ...acc,
    membershipLevel: level,
    tier: level >= 2 ? "VIP" : "MEMBER",
    updatedAt: new Date().toISOString(),
  };
  setSession(next);
  return next;
}

// ---- catalog ----
export function galleryWeekly(): WeeklyGallery {
  return { week: db.WEEK, supplyCap: 32, lots: db.lots.map((l) => ({ ...l })) };
}
export function lotDetail(id: string): LotDetail {
  const lot = db.lots.find((l) => l.id === id);
  if (!lot) throw { message: "lot not found", code: "404" };
  return {
    lot: { ...lot },
    certified: lot.state === "CERTIFIED" || lot.state === "SCHEDULED",
    attestations: [
      { id: uid("att"), lotId: lot.id, inspectorId: "0xA1", result: "PASS", recordedAt: lot.createdAt },
    ],
  };
}

// ---- dutch ----
export function getDutch(id: string): DutchAuction {
  const a = db.dutchAuction(id);
  if (!a) throw { message: "auction not found", code: "404" };
  return a;
}
export function reserve(id: string): Reservation {
  db.reservations[id] = "REQUESTED";
  const a = db.dutchAuction(id);
  return {
    id: uid("rsv"), auctionId: id, accountId: db.account.id,
    kind: "DEPOSIT_10", state: "LOCKED",
    amountCents: Math.round((a?.floorCents ?? 0) * 0.10),
    escrowRef: uid("esc"), createdAt: new Date().toISOString(),
  };
}
export function lock(id: string): Reservation {
  db.reservations[id] = "FULL";
  const a = db.dutchAuction(id);
  return {
    id: uid("rsv"), auctionId: id, accountId: db.account.id,
    kind: "FULL_LOCK", state: "LOCKED",
    amountCents: a?.floorCents ?? 0,
    escrowRef: uid("esc"), createdAt: new Date().toISOString(),
  };
}
export function buy(id: string): DutchAuction {
  const a = getDutch(id);
  return {
    ...a, state: "HAMMER",
    winnerAccountId: db.account.id, hammerPriceCents: a.currentPriceCents,
    hammerAt: new Date().toISOString(),
  };
}

// ---- passive ----
export function getPassive(id: string): PassiveAuction {
  const a = db.passiveAuction(id);
  if (!a) throw { message: "auction not found", code: "404" };
  return a;
}
export function placeBid(id: string, priceCents: number): BidResp {
  if (db.wallet.balanceCredits <= 0) throw { message: "out of credits", code: "RESOURCE_INVALID" };
  db.wallet.balanceCredits -= 1;
  db.wallet.debits.push({ id: uid("dbt"), amountCredits: 1, idempotencyKey: uid("idem"), auctionId: id, createdAt: new Date().toISOString() });
  const lot = db.lots.find((l) => l.id === id);
  if (lot?.atype === "VICKREY") {
    db.sealedBids[id] = priceCents;
  } else {
    db.placedBids[id] = [...(db.placedBids[id] ?? []), priceCents];
  }
  return { id: uid("bid"), auctionId: id, priceCents, placedAt: new Date().toISOString() };
}
export function standing(id: string): Standing {
  const a = getPassive(id);
  const taken = db.simTakenCents(id);
  const mine = a.atype === "VICKREY"
    ? (db.sealedBids[id] != null ? [db.sealedBids[id]] : [])
    : (db.placedBids[id] ?? []);
  const counts = new Map<number, number>();
  [...taken, ...mine].forEach((p) => counts.set(p, (counts.get(p) ?? 0) + 1));
  const prices = mine.map((priceCents) => ({
    priceCents,
    isLowestUnique: false,
    placedAt: new Date().toISOString(),
  }));
  if (a.atype === "UNIQBID") {
    const uniques = [...counts.entries()].filter(([, n]) => n === 1).map(([p]) => p).sort((x, y) => x - y);
    const lowest = uniques[0];
    prices.forEach((p) => { p.isLowestUnique = p.priceCents === lowest; });
  }
  return { auctionId: id, atype: a.atype, state: a.state, closesAt: a.closesAt, prices };
}

// ---- bids ----
export function getWallet(): Wallet {
  return { ...db.wallet, purchases: [...db.wallet.purchases], debits: [...db.wallet.debits] };
}
export function getPackages(): BidPackage[] {
  return db.packages.map((p) => ({ ...p }));
}
export function buyBids(packageId: string): BuyBidsResp {
  const pkg = db.packages.find((p) => p.id === packageId);
  if (!pkg) throw { message: "unknown package", code: "404" };
  db.wallet.balanceCredits += pkg.credits;
  db.wallet.purchases.push({ id: uid("pur"), packageId: pkg.id, creditsGranted: pkg.credits, usdcChargedCents: pkg.priceCents, createdAt: new Date().toISOString() });
  return { creditsGranted: pkg.credits, usdcChargedCents: pkg.priceCents, balanceCredits: db.wallet.balanceCredits };
}

// ---- vault ----
export function getVault(): VaultView {
  return { objects: db.vault.objects.map((o) => ({ ...o })), creditBalanceCents: db.vault.creditBalanceCents };
}
export function addObject(req: import("@/types").CreateObjectReq): VaultObject {
  const title = req.maison ? `${req.maison} — ${req.title}` : req.title;
  const obj: VaultObject = {
    id: uid("obj"),
    title,
    description: req.description ?? "",
    appraisedValueCents: req.appraisedValueCents,
    state: "IN_VAULT",
    category: req.category,
    imageRefs: (req.imageRefs ?? []).slice(0, 7),
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
  };
  db.vault.objects.unshift(obj);
  return { ...obj };
}
export function listObject(id: string, atype: AType, durationDays?: number) {
  const obj = db.vault.objects.find((o) => o.id === id);
  if (!obj) throw { message: "object not found", code: "404" };
  // Publish a live gallery lot from this object (carries its category icon +
  // photos), and mark the object as in-auction.
  db.listFromObject(obj, atype, durationDays);
  obj.state = "IN_AUCTION";
  obj.updatedAt = new Date().toISOString();
  return { ...obj };
}
export function buyback(id: string, mode: BuybackMode): BuybackResp {
  const obj = db.vault.objects.find((o) => o.id === id);
  if (!obj) throw { message: "object not found", code: "404" };
  const payout = Math.round(obj.appraisedValueCents * (mode === "CASH" ? 0.5 : 0.85));
  obj.state = "BOUGHT_BACK";
  obj.updatedAt = new Date().toISOString();
  if (mode === "CREDIT") db.vault.creditBalanceCents += payout;
  return { object: { ...obj }, mode, payoutCents: payout, balanceCents: db.vault.creditBalanceCents };
}

// ---- invite ----
const VALID_CODES = ["LUX-7F2A-9KQ", "MAISON-04"];
export function redeemInvite(code: string): RedeemInviteResp {
  const c = code.trim().toUpperCase();
  if (!VALID_CODES.includes(c)) throw { message: "Invalid or expired code.", code: "RESOURCE_INVALID" };
  db.account.tier = "MEMBER";
  return { code: c, redeemedBy: db.account.id, issuedBy: "0b11c000-0000-4000-8000-0000000b11c0" };
}

// ---- kyc ----
let kycSub: KycSubmission | null = null;
export function startKyc(phone: string): StartKycResp {
  const id = uid("kyc");
  kycSub = {
    id, accountId: db.account.id, docType: "NATIONAL_ID", docRef: "pending", phone,
    state: "STARTED", submittedAt: new Date().toISOString(),
  };
  return { submissionId: id, challengeId: uid("chl"), state: "STARTED", expiresAt: new Date(Date.now() + 300_000).toISOString(), devCode: "0000" };
}
export function verifyKyc(_code: string): KycSubmission {
  const acc = currentAccount();
  if (!kycSub) kycSub = { id: uid("kyc"), accountId: acc.id, docType: "PASSPORT", docRef: "pending", phone: acc.mobileE164 || "+0000000000", state: "STARTED", submittedAt: new Date().toISOString() };
  kycSub = { ...kycSub, state: "APPROVED", submittedAt: new Date().toISOString() };
  // KYC approval is the membership trigger: GUEST -> MEMBER + KYC APPROVED.
  if (acc.id !== "guest") {
    setSession({ ...acc, tier: acc.tier === "GUEST" ? "MEMBER" : acc.tier, kycStatus: "APPROVED", eligible: true, updatedAt: new Date().toISOString() });
  }
  return { ...kycSub };
}
export function kycStatus(): KycSubmission {
  const acc = currentAccount();
  if (!kycSub) {
    return { id: uid("kyc"), accountId: acc.id, docType: "PASSPORT", docRef: "approved", phone: acc.mobileE164 || "+0000000000", state: acc.kycStatus === "APPROVED" ? "APPROVED" : "STARTED", submittedAt: new Date().toISOString() };
  }
  return { ...kycSub };
}

// ---- escrow ----
export function getTrade(id: string, priceCents?: number): Trade {
  return db.tradeFor(id, priceCents != null ? { priceCents } : undefined);
}
export function fundTrade(id: string, _amountCents: number): TradeState {
  const t = db.tradeFor(id);
  t.state = "HELD";
  t.updatedAt = new Date().toISOString();
  t.conservation = { inflowsCents: t.obligationCents, disbursedCents: 0, balanced: false };
  return { id: t.id, state: t.state, kind: t.kind };
}
export function confirmTrade(id: string, mode: ReleaseMode): TradeState {
  const t = db.tradeFor(id);
  t.state = "RELEASED";
  t.releaseMode = mode;
  t.updatedAt = new Date().toISOString();
  t.conservation = { inflowsCents: t.obligationCents, disbursedCents: t.obligationCents, balanced: true };
  return { id: t.id, state: t.state, kind: t.kind, releaseMode: mode };
}
