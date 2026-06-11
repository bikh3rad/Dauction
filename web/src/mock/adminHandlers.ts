// Mock implementations of the House-Operations endpoints, operating on the
// in-memory adminDb. Mutations persist for the session so the console feels
// live. Each returns the response shape the real `/apis/admin/*` route will.

import * as db from "./adminDb";
import type {
  AccountStatus, AdminAccount, AdminAuction, AdminCertReview, AdminEscrowRow,
  AdminInvite, AdminKycReview, AdminStats, AdminVaultObject, CreateAuctionReq,
  DisputeRuling,
} from "@/types/admin";
import type { Role, Tier } from "@/types";

const clone = <T>(x: T): T => JSON.parse(JSON.stringify(x));
const uid = (p: string) => `${p}-${Math.random().toString(16).slice(2, 8)}`;

// ---- overview ----
export function stats(): AdminStats {
  const locked = db.escrow
    .filter((e) => ["FULL_LOCKED", "HELD", "DEPOSIT_LOCKED"].includes(e.state))
    .reduce((a, e) => a + e.amountCents + e.premiumCents, 0);
  return {
    activeInvites: db.invites.filter((i) => i.status === "ACTIVE").length,
    flaggedInvites: db.invites.filter((i) => i.status === "FLAGGED").length,
    pendingKyc: db.kycQueue.filter((k) => k.status === "PENDING").length,
    members: db.accounts.length,
    lotsThisWeek: db.auctions.length,
    supplyCap: 32,
    openAuctions: db.auctions.filter((a) => a.state === "OPEN" || a.state === "CLOSING").length,
    escrowLockedCents: locked,
  };
}

// ---- invites ----
export function listInvites(): AdminInvite[] {
  return clone(db.invites);
}
export function revokeInvite(code: string): AdminInvite {
  const iv = db.invites.find((i) => i.code === code);
  if (!iv) throw { message: "invite not found", code: "404" };
  iv.status = "REVOKED";
  return clone(iv);
}

// ---- accounts ----
export function listAccounts(): AdminAccount[] {
  return clone(db.accounts);
}
export function setAccountStatus(id: string, status: AccountStatus): AdminAccount {
  const a = db.accounts.find((x) => x.id === id);
  if (!a) throw { message: "account not found", code: "404" };
  a.status = status;
  return clone(a);
}
export function setAccountTier(id: string, tier: Tier): AdminAccount {
  const a = db.accounts.find((x) => x.id === id);
  if (!a) throw { message: "account not found", code: "404" };
  a.tier = tier;
  return clone(a);
}
// Grant/revoke a functional role (e.g. promote a user to INSPECTOR).
export function setAccountRole(id: string, role: Role, grant: boolean): AdminAccount {
  const a = db.accounts.find((x) => x.id === id);
  if (!a) throw { message: "account not found", code: "404" };
  const current = new Set(a.roles ?? []);
  if (grant) current.add(role); else current.delete(role);
  a.roles = [...current];
  return clone(a);
}

// ---- memberships (KYC) ----
export function listKyc(): AdminKycReview[] {
  return clone(db.kycQueue);
}
export function decideKyc(id: string, approve: boolean): AdminKycReview {
  const k = db.kycQueue.find((x) => x.id === id);
  if (!k) throw { message: "review not found", code: "404" };
  k.status = approve ? "APPROVED" : "REJECTED";
  const acct = db.accounts.find((a) => a.id === k.accountId);
  if (acct) acct.kycStatus = approve ? "APPROVED" : "REJECTED";
  return clone(k);
}

// ---- certification ----
export function listCert(): AdminCertReview[] {
  return clone(db.certQueue);
}
export function certify(lotId: string): AdminCertReview {
  const c = db.certQueue.find((x) => x.lotId === lotId);
  if (!c) throw { message: "lot not found", code: "404" };
  c.status = "CERTIFIED";
  return clone(c);
}

// ---- auctions ----
export function listAuctions(): AdminAuction[] {
  return clone(db.auctions);
}
export function createAuction(req: CreateAuctionReq): AdminAuction {
  const a: AdminAuction = {
    id: uid("auc"),
    lotId: req.lotId,
    title: req.lotId,
    maison: "—",
    atype: req.atype,
    state: "SCHEDULED",
    priceCents: req.floorCents,
    participants: 0,
    closesAt: req.atype === "DUTCH" ? undefined : `+${req.durationDays ?? 2}d`,
  };
  db.auctions.unshift(a);
  return clone(a);
}
export function setAuctionState(id: string, state: AdminAuction["state"]): AdminAuction {
  const a = db.auctions.find((x) => x.id === id);
  if (!a) throw { message: "auction not found", code: "404" };
  a.state = state;
  return clone(a);
}

// ---- member vaults ----
export function listVault(): AdminVaultObject[] {
  return clone(db.memberVault);
}

// ---- escrow + disputes ----
export function listEscrow(): AdminEscrowRow[] {
  return clone(db.escrow);
}
export function holdRelease(id: string): AdminEscrowRow {
  const e = db.escrow.find((x) => x.id === id);
  if (!e) throw { message: "escrow not found", code: "404" };
  e.state = "DISPUTED";
  return clone(e);
}
export function ruleDispute(id: string, ruling: DisputeRuling): AdminEscrowRow {
  const e = db.escrow.find((x) => x.id === id);
  if (!e) throw { message: "escrow not found", code: "404" };
  e.state = ruling === "REFUND_BUYER" ? "REFUNDED" : "RELEASED";
  return clone(e);
}
