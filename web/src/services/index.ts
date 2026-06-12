// Public service API. Every function tries the real gateway endpoint and falls
// back to a schema-accurate mock when the backend is unavailable (see
// withFallback). Components/hooks import only from here.

import { get, post } from "./client";
import { withFallback } from "./withFallback";
import * as mock from "@/mock/handlers";
import type {
  Account, BidPackage, BuyBidsResp, BuybackMode, BuybackResp, DutchAuction,
  KycSubmission, LotDetail, PassiveAuction, RedeemInviteResp, ReleaseMode,
  Reservation, Standing, StartKycResp, Trade, TradeState, VaultObject,
  VaultView, WeeklyGallery, Wallet, BidResp, AType, DocType,
  RequestOtpResp, SessionResp, OAuthProvider, CreateObjectReq,
} from "@/types";

// ---------- profile (account + membership, merged into one service) ----------
export const profile = {
  me: () => withFallback<Account>(() => get("/me"), mock.me),
  upgrade: (level: number) =>
    withFallback<Account>(() => post("/membership/upgrade", { level }), () => mock.upgradeMembership(level)),
  setAvatar: (dataUrl: string) => Promise.resolve(mock.updateAvatar(dataUrl)),
};

// ---------- auth (mobile OTP + social OAuth + 2-step register wizard) ----------
export const auth = {
  requestOtp: (mobileE164: string, purpose = "SIGNUP") =>
    withFallback<RequestOtpResp>(
      () => post("/auth/otp/request", { mobileE164, purpose }),
      () => mock.requestOtp(mobileE164, purpose),
    ),
  // register wizard step 1: verify the mobile code without creating the account.
  checkOtp: (mobileE164: string, code: string) =>
    withFallback<{ ok: boolean }>(
      () => post("/auth/otp/check", { mobileE164, code }),
      () => mock.checkOtp(mobileE164, code),
    ),
  verifyOtp: (mobileE164: string, code: string, handle?: string) =>
    withFallback<SessionResp>(
      () => post("/auth/otp/verify", { mobileE164, code, handle }),
      () => mock.verifyOtp(mobileE164, code, handle),
    ),
  // register wizard step 2 (and one-tap social login): connect Google/Facebook,
  // which supplies the profile image. opts carries the verified mobile + name.
  oauth: (provider: OAuthProvider, opts?: { mobile?: string; name?: string }) =>
    withFallback<SessionResp>(
      () => get(`/auth/oauth/${provider.toLowerCase()}/callback`, { params: { code: "demo" } }),
      () => mock.oauthLogin(provider, opts),
    ),
  // Demo profiles (member / gold / platinum / inspector) — mock-only convenience.
  demo: (profile: string) => Promise.resolve(mock.demoLogin(profile)),
};

// ---------- catalog ----------
export const catalog = {
  weekly: (week?: string) =>
    withFallback<WeeklyGallery>(
      () => get("/gallery/weekly", { params: week ? { week } : undefined }),
      mock.galleryWeekly,
    ),
  lot: (id: string) =>
    withFallback<LotDetail>(() => get(`/lots/${id}`), () => mock.lotDetail(id)),
};

// ---------- dutch auction ----------
export const dutch = {
  get: (id: string) => withFallback<DutchAuction>(() => get(`/auctions/${id}`), () => mock.getDutch(id)),
  reserve: (id: string) => withFallback<Reservation>(() => post(`/auctions/${id}/reserve`), () => mock.reserve(id)),
  lock: (id: string) => withFallback<Reservation>(() => post(`/auctions/${id}/lock`), () => mock.lock(id)),
  buy: (id: string) => withFallback<DutchAuction>(() => post(`/auctions/${id}/buy`), () => mock.buy(id)),
};

// ---------- passive auction ----------
export const passive = {
  get: (id: string) => withFallback<PassiveAuction>(() => get(`/auctions/${id}`), () => mock.getPassive(id)),
  standing: (id: string) => withFallback<Standing>(() => get(`/auctions/${id}/standing`), () => mock.standing(id)),
  bid: (id: string, priceCents: number, requestId?: string) =>
    withFallback<BidResp>(
      () => post(`/auctions/${id}/bid`, { priceCents, requestId }),
      () => mock.placeBid(id, priceCents),
    ),
};

// ---------- bids ----------
export const bids = {
  wallet: () => withFallback<Wallet>(() => get("/bids/wallet"), mock.getWallet),
  packages: () => withFallback<BidPackage[]>(() => get("/bids/packages"), mock.getPackages),
  buy: (packageId: string) =>
    withFallback<BuyBidsResp>(() => post("/bids/buy", { packageId }), () => mock.buyBids(packageId)),
};

// ---------- vault ----------
export const vault = {
  view: () => withFallback<VaultView>(() => get("/vault"), mock.getVault),
  add: (req: CreateObjectReq) =>
    withFallback<VaultObject>(() => post("/vault/objects", req), () => mock.addObject(req)),
  list: (id: string, atype: AType, durationDays?: number) =>
    withFallback<VaultObject>(
      () => post(`/vault/objects/${id}/list`, { atype, durationDays }),
      () => mock.listObject(id, atype, durationDays),
    ),
  buyback: (id: string, mode: BuybackMode) =>
    withFallback<BuybackResp>(() => post(`/vault/objects/${id}/buyback`, { mode }), () => mock.buyback(id, mode)),
};

// ---------- invite ----------
export const invite = {
  redeem: (code: string) =>
    withFallback<RedeemInviteResp>(() => post("/invites/redeem", { code }), () => mock.redeemInvite(code)),
};

// ---------- kyc ----------
export const kyc = {
  start: (docType: DocType, docRef: string, phone: string) =>
    withFallback<StartKycResp>(() => post("/kyc/start", { docType, docRef, phone }), () => mock.startKyc(phone)),
  verify: (code: string) =>
    withFallback<KycSubmission>(() => post("/kyc/verify", { code }), () => mock.verifyKyc(code)),
  status: () => withFallback<KycSubmission>(() => get("/kyc/status"), mock.kycStatus),
};

// ---------- escrow ----------
export const escrow = {
  get: (id: string, priceCents?: number) =>
    withFallback<Trade>(() => get(`/escrow/${id}`), () => mock.getTrade(id, priceCents)),
  fund: (id: string, amountCents: number) =>
    withFallback<TradeState>(() => post(`/escrow/${id}/fund`, { amountCents }), () => mock.fundTrade(id, amountCents)),
  confirm: (id: string, mode: ReleaseMode) =>
    withFallback<TradeState>(() => post(`/escrow/${id}/confirm`, { mode }), () => mock.confirmTrade(id, mode)),
};
