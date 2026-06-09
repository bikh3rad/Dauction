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
} from "@/types";

// ---------- identity ----------
export const identity = {
  me: () => withFallback<Account>(() => get("/me"), mock.me),
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
